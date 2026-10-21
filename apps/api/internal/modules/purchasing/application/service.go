package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.PurchaseOrderClient plus PO administration.
// Purchase orders never move stock — receipt does (goodsreceipt, L5).
type Service struct {
	// dep holds what hangs off an order; wired through SetDependents.
	dep      dependents
	repo     *infrastructure.Repository
	parties  partycontracts.PartyClient
	products productcontracts.ProductClient
	branches branchcontracts.BranchClient
	audit    auditcontracts.AuditClient
	// users names who created / is in charge of an order and validates the
	// person in charge (PO-01/PO-03).
	users usercontracts.UserClient
}

type dependents struct {
	receipts goodsreceiptcontracts.GoodsReceiptClient
	payments paymentcontracts.PaymentClient
	returns  purchasereturncontracts.PurchaseReturnClient
}

// SetDependents closes the late-bound edge to the modules that build on an
// order (receipts, payments, returns). Cancelling an order consults them;
// without them it refuses to cancel.
func (s *Service) SetDependents(receipts goodsreceiptcontracts.GoodsReceiptClient, payments paymentcontracts.PaymentClient, returns purchasereturncontracts.PurchaseReturnClient) {
	s.dep = dependents{receipts: receipts, payments: payments, returns: returns}
}

// errDependentsNotWired is a programmer error: bootstrap must call SetDependents.
var errDependentsNotWired = errors.New("purchasing dependents not wired")

// NewService wires purchasing use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
	users usercontracts.UserClient,
) *Service {
	return &Service{repo: repo, parties: parties, products: products, branches: branches, audit: audit, users: users}
}

// GetByID resolves a document with lines and party name, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.PurchaseOrder, error) {
	o, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, nil
	}
	return s.enrich(ctx, o)
}

func (s *Service) enrich(ctx context.Context, o *contracts.PurchaseOrder) (*contracts.PurchaseOrder, error) {
	items, err := s.repo.ItemsByOrder(ctx, o.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*contracts.Item{}
	}
	o.Items = items
	p, err := s.parties.GetByID(ctx, o.PartyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p != nil {
		o.PartyName = p.Name
	}
	s.decorateUsers(ctx, []*contracts.PurchaseOrder{o})
	return o, nil
}

// decorateUsers fills CreatedByName and AssignedUserName for a page in one
// grouped read. Names are display-only, so a failure leaves them empty.
func (s *Service) decorateUsers(ctx context.Context, orders []*contracts.PurchaseOrder) {
	if s.users == nil || len(orders) == 0 {
		return
	}
	ids := make([]int64, 0, 2*len(orders))
	for _, o := range orders {
		if o.CreatedBy > 0 {
			ids = append(ids, o.CreatedBy)
		}
		if o.AssignedUserID > 0 {
			ids = append(ids, o.AssignedUserID)
		}
	}
	if len(ids) == 0 {
		return
	}
	names, err := s.users.NamesByIDs(ctx, ids)
	if err != nil {
		return
	}
	for _, o := range orders {
		o.CreatedByName = names[o.CreatedBy]
		o.AssignedUserName = names[o.AssignedUserID]
	}
}

// SetStatus moves confirmed→completed (goods receipt) or any open
// document→cancelled. Drafts complete only through confirmation first.
// Branch-scoped via mustOwnBranch. The move is a CAS from the legal origin
// statuses; asking for the status the order already has is a successful
// no-op, so a repeated request after a partial failure is safe.
func (s *Service) SetStatus(ctx context.Context, branchID, id int64, status string, actorID int64) error {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return err
	}
	var from []string
	var refused string
	switch status {
	case contracts.StatusCompleted:
		from, refused = []string{contracts.StatusConfirmed}, "Order harus dikonfirmasi dulu"
	case contracts.StatusCancelled:
		from, refused = []string{contracts.StatusDraft, contracts.StatusConfirmed}, "Order sudah final"
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "status", Message: "status tidak dikenal"}})
	}
	if o.Status == status {
		return nil
	}
	won, err := s.repo.Transition(ctx, id, from, status, actorID)
	if err != nil {
		return apperror.Internal(err)
	}
	if won {
		return nil
	}
	cur, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if cur != nil && cur.Status == status {
		return nil
	}
	return apperror.Conflict(refused)
}

