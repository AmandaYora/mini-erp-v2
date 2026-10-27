package application

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Order from surat jalan (ADR-0022): the goods already left on one or more
// SJ Langsung of one customer; the admin now prices them. The order's stock
// lines are BUILT FROM THE NOTES (quantities locked), the client only sends
// prices, terms and optional non-stock lines. One orchestrated step (ADR-0016),
// idempotent through clientRef:
//
//  1. read the notes: confirmed, direct, unordered, same branch + customer;
//  2. price the lines (priceLines: member/catalog/manual, minimum price);
//  3. create the order (origin "delivery") and confirm it;
//  4. LinkDirect (delivery): all notes or none; a note taken by another order
//     meanwhile cancels the fresh order (it has no dependents yet);
//  5. cash terms with money still owed: pay now or switch to net;
//  6. complete the order (every stock line is delivered by construction).
//
// A repeat with the same clientRef resumes from the order's status instead of
// creating a second order.

// MaxFromDeliveries caps how many notes one order may bundle (K2).
const MaxFromDeliveries = 20

// FromDeliveriesLine prices one stock line of the notes. Qty is not accepted:
// the note decides it.
type FromDeliveriesLine struct {
	ProductID       int64
	VariantID       int64
	UOM             string
	UnitPrice       *int64
	DiscountPct     float64
	DiscountNominal int64
	DiscountReason  string
	Note            string
	Taxable         *bool
}

// FromDeliveriesInput is the payload of preview and create.
type FromDeliveriesInput struct {
	DeliveryIDs      []int64
	OrderDate        string
	PaymentTerms     string // net | cod (prepaid refused: the goods are gone)
	DueDate          string
	TaxType          string
	TaxRate          float64
	Notes            string
	AssignedUserID   int64
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	// Lines price the stock lines of the notes (matched by product/variant/uom;
	// a line the notes do not carry is refused). Missing = catalog/member price.
	Lines []FromDeliveriesLine
	// ExtraItems are service / non-stock lines added on the order (K6).
	ExtraItems []LineInput
	// Settlement is required for cod when the order is not paid in full:
	// pay now (payment.create) or switch to net (sales.approve).
	Settlement *FromDeliveriesSettlement
	// ClientRef is mandatory: it is what makes a retry resume (≤ 64 chars).
	ClientRef string
}

// Settlement modes of a cash order built from notes (same vocabulary as the
// delivery COD settlement).
const (
	SettlePayNow      = "pay_now"
	SettleSwitchToNet = "switch_to_net"
)

// FromDeliveriesSettlement mirrors the delivery settlement.
type FromDeliveriesSettlement struct {
	Mode           string
	Method         string
	PaymentDate    string
	Reference      string
	AmountTendered int64
	DueDate        string
	// CashAccountID is the company rekening of a transfer/check (ADR-0026).
	CashAccountID int64
}

// FromDeliveriesPreview is the order as it would be written: the customer and
// ship-to of the notes, the priced lines, the totals and soft warnings.
type FromDeliveriesPreview struct {
	PartyID         int64             `json:"partyId"`
	PartyName       string            `json:"partyName"`
	ShipToLabel     string            `json:"shipToLabel"`
	ShipToRecipient string            `json:"shipToRecipient"`
	ShipToPhone     string            `json:"shipToPhone"`
	ShipToAddress   string            `json:"shipToAddress"`
	DeliveryDate    string            `json:"deliveryDate"` // latest note date, YYYY-MM-DD
	Notes           []PreviewNote     `json:"deliveries"`
	Items           []*contracts.Item `json:"items"`
	Subtotal        int64             `json:"subtotal"`
	DiscountTotal   int64             `json:"discountTotal"`
	TaxTotal        int64             `json:"taxTotal"`
	GrandTotal      int64             `json:"grandTotal"`
	Warnings        []string          `json:"warnings"`
}

// PreviewNote is one source note of the preview.
type PreviewNote struct {
	ID           int64  `json:"id"`
	Number       string `json:"number"`
	DeliveryDate string `json:"deliveryDate"`
}

