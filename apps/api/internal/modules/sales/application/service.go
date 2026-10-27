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
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.SalesOrderClient plus SO administration.
// Confirmed orders hold no stock: inventory decreases at delivery
// (contract §7.12), so cancel needs no stock action either.
type Service struct {
	repo     *infrastructure.Repository
	parties  partycontracts.PartyClient
	products productcontracts.ProductClient
	pricing  partycontracts.PricingClient
	branches branchcontracts.BranchClient
	audit    auditcontracts.AuditClient
	users    usercontracts.UserClient
	// margin feeds the export summary (see MarginReader); nil = no margin line.
	margin MarginReader
	// dep holds what hangs off an order (shipments, payments, returns). They
	// are built after the orders, so main wires them through SetDependents.
	dep dependents
}

type dependents struct {
	deliveries deliverycontracts.DeliveryClient
	payments   paymentcontracts.PaymentClient
	returns    salesreturncontracts.SalesReturnClient
}

// SetDependents closes the late-bound edge to the modules that build on an
// order. Cancelling an order consults them; without them it refuses to cancel.
func (s *Service) SetDependents(deliveries deliverycontracts.DeliveryClient, payments paymentcontracts.PaymentClient, returns salesreturncontracts.SalesReturnClient) {
	s.dep = dependents{deliveries: deliveries, payments: payments, returns: returns}
}

// NewService wires sales use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	pricing partycontracts.PricingClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
	users usercontracts.UserClient,
) *Service {
	return &Service{repo: repo, parties: parties, products: products, pricing: pricing, branches: branches, audit: audit, users: users}
}

// GetByID resolves a document with lines and party name, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.SalesOrder, error) {
	o, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, nil
	}
	return s.enrich(ctx, o)
}

func (s *Service) enrich(ctx context.Context, o *contracts.SalesOrder) (*contracts.SalesOrder, error) {
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
		o.PartyName, o.PartyPhone, o.PartyAddress = p.Name, p.Phone, p.Address
	} else if o.PartyID == 0 {
		o.PartyName = "Tunai"
	}
	return o, nil
}

// SetStatus moves confirmed→completed (delivery) or any open
// document→cancelled. Branch-scoped via mustOwnBranch. The move is a CAS from
// the legal origin statuses; asking for the status the order already has is a
// successful no-op, so a repeated request after a partial failure is safe.
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

// mustOwnBranch loads the document and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.SalesOrder, error) {
	o, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil || o.BranchID != branchID {
		return nil, apperror.NotFound("Sales order")
	}
	return o, nil
}

// LineInput is one order line. Unit prices are NOT accepted: the server
// prices every line (member quote or catalog) so clients cannot tamper.
type LineInput struct {
	ProductID       int64
	VariantID       int64
	UOM             string
	Qty             float64
	DiscountPct     float64
	DiscountNominal int64
	// UnitPrice overrides the catalog price for this line (manual pricing,
	// BR-08). Member-rule prices still win, and a manual price may never fall
	// below the product's minimum selling price (BR-25).
	UnitPrice      *int64
	DiscountReason string
	Note           string
	// Taxable false exempts this line from the order's tax.
	Taxable *bool
}

// OrderInput is the create/update payload.
type OrderInput struct {
	PartyID         int64
	Channel         string
	OrderDate       string
	DueDate         string
	PaymentTerms    string
	TaxType         string
	TaxRate         float64
	Notes           string
	ShipToAddressID int64
	// Ad-hoc ship-to (SL-01): a one-off address typed on the order, snapshotted
	// without touching the customer's address book. Used only when
	// ShipToAddressID is 0; ShipToAddress (the text) is then required.
	ShipToLabel     string
	ShipToRecipient string
	ShipToPhone     string
	ShipToAddress   string
	// AssignedUserID is the person in charge of the order (SL-09); 0 = none.
	AssignedUserID   int64
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	// ClientRef makes creation idempotent per client cart (<= 64 chars).
	ClientRef string
	// Reason is the optional why of a correction (update of a confirmed
	// order); it rides the audit trail only.
	Reason string
	// HeaderOnly restricts an update of a confirmed order to the header-light
	// fields (refused when anything economic differs).
	HeaderOnly bool
	// Origin marks an order built from direct delivery notes (ADR-0022); empty
	// = standard. Set by CreateFromDeliveries only, never by the HTTP payload.
	Origin string
	Items  []LineInput
}