// SwitchToNet moves a confirmed cash-terms order (cod/prepaid) to net with a
// due date, as a CAS on the terms. An order already net with that due date
// replays successfully; any other state is a conflict.
func (s *Service) SwitchToNet(ctx context.Context, branchID, id int64, dueDate string, actorID int64) error {
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return err
	}
	due := strings.TrimSpace(dueDate)
	if due == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "dueDate", Message: "wajib diisi"}})
	}
	if _, err := timeutil.ParseDateInput(due); err != nil {
		return apperror.Validation("", []apperror.FieldError{{Field: "dueDate", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	won, err := s.repo.SwitchToNet(ctx, id, due, actorID)
	if err != nil {
		return apperror.Internal(err)
	}
	if won {
		return nil
	}
	cur, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if cur != nil && cur.Status == contracts.StatusConfirmed && cur.PaymentTerms == "net" && strings.HasPrefix(cur.DueDate, due) {
		return nil
	}
	return apperror.Conflict("Order tidak lagi berstatus terkonfirmasi dengan termin tunai")
}

// LineInput is one order line (client view).
type LineInput struct {
	ProductID       int64
	VariantID       int64
	UOM             string
	Qty             float64
	UnitPrice       int64
	DiscountPct     float64
	DiscountNominal int64
	DiscountReason  string
	Note            string
	// Taxable false exempts this line from the order's tax.
	Taxable *bool
	// TaxRate overrides the order's tax rate for this line (0..100, PO-03);
	// nil = the order's rate. Ignored when the order is untaxed.
	TaxRate *float64
}

// OrderInput is the create/update payload.
type OrderInput struct {
	PartyID               int64
	OrderDate             string
	DueDate               string
	PaymentTerms          string
	TaxType               string
	TaxRate               float64
	Notes                 string
	SupplierInvoiceNumber string
	SupplierInvoiceDate   string
	// TaxInvoiceNumber/Date: the supplier's tax invoice (faktur pajak, PO-03).
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	// AssignedUserID is the person in charge of the order (0 = none).
	AssignedUserID int64
	// PaymentTermDays is the optional credit period (0..3650).
	PaymentTermDays int
	Items           []LineInput
}

type pricedLine struct {
	item            *contracts.Item
	net, tax, total int64
}

// priceLines validates and prices every line: product active, variant known,
// UOM resolvable, quantities and money sane. Prices are entered (supplier
// pricing has no catalog authority) but never negative.
func (s *Service) priceLines(ctx context.Context, in OrderInput) ([]pricedLine, []apperror.FieldError) {
	var fields []apperror.FieldError
	fail := func(msg string) {
		fields = append(fields, apperror.FieldError{Field: "items", Message: msg})
	}
	if len(in.Items) == 0 {
		return nil, []apperror.FieldError{{Field: "items", Message: "wajib diisi"}}
	}
	// One batch read for every product on the document (F4-7).
	productIDs := make([]int64, 0, len(in.Items))
	for _, line := range in.Items {
		productIDs = append(productIDs, line.ProductID)
	}
	loaded, err := s.products.GetByIDs(ctx, productIDs)
	if err != nil {
		return nil, []apperror.FieldError{{Field: "items", Message: "gagal memuat produk"}}
	}
	seen := map[string]bool{}
	var out []pricedLine
	for i, line := range in.Items {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		p := loaded[line.ProductID]
		if p == nil || p.Status != "active" {
			fail(tag + "produk tidak ditemukan")
			continue
		}
		var variantName, variantCode string
		found := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				variantName, variantCode, found = v.Name, v.Code, true
				break
			}
		}
		if !found {
			fail(tag + "varian tidak dikenal")
			continue
		}
		factor, _, uomOK := p.UOMFactor(line.UOM)
		if !uomOK {
			fail(tag + "satuan tidak dikenal")
			continue
		}
		if line.Qty <= 0 {
			fail(tag + "qty harus lebih dari 0")
			continue
		}
		if line.UnitPrice < 0 {
			fail(tag + "harga tidak boleh negatif")
			continue
		}
		if line.DiscountPct < 0 || line.DiscountPct > 100 {
			fail(tag + "diskon harus 0–100")
			continue
		}
		if line.DiscountNominal < 0 {
			fail(tag + "diskon nominal tidak boleh negatif")
			continue
		}
		if len(strings.TrimSpace(line.DiscountReason)) > 255 || len(strings.TrimSpace(line.Note)) > 255 {
			fail(tag + "alasan diskon / catatan maksimal 255 karakter")
			continue
		}
		key := strconv.FormatInt(line.ProductID, 10) + "/" + strconv.FormatInt(line.VariantID, 10) + "/" + strings.TrimSpace(line.UOM)
		if seen[key] {
			fail(tag + "baris ganda (produk/varian/satuan sama)")
			continue
		}
		seen[key] = true
		taxable := line.Taxable == nil || *line.Taxable
		lineTaxType, lineTaxRate := in.TaxType, in.TaxRate
		if line.TaxRate != nil && in.TaxType != "none" {
			if *line.TaxRate < 0 || *line.TaxRate > 100 {
				fail(tag + "tarif pajak baris harus 0–100")
				continue
			}
			lineTaxRate = *line.TaxRate
		}
		if !taxable {
			lineTaxType, lineTaxRate = "none", 0
		}
		net, base, tax, total := money.LineAmounts(line.Qty, line.UnitPrice, line.DiscountPct, line.DiscountNominal, lineTaxType, lineTaxRate).Split()
		if total < 0 {
			fail(tag + "total baris negatif")
			continue
		}
		qtyBase := line.Qty * factor
		out = append(out, pricedLine{
			item: &contracts.Item{
				ProductID: line.ProductID, VariantID: line.VariantID,
				ProductCode: p.Code, ProductName: p.Name, UOM: strings.TrimSpace(line.UOM),
				UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
				UnitPrice: line.UnitPrice, DiscountPct: line.DiscountPct,
				DiscountNominal: line.DiscountNominal, LineTotal: total,
				StockTracked: p.Tracked && p.Type != "jasa", ItemType: p.Type,
				VariantName: variantName, VariantCode: variantCode, BaseUOM: p.BaseUOM,
				DiscountReason: strings.TrimSpace(line.DiscountReason), Note: strings.TrimSpace(line.Note),
				Taxable: taxable, TaxRate: lineTaxRate, TaxBase: base, TaxAmount: tax,
			},
			net: net, tax: tax, total: total,
		})
	}
	if len(fields) > 0 {
		return nil, fields
	}
	return out, nil
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// totals folds priced lines into header money.
// totals folds priced lines into header money: subtotal = Σnet,
// discount = Σ(gross−net), tax/grand summed from lines.
func totals(lines []pricedLine) (subtotal, discount, tax, grand int64) {
	for _, l := range lines {
		gross := int64(float64(l.item.UnitPrice)*l.item.Qty + 0.5)
		subtotal += l.net
		discount += gross - l.net
		tax += l.tax
		grand += l.total
	}
	return subtotal, discount, tax, grand
}

// validateHeader checks party, dates, terms, and tax. It returns the party
// for the caller (supplier type + active enforced here).
func (s *Service) validateHeader(ctx context.Context, in OrderInput) ([]apperror.FieldError, *partycontracts.Party) {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	p, err := s.parties.GetByID(ctx, in.PartyID)
	if err != nil || p == nil || p.Type != partycontracts.PartySupplier || p.Status != "active" {
		fail("partyId", "supplier tidak ditemukan")
		return fields, nil
	}
	if in.PaymentTerms != "net" && in.PaymentTerms != "cod" && in.PaymentTerms != "prepaid" {
		fail("paymentTerms", "harus net, cod, atau prepaid")
	}
	if in.PaymentTermDays < 0 || in.PaymentTermDays > 3650 {
		fail("paymentTermDays", "harus 0–3650")
	}
	if in.PaymentTerms == "net" && strings.TrimSpace(in.DueDate) == "" &&
		!(in.PaymentTermDays > 0 && strings.TrimSpace(in.SupplierInvoiceDate) != "") {
		fail("dueDate", "wajib diisi untuk termin net (atau isi hari termin dan tanggal faktur supplier)")
	}
	if in.TaxType != "none" && in.TaxType != "include" && in.TaxType != "exclude" {
		fail("taxType", "harus none, include, atau exclude")
	}
	if in.TaxRate < 0 || in.TaxRate > 100 {
		fail("taxRate", "harus 0–100")
	}
	if in.TaxType == "none" && in.TaxRate != 0 {
		fail("taxRate", "harus 0 bila tanpa pajak")
	}
	if strings.TrimSpace(in.OrderDate) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(in.OrderDate)); err != nil {
			fail("orderDate", "format tanggal harus YYYY-MM-DD")
		}
	}
	if strings.TrimSpace(in.DueDate) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(in.DueDate)); err != nil {
			fail("dueDate", "format tanggal harus YYYY-MM-DD")
		}
	}
	if len(fields) > 0 {
		return fields, nil
	}
	return nil, p
}

