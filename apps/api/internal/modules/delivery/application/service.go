package application

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.DeliveryClient plus SJ administration.
// Stock decreases at confirm (contract §7.12); drafts move nothing.
type Service struct {
	repo     *infrastructure.Repository
	sales    Sales
	products productcontracts.ProductClient
	stock    stockcontracts.StockClient
	branches branchcontracts.BranchClient
	media    mediacontracts.MediaClient
	audit    auditcontracts.AuditClient
	payments Payments
	settings Settings
}

// Settings reads the company's operational settings (late-bound).
type Settings interface {
	GetSettings(ctx context.Context) (map[string]string, error)
}

// SetSettings wires the company settings reader.
func (s *Service) SetSettings(st Settings) { s.settings = st }

// EvidenceSetting = "false" turns the shipment-evidence requirement off.
const EvidenceSetting = "delivery_require_evidence"

// requireEvidence checks the paperwork a regular shipment needs before it may
// be confirmed: driver and warehouse staff on the document, a named recipient
// with a signature status, and at least one delivery photo. POS handovers and
// replacement shipments are exempt (the goods leave at the counter / are tied
// to a return). A company may switch the rule off with EvidenceSetting=false.
func (s *Service) requireEvidence(ctx context.Context, so *salescontracts.SalesOrder, note *contracts.DeliveryNote, in ConfirmInput) error {
	if so.Channel == salescontracts.ChannelPOS || note.DocumentKind == contracts.DocumentKindReplacement {
		return nil
	}
	if s.settings != nil {
		if cfg, err := s.settings.GetSettings(ctx); err == nil && strings.EqualFold(strings.TrimSpace(cfg[EvidenceSetting]), "false") {
			return nil
		}
	}
	var fields []apperror.FieldError
	if strings.TrimSpace(note.DriverName) == "" {
		fields = append(fields, apperror.FieldError{Field: "driverName", Message: "nama sopir wajib diisi"})
	}
	if strings.TrimSpace(note.WarehouseStaffName) == "" {
		fields = append(fields, apperror.FieldError{Field: "warehouseStaffName", Message: "nama petugas gudang wajib diisi"})
	}
	if strings.TrimSpace(in.RecipientSignatureStatus) == "" || strings.TrimSpace(in.RecipientName) == "" {
		fields = append(fields, apperror.FieldError{Field: "recipientName", Message: "nama penerima dan status tanda tangan wajib diisi"})
	}
	if s.media != nil {
		// DL-01: the document that must come back is the signed physical SJ
		// (scan/photo); a location photo is optional and never satisfies it.
		if files, err := s.media.List(ctx, mediacontracts.OwnerDeliveryDocument, note.ID); err == nil && len(files) == 0 {
			fields = append(fields, apperror.FieldError{Field: "signedDocument", Message: "foto/scan surat jalan fisik bertanda tangan wajib diunggah"})
		}
	}
	if len(fields) > 0 {
		return apperror.Validation("Dokumen surat jalan belum lengkap", fields)
	}
	return nil
}

// Payments is the payment surface delivery consumes for order settlement:
// the paid-so-far figure and the idempotent "pay this order in full".
type Payments interface {
	RecordForOrder(ctx context.Context, in paymentcontracts.RecordInput) (*paymentcontracts.Payment, bool, error)
	PaidForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error)
}

// SetPayments wires the payment module (built after delivery).
func (s *Service) SetPayments(p Payments) { s.payments = p }

// Settlement modes for a cash-terms (COD) order that this shipment completes.
const (
	SettlePayNow      = "pay_now"
	SettleSwitchToNet = "switch_to_net"
)

// Settlement says how an unpaid COD order is settled when its last shipment
// goes out: paid on the spot, or converted to net terms with a due date.
type Settlement struct {
	// PosOnly marks a pay_now requested without payment.create: valid only for
	// a POS-channel order (the counter cashier takes the money).
	PosOnly        bool
	Mode           string
	Method         string
	PaymentDate    string
	Reference      string
	AmountTendered int64
	DueDate        string
}

// Sales is the sales surface the delivery module consumes: document reads
// plus the grouped open-shipments read for the work queue (J3). Implemented
// by the sales module; main wires the concrete service, tests wire fakes.
type Sales interface {
	salescontracts.SalesOrderClient
	salescontracts.SalesOpsClient
	salescontracts.OpenShipmentSearcher
}

// NewService wires delivery use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	sales Sales,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	media mediacontracts.MediaClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, sales: sales, products: products,
		stock: stock, branches: branches, media: media, audit: audit}
}

// GetByID resolves a note with lines and order number, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.DeliveryNote, error) {
	n, err := s.repo.GetNote(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n == nil {
		return nil, nil
	}
	return s.enrich(ctx, n)
}