type pricedLine struct {
	item            *contracts.Item
	net, tax, total int64
}

// priceLines validates and server-prices every line. Member prices come from
// one batch quote; walk-ins and inapplicable lines use catalog selling price.
func (s *Service) priceLines(ctx context.Context, in OrderInput) ([]pricedLine, string, []apperror.FieldError) {
	var fields []apperror.FieldError
	fail := func(msg string) {
		fields = append(fields, apperror.FieldError{Field: "items", Message: msg})
	}
	if len(in.Items) == 0 {
		return nil, "", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}}
	}
	productIDs := make([]int64, 0, len(in.Items))
	seenProd := map[int64]bool{}
	for _, line := range in.Items {
		if !seenProd[line.ProductID] {
			seenProd[line.ProductID] = true
			productIDs = append(productIDs, line.ProductID)
		}
	}
	quote, err := s.pricing.Quote(ctx, in.PartyID, productIDs)
	if err != nil {
		msg := "gagal menghitung harga member"
		var ae *apperror.AppError
		if errors.As(err, &ae) && ae.Message != "" {
			msg = ae.Message
		}
		return nil, "", []apperror.FieldError{{Field: "items", Message: msg}}
	}
	quoted := map[int64]partycontracts.QuoteLine{}
	for _, l := range quote.Lines {
		quoted[l.ProductID] = l
	}
	// One batch read for every product on the document (F4-7): constant
	// statements per request, not ~4 per line.
	loaded, err := s.products.GetByIDs(ctx, productIDs)
	if err != nil {
		return nil, "", []apperror.FieldError{{Field: "items", Message: "gagal memuat produk"}}
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
		found := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				found = true
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
		// Price source: a member rule wins; else a manual price; else the
		// catalog. The minimum-price floor guards manual/catalog lines but never
		// member prices (those are rule-governed). Catalog, member and minimum
		// prices are per SALES unit: a line in another unit (e.g. a carton of
		// the purchase unit) is priced by the unit's base-qty ratio, so one
		// carton of 36 never sells at the price of one piece.
		perUnit := func(salesPrice int64) int64 { return money.PriceForUnit(salesPrice, factor, p.SalesFactor) }
		unit, source := perUnit(p.SellingPrice), contracts.PricingCatalog
		if q, ok := quoted[line.ProductID]; ok && q.Error != "" && q.Error != "produk tidak ditemukan" {
			// The member rule could not price this product (empty basis
			// price, invalid unit conversion...): surface it on the line.
			fail(tag + q.Error)
			continue
		}
		if q, ok := quoted[line.ProductID]; ok && q.Applied {
			unit, source = perUnit(q.MemberPrice), contracts.PricingMemberRule
		} else if line.UnitPrice != nil {
			if *line.UnitPrice < 0 {
				fail(tag + "harga tidak boleh negatif")
				continue
			}
			unit, source = *line.UnitPrice, contracts.PricingManual
		}
		taxable := line.Taxable == nil || *line.Taxable
		lineTaxType, lineTaxRate := in.TaxType, in.TaxRate
		if !taxable {
			lineTaxType, lineTaxRate = "none", 0
		}
		net, base, tax, total := money.LineAmounts(line.Qty, unit, line.DiscountPct, line.DiscountNominal, lineTaxType, lineTaxRate).Split()
		if total < 0 {
			fail(tag + "total baris negatif")
			continue
		}
		if source != contracts.PricingMemberRule && net < money.MinLineNet(p.MinSellingPrice, line.Qty*factor, p.SalesFactor) {
			fail(tag + "harga " + p.Name + " di bawah harga minimum Rp" + strconv.FormatInt(p.MinSellingPrice, 10) + " per " + p.SalesUOM)
			continue
		}
		if len(strings.TrimSpace(line.DiscountReason)) > 255 || len(strings.TrimSpace(line.Note)) > 255 {
			fail(tag + "alasan diskon / catatan maksimal 255 karakter")
			continue
		}
		var variantName, variantCode string
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				variantName, variantCode = v.Name, v.Code
			}
		}
		out = append(out, pricedLine{
			item: &contracts.Item{
				ProductID: line.ProductID, VariantID: line.VariantID,
				ProductCode: p.Code, ProductName: p.Name, UOM: strings.TrimSpace(line.UOM),
				UOMFactor: factor, Qty: line.Qty, QtyBase: line.Qty * factor,
				UnitPrice: unit, DiscountPct: line.DiscountPct,
				DiscountNominal: line.DiscountNominal, LineTotal: total,
				StockTracked: p.Tracked && p.Type != "jasa", ItemType: p.Type,
				VariantName: variantName, VariantCode: variantCode, BaseUOM: p.BaseUOM,
				PricingSource: source, BasePrice: perUnit(p.SellingPrice),
				DiscountReason: strings.TrimSpace(line.DiscountReason), Note: strings.TrimSpace(line.Note),
				Taxable: taxable, TaxBase: base, TaxAmount: tax,
			},
			net: net, tax: tax, total: total,
		})
	}
	if len(fields) > 0 {
		return nil, "", fields
	}
	return out, quote.MemberType, nil
}