// CreateOrder validates, numbers, and inserts a draft PO.
func (s *Service) CreateOrder(ctx context.Context, actorID, branchID int64, in OrderInput) (*contracts.PurchaseOrder, error) {
	if fields, _ := s.validateHeader(ctx, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	lines, fields := s.priceLines(ctx, in)
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocPurchaseOrder)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	subtotal, discount, tax, grand := totals(lines)
	items := make([]*contracts.Item, 0, len(lines))
	for _, l := range lines {
		items = append(items, l.item)
	}
	invNo, invDate, err := resolveSupplierInvoice(in.SupplierInvoiceNumber, in.SupplierInvoiceDate)
	if err != nil {
		return nil, err
	}
	taxNo, taxDate, err := resolveTaxInvoice(in.TaxInvoiceNumber, in.TaxInvoiceDate)
	if err != nil {
		return nil, err
	}
	if err := s.checkAssignee(ctx, branchID, in.AssignedUserID); err != nil {
		return nil, err
	}
	id, err := s.repo.CreateOrder(ctx, &contracts.PurchaseOrder{
		Number: number, BranchID: branchID, PartyID: in.PartyID,
		OrderDate: timeutil.DateOrToday(in.OrderDate), DueDate: resolveDueDate(in.DueDate, invDate, in.PaymentTermDays),
		PaymentTerms: in.PaymentTerms, TaxType: in.TaxType, TaxRate: in.TaxRate,
		Subtotal: subtotal, DiscountTotal: discount, TaxTotal: tax, GrandTotal: grand,
		Notes: in.Notes, Items: items, PaymentTermDays: in.PaymentTermDays,
		SupplierInvoiceNumber: invNo, SupplierInvoiceDate: invDate,
		TaxInvoiceNumber: taxNo, TaxInvoiceDate: taxDate, AssignedUserID: in.AssignedUserID,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.create", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.GetByID(ctx, id)
}

// mustOwnBranch loads the document and denies cross-branch access as
// missing. The check runs BEFORE any mutation — never write-then-check.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.PurchaseOrder, error) {
	o, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil || o.BranchID != branchID {
		return nil, apperror.NotFound("Purchase order")
	}
	return o, nil
}

// UpdateOrder rewrites a draft PO (header + lines replace).
func (s *Service) UpdateOrder(ctx context.Context, actorID, branchID, id int64, in OrderInput) (*contracts.PurchaseOrder, error) {
	var out *contracts.PurchaseOrder
	err := doclock.Lock("po:"+itoa64(id), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			res, err := s.updateOrderLocked(ctx, actorID, branchID, id, in)
			out = res
			return err
		})
	})
	return out, err
}