// ListBySO lists an order's shipments without their lines (state readers).
func (s *Service) ListBySO(ctx context.Context, branchID, soID int64) ([]*contracts.DeliveryNote, error) {
	notes, err := s.repo.List(ctx, branchID, soID, "", 200, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return notes, nil
}

// ConfirmedQtyBefore implements the contract (see there).
func (s *Service) ConfirmedQtyBefore(ctx context.Context, soID, noteID int64) (map[string]float64, error) {
	out, err := s.repo.ConfirmedQtyBefore(ctx, soID, noteID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ActiveNoteCount counts the order's live (draft + confirmed) shipments.
func (s *Service) ActiveNoteCount(ctx context.Context, soID int64) (int64, error) {
	n, err := s.repo.ActiveNoteCount(ctx, soID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// TransportByOrders implements the contract (see there).
func (s *Service) TransportByOrders(ctx context.Context, orderIDs []int64) (map[int64]contracts.OrderTransport, error) {
	out, err := s.repo.TransportByOrders(ctx, orderIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// DeliveredQty sums confirmed-delivered base qty for one SO line position.
func (s *Service) DeliveredQty(ctx context.Context, soID, productID, variantID int64) (float64, error) {
	qty, err := s.repo.DeliveredQty(ctx, soID, productID, variantID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return qty, nil
}

func (s *Service) enrich(ctx context.Context, n *contracts.DeliveryNote) (*contracts.DeliveryNote, error) {
	items, err := s.repo.ItemsByDelivery(ctx, n.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*contracts.Item{}
	}
	n.Items = items
	so, err := s.sales.GetByID(ctx, n.SalesOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so != nil {
		n.OrderNumber = so.Number
	}
	return n, nil
}

// mustOwnBranch loads the note and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.DeliveryNote, error) {
	n, err := s.repo.GetNote(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n == nil || n.BranchID != branchID {
		return nil, apperror.NotFound("Surat jalan")
	}
	return n, nil
}

// LineInput is one SJ line.
type LineInput struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
	// ReservationKey is the stock hold (POS cart) this line consumes at confirm.
	ReservationKey string
}

// DocInput carries the document-face fields captured at draft time.
type DocInput struct {
	DriverName         string
	VehiclePlate       string
	WarehouseStaffName string
	DropLocationNote   string
	// ClientRef makes drafting idempotent per client cart: repeating the call
	// returns the note created the first time (HTTP 200, replayed).
	ClientRef string
}

// ConfirmInput carries the receipt evidence captured at confirm time.
type ConfirmInput struct {
	RecipientName                   string
	RecipientSignatureStatus        string
	RecipientSignatureMissingReason string
	// Settlement is required when this shipment completes an unpaid COD order.
	Settlement *Settlement
}

// Create drafts an SJ against a confirmed SO. Drafts move no stock but DO
// hold their share of the order's remaining quantity (P6): the quota is
// ordered - (draft + confirmed notes) - lines already accepted in this request.
//
// Serialized per SO (doclock) and the SO is re-read inside the lock, so two
// simultaneous drafts cannot both claim the same remainder.
func (s *Service) Create(ctx context.Context, actorID, branchID, soID int64, deliveryDate, notes string, doc DocInput, lines []LineInput) (*contracts.DeliveryNote, error) {
	out, _, err := s.CreateIdempotent(ctx, actorID, branchID, soID, deliveryDate, notes, doc, lines)
	return out, err
}

// CreateIdempotent is Create that also reports whether the call was a replay
// of an earlier one with the same ClientRef.
func (s *Service) CreateIdempotent(ctx context.Context, actorID, branchID, soID int64, deliveryDate, notes string, doc DocInput, lines []LineInput) (*contracts.DeliveryNote, bool, error) {
	doc.ClientRef = strings.TrimSpace(doc.ClientRef)
	if len(doc.ClientRef) > 64 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "maksimal 64 karakter"}})
	}
	var out *contracts.DeliveryNote
	var replayed bool
	err := doclock.Lock("so:"+strconv.FormatInt(soID, 10), func() error {
		if doc.ClientRef != "" {
			prior, err := s.byClientRef(ctx, branchID, soID, doc.ClientRef)
			if err != nil {
				return err
			}
			if prior != nil {
				out, replayed = prior, true
				return nil
			}
		}
		res, err := s.createLocked(ctx, actorID, branchID, soID, deliveryDate, notes, doc, lines)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return out, replayed, nil
}

// byClientRef returns the note already created under a client reference, nil
// when none; a reference reused for another order is a conflict.
func (s *Service) byClientRef(ctx context.Context, branchID, soID int64, ref string) (*contracts.DeliveryNote, error) {
	id, err := s.repo.NoteIDByClientRef(ctx, branchID, ref)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if id == 0 {
		return nil, nil
	}
	n, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n == nil {
		return nil, nil
	}
	if n.SalesOrderID != soID {
		return nil, apperror.Conflict("Referensi klien sudah dipakai untuk order lain")
	}
	return n, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, soID int64, deliveryDate, notes string, doc DocInput, lines []LineInput) (*contracts.DeliveryNote, error) {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return nil, apperror.NotFound("Sales order")
	}
	if so.Status != salescontracts.StatusConfirmed {
		return nil, apperror.Conflict("Order harus dikonfirmasi dulu")
	}
	if so.PaymentTerms == "prepaid" {
		if out, err := s.outstanding(ctx, so); err != nil {
			return nil, err
		} else if out > 0 {
			return nil, apperror.Conflict("Order prabayar belum lunas; surat jalan baru dapat dibuat setelah pembayaran")
		}
	}
	if strings.TrimSpace(deliveryDate) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(deliveryDate)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(deliveryDate) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "tidak boleh di masa depan"}})
		}
	} else {
		deliveryDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	if len(doc.DriverName) > 100 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "driverName", Message: "maksimal 100 karakter"}})
	}
	if len(doc.VehiclePlate) > 20 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "vehiclePlate", Message: "maksimal 20 karakter"}})
	}
	if len(doc.WarehouseStaffName) > 100 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "warehouseStaffName", Message: "maksimal 100 karakter"}})
	}
	committed, err := s.repo.CommittedQtyBySO(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	used := map[string]float64{} // base qty accepted so far in THIS request
	// Service / non-stock lines carry no location and never move stock.
	untracked := map[string]bool{}
	for _, l := range so.Items {
		if !l.StockTracked {
			untracked[lineKey(l.ProductID, l.VariantID)] = true
		}
	}
	soLines := map[string][]*salescontracts.Item{}
	for _, l := range so.Items {
		key := lineKey(l.ProductID, l.VariantID)
		soLines[key] = append(soLines[key], l)
	}
	// Aggregate base qty across same-product lines (a SO may carry one
	// product in two UOMs).
	orderedBase := map[string]float64{}
	uomFactor := map[string]map[string]float64{}
	for key, lines := range soLines {
		for _, l := range lines {
			orderedBase[key] += l.QtyBase
			if uomFactor[key] == nil {
				uomFactor[key] = map[string]float64{}
			}
			uomFactor[key][l.UOM] = l.UOMFactor
		}
	}
	items := make([]*contracts.Item, 0, len(lines))
	// One batch read for the whole document (F4-7), not a GetByID per line.
	productIDs := make([]int64, 0, len(lines))
	for _, l := range lines {
		productIDs = append(productIDs, l.ProductID)
	}
	loadedProducts, perr := s.products.GetByIDs(ctx, productIDs)
	if perr != nil {
		return nil, apperror.Internal(perr)
	}
	for i, line := range lines {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "qty harus lebih dari 0"}})
		}
		key := lineKey(line.ProductID, line.VariantID)
		factors, ok := uomFactor[key]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk tidak ada di SO"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, ok := factors[uom]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan harus sama dengan SO"}})
		}
		qtyBase := line.Qty * factor
		taken := committed[key] + used[key]
		if taken+qtyBase > orderedBase[key]+1e-9 {
			return nil, apperror.Conflict("Melebihi sisa SO (" + tag + "sisa " + strconv.FormatFloat(orderedBase[key]-taken, 'f', -1, 64) + ")")
		}
		used[key] += qtyBase
		// No location chosen on a stock line: allocate across the branch's
		// positions (oldest location first), one item per location.
		var allocated []stockcontracts.Suggestion
		if !untracked[key] && line.LocationID == 0 {
			sug, short, err := s.stock.Allocate(ctx, branchID, line.ProductID, line.VariantID, qtyBase)
			if err != nil {
				return nil, err
			}
			if short > 1e-9 {
				return nil, apperror.Conflict("Stok tidak mencukupi (" + tag + "kurang " + strconv.FormatFloat(short, 'f', -1, 64) + ")")
			}
			allocated = sug
		} else if !untracked[key] {
			if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
				return nil, err
			}
		}
		p := loadedProducts[line.ProductID]
		if p == nil || p.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk sudah diarsipkan"}})
		}
		known := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				known = true
				break
			}
		}
		if !known {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "varian sudah diarsipkan"}})
		}
		if len(allocated) > 0 {
			for _, a := range allocated {
				items = append(items, &contracts.Item{
					ProductID: line.ProductID, VariantID: line.VariantID, LocationID: a.LocationID,
					UOM: uom, UOMFactor: factor, Qty: a.Qty / factor, QtyBase: a.Qty,
				})
			}
			continue
		}
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor,
			Qty: line.Qty, QtyBase: qtyBase, ReservationKey: strings.TrimSpace(line.ReservationKey),
		})
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocDeliveryNote)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateNote(ctx, &contracts.DeliveryNote{
		Number: number, BranchID: branchID, SalesOrderID: soID,
		DeliveryDate:       strings.TrimSpace(deliveryDate),
		DriverName:         strings.TrimSpace(doc.DriverName),
		VehiclePlate:       strings.TrimSpace(doc.VehiclePlate),
		WarehouseStaffName: strings.TrimSpace(doc.WarehouseStaffName),
		DropLocationNote:   strings.TrimSpace(doc.DropLocationNote),
		Notes:              notes, Items: items, ClientRef: doc.ClientRef,
	}, actorID)
	if err != nil {
		if doc.ClientRef != "" && dberr.IsDuplicate(err) {
			if prior, perr := s.byClientRef(ctx, branchID, soID, doc.ClientRef); perr == nil && prior != nil {
				return prior, nil
			}
		}
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.create", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	out, err := s.GetByID(ctx, id)
	if err != nil || out == nil {
		return out, err
	}
	out.Warnings = s.draftWarnings(ctx, branchID, so, items)
	return out, nil
}