// totals folds priced lines into header money.
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

func (s *Service) validateHeader(ctx context.Context, in OrderInput) ([]apperror.FieldError, *partycontracts.Party) {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	if in.PartyID == 0 {
		// Walk-in cash sale: no party link, no member pricing.
	} else if p, err := s.parties.GetByID(ctx, in.PartyID); err != nil || p == nil || p.Type != partycontracts.PartyCustomer || p.Status != "active" {
		fail("partyId", "customer tidak ditemukan")
		return fields, nil
	}
	if in.Channel != contracts.ChannelRegular && in.Channel != contracts.ChannelPOS {
		fail("channel", "harus regular atau pos")
	}
	fields = append(fields, s.assignedUserErrors(ctx, in.AssignedUserID)...)
	if in.PaymentTerms != "net" && in.PaymentTerms != "cod" && in.PaymentTerms != "prepaid" {
		fail("paymentTerms", "harus net, cod, atau prepaid")
	}
	if in.PaymentTerms == "net" {
		// Credit goes only to a known customer we can reach.
		if in.PartyID == 0 {
			fail("paymentTerms", "termin net tidak untuk penjualan tunai tanpa customer")
		} else if p, err := s.parties.GetByID(ctx, in.PartyID); err == nil && p != nil && strings.TrimSpace(p.Phone) == "" {
			fail("paymentTerms", "customer termin net wajib memiliki nomor telepon")
		}
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
	return nil, nil
}

// CreateOrder validates, server-prices, numbers, and inserts a draft SO.
func (s *Service) CreateOrder(ctx context.Context, actorID, branchID int64, in OrderInput) (*contracts.SalesOrder, error) {
	o, _, err := s.CreateOrderIdempotent(ctx, actorID, branchID, in)
	return o, err
}

// CreateOrderIdempotent is CreateOrder that also reports a replay: the same
// ClientRef with the same content returns the order created the first time; the
// same ClientRef with different content is a conflict.
func (s *Service) CreateOrderIdempotent(ctx context.Context, actorID, branchID int64, in OrderInput) (*contracts.SalesOrder, bool, error) {
	in.ClientRef = strings.TrimSpace(in.ClientRef)
	if len(in.ClientRef) > 64 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "maksimal 64 karakter"}})
	}
	if in.ClientRef != "" {
		prior, err := s.priorByClientRef(ctx, branchID, in)
		if err != nil || prior != nil {
			return prior, prior != nil, err
		}
	}
	o, err := s.createOrder(ctx, actorID, branchID, in)
	if err != nil && in.ClientRef != "" && errors.Is(err, errDuplicateRef) {
		// Lost the insert race to the same cart: serve the winner.
		if prior, perr := s.priorByClientRef(ctx, branchID, in); perr == nil && prior != nil {
			return prior, true, nil
		}
	}
	return o, false, err
}