// fromDeliveriesPlan is the validated, priced order before it is written.
type fromDeliveriesPlan struct {
	notes    []*deliverycontracts.DeliveryNote
	input    OrderInput
	preview  *FromDeliveriesPreview
	noteIDs  []int64
	lastDate string
}

// PreviewFromDeliveries prices the order without writing (the same builder
// feeds CreateFromDeliveries, so the form shows what will be saved).
func (s *Service) PreviewFromDeliveries(ctx context.Context, branchID int64, in FromDeliveriesInput) (*FromDeliveriesPreview, error) {
	plan, err := s.planFromDeliveries(ctx, branchID, in)
	if err != nil {
		return nil, err
	}
	return plan.preview, nil
}

// planFromDeliveries validates the notes and the header, builds the lines
// from the notes and prices them.
func (s *Service) planFromDeliveries(ctx context.Context, branchID int64, in FromDeliveriesInput) (*fromDeliveriesPlan, error) {
	if s.dep.deliveries == nil {
		return nil, apperror.Internal(errDependentsNotWired)
	}
	ids := distinctIDs(in.DeliveryIDs)
	if len(ids) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryIds", Message: "wajib diisi"}})
	}
	if len(ids) > MaxFromDeliveries {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryIds", Message: "maksimal 20 surat jalan per order"}})
	}
	found, err := s.dep.deliveries.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	notes := make([]*deliverycontracts.DeliveryNote, 0, len(ids))
	for _, id := range ids {
		n := found[id]
		if n == nil || n.BranchID != branchID {
			return nil, apperror.NotFound("Surat jalan")
		}
		notes = append(notes, n)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	first := notes[0]
	var lastDate string
	for _, n := range notes {
		switch {
		case n.Origin != deliverycontracts.OriginDirect || n.DocumentKind != deliverycontracts.DocumentKindOrder:
			return nil, apperror.Conflict("Surat jalan " + n.Number + " bukan surat jalan langsung")
		case n.Status != deliverycontracts.StatusConfirmed:
			return nil, apperror.Conflict("Surat jalan " + n.Number + " belum dikonfirmasi (atau sudah dibatalkan)")
		case n.SalesOrderID != 0:
			return nil, apperror.Conflict("Surat jalan " + n.Number + " sudah diorder")
		case n.PartyID == 0:
			return nil, apperror.Conflict("Surat jalan " + n.Number + " tidak punya pelanggan")
		case n.PartyID != first.PartyID:
			return nil, apperror.Conflict("Surat jalan " + n.Number + " milik pelanggan lain; satu order untuk satu pelanggan")
		}
		if d := day(n.DeliveryDate); d > lastDate {
			lastDate = d
		}
	}
	// Stock lines: one per product/variant/uom, quantities summed over every
	// note (a note split across locations stays one order line).
	type agg struct {
		key              string
		product, variant int64
		uom              string
		qty              float64
		priced           *FromDeliveriesLine
	}
	var order []*agg
	byKey := map[string]*agg{}
	for _, n := range notes {
		for _, it := range n.Items {
			key := fromDeliveriesKey(it.ProductID, it.VariantID, it.UOM)
			a := byKey[key]
			if a == nil {
				a = &agg{key: key, product: it.ProductID, variant: it.VariantID, uom: strings.TrimSpace(it.UOM)}
				byKey[key] = a
				order = append(order, a)
			}
			a.qty += it.Qty
		}
	}
	if len(order) == 0 {
		return nil, apperror.Conflict("Surat jalan tidak memuat barang")
	}
	for i := range in.Lines {
		l := &in.Lines[i]
		a := byKey[fromDeliveriesKey(l.ProductID, l.VariantID, l.UOM)]
		if a == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines",
				Message: "baris " + strconv.Itoa(i+1) + ": produk/satuan tidak ada di surat jalan (barang stok hanya dari surat jalan)"}})
		}
		if a.priced != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "baris " + strconv.Itoa(i+1) + ": baris ganda"}})
		}
		a.priced = l
	}
	items := make([]LineInput, 0, len(order)+len(in.ExtraItems))
	for _, a := range order {
		li := LineInput{ProductID: a.product, VariantID: a.variant, UOM: a.uom, Qty: a.qty}
		if p := a.priced; p != nil {
			li.UnitPrice, li.DiscountPct, li.DiscountNominal = p.UnitPrice, p.DiscountPct, p.DiscountNominal
			li.DiscountReason, li.Note, li.Taxable = p.DiscountReason, p.Note, p.Taxable
		}
		items = append(items, li)
	}
	// Extra lines must be non-stock (K6): a stock product belongs on a note.
	if len(in.ExtraItems) > 0 {
		extraIDs := make([]int64, 0, len(in.ExtraItems))
		for _, e := range in.ExtraItems {
			extraIDs = append(extraIDs, e.ProductID)
		}
		loaded, err := s.products.GetByIDs(ctx, extraIDs)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for i, e := range in.ExtraItems {
			p := loaded[e.ProductID]
			if p == nil {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "extraItems", Message: "baris " + strconv.Itoa(i+1) + ": produk tidak ditemukan"}})
			}
			if p.Tracked && p.Type == "barang" {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "extraItems",
					Message: "baris " + strconv.Itoa(i+1) + ": " + p.Name + " adalah barang berstok; kirim lewat surat jalan, bukan ditambahkan di order"}})
			}
			items = append(items, e)
		}
	}
	terms := strings.TrimSpace(in.PaymentTerms)
	if terms == "" {
		terms = "cod"
	}
	if terms == "prepaid" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "paymentTerms", Message: "prabayar tidak berlaku: barang sudah terkirim; pilih tempo (net) atau tunai (cod)"}})
	}
	taxType := strings.TrimSpace(in.TaxType)
	if taxType == "" {
		taxType = "none"
	}
	orderDate := strings.TrimSpace(in.OrderDate)
	if orderDate == "" {
		orderDate = timeutil.TodayWIB()
	} else if _, err := timeutil.ParseDateInput(orderDate); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderDate", Message: "format tanggal harus YYYY-MM-DD"}})
	} else if timeutil.AfterTodayWIB(orderDate) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderDate", Message: "tidak boleh di masa depan"}})
	}
	if orderDate < lastDate {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderDate", Message: "tidak boleh sebelum tanggal surat jalan terakhir (" + lastDate + ")"}})
	}
	oi := OrderInput{
		PartyID: first.PartyID, Channel: contracts.ChannelRegular, OrderDate: orderDate,
		DueDate: strings.TrimSpace(in.DueDate), PaymentTerms: terms, TaxType: taxType, TaxRate: in.TaxRate,
		Notes: in.Notes, AssignedUserID: in.AssignedUserID,
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate,
		ShipToLabel: first.ShipToLabel, ShipToRecipient: first.ShipToRecipient,
		ShipToPhone: first.ShipToPhone, ShipToAddress: first.ShipToAddress,
		ClientRef: strings.TrimSpace(in.ClientRef), Items: items,
	}
	if fields, _ := s.validateHeader(ctx, oi); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	lines, memberCode, fields := s.priceLines(ctx, oi)
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	_ = memberCode
	subtotal, discount, tax, grand := totals(lines)
	preview := &FromDeliveriesPreview{
		PartyID: first.PartyID, PartyName: first.PartyName,
		ShipToLabel: first.ShipToLabel, ShipToRecipient: first.ShipToRecipient,
		ShipToPhone: first.ShipToPhone, ShipToAddress: first.ShipToAddress,
		DeliveryDate: lastDate, Notes: []PreviewNote{}, Items: []*contracts.Item{},
		Subtotal: subtotal, DiscountTotal: discount, TaxTotal: tax, GrandTotal: grand,
		Warnings: []string{},
	}
	for _, n := range notes {
		preview.Notes = append(preview.Notes, PreviewNote{ID: n.ID, Number: n.Number, DeliveryDate: day(n.DeliveryDate)})
	}
	for _, l := range lines {
		preview.Items = append(preview.Items, l.item)
	}
	// F-03: VAT is due at delivery; an invoice dated in another tax month is
	// allowed but worth a look.
	if d := strings.TrimSpace(in.TaxInvoiceDate); len(d) >= 7 && len(lastDate) >= 7 && d[:7] != lastDate[:7] && taxType != "none" {
		preview.Warnings = append(preview.Warnings, "Masa faktur pajak ("+d[:7]+") berbeda dari masa penyerahan barang ("+lastDate[:7]+"); PPN terutang saat penyerahan.")
	}
	if preview.PartyName == "" {
		if p, err := s.parties.GetByID(ctx, first.PartyID); err == nil && p != nil {
			preview.PartyName = p.Name
		}
	}
	return &fromDeliveriesPlan{notes: notes, input: oi, preview: preview, noteIDs: ids, lastDate: lastDate}, nil
}