// draftWarnings is the soft availability check of a fresh draft (DL-04, ADR-0018
// D2): a draft moves and holds no stock, so it always carries the "stock may be
// used before you confirm" notice, plus one line per position whose available
// quantity is already below what the SJ needs. It never blocks and never fails:
// an unreadable balance simply yields no per-position line.
func (s *Service) draftWarnings(ctx context.Context, branchID int64, so *salescontracts.SalesOrder, items []*contracts.Item) []string {
	if so.Channel == salescontracts.ChannelPOS {
		return nil // the cart hold already reserves the goods; the counter ships at once
	}
	untracked := map[string]bool{}
	for _, l := range so.Items {
		if !l.StockTracked {
			untracked[lineKey(l.ProductID, l.VariantID)] = true
		}
	}
	type pos struct{ product, variant, location int64 }
	need := map[pos]float64{}
	var order []pos
	for _, it := range items {
		if untracked[lineKey(it.ProductID, it.VariantID)] || it.LocationID == 0 {
			continue
		}
		p := pos{it.ProductID, it.VariantID, it.LocationID}
		if _, ok := need[p]; !ok {
			order = append(order, p)
		}
		need[p] += it.QtyBase
	}
	if len(order) == 0 {
		return nil
	}
	names := map[string]string{}
	for _, l := range so.Items {
		names[lineKey(l.ProductID, l.VariantID)] = l.ProductName
	}
	warnings := []string{"Stok belum dikurangi atau ditahan oleh surat jalan draf; penjualan lain dapat memakainya sebelum Anda mengonfirmasi."}
	for _, p := range order {
		bal, err := s.stock.GetBalance(ctx, branchID, p.product, p.variant, p.location)
		if err != nil || bal == nil || bal.Available()+1e-9 >= need[p] {
			continue
		}
		name := names[lineKey(p.product, p.variant)]
		if name == "" {
			name = "produk " + strconv.FormatInt(p.product, 10)
		}
		warnings = append(warnings, "Stok "+name+" yang tersedia saat ini "+strconv.FormatFloat(bal.Available(), 'f', -1, 64)+
			", surat jalan butuh "+strconv.FormatFloat(need[p], 'f', -1, 64)+" (satuan dasar); konfirmasi akan gagal bila stok tidak bertambah.")
	}
	return warnings
}