// withPaymentLock runs fn under the payment module's lock for this order
// (paymentcontracts.LockKey), nested inside the caller's po:<id> lock, so the
// "no payment hangs off it" check of a cancel or repricing cannot race a
// payment that is about to insert.
func (s *Service) withPaymentLock(ctx context.Context, branchID, id int64, fn func() error) error {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return err
	}
	return doclock.Lock(paymentcontracts.LockKey(o.PartyID, paymentcontracts.OrderPurchase, o.ID), fn)
}

// resolveDueDate keeps an explicit due date; otherwise a credit period counts
// from the supplier invoice date (legacy resolveOrderDueDate).
func resolveDueDate(due, invoiceDate string, days int) string {
	if d := strings.TrimSpace(due); d != "" {
		return d
	}
	if days > 0 && strings.TrimSpace(invoiceDate) != "" {
		if t, err := timeutil.ParseDateInput(strings.TrimSpace(invoiceDate)); err == nil {
			return t.AddDate(0, 0, days).Format("2006-01-02")
		}
	}
	return ""
}

func poSignature(rows []string) string {
	sort.Strings(rows)
	return strings.Join(rows, "|")
}

// lightOnly: no economic field changes (party, terms, tax, lines).
func (s *Service) lightOnly(existing *contracts.PurchaseOrder, in OrderInput) bool {
	if in.PartyID != existing.PartyID || in.PaymentTerms != existing.PaymentTerms ||
		in.TaxType != existing.TaxType || in.TaxRate != existing.TaxRate {
		return false
	}
	have := make([]string, 0, len(existing.Items))
	for _, it := range existing.Items {
		have = append(have, fmt.Sprintf("%d:%d:%s:%g", it.ProductID, it.VariantID, it.UOM, it.Qty))
	}
	want := make([]string, 0, len(in.Items))
	for _, it := range in.Items {
		want = append(want, fmt.Sprintf("%d:%d:%s:%g", it.ProductID, it.VariantID, it.UOM, it.Qty))
	}
	return poSignature(have) == poSignature(want)
}

