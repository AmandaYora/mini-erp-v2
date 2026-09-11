package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/domain"
	"mini-erp/internal/modules/purchasing/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.PurchaseOrderClient plus PO administration.
// Purchase orders never move stock — receipt does (goodsreceipt, L5).
type Service struct {
	repo     *infrastructure.Repository
	parties  partycontracts.PartyClient
	products productcontracts.ProductClient
	branches branchcontracts.BranchClient
	audit    auditcontracts.AuditClient
}

// NewService wires purchasing use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, parties: parties, products: products, branches: branches, audit: audit}
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
	return o, nil
}

// SetStatus moves confirmed→completed (goods receipt) or any open
// document→cancelled. Drafts complete only through confirmation first.
// Branch-scoped via mustOwnBranch.
func (s *Service) SetStatus(ctx context.Context, branchID, id int64, status string, actorID int64) error {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return err
	}
	switch status {
	case contracts.StatusCompleted:
		if o.Status != contracts.StatusConfirmed {
			return apperror.Conflict("Order harus dikonfirmasi dulu")
		}
	case contracts.StatusCancelled:
		if o.Status == contracts.StatusCompleted || o.Status == contracts.StatusCancelled {
			return apperror.Conflict("Order sudah final")
		}
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "status", Message: "status tidak dikenal"}})
	}
	if err := s.repo.SetStatus(ctx, id, status, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
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
	Items                 []LineInput
}

type pricedLine struct {
	item                  *contracts.Item
	net, base, tax, total int64
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
	seen := map[string]bool{}
	var out []pricedLine
	for i, line := range in.Items {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		p, err := s.products.GetByID(ctx, line.ProductID)
		if err != nil || p == nil || p.Status != "active" {
			fail(tag + "produk tidak ditemukan")
			continue
		}
		var variantName string
		found := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				variantName, found = v.Name, true
				break
			}
		}
		if !found {
			fail(tag + "varian tidak dikenal")
			continue
		}
		factor, baseUOM, err := s.products.ResolveUOM(ctx, line.ProductID, line.UOM)
		if err != nil {
			fail(tag + "satuan tidak dikenal")
			continue
		}
		_ = baseUOM
		_ = variantName
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
		key := strconv.FormatInt(line.ProductID, 10) + "/" + strconv.FormatInt(line.VariantID, 10) + "/" + strings.TrimSpace(line.UOM)
		if seen[key] {
			fail(tag + "baris ganda (produk/varian/satuan sama)")
			continue
		}
		seen[key] = true
		net, _, tax, total := domain.CalcLine(line.Qty, line.UnitPrice, line.DiscountPct, line.DiscountNominal, in.TaxType, in.TaxRate)
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
			},
			net: net, tax: tax, total: total,
		})
	}
	if len(fields) > 0 {
		return nil, fields
	}
	return out, nil
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return "10+"
}

func itoa64(n int64) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return "10+"
}

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
	if in.PaymentTerms != "net" && in.PaymentTerms != "cod" {
		fail("paymentTerms", "harus net atau cod")
	}
	if in.PaymentTerms == "net" && strings.TrimSpace(in.DueDate) == "" {
		fail("dueDate", "wajib diisi untuk termin net")
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

// resolveDate defaults empty order dates to today (WIB calendar day).
func resolveDate(raw string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
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
	id, err := s.repo.CreateOrder(ctx, &contracts.PurchaseOrder{
		Number: number, BranchID: branchID, PartyID: in.PartyID,
		OrderDate: resolveDate(in.OrderDate), DueDate: strings.TrimSpace(in.DueDate),
		PaymentTerms: in.PaymentTerms, TaxType: in.TaxType, TaxRate: in.TaxRate,
		Subtotal: subtotal, DiscountTotal: discount, TaxTotal: tax, GrandTotal: grand,
		Notes: in.Notes, Items: items,
		SupplierInvoiceNumber: invNo, SupplierInvoiceDate: invDate,
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
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa diubah")
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
	existing.PartyID = in.PartyID
	existing.OrderDate = resolveDate(in.OrderDate)
	existing.DueDate = strings.TrimSpace(in.DueDate)
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

// ConfirmOrder locks a draft into confirmed (supplier commitment).
func (s *Service) ConfirmOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	if err := s.repo.SetStatus(ctx, id, contracts.StatusConfirmed, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.confirm", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// CancelOrder voids a draft or confirmed PO (no stock ever moved here).
func (s *Service) CancelOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status == contracts.StatusCompleted || existing.Status == contracts.StatusCancelled {
		return nil, apperror.Conflict("Order sudah final")
	}
	if err := s.repo.SetStatus(ctx, id, contracts.StatusCancelled, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_orders.cancel", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
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

// List searches numbers with optional status filter (branch-scoped).
func (s *Service) List(ctx context.Context, branchID int64, search, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, search, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	orders, err := s.repo.List(ctx, branchID, search, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if orders == nil {
		orders = []*contracts.PurchaseOrder{}
	}
	return &ListResult{Orders: orders, Total: total}, nil
}