// priorByClientRef loads the order of this client reference and checks that it
// carries the same content (party + positions + quantities).
func (s *Service) priorByClientRef(ctx context.Context, branchID int64, in OrderInput) (*contracts.SalesOrder, error) {
	o, err := s.repo.OrderByClientRef(ctx, branchID, in.ClientRef)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, nil
	}
	o, err = s.enrich(ctx, o)
	if err != nil {
		return nil, err
	}
	if o.PartyID != in.PartyID || orderSignature(o.Items) != inputSignature(in.Items) {
		return nil, apperror.Conflict("Referensi klien sudah dipakai untuk isi order yang berbeda")
	}
	return o, nil
}

func signature(rows []string) string {
	sort.Strings(rows)
	return strings.Join(rows, "|")
}

func orderSignature(items []*contracts.Item) string {
	rows := make([]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, fmt.Sprintf("%d:%d:%s:%g", it.ProductID, it.VariantID, it.UOM, it.Qty))
	}
	return signature(rows)
}

func inputSignature(items []LineInput) string {
	rows := make([]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, fmt.Sprintf("%d:%d:%s:%g", it.ProductID, it.VariantID, it.UOM, it.Qty))
	}
	return signature(rows)
}

var errDuplicateRef = errors.New("duplicate client ref")

func (s *Service) createOrder(ctx context.Context, actorID, branchID int64, in OrderInput) (*contracts.SalesOrder, error) {
	if in.Channel == "" {
		in.Channel = contracts.ChannelRegular
	}
	if fields, _ := s.validateHeader(ctx, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	lines, memberCode, fields := s.priceLines(ctx, in)
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocSalesOrder)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	subtotal, discount, tax, grand := totals(lines)
	items := make([]*contracts.Item, 0, len(lines))
	for _, l := range lines {
		items = append(items, l.item)
	}
	shipID, shipLabel, shipRecipient, shipPhone, shipAddr, taxNo, taxDate, err := s.resolveShipTo(ctx, in)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.CreateOrder(ctx, &contracts.SalesOrder{
		Number: number, BranchID: branchID, PartyID: in.PartyID, Channel: in.Channel, Origin: in.Origin,
		MemberCode: memberCode, OrderDate: timeutil.DateOrToday(in.OrderDate),
		DueDate: strings.TrimSpace(in.DueDate), PaymentTerms: in.PaymentTerms,
		TaxType: in.TaxType, TaxRate: in.TaxRate,
		Subtotal: subtotal, DiscountTotal: discount, TaxTotal: tax, GrandTotal: grand,
		Notes: in.Notes, Items: items,
		ShipToAddressID: shipID, ShipToLabel: shipLabel, ShipToRecipient: shipRecipient,
		ShipToPhone: shipPhone, ShipToAddress: shipAddr, AssignedUserID: in.AssignedUserID,
		TaxInvoiceNumber: taxNo, TaxInvoiceDate: taxDate, ClientRef: in.ClientRef,
	}, actorID)
	if err != nil {
		if in.ClientRef != "" && dberr.IsDuplicate(err) {
			return nil, errDuplicateRef
		}
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.create", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.GetByID(ctx, id)
}

// UpdateOrder rewrites a draft SO, or corrects a confirmed one (re-priced from
// live catalog/member rules). A confirmed order with shipments, payments or
// returns only takes the header-light fields (see lightOnly).
func (s *Service) UpdateOrder(ctx context.Context, actorID, branchID, id int64, in OrderInput) (*contracts.SalesOrder, error) {
	var out *contracts.SalesOrder
	err := doclock.Lock("so:"+strconv.FormatInt(id, 10), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			res, err := s.updateOrderLocked(ctx, actorID, branchID, id, in)
			out = res
			return err
		})
	})
	return out, err
}