func fromDeliveriesKey(productID, variantID int64, uom string) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10) + "/" + strings.TrimSpace(uom)
}

func distinctIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// FromDeliveriesResult is the outcome of CreateFromDeliveries.
type FromDeliveriesResult struct {
	Order    *contracts.SalesOrder
	Warnings []string
	Replayed bool
}

// CreateFromDeliveries runs the orchestration described in the file header.
// Serialized per clientRef so a double-click never builds two orders.
func (s *Service) CreateFromDeliveries(ctx context.Context, actorID, branchID int64, in FromDeliveriesInput, settleAllowed func(mode string) bool) (*FromDeliveriesResult, error) {
	in.ClientRef = strings.TrimSpace(in.ClientRef)
	if in.ClientRef == "" || len(in.ClientRef) > 64 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "wajib diisi (maksimal 64 karakter)"}})
	}
	if st := in.Settlement; st != nil {
		switch st.Mode {
		case SettlePayNow, SettleSwitchToNet:
			if settleAllowed != nil && !settleAllowed(st.Mode) {
				return nil, apperror.Forbidden()
			}
		default:
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.mode", Message: "harus pay_now atau switch_to_net"}})
		}
	}
	var res *FromDeliveriesResult
	err := doclock.Lock("sjl:"+in.ClientRef, func() error {
		out, err := s.createFromDeliveriesLocked(ctx, actorID, branchID, in)
		res = out
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) createFromDeliveriesLocked(ctx context.Context, actorID, branchID int64, in FromDeliveriesInput) (*FromDeliveriesResult, error) {
	// Resume: an order of this cart already exists.
	existing, err := s.repo.OrderByClientRef(ctx, branchID, in.ClientRef)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var warnings []string
	if existing == nil {
		plan, err := s.planFromDeliveries(ctx, branchID, in)
		if err != nil {
			return nil, err
		}
		warnings = plan.preview.Warnings
		if plan.input.PaymentTerms == "cod" && in.Settlement == nil && plan.preview.GrandTotal > 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement",
				Message: "order tunai: pilih pay_now (bayar sekarang) atau switch_to_net (jadikan tempo)"}})
		}
		if in.Settlement != nil && in.Settlement.Mode == SettleSwitchToNet {
			if fields := lightDueDateErrors("net", in.Settlement.DueDate); len(fields) > 0 {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.dueDate", Message: fields[0].Message}})
			}
		}
		plan.input.Origin = contracts.OriginDelivery
		created, err := s.createOrder(ctx, actorID, branchID, plan.input)
		if err != nil {
			if errors.Is(err, errDuplicateRef) {
				if prior, perr := s.repo.OrderByClientRef(ctx, branchID, in.ClientRef); perr == nil && prior != nil {
					existing = prior
				}
			}
			if existing == nil {
				return nil, err
			}
		} else {
			existing = created
			_ = s.audit.Log(ctx, auditcontracts.Entry{
				Action: "sales_orders.create_from_delivery", Entity: "sales_order", EntityID: created.ID, BranchID: branchID, ActorID: actorID,
				Note:     created.Number + " ← " + joinNumbers(plan.notes),
				Metadata: map[string]any{"deliveryIds": plan.noteIDs},
			})
		}
	}
	if existing.Origin != contracts.OriginDelivery {
		return nil, apperror.Conflict("Referensi klien sudah dipakai untuk order biasa")
	}
	o := existing
	switch o.Status {
	case contracts.StatusCancelled:
		return nil, apperror.Conflict("Order " + o.Number + " sudah dibatalkan; ulangi dengan referensi baru")
	case contracts.StatusCompleted:
		full, err := s.GetByID(ctx, o.ID)
		return &FromDeliveriesResult{Order: full, Warnings: warnings, Replayed: true}, err
	case contracts.StatusDraft:
		if _, err := s.ConfirmOrder(ctx, actorID, branchID, o.ID); err != nil {
			return nil, err
		}
	}
	ids := distinctIDs(in.DeliveryIDs)
	if err := s.dep.deliveries.LinkDirect(ctx, actorID, branchID, o.ID, ids); err != nil {
		// The notes went to another order (or were cancelled) between the
		// preview and now: the fresh order bills nothing, void it.
		var ae *apperror.AppError
		if errors.As(err, &ae) && ae.Code == apperror.CodeConflict {
			if n, cerr := s.dep.deliveries.ActiveNoteCount(ctx, o.ID); cerr == nil && n == 0 {
				_, _ = s.CancelOrderWithReason(ctx, actorID, branchID, o.ID, "Surat jalan sudah diorder/dibatalkan sebelum order ini selesai")
			}
		}
		return nil, err
	}
	full, err := s.GetByID(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	if err := s.settleFromDeliveries(ctx, actorID, branchID, full, in.Settlement); err != nil {
		return nil, err
	}
	if err := s.SetStatus(ctx, branchID, o.ID, contracts.StatusCompleted, actorID); err != nil {
		return nil, err
	}
	full, err = s.GetByID(ctx, o.ID)
	return &FromDeliveriesResult{Order: full, Warnings: warnings}, err
}