// errReplacementManagedByReturn refuses the generic confirm/cancel on an
// exchange's replacement note. The sales return owns that note's lifecycle:
// confirming it here would skip the return's status and surcharge steps, and
// cancelling it would strand the return in "dispatched", where neither the
// return nor a new replacement can move it again.
func errReplacementManagedByReturn() error {
	return apperror.Conflict("Surat jalan pengganti dikelola dari halaman retur penjualan")
}

// Confirm ships the note: stock out, status flip, order completion — as a
// replay-safe sequence (P4). Every step is idempotent and the status flip is
// the LAST write that decides "done", so any failure in the middle is
// recovered by repeating the same request:
//
//  1. re-read the note inside the SO lock (a confirmed note replays step 5);
//  2. re-check the quota: Σ(draft+confirmed incl. this note) <= ordered;
//  3. stock.Post (idempotent per document; shortage rolls back whole note);
//  4. one CAS UPDATE draft -> confirmed (+ receipt evidence);
//  5. complete the SO when every position is fully delivered.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64, in ConfirmInput) (*contracts.DeliveryNote, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if n.DocumentKind == contracts.DocumentKindReplacement {
		return nil, errReplacementManagedByReturn()
	}
	if n.Status == contracts.StatusDraft {
		if err := validateRecipient(in); err != nil {
			return nil, err
		}
	}
	var out *contracts.DeliveryNote
	err = doclock.Lock("so:"+strconv.FormatInt(n.SalesOrderID, 10), func() error {
		res, _, err := s.confirmLocked(ctx, actorID, branchID, id, in)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID, id int64, in ConfirmInput) (*contracts.DeliveryNote, bool, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, false, err
	}
	switch n.Status {
	case contracts.StatusConfirmed:
		// Already final: only the post-final steps may be missing.
		if full, err := s.enrich(ctx, n); err == nil {
			s.releaseHolds(ctx, actorID, branchID, full)
		}
		if err := s.completeOrderIfDone(ctx, actorID, branchID, n.SalesOrderID); err != nil {
			return nil, false, err
		}
		out, err := s.GetByID(ctx, n.ID)
		return out, true, err
	case contracts.StatusDraft:
	default:
		return nil, false, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	so, err := s.sales.GetByID(ctx, n.SalesOrderID)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID || so.Status != salescontracts.StatusConfirmed {
		return nil, false, apperror.Conflict("Order sudah final atau dibatalkan")
	}
	full, err := s.enrich(ctx, n)
	if err != nil {
		return nil, false, err
	}
	if err := s.checkQuota(ctx, so, full); err != nil {
		return nil, false, err
	}
	if err := s.requireEvidence(ctx, so, full, in); err != nil {
		return nil, false, err
	}
	// Settlement is validated BEFORE stock moves: a refused settlement leaves
	// nothing to undo.
	settle, err := s.planSettlement(ctx, so, full, in.Settlement)
	if err != nil {
		return nil, false, err
	}
	// One atomic, idempotent posting for the whole note: a shortage rolls the
	// transaction back (note stays draft), and a retry after a failure further
	// down replays the already-written movements as a no-op (F2a-0).
	if err := s.postNote(ctx, branchID, actorID, full, so); err != nil {
		return nil, false, err
	}
	// Money effect: idempotent (client reference sj:<id> / CAS on the terms),
	// so repeating a half-finished confirm never pays or converts twice.
	if err := s.applySettlement(ctx, actorID, branchID, so, full, settle); err != nil {
		return nil, false, err
	}
	won, err := s.repo.Confirm(ctx, full.ID, &contracts.DeliveryNote{
		RecipientName:                   strings.TrimSpace(in.RecipientName),
		RecipientSignatureStatus:        strings.TrimSpace(in.RecipientSignatureStatus),
		RecipientSignatureMissingReason: strings.TrimSpace(in.RecipientSignatureMissingReason),
	}, actorID, s.confirmAudit(branchID, actorID, full))
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	replayed := false
	if !won {
		// Lost a race (or an earlier attempt already flipped it): re-read.
		cur, err := s.repo.GetNote(ctx, full.ID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if cur == nil || cur.Status != contracts.StatusConfirmed {
			return nil, false, apperror.Conflict("Status surat jalan berubah, muat ulang")
		}
		replayed = true
	}
	s.releaseHolds(ctx, actorID, branchID, full)
	if err := s.completeOrderIfDone(ctx, actorID, branchID, so.ID); err != nil {
		return nil, false, err
	}
	out, err := s.GetByID(ctx, full.ID)
	return out, replayed, err
}

// releaseHolds frees what is left of the stock holds the note consumed (a
// line shipped less than was held). Idempotent and best-effort: a failure here
// only delays the TTL sweep, it never undoes a shipment.
func (s *Service) releaseHolds(ctx context.Context, actorID, branchID int64, n *contracts.DeliveryNote) {
	seen := map[string]bool{}
	for _, it := range n.Items {
		if it.ReservationKey == "" || seen[it.ReservationKey] {
			continue
		}
		seen[it.ReservationKey] = true
		_ = s.stock.Release(ctx, actorID, branchID, it.ReservationKey)
	}
}

// checkQuota enforces Σ(draft + confirmed order notes, incl. this one) <=
// ordered for every position the note touches.
func (s *Service) checkQuota(ctx context.Context, so *salescontracts.SalesOrder, note *contracts.DeliveryNote) error {
	committed, err := s.repo.CommittedQtyBySO(ctx, so.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	ordered := map[string]float64{}
	for _, l := range so.Items {
		ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	for _, it := range note.Items {
		key := lineKey(it.ProductID, it.VariantID)
		if committed[key] > ordered[key]+1e-9 {
			return apperror.Conflict("Jumlah surat jalan melebihi sisa order, periksa surat jalan lain pada order ini")
		}
	}
	return nil
}

// completeOrderIfDone is the idempotent post-final step: when every ordered
// position is fully delivered by confirmed notes, the SO completes. Safe to
// repeat; it never touches an order that is no longer "confirmed".
func (s *Service) completeOrderIfDone(ctx context.Context, actorID, branchID, soID int64) error {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return apperror.Internal(err)
	}
	if so == nil || so.Status != salescontracts.StatusConfirmed {
		return nil
	}
	delivered, err := s.repo.DeliveredQtyBulk(ctx, []int64{soID})
	if err != nil {
		return apperror.Internal(err)
	}
	ordered := map[string]float64{}
	for _, l := range so.Items {
		ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	for _, l := range so.Items {
		got := delivered[infrastructure.BulkKey(soID, l.ProductID, l.VariantID)]
		if got+1e-9 < ordered[lineKey(l.ProductID, l.VariantID)] {
			return nil
		}
	}
	if err := s.sales.SetStatus(ctx, branchID, soID, salescontracts.StatusCompleted, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// postNote writes the note's stock-out as one document posting. Lines of
// service / non-stock products (per the order snapshot) are skipped; a note
// made only of those posts nothing.
func (s *Service) postNote(ctx context.Context, branchID, actorID int64, n *contracts.DeliveryNote, so *salescontracts.SalesOrder) error {
	untracked := map[string]bool{}
	if so != nil {
		for _, l := range so.Items {
			if !l.StockTracked {
				untracked[lineKey(l.ProductID, l.VariantID)] = true
			}
		}
	}
	lines := make([]stockcontracts.PostLine, 0, len(n.Items))
	for _, it := range n.Items {
		if untracked[lineKey(it.ProductID, it.VariantID)] {
			continue
		}
		lines = append(lines, stockcontracts.PostLine{ProductID: it.ProductID, VariantID: it.VariantID, LocationID: it.LocationID, Qty: it.QtyBase, ReservationKey: it.ReservationKey})
	}
	if len(lines) == 0 {
		return nil
	}
	_, err := s.stock.Post(ctx, stockcontracts.PostRequest{
		BranchID: branchID, Direction: "out", RefType: stockcontracts.RefDelivery, RefID: n.ID,
		ActorID: actorID, Lines: lines, MovedAt: movedAt(n.DeliveryDate),
	})
	return explainShortage(err, n.Number)
}

// explainShortage turns the stock module's terse shortage conflict into what the
// operator needs (DL-04): why it happened (a draft never held the stock), that
// nothing was shipped, and the ways forward. Other errors pass through.
func explainShortage(err error, number string) error {
	var ae *apperror.AppError
	if err == nil || !errors.As(err, &ae) || ae.Code != apperror.CodeConflict || !strings.Contains(ae.Message, "tidak mencukupi") {
		return err
	}
	return apperror.Conflict("Surat jalan " + number + " belum bisa dikonfirmasi: " + ae.Message +
		". Surat jalan draf tidak menahan stok, jadi stok bisa terpakai penjualan lain sejak draf dibuat. " +
		"Tidak ada stok yang keluar dan surat jalan tetap draf: tambah stok atau ganti lokasi lalu konfirmasi ulang, atau batalkan draf ini")
}

// movedAt turns the SJ's delivery date into the effective movement time (DL-03):
// a shipment dated today (or in the future) is stamped "now" so the stock card
// keeps its natural order; an earlier date is stamped at the start of that
// Jakarta day, the same rule goods receipts use.
func movedAt(deliveryDate string) time.Time {
	if len(deliveryDate) < 10 {
		return time.Time{}
	}
	d, err := time.ParseInLocation("2006-01-02", deliveryDate[:10], timeutil.Jakarta)
	if err != nil {
		return time.Time{}
	}
	if d.Format("2006-01-02") >= timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02") {
		return time.Time{}
	}
	return d
}

// validateRecipient guards the receipt evidence: a signed handover needs a
// name; a missing signature needs its reason instead of silence.
func validateRecipient(in ConfirmInput) *apperror.AppError {
	status := strings.TrimSpace(in.RecipientSignatureStatus)
	if status != "" && status != "signed" && status != "missing" {
		return apperror.Validation("", []apperror.FieldError{{Field: "recipientSignatureStatus", Message: "harus signed atau missing"}})
	}
	if len(in.RecipientName) > 150 {
		return apperror.Validation("", []apperror.FieldError{{Field: "recipientName", Message: "maksimal 150 karakter"}})
	}
	if status == "" {
		return nil
	}
	if strings.TrimSpace(in.RecipientName) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "recipientName", Message: "wajib diisi bila ada status tanda terima"}})
	}
	if status == "missing" && strings.TrimSpace(in.RecipientSignatureMissingReason) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "recipientSignatureMissingReason", Message: "wajib diisi bila tanda tangan tidak ada"}})
	}
	return nil
}

