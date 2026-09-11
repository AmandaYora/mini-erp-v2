package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/domain"
	"mini-erp/internal/modules/sales/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
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
}

// NewService wires sales use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	pricing partycontracts.PricingClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, parties: parties, products: products, pricing: pricing, branches: branches, audit: audit}
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
		o.PartyName = p.Name
	} else if o.PartyID == 0 {
		o.PartyName = "Tunai"
	}
	return o, nil
}

// SetStatus moves confirmed→completed (delivery) or any open
// document→cancelled. Branch-scoped via mustOwnBranch.
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
}

// OrderInput is the create/update payload.
type OrderInput struct {
	PartyID          int64
	Channel          string
	OrderDate        string
	DueDate          string
	PaymentTerms     string
	TaxType          string
	TaxRate          float64
	Notes            string
	ShipToAddressID  int64
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	Items            []LineInput
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
		return nil, "", []apperror.FieldError{{Field: "items", Message: "gagal menghitung harga member"}}
	}
	quoted := map[int64]partycontracts.QuoteLine{}
	for _, l := range quote.Lines {
		quoted[l.ProductID] = l
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
		factor, _, err := s.products.ResolveUOM(ctx, line.ProductID, line.UOM)
		if err != nil {
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
		unit := p.SellingPrice
		if q, ok := quoted[line.ProductID]; ok && q.Applied {
			unit = q.MemberPrice
		}
		net, _, tax, total := domain.CalcLine(line.Qty, unit, line.DiscountPct, line.DiscountNominal, in.TaxType, in.TaxRate)
		if total < 0 {
			fail(tag + "total baris negatif")
			continue
		}
		out = append(out, pricedLine{
			item: &contracts.Item{
				ProductID: line.ProductID, VariantID: line.VariantID,
				ProductCode: p.Code, ProductName: p.Name, UOM: strings.TrimSpace(line.UOM),
				UOMFactor: factor, Qty: line.Qty, QtyBase: line.Qty * factor,
				UnitPrice: unit, DiscountPct: line.DiscountPct,
				DiscountNominal: line.DiscountNominal, LineTotal: total,
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
	return nil, nil
}

func resolveDate(raw string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
}

// CreateOrder validates, server-prices, numbers, and inserts a draft SO.
func (s *Service) CreateOrder(ctx context.Context, actorID, branchID int64, in OrderInput) (*contracts.SalesOrder, error) {
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
		Number: number, BranchID: branchID, PartyID: in.PartyID, Channel: in.Channel,
		MemberCode: memberCode, OrderDate: resolveDate(in.OrderDate),
		DueDate: strings.TrimSpace(in.DueDate), PaymentTerms: in.PaymentTerms,
		TaxType: in.TaxType, TaxRate: in.TaxRate,
		Subtotal: subtotal, DiscountTotal: discount, TaxTotal: tax, GrandTotal: grand,
		Notes: in.Notes, Items: items,
		ShipToAddressID: shipID, ShipToLabel: shipLabel, ShipToRecipient: shipRecipient,
		ShipToPhone: shipPhone, ShipToAddress: shipAddr,
		TaxInvoiceNumber: taxNo, TaxInvoiceDate: taxDate,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.create", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.GetByID(ctx, id)
}

// UpdateOrder rewrites a draft SO (re-priced from live catalog/member rules).
func (s *Service) UpdateOrder(ctx context.Context, actorID, branchID, id int64, in OrderInput) (*contracts.SalesOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa diubah")
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
	existing.OrderDate = resolveDate(in.OrderDate)
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
	existing.Items = nil
	for _, l := range lines {
		existing.Items = append(existing.Items, l.item)
	}
	if err := s.repo.UpdateOrder(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.update", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// resolveShipTo snapshots the customer address (C2: frozen at write time,
// never follows later master edits) and validates the tax invoice date
// (C3). Zero address id means no ship-to (walk-in / ambil sendiri).
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

// ConfirmOrder locks a draft into confirmed (stock moves only at delivery).
func (s *Service) ConfirmOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesOrder, error) {
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
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.confirm", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// CancelOrder voids a draft or confirmed SO.
func (s *Service) CancelOrder(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesOrder, error) {
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
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.cancel", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
	return s.GetByID(ctx, id)
}

// ApproveCredit switches a confirmed COD order to net terms with a due date
// (legacy approve-credit: pay_now/switch_to_net settlement path).
func (s *Service) ApproveCredit(ctx context.Context, actorID, branchID, id int64, dueDate string) (*contracts.SalesOrder, error) {
	existing, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if existing.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Hanya order terkonfirmasi yang bisa disetujui kreditnya")
	}
	if existing.PaymentTerms != "cod" {
		return nil, apperror.Conflict("Order ini bukan tunai")
	}
	if strings.TrimSpace(dueDate) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "dueDate", Message: "wajib diisi"}})
	}
	if _, err := timeutil.ParseDateInput(strings.TrimSpace(dueDate)); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "dueDate", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if err := s.repo.SetTerms(ctx, id, "net", strings.TrimSpace(dueDate), actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_orders.approve_credit", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: existing.Number})
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

// ListResult is a paginated SO page.
type ListResult struct {
	Orders []*contracts.SalesOrder
	Total  int64
}

// List searches numbers with optional status/channel filters (branch-scoped).
func (s *Service) List(ctx context.Context, branchID int64, search, status, channel string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, search, status, channel)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	orders, err := s.repo.List(ctx, branchID, search, status, channel, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if orders == nil {
		orders = []*contracts.SalesOrder{}
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