// withPaymentLock runs fn under the payment module's lock for this order
// (paymentcontracts.LockKey), nested inside the caller's so:<id> lock. The
// "no payment hangs off it" check of a cancel or repricing then cannot race a
// payment that read the order a moment earlier and is about to insert.
func (s *Service) withPaymentLock(ctx context.Context, branchID, id int64, fn func() error) error {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return err
	}
	return doclock.Lock(paymentcontracts.LockKey(o.PartyID, paymentcontracts.OrderSales, o.ID), fn)
}

// lightOnly reports whether an update leaves every economic field untouched:
// party, channel, terms, tax and lines. Only then may it touch an order that
// already has shipments, payments or returns.
func (s *Service) lightOnly(existing *contracts.SalesOrder, in OrderInput) bool {
	if in.Channel != "" && in.Channel != existing.Channel {
		return false
	}
	if in.PartyID != existing.PartyID || in.PaymentTerms != existing.PaymentTerms ||
		in.TaxType != existing.TaxType || in.TaxRate != existing.TaxRate {
		return false
	}
	return orderSignature(existing.Items) == inputSignature(in.Items)
}

func (s *Service) updateOrderLocked(ctx context.Context, actorID, branchID, id int64, in OrderInput) (*contracts.SalesOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusDraft && existing.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Order yang sudah selesai atau dibatalkan tidak bisa diubah")
	}
	existing, err = s.enrich(ctx, existing)
	if err != nil {
		return nil, err
	}
	if fields := reasonErrors(in.Reason, false); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	// The trail carries before/after of what the correction changed (header
	// fields, totals, lines) plus the optional reason.
	before := orderSnapshot(existing)
	{
		// Economic changes need a clean order; header notes/ship-to/due date/tax
		// invoice stay editable while documents hang off it. HeaderOnly asks for
		// that light path explicitly (the "ubah info order" dialog), so a
		// dependent that vanished meanwhile never turns it into a re-price.
		// Drafts included: a new draft never takes money, but a migrated one
		// can (legacy allowed it), and repricing it below what was paid would
		// over-allocate it.
		depErr := s.assertNoDependents(ctx, id)
		if depErr != nil {
			var ae *apperror.AppError
			if !errors.As(depErr, &ae) || ae.Code != apperror.CodeConflict {
				return nil, depErr
			}
		}
		if depErr != nil || in.HeaderOnly {
			if !s.lightOnly(existing, in) {
				if depErr != nil {
					return nil, depErr
				}
				return nil, apperror.Conflict("Ubah info order hanya untuk jatuh tempo, catatan, alamat kirim, faktur pajak, dan penanggung jawab")
			}
			if fields := lightDueDateErrors(existing.PaymentTerms, in.DueDate); len(fields) > 0 {
				return nil, apperror.Validation("", fields)
			}
			shipID, shipLabel, shipRecipient, shipPhone, shipAddr, taxNo, taxDate, serr := s.resolveShipTo(ctx, in)
			if serr != nil {
				return nil, serr
			}
			existing.DueDate = strings.TrimSpace(in.DueDate)
			existing.Notes = in.Notes
			existing.ShipToAddressID = shipID
			existing.ShipToLabel, existing.ShipToRecipient = shipLabel, shipRecipient
			existing.ShipToPhone, existing.ShipToAddress = shipPhone, shipAddr
			existing.TaxInvoiceNumber, existing.TaxInvoiceDate = taxNo, taxDate
			if fields := s.assignedUserErrors(ctx, in.AssignedUserID); len(fields) > 0 {
				return nil, apperror.Validation("", fields)
			}
			existing.AssignedUserID = in.AssignedUserID
			if err := s.repo.UpdateHeaderLight(ctx, existing, actorID); err != nil {
				return nil, dberr.Map(err)
			}
			_ = s.audit.Log(ctx, withDiff(auditcontracts.Entry{
				Action: "sales_orders.update", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number + " (header)",
			}, before, orderSnapshot(existing), correctionMeta(EditHeader, in.Reason)))
			return s.GetByID(ctx, id)
		}
	}
	if in.Channel == "" {
		in.Channel = existing.Channel
	}
	if fields, _ := s.validateHeader(ctx, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	lines, memberCode, fields := s.priceLines(ctx, in)
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	subtotal, discount, tax, grand := totals(lines)
	shipID, shipLabel, shipRecipient, shipPhone, shipAddr, taxNo, taxDate, err := s.resolveShipTo(ctx, in)
	if err != nil {
		return nil, err
	}
	existing.PartyID = in.PartyID
	existing.Channel = in.Channel
	existing.MemberCode = memberCode
	existing.OrderDate = timeutil.DateOrToday(in.OrderDate)
	existing.DueDate = strings.TrimSpace(in.DueDate)
	existing.PaymentTerms = in.PaymentTerms
	existing.TaxType = in.TaxType
	existing.TaxRate = in.TaxRate
	existing.Subtotal, existing.DiscountTotal = subtotal, discount
	existing.TaxTotal, existing.GrandTotal = tax, grand
	existing.Notes = in.Notes
	existing.ShipToAddressID = shipID
	existing.ShipToLabel, existing.ShipToRecipient = shipLabel, shipRecipient
	existing.ShipToPhone, existing.ShipToAddress = shipPhone, shipAddr
	existing.TaxInvoiceNumber, existing.TaxInvoiceDate = taxNo, taxDate
	existing.AssignedUserID = in.AssignedUserID
	existing.Items = nil
	for _, l := range lines {
		existing.Items = append(existing.Items, l.item)
	}
	if err := s.repo.UpdateOrder(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, withDiff(auditcontracts.Entry{
		Action: "sales_orders.update", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number,
	}, before, orderSnapshot(existing), correctionMeta(EditFull, in.Reason)))
	return s.GetByID(ctx, id)
}