// Cancel voids a draft. Confirmed notes move stock and cannot be cancelled —
// corrections flow through sales returns (L6). A draft whose stock already
// went out (a Confirm that failed after the posting) is reversed first so the
// balances end where they began.
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.DeliveryNote, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if n.DocumentKind == contracts.DocumentKindReplacement {
		return nil, errReplacementManagedByReturn()
	}
	var replayed bool
	err = doclock.Lock("so:"+strconv.FormatInt(n.SalesOrderID, 10), func() error {
		cur, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		switch cur.Status {
		case contracts.StatusCancelled:
			replayed = true
			return nil
		case contracts.StatusDraft:
		default:
			return apperror.Conflict("Surat jalan terkonfirmasi tidak dapat dibatalkan (gunakan retur)")
		}
		moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefDelivery, cur.ID)
		if err != nil {
			return err
		}
		if moved {
			if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefDelivery, cur.ID, actorID); err != nil {
				return err
			}
		}
		won, err := s.repo.Transition(ctx, cur.ID, []string{contracts.StatusDraft}, contracts.StatusCancelled, actorID)
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			latest, err := s.repo.GetNote(ctx, cur.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if latest == nil || latest.Status != contracts.StatusCancelled {
				return apperror.Conflict("Status surat jalan berubah, muat ulang")
			}
			replayed = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.cancel", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: n.Number})
	}
	return s.GetByID(ctx, id)
}