func (s *Service) updateOrderLocked(ctx context.Context, actorID, branchID, id int64, in OrderInput) (*contracts.PurchaseOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusDraft && existing.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Order yang sudah selesai atau dibatalkan tidak bisa diubah")
	}
	if existing.Status == contracts.StatusConfirmed {
		if derr := s.assertNoDependents(ctx, id); derr != nil {
			var ae *apperror.AppError
			if !errors.As(derr, &ae) || ae.Code != apperror.CodeConflict {
				return nil, derr
			}
			full, gerr := s.GetByID(ctx, id)
			if gerr != nil || full == nil {
				return nil, derr
			}
			if !s.lightOnly(full, in) {
				return nil, derr
			}
			invNo, invDate, ierr := resolveSupplierInvoice(in.SupplierInvoiceNumber, in.SupplierInvoiceDate)
			if ierr != nil {
				return nil, ierr
			}
			taxNo, taxDate, terr := resolveTaxInvoice(in.TaxInvoiceNumber, in.TaxInvoiceDate)
			if terr != nil {
				return nil, terr
			}
			if aerr := s.checkAssignee(ctx, branchID, in.AssignedUserID); aerr != nil {
				return nil, aerr
			}
			existing.DueDate = resolveDueDate(in.DueDate, invDate, in.PaymentTermDays)
			existing.Notes = in.Notes
			existing.SupplierInvoiceNumber, existing.SupplierInvoiceDate = invNo, invDate
			existing.TaxInvoiceNumber, existing.TaxInvoiceDate, existing.AssignedUserID = taxNo, taxDate, in.AssignedUserID
			existing.PaymentTermDays = in.PaymentTermDays
			if err := s.repo.UpdateHeaderLight(ctx, existing, actorID); err != nil {
				return nil, dberr.Map(err)
			}
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.update", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number + " (header)"})
			return s.GetByID(ctx, id)
		}
	}
	if fields, _ := s.validateHeader(ctx, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	lines, fields := s.priceLines(ctx, in)
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	subtotal, discount, tax, grand := totals(lines)
	invNo, invDate, err := resolveSupplierInvoice(in.SupplierInvoiceNumber, in.SupplierInvoiceDate)
	if err != nil {
		return nil, err
	}
	taxNo, taxDate, err := resolveTaxInvoice(in.TaxInvoiceNumber, in.TaxInvoiceDate)
	if err != nil {
		return nil, err
	}
	if err := s.checkAssignee(ctx, branchID, in.AssignedUserID); err != nil {
		return nil, err
	}
	existing.TaxInvoiceNumber, existing.TaxInvoiceDate, existing.AssignedUserID = taxNo, taxDate, in.AssignedUserID
	existing.PartyID = in.PartyID
	existing.OrderDate = timeutil.DateOrToday(in.OrderDate)
	existing.DueDate = resolveDueDate(in.DueDate, invDate, in.PaymentTermDays)
	existing.PaymentTermDays = in.PaymentTermDays
	existing.PaymentTerms = in.PaymentTerms
	existing.TaxType = in.TaxType
	existing.TaxRate = in.TaxRate
	existing.Subtotal, existing.DiscountTotal = subtotal, discount
	existing.TaxTotal, existing.GrandTotal = tax, grand
	existing.Notes = in.Notes
	existing.SupplierInvoiceNumber, existing.SupplierInvoiceDate = invNo, invDate
	existing.Items = nil
	for _, l := range lines {
		existing.Items = append(existing.Items, l.item)
	}
	if err := s.repo.UpdateOrder(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.update", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// resolveSupplierInvoice trims the supplier invoice pair and validates the
// date shape (C3). Empty date stays empty; it is metadata, not scheduling.
func resolveSupplierInvoice(number, date string) (string, string, error) {
	no := strings.TrimSpace(number)
	d := strings.TrimSpace(date)
	if d != "" {
		if _, err := timeutil.ParseDateInput(d); err != nil {
			return "", "", apperror.Validation("", []apperror.FieldError{{Field: "supplierInvoiceDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}
	return no, d, nil
}

// resolveTaxInvoice trims the supplier tax-invoice pair (faktur pajak) and
// validates the date shape and the number length (PO-03).
func resolveTaxInvoice(number, date string) (string, string, error) {
	no := strings.TrimSpace(number)
	d := strings.TrimSpace(date)
	if len(no) > 100 {
		return "", "", apperror.Validation("", []apperror.FieldError{{Field: "taxInvoiceNumber", Message: "maksimal 100 karakter"}})
	}
	if d != "" {
		if _, err := timeutil.ParseDateInput(d); err != nil {
			return "", "", apperror.Validation("", []apperror.FieldError{{Field: "taxInvoiceDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}
	return no, d, nil
}

// checkAssignee validates the person in charge: an active user with access to
// the order's branch. 0 means nobody is assigned (PO-03).
func (s *Service) checkAssignee(ctx context.Context, branchID, userID int64) error {
	if userID == 0 {
		return nil
	}
	fail := func(msg string) error {
		return apperror.Validation("", []apperror.FieldError{{Field: "assignedUserId", Message: msg}})
	}
	if s.users == nil {
		return apperror.Internal(errors.New("purchasing users not wired"))
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	if u == nil || u.Status != "active" {
		return fail("penanggung jawab tidak ditemukan atau nonaktif")
	}
	access, err := s.users.GetBranchAccess(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, a := range access {
		if a.BranchID == branchID {
			return nil
		}
	}
	return fail("penanggung jawab tidak punya akses ke cabang ini")
}

// ConfirmOrder locks a draft into confirmed (supplier commitment). A CAS from
// draft; confirming an order that is already confirmed is a replay.
func (s *Service) ConfirmOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status == contracts.StatusConfirmed {
		return s.GetByID(ctx, id)
	}
	if existing.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	won, err := s.repo.Transition(ctx, id, []string{contracts.StatusDraft}, contracts.StatusConfirmed, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !won {
		cur, err := s.repo.GetOrder(ctx, id)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if cur == nil || cur.Status != contracts.StatusConfirmed {
			return nil, apperror.Conflict("Status order berubah, muat ulang")
		}
		return s.GetByID(ctx, id)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.confirm", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// CancelOrder voids a draft or confirmed PO — only while nothing hangs off it.
// Serialized per order; the status is re-read inside the lock, the dependents
// are consulted (live receipts, active payments, live returns) and the move
// is a CAS. A repeat on a cancelled order is a successful no-op.
func (s *Service) CancelOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseOrder, error) {
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return nil, err
	}
	var number string
	var replayed bool
	err := doclock.Lock("po:"+itoa64(id), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			existing, err := s.mustOwnBranch(ctx, id, branchID)
			if err != nil {
				return err
			}
			number = existing.Number
			switch existing.Status {
			case contracts.StatusCancelled:
				replayed = true
				return nil
			case contracts.StatusCompleted:
				return apperror.Conflict("Order sudah final")
			}
			if err := s.assertNoDependents(ctx, id); err != nil {
				return err
			}
			won, err := s.repo.Transition(ctx, id, []string{contracts.StatusDraft, contracts.StatusConfirmed}, contracts.StatusCancelled, actorID)
			if err != nil {
				return apperror.Internal(err)
			}
			if !won {
				cur, err := s.repo.GetOrder(ctx, id)
				if err != nil {
					return apperror.Internal(err)
				}
				if cur == nil || cur.Status != contracts.StatusCancelled {
					return apperror.Conflict("Status order berubah, muat ulang")
				}
				replayed = true
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.cancel", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	}
	return s.GetByID(ctx, id)
}

// assertNoDependents refuses to cancel an order that already has live
// receipts, active payments or live returns, naming what blocks it.
func (s *Service) assertNoDependents(ctx context.Context, poID int64) error {
	if s.dep.receipts == nil || s.dep.payments == nil || s.dep.returns == nil {
		return apperror.Internal(errDependentsNotWired)
	}
	if n, err := s.dep.receipts.ActiveReceiptCount(ctx, poID); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Order sudah memiliki penerimaan barang (pending atau terposting); batalkan penerimaannya dulu")
	}
	if used, err := s.dep.payments.HasActiveAllocations(ctx, paymentcontracts.OrderPurchase, poID); err != nil {
		return apperror.Internal(err)
	} else if used {
		return apperror.Conflict("Order sudah memiliki pembayaran aktif; batalkan pembayarannya dulu")
	}
	if n, err := s.dep.returns.ActiveReturnCount(ctx, poID); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Order sudah memiliki retur (draf atau terkonfirmasi); batalkan returnya dulu")
	}
	return nil
}

// ListByParty lists a party's documents for balance computation.
func (s *Service) ListByParty(ctx context.Context, branchID, partyID int64) ([]*contracts.OrderSummary, error) {
	orders, err := s.repo.ListByParty(ctx, branchID, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if orders == nil {
		orders = []*contracts.OrderSummary{}
	}
	return orders, nil
}

// ListOrders lists lightweight rows for operational readers (assistant bot
// tools). Party names resolve best-effort in one batch read (A3).
func (s *Service) ListOrders(ctx context.Context, branchID int64, statuses []string, from, to string, limit int) ([]*contracts.OrderSummary, error) {
	rows, err := s.repo.ListSummaries(ctx, branchID, statuses, from, to, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		r.PartyName = names[r.PartyID]
	}
	return rows, nil
}

// GetByNumber resolves one document by exact number, branch-scoped
// (assistant order-status tool), or nil when missing.
func (s *Service) GetByNumber(ctx context.Context, branchID int64, number string) (*contracts.PurchaseOrder, error) {
	o, err := s.repo.GetOrderByNumber(ctx, branchID, strings.TrimSpace(number))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, nil
	}
	return s.enrich(ctx, o)
}

// ListResult is a paginated PO page.
type ListResult struct {
	Orders []*contracts.PurchaseOrder
	Total  int64
}

// ListFilter is the filter set of the purchase order list/export: search
// (number or supplier), status CSV, payment terms, party, order-date range.
type ListFilter = infrastructure.ListFilter

// prepareFilter validates the enum filters and resolves the free-text term to
// supplier ids through the party contract (no cross-module join). Terms
// shorter than two characters never search parties (decision A-15).
func (s *Service) prepareFilter(ctx context.Context, f *ListFilter) error {
	if err := listquery.ValidateEnum("paymentTerms", f.PaymentTerms, "cod", "net", "prepaid"); err != nil {
		return err
	}
	f.SearchPartyIDs = nil
	if strings.TrimSpace(f.Search) != "" {
		ids, err := s.parties.SearchIDs(ctx, partycontracts.PartySupplier, f.Search, listquery.MaxIn)
		if err != nil {
			return err
		}
		f.SearchPartyIDs = ids
	}
	return nil
}

// decorateParties fills PartyName for a page in one batch read.
func (s *Service) decorateParties(ctx context.Context, orders []*contracts.PurchaseOrder) error {
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, o := range orders {
		o.PartyName = names[o.PartyID]
	}
	return nil
}

// List filters the branch's purchase orders; the count uses the same where.
func (s *Service) List(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	if err := s.prepareFilter(ctx, &f); err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, branchID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	orders, err := s.repo.List(ctx, branchID, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if orders == nil {
		orders = []*contracts.PurchaseOrder{}
	}
	if err := s.decorateParties(ctx, orders); err != nil {
		return nil, err
	}
	s.decorateUsers(ctx, orders)
	return &ListResult{Orders: orders, Total: total}, nil
}