// lightDueDateErrors validates the due date of a header-light correction: a
// net order keeps one, and it must be a date.
func lightDueDateErrors(terms, due string) []apperror.FieldError {
	due = strings.TrimSpace(due)
	if due == "" {
		if terms == "net" {
			return []apperror.FieldError{{Field: "dueDate", Message: "wajib diisi untuk termin net"}}
		}
		return nil
	}
	if _, err := timeutil.ParseDateInput(due); err != nil {
		return []apperror.FieldError{{Field: "dueDate", Message: "format tanggal harus YYYY-MM-DD"}}
	}
	return nil
}

// resolveShipTo snapshots the customer address (C2: frozen at write time,
// never follows later master edits) and validates the tax invoice date
// (C3). A saved address (ShipToAddressID) wins; otherwise an ad-hoc address
// typed on the order (label/recipient/phone/text, SL-01) is snapshotted as-is
// with address id 0 — walk-in orders may use it too. Neither = no ship-to
// (walk-in / ambil sendiri).
func (s *Service) resolveShipTo(ctx context.Context, in OrderInput) (addrID int64, label, recipient, phone, address, taxNo, taxDate string, err error) {
	if in.ShipToAddressID != 0 {
		a, err := s.parties.GetAddress(ctx, in.PartyID, in.ShipToAddressID)
		if err != nil {
			return 0, "", "", "", "", "", "", apperror.Internal(err)
		}
		if a == nil {
			return 0, "", "", "", "", "", "", apperror.Validation("", []apperror.FieldError{{Field: "shipToAddressId", Message: "alamat kirim tidak ditemukan"}})
		}
		addrID, label, recipient, phone, address = a.ID, a.Label, a.Recipient, a.Phone, a.Text
	} else if fields := adHocShipToErrors(in); len(fields) > 0 {
		return 0, "", "", "", "", "", "", apperror.Validation("", fields)
	} else {
		label, recipient = strings.TrimSpace(in.ShipToLabel), strings.TrimSpace(in.ShipToRecipient)
		phone, address = strings.TrimSpace(in.ShipToPhone), strings.TrimSpace(in.ShipToAddress)
	}
	taxNo = strings.TrimSpace(in.TaxInvoiceNumber)
	if d := strings.TrimSpace(in.TaxInvoiceDate); d != "" {
		if _, err := timeutil.ParseDateInput(d); err != nil {
			return 0, "", "", "", "", "", "", apperror.Validation("", []apperror.FieldError{{Field: "taxInvoiceDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		taxDate = d
	}
	return addrID, label, recipient, phone, address, taxNo, taxDate, nil
}

// Column widths of the ship-to snapshot (sales_orders ship_to_*).
const (
	maxShipToLabel     = 120
	maxShipToRecipient = 150
	maxShipToPhone     = 30
	maxShipToAddress   = 1000
)

// adHocShipToErrors validates a one-off ship-to: any detail without the
// address text is refused (a recipient with nowhere to go is a typo).
func adHocShipToErrors(in OrderInput) []apperror.FieldError {
	label, recipient := strings.TrimSpace(in.ShipToLabel), strings.TrimSpace(in.ShipToRecipient)
	phone, address := strings.TrimSpace(in.ShipToPhone), strings.TrimSpace(in.ShipToAddress)
	if label == "" && recipient == "" && phone == "" && address == "" {
		return nil
	}
	var fields []apperror.FieldError
	fail := func(field, msg string) { fields = append(fields, apperror.FieldError{Field: field, Message: msg}) }
	if address == "" {
		fail("shipToAddress", "alamat kirim wajib diisi bila memakai alamat sekali pakai")
	}
	if len([]rune(address)) > maxShipToAddress {
		fail("shipToAddress", "maksimal 1000 karakter")
	}
	if len([]rune(label)) > maxShipToLabel {
		fail("shipToLabel", "maksimal 120 karakter")
	}
	if len([]rune(recipient)) > maxShipToRecipient {
		fail("shipToRecipient", "maksimal 150 karakter")
	}
	if len([]rune(phone)) > maxShipToPhone {
		fail("shipToPhone", "maksimal 30 karakter")
	}
	return fields
}

// ConfirmOrder locks a draft into confirmed (stock moves only at delivery).
// A CAS from draft; confirming an order that is already confirmed is a replay
// (the POS retries the same checkout) and succeeds without a second audit row.
func (s *Service) ConfirmOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesOrder, error) {
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
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.confirm", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// CancelOrder voids a draft or confirmed SO without a reason (internal
// callers); see CancelOrderWithReason.
func (s *Service) CancelOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesOrder, error) {
	return s.CancelOrderWithReason(ctx, actorID, branchID, id, "")
}

// assertNoDependents refuses to cancel an order that already has live
// shipments, active payments or live returns, naming what blocks it.
func (s *Service) assertNoDependents(ctx context.Context, soID int64) error {
	if s.dep.deliveries == nil || s.dep.payments == nil || s.dep.returns == nil {
		return apperror.Internal(errDependentsNotWired)
	}
	if n, err := s.dep.deliveries.ActiveNoteCount(ctx, soID); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Order sudah memiliki surat jalan (draf atau terkonfirmasi); batalkan surat jalannya dulu")
	}
	if used, err := s.dep.payments.HasActiveAllocations(ctx, paymentcontracts.OrderSales, soID); err != nil {
		return apperror.Internal(err)
	} else if used {
		return apperror.Conflict("Order sudah memiliki pembayaran aktif; batalkan pembayarannya dulu")
	}
	if n, err := s.dep.returns.ActiveReturnCount(ctx, soID); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Order sudah memiliki retur (draf atau terkonfirmasi); batalkan returnya dulu")
	}
	return nil
}

// errDependentsNotWired is a programmer error: bootstrap must call SetDependents.
var errDependentsNotWired = errors.New("sales dependents not wired")

// ApproveCredit switches a confirmed cash-terms order (cod OR prepaid) to net
// terms with a due date (legacy approve-credit: any non-net order). It shares
// the exact eligibility of SwitchToNet (SL-10 decision, PLAN §7): both accept
// cod and prepaid, so the endpoint and the delivery settlement can never
// disagree about which orders may go on credit. Who approved and when is
// stamped on the order by the switch itself.
func (s *Service) ApproveCredit(ctx context.Context, actorID, branchID, id int64, dueDate string) (*contracts.SalesOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Hanya order terkonfirmasi yang bisa disetujui kreditnya")
	}
	if existing.PaymentTerms != "cod" && existing.PaymentTerms != "prepaid" {
		return nil, apperror.Conflict("Order ini sudah bertermin tempo")
	}
	if err := s.SwitchToNet(ctx, branchID, id, dueDate, actorID); err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.approve_credit", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// SwitchToNet moves a confirmed cash-terms order (cod/prepaid) to net with a
// due date, as a CAS on the terms. An order that already is net with the same
// due date replays successfully; any other state is a conflict.
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

// ListResult is a paginated SO page.
type ListResult struct {
	Orders []*contracts.SalesOrder
	Total  int64
}

// ListFilter is the filter set of the order list/export (see
// infrastructure.ListFilter): search (number or customer), status CSV,
// channel, payment terms, party, order-date range.
type ListFilter = infrastructure.ListFilter

// prepareFilter validates what the handler cannot and resolves the free-text
// term to customer ids through the party contract (no cross-module join):
// a row matches by number OR by one of those customers. Terms shorter than two
// characters never search parties (decision A-15).
func (s *Service) prepareFilter(ctx context.Context, f *ListFilter) error {
	if err := listquery.ValidateEnum("paymentTerms", f.PaymentTerms, "cod", "net", "prepaid"); err != nil {
		return err
	}
	if err := listquery.ValidateEnum("channel", f.Channel, contracts.ChannelRegular, contracts.ChannelPOS); err != nil {
		return err
	}
	if err := listquery.ValidateEnum("origin", f.Origin, contracts.OriginStandard, contracts.OriginDelivery); err != nil {
		return err
	}
	if err := s.preparePaymentFilter(f); err != nil {
		return err
	}
	f.SearchPartyIDs = nil
	if strings.TrimSpace(f.Search) != "" {
		ids, err := s.parties.SearchIDs(ctx, partycontracts.PartyCustomer, f.Search, listquery.MaxIn)
		if err != nil {
			return err
		}
		f.SearchPartyIDs = ids
	}
	return nil
}

// decorateParties fills PartyName for a page in one batch read; walk-in reads
// "Tunai". Best-effort for deleted parties (name stays empty).
func (s *Service) decorateParties(ctx context.Context, orders []*contracts.SalesOrder) error {
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if o.PartyID == 0 {
			o.PartyName = "Tunai"
			continue
		}
		o.PartyName = names[o.PartyID]
	}
	return nil
}

// List filters the branch's orders (newest first unless a whitelisted sort is
// set); the count uses the identical where.
func (s *Service) List(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	if err := s.prepareFilter(ctx, &f); err != nil {
		return nil, err
	}
	if f.PaymentStatus != "" {
		return s.listByPaymentStatus(ctx, branchID, f, page, limit)
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
		orders = []*contracts.SalesOrder{}
	}
	if err := s.decorateParties(ctx, orders); err != nil {
		return nil, err
	}
	return &ListResult{Orders: orders, Total: total}, nil
}

// ListOrders lists lightweight rows for operational readers (assistant bot
// tools). Party names resolve best-effort in one batch read (A3); walk-in
// reads "Tunai".
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
		if r.PartyID == 0 {
			r.PartyName = "Tunai"
			continue
		}
		r.PartyName = names[r.PartyID]
	}
	return rows, nil
}

// GetByNumber resolves one document by exact number, branch-scoped
// (assistant order-status tool), or nil when missing.
func (s *Service) GetByNumber(ctx context.Context, branchID int64, number string) (*contracts.SalesOrder, error) {
	o, err := s.repo.GetOrderByNumber(ctx, branchID, strings.TrimSpace(number))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, nil
	}
	return s.enrich(ctx, o)
}