// settleFromDeliveries applies the cash-terms rule of the last shipment to an
// order whose goods all shipped: money still owed on a cod order is paid now
// (idempotent per order) or the order goes on net terms. Net orders and fully
// paid orders need nothing.
func (s *Service) settleFromDeliveries(ctx context.Context, actorID, branchID int64, o *contracts.SalesOrder, st *FromDeliveriesSettlement) error {
	if o.PaymentTerms != "cod" {
		return nil
	}
	if s.dep.payments == nil {
		return apperror.Internal(errDependentsNotWired)
	}
	paid, err := s.dep.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, []int64{o.ID})
	if err != nil {
		return apperror.Internal(err)
	}
	if o.GrandTotal-paid[o.ID] <= 0 {
		return nil
	}
	if st == nil {
		return apperror.Validation("", []apperror.FieldError{{Field: "settlement",
			Message: "order tunai belum lunas: pilih pay_now (bayar sekarang) atau switch_to_net (jadikan tempo)"}})
	}
	switch st.Mode {
	case SettlePayNow:
		method := st.Method
		if method == "" {
			method = paymentcontracts.MethodCash
		}
		_, _, err := s.dep.payments.RecordForOrder(ctx, paymentcontracts.RecordInput{
			ActorID: actorID, BranchID: branchID, OrderType: paymentcontracts.OrderSales, OrderID: o.ID,
			Method: method, PaidAt: st.PaymentDate, Reference: st.Reference,
			AmountTendered: st.AmountTendered, CashAccountID: st.CashAccountID,
			ClientRef: "sjl:" + strconv.FormatInt(o.ID, 10),
			Notes:     "Pelunasan order dari surat jalan " + o.Number,
		})
		return err
	case SettleSwitchToNet:
		return s.SwitchToNet(ctx, branchID, o.ID, st.DueDate, actorID)
	}
	return apperror.Validation("", []apperror.FieldError{{Field: "settlement.mode", Message: "harus pay_now atau switch_to_net"}})
}

func joinNumbers(notes []*deliverycontracts.DeliveryNote) string {
	parts := make([]string, 0, len(notes))
	for _, n := range notes {
		parts = append(parts, n.Number)
	}
	return strings.Join(parts, ", ")
}