// Proof purposes (legacy media purposes, kept verbatim on the wire).
const (
	PurposeSignedDocument = "delivery_signed_document"
	PurposeLocationPhoto  = "delivery_location_photo"
)

// proofOwner maps a purpose onto the media owner type that stores it. An empty
// purpose means the location photo (what the endpoint accepted before DL-01).
func proofOwner(purpose string) (string, error) {
	switch strings.TrimSpace(purpose) {
	case "", PurposeLocationPhoto:
		return mediacontracts.OwnerDeliveryProof, nil
	case PurposeSignedDocument:
		return mediacontracts.OwnerDeliveryDocument, nil
	}
	return "", apperror.Validation("", []apperror.FieldError{{Field: "purpose", Message: "harus delivery_signed_document atau delivery_location_photo"}})
}

// Proof records a delivery proof under the note: the signed SJ document or an
// optional location photo, by purpose.
func (s *Service) Proof(ctx context.Context, actorID, branchID, id int64, purpose string, up mediacontracts.Upload) (string, error) {
	owner, err := proofOwner(purpose)
	if err != nil {
		return "", err
	}
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return "", err
	}
	f, err := s.media.Upload(ctx, owner, n.ID, up, actorID)
	if err != nil {
		return "", err
	}
	url, err := s.media.GetURL(ctx, f)
	if err != nil {
		return "", err
	}
	return url, nil
}

// Proofs lists a note's delivery photos.
func (s *Service) Proofs(ctx context.Context, branchID, id int64) ([]map[string]any, error) {
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, kind := range []struct{ owner, purpose string }{
		{mediacontracts.OwnerDeliveryDocument, PurposeSignedDocument},
		{mediacontracts.OwnerDeliveryProof, PurposeLocationPhoto},
	} {
		files, err := s.media.List(ctx, kind.owner, id)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, f := range files {
			url, err := s.media.GetURL(ctx, f)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			out = append(out, map[string]any{
				"id": f.ID, "originalName": f.OriginalName, "mime": f.MIME,
				"sizeBytes": f.SizeBytes, "url": url, "purpose": kind.purpose,
			})
		}
	}
	return out, nil
}

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// checkStockLine validates one outbound line against live master data:
// active product, known variant, resolvable UOM, valid branch location.
func (s *Service) checkStockLine(ctx context.Context, branchID int64, productID, variantID, locationID int64, uom, tag string) (factor float64, err error) {
	p, err := s.products.GetByID(ctx, productID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if p == nil || p.Status != "active" {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk sudah diarsipkan"}})
	}
	known := false
	for _, v := range p.Variants {
		if v.ID == variantID {
			known = true
			break
		}
	}
	if !known {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "varian sudah diarsipkan"}})
	}
	factor, _, err = s.products.ResolveUOM(ctx, productID, strings.TrimSpace(uom))
	if err != nil {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan tidak dikenal"}})
	}
	if err := s.stock.CheckLocation(ctx, branchID, locationID); err != nil {
		return 0, err
	}
	return factor, nil
}

// CreateReplacement drafts a replacement shipment for an exchange return.
// Unlike Create it is not bound to SO qty or SO status: the order is
// already completed. Lines were validated at return creation; they are
// revalidated here because master data may have changed since.
func (s *Service) CreateReplacement(ctx context.Context, actorID, branchID int64, in contracts.ReplacementDraft) (*contracts.DeliveryNote, error) {
	if in.SalesReturnID == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "salesReturnId", Message: "wajib diisi"}})
	}
	if len(in.Items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	// RT-03: driver + warehouse staff are mandatory (unless evidence is off).
	if err := s.requireReplacementCrew(ctx, in); err != nil {
		return nil, err
	}
	deliveryDate := strings.TrimSpace(in.DeliveryDate)
	if deliveryDate != "" {
		if _, err := timeutil.ParseDateInput(deliveryDate); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(deliveryDate) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "tidak boleh di masa depan"}})
		}
	} else {
		deliveryDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	items := make([]*contracts.Item, 0, len(in.Items))
	for i, line := range in.Items {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "qty harus lebih dari 0"}})
		}
		factor, err := s.checkStockLine(ctx, branchID, line.ProductID, line.VariantID, line.LocationID, line.UOM, tag)
		if err != nil {
			return nil, err
		}
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: strings.TrimSpace(line.UOM), UOMFactor: factor,
			Qty: line.Qty, QtyBase: line.Qty * factor,
		})
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocDeliveryNote)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateNote(ctx, &contracts.DeliveryNote{
		Number: number, BranchID: branchID, SalesOrderID: in.SalesOrderID,
		DeliveryDate:       deliveryDate,
		DriverName:         strings.TrimSpace(in.DriverName),
		VehiclePlate:       strings.TrimSpace(in.VehiclePlate),
		WarehouseStaffName: strings.TrimSpace(in.WarehouseStaff),
		DropLocationNote:   strings.TrimSpace(in.DropNote),
		DocumentKind:       contracts.DocumentKindReplacement,
		SalesReturnID:      in.SalesReturnID,
		Notes:              in.Notes, Items: items,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.create", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.GetByID(ctx, id)
}

// ConfirmReplacement confirms a draft replacement shipment: stock out with
// the same saga rollback as order shipments, but no SO completion (the
// order is already completed). It takes NO lock itself: the caller
// (salesreturn ConfirmReplacementDelivery) serializes per return, and this
// method must stay callable under that lock (doclock is non-reentrant).
func (s *Service) ConfirmReplacement(ctx context.Context, actorID, branchID, id int64, in contracts.ReplacementConfirm) (*contracts.DeliveryNote, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if n.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	if n.DocumentKind != contracts.DocumentKindReplacement {
		return nil, apperror.Conflict("Bukan surat jalan pengganti")
	}
	if err := validateRecipient(ConfirmInput{
		RecipientName:                   in.RecipientName,
		RecipientSignatureStatus:        in.RecipientSignatureStatus,
		RecipientSignatureMissingReason: in.MissingReason,
	}); err != nil {
		return nil, err
	}
	full, err := s.enrich(ctx, n)
	if err != nil {
		return nil, err
	}
	// The surcharge is validated BEFORE stock moves: a refused payment leaves
	// nothing to undo (K-04e).
	if err := validateReplacementSettlement(full, in.Settlement); err != nil {
		return nil, err
	}
	if err := s.postNote(ctx, branchID, actorID, full, nil); err != nil {
		return nil, err
	}
	// Money effect rides the same saga as a COD pay_now: idempotent (client
	// reference sjr:<id>) and applied before the status flip, so a payment that
	// fails leaves a DRAFT note (stock reversible by Cancel, retry replays both
	// steps) and never a confirmed shipment with its surcharge unpaid.
	if err := s.applyReplacementSettlement(ctx, actorID, branchID, full, in.Settlement); err != nil {
		return nil, err
	}
	won, err := s.repo.Confirm(ctx, full.ID, &contracts.DeliveryNote{
		RecipientName:                   strings.TrimSpace(in.RecipientName),
		RecipientSignatureStatus:        strings.TrimSpace(in.RecipientSignatureStatus),
		RecipientSignatureMissingReason: strings.TrimSpace(in.MissingReason),
	}, actorID, s.confirmAudit(branchID, actorID, full))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !won {
		cur, err := s.repo.GetNote(ctx, full.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if cur == nil || cur.Status != contracts.StatusConfirmed {
			return nil, apperror.Conflict("Status surat jalan berubah, muat ulang")
		}
	}
	out, err := s.GetByID(ctx, full.ID)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// confirmAudit builds the trail hook that rides the confirm CAS transaction.
func (s *Service) confirmAudit(branchID, actorID int64, n *contracts.DeliveryNote) func(ctx context.Context, tx *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "deliveries.confirm", Entity: "delivery", EntityID: n.ID, BranchID: branchID, ActorID: actorID, Note: n.Number,
			Before: map[string]any{"status": contracts.StatusDraft}, After: map[string]any{"status": contracts.StatusConfirmed},
		})
	}
}

// ListResult is a paginated note page.
type ListResult struct {
	Notes []*contracts.DeliveryNote
	Total int64
}

// List searches notes of a branch, newest first.
func (s *Service) List(ctx context.Context, branchID, soID int64, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, soID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	notes, err := s.repo.List(ctx, branchID, soID, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if notes == nil {
		notes = []*contracts.DeliveryNote{}
	}
	return &ListResult{Notes: notes, Total: total}, nil
}

// ListConfirmed lists confirmed notes newest-first for the derived posting
// queue. One bounded read; the journal index decides what is still unposted.
func (s *Service) ListConfirmed(ctx context.Context, branchID int64, limit int) ([]*contracts.DeliveryNote, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	res, err := s.List(ctx, branchID, 0, "confirmed", 1, limit)
	if err != nil {
		return nil, err
	}
	return res.Notes, nil
}

// ListConfirmedAfter pages confirmed notes by ascending id after a cursor.
func (s *Service) ListConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.DeliveryNote, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	notes, err := s.repo.ConfirmedAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return notes, nil
}

// OrderIDsByNotes maps note id -> sales order id in one grouped read.
// Finance uses it to attribute journal entries to the order behind them.
func (s *Service) OrderIDsByNotes(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out, err := s.repo.OrderIDsByNotes(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// outstanding is what the order still owes: grand total minus active payments.
func (s *Service) outstanding(ctx context.Context, so *salescontracts.SalesOrder) (int64, error) {
	if s.payments == nil {
		return 0, apperror.Internal(errPaymentsNotWired)
	}
	paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, []int64{so.ID})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return so.GrandTotal - paid[so.ID], nil
}

// errPaymentsNotWired is a programmer error: bootstrap must call SetPayments.
var errPaymentsNotWired = errors.New("delivery payments not wired")

// completesOrder reports whether confirming this note delivers every ordered
// position in full (confirmed deliveries + this note >= ordered).
func (s *Service) completesOrder(ctx context.Context, so *salescontracts.SalesOrder, note *contracts.DeliveryNote) (bool, error) {
	delivered, err := s.repo.DeliveredQtyBulk(ctx, []int64{so.ID})
	if err != nil {
		return false, apperror.Internal(err)
	}
	ordered := map[string]float64{}
	for _, l := range so.Items {
		ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	inNote := map[string]float64{}
	for _, it := range note.Items {
		inNote[lineKey(it.ProductID, it.VariantID)] += it.QtyBase
	}
	for _, l := range so.Items {
		key := lineKey(l.ProductID, l.VariantID)
		if delivered[infrastructure.BulkKey(so.ID, l.ProductID, l.VariantID)]+inNote[key]+1e-9 < ordered[key] {
			return false, nil
		}
	}
	return true, nil
}

// planSettlement applies the payment-terms rules to this confirmation and
// returns the settlement to execute (nil = none needed):
//
//   - prepaid: must already be fully paid; a settlement makes no sense.
//   - net: no settlement.
//   - cod: only the shipment that COMPLETES the order settles it. If money is
//     still owed then, a settlement (pay_now | switch_to_net) is mandatory.
func (s *Service) planSettlement(ctx context.Context, so *salescontracts.SalesOrder, note *contracts.DeliveryNote, in *Settlement) (*Settlement, error) {
	if note.DocumentKind == contracts.DocumentKindReplacement {
		return nil, nil
	}
	switch so.PaymentTerms {
	case "prepaid":
		if in != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement", Message: "tidak berlaku untuk order prabayar"}})
		}
		out, err := s.outstanding(ctx, so)
		if err != nil {
			return nil, err
		}
		if out > 0 {
			return nil, apperror.Conflict("Order prabayar belum lunas; surat jalan tidak dapat dikonfirmasi")
		}
		return nil, nil
	case "cod":
		last, err := s.completesOrder(ctx, so, note)
		if err != nil || !last {
			return nil, err // not the final shipment: nothing to settle yet
		}
		out, err := s.outstanding(ctx, so)
		if err != nil {
			return nil, err
		}
		if out <= 0 {
			return nil, nil
		}
		if in == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement",
				Message: "order tunai belum lunas: pilih pay_now (bayar sekarang) atau switch_to_net (jadikan tempo)"}})
		}
		switch in.Mode {
		case SettlePayNow:
			if in.PosOnly && so.Channel != salescontracts.ChannelPOS {
				return nil, apperror.Forbidden()
			}
			method := in.Method
			if method == "" {
				method = paymentcontracts.MethodCash
			}
			c := *in
			c.Method = method
			return &c, nil
		case SettleSwitchToNet:
			if strings.TrimSpace(in.DueDate) == "" {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.dueDate", Message: "wajib diisi untuk switch_to_net"}})
			}
			if _, err := timeutil.ParseDateInput(strings.TrimSpace(in.DueDate)); err != nil {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.dueDate", Message: "format tanggal harus YYYY-MM-DD"}})
			}
			c := *in
			return &c, nil
		default:
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.mode", Message: "harus pay_now atau switch_to_net"}})
		}
	default: // net
		if in != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement", Message: "hanya untuk order tunai (COD)"}})
		}
		return nil, nil
	}
}

// applySettlement executes a planned settlement. Both effects are idempotent.
func (s *Service) applySettlement(ctx context.Context, actorID, branchID int64, so *salescontracts.SalesOrder, note *contracts.DeliveryNote, st *Settlement) error {
	if st == nil {
		return nil
	}
	switch st.Mode {
	case SettlePayNow:
		if s.payments == nil {
			return apperror.Internal(errPaymentsNotWired)
		}
		_, _, err := s.payments.RecordForOrder(ctx, paymentcontracts.RecordInput{
			ActorID: actorID, BranchID: branchID, OrderType: paymentcontracts.OrderSales, OrderID: so.ID,
			Method: st.Method, PaidAt: st.PaymentDate, Reference: st.Reference,
			AmountTendered: st.AmountTendered, ClientRef: "sj:" + strconv.FormatInt(note.ID, 10),
			Notes: "Pelunasan saat serah " + note.Number,
		})
		return err
	case SettleSwitchToNet:
		return s.sales.SwitchToNet(ctx, branchID, so.ID, st.DueDate, actorID)
	}
	return nil
}

// validateReplacementSettlement checks the shape of a pay_now surcharge; the
// charge rules (kind, cap) are the return owner's, checked before the call.
func validateReplacementSettlement(note *contracts.DeliveryNote, st *contracts.ReplacementSettlement) error {
	if st == nil {
		return nil
	}
	if note.SalesReturnID == 0 {
		return apperror.Conflict("Surat jalan ini tidak terkait retur tukar")
	}
	if st.Amount <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "settlement.amount", Message: "harus lebih dari 0"}})
	}
	switch st.Method {
	case "", paymentcontracts.MethodCash, paymentcontracts.MethodTransfer, paymentcontracts.MethodCheck:
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "settlement.method", Message: "harus cash, transfer, atau check"}})
	}
	if d := strings.TrimSpace(st.PaymentDate); d != "" {
		if _, err := timeutil.ParseDateInput(d); err != nil {
			return apperror.Validation("", []apperror.FieldError{{Field: "settlement.paymentDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(d) {
			return apperror.Validation("", []apperror.FieldError{{Field: "settlement.paymentDate", Message: "tidak boleh di masa depan"}})
		}
	}
	return nil
}

// applyReplacementSettlement records the surcharge payment against the sales
// return (charge kind). Idempotent per ClientRef sjr:<note id>.
func (s *Service) applyReplacementSettlement(ctx context.Context, actorID, branchID int64, note *contracts.DeliveryNote, st *contracts.ReplacementSettlement) error {
	if st == nil {
		return nil
	}
	if s.payments == nil {
		return apperror.Internal(errPaymentsNotWired)
	}
	method := st.Method
	if method == "" {
		method = paymentcontracts.MethodCash
	}
	_, _, err := s.payments.RecordForOrder(ctx, paymentcontracts.RecordInput{
		ActorID: actorID, BranchID: branchID, OrderType: paymentcontracts.OrderSalesReturn, OrderID: note.SalesReturnID,
		Amount: st.Amount, Method: method, PaidAt: st.PaymentDate, Reference: st.Reference,
		AmountTendered: st.AmountTendered, ClientRef: "sjr:" + strconv.FormatInt(note.ID, 10),
		Notes: "Tambah bayar tukar saat serah " + note.Number,
	})
	return err
}
