package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	mediacontracts "mini-erp/internal/modules/media/contracts"
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
}

// Sales is the sales surface the delivery module consumes: document reads
// plus the grouped open-shipments read for the work queue (J3). Implemented
// by the sales module; main wires the concrete service, tests wire fakes.
type Sales interface {
	salescontracts.SalesOrderClient
	salescontracts.SalesOpsClient
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
}

// DocInput carries the document-face fields captured at draft time.
type DocInput struct {
	DriverName         string
	VehiclePlate       string
	WarehouseStaffName string
	DropLocationNote   string
}

// ConfirmInput carries the receipt evidence captured at confirm time.
type ConfirmInput struct {
	RecipientName                   string
	RecipientSignatureStatus        string
	RecipientSignatureMissingReason string
}

// Create drafts an SJ against a confirmed SO. Drafts move no stock; over-SO
// lines are rejected against confirmed deliveries.
func (s *Service) Create(ctx context.Context, actorID, branchID, soID int64, deliveryDate, notes string, doc DocInput, lines []LineInput) (*contracts.DeliveryNote, error) {
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
	if strings.TrimSpace(deliveryDate) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(deliveryDate)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "format tanggal harus YYYY-MM-DD"}})
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
		delivered, err := s.repo.DeliveredQty(ctx, soID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		qtyBase := line.Qty * factor
		if delivered+qtyBase > orderedBase[key]+1e-9 {
			return nil, apperror.Conflict("Melebihi sisa SO (" + tag + "sisa " + strconv.FormatFloat(orderedBase[key]-delivered, 'f', -1, 64) + ")")
		}
		if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
			return nil, err
		}
		p, err := s.products.GetByID(ctx, line.ProductID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
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
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor,
			Qty: line.Qty, QtyBase: qtyBase,
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
		Notes:              notes, Items: items,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.create", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.GetByID(ctx, id)
}

// Confirm posts one MoveOut per line (atomic with the status flip) and
// completes the SO once every line is fully delivered.
//
// Serialized per SO (doclock): the SO is re-read inside the lock, so a
// second sequential confirm sees the completed status and stops before
// moving anything. Completion is decided before any write, from data already
// in hand plus confirmed-delivery sums.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64, in ConfirmInput) (*contracts.DeliveryNote, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if n.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	if err := validateRecipient(in); err != nil {
		return nil, err
	}
	var out *contracts.DeliveryNote
	err = doclock.Lock("so:"+strconv.FormatInt(n.SalesOrderID, 10), func() error {
		res, err := s.confirmLocked(ctx, actorID, branchID, n, in)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.confirm", Entity: "delivery", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID int64, n *contracts.DeliveryNote, in ConfirmInput) (*contracts.DeliveryNote, error) {
	so, err := s.sales.GetByID(ctx, n.SalesOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID || so.Status != salescontracts.StatusConfirmed {
		return nil, apperror.Conflict("Order sudah final atau dibatalkan")
	}
	full, err := s.enrich(ctx, n)
	if err != nil {
		return nil, err
	}
	// Decide completion before writing: per (product, variant) aggregate,
	// confirmed-delivered + this note >= ordered?
	orderedBase := map[string]float64{}
	for _, l := range so.Items {
		orderedBase[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	noteBase := map[string]float64{}
	for _, it := range full.Items {
		noteBase[lineKey(it.ProductID, it.VariantID)] += it.QtyBase
	}
	shouldComplete := true
	for key, ordered := range orderedBase {
		var pid, vid int64
		for _, it := range full.Items {
			if lineKey(it.ProductID, it.VariantID) == key {
				pid, vid = it.ProductID, it.VariantID
				break
			}
		}
		if pid == 0 {
			// Aggregate untouched by this note: prior confirmed deliveries
			// alone must already cover it.
			for _, l := range so.Items {
				if lineKey(l.ProductID, l.VariantID) == key {
					pid, vid = l.ProductID, l.VariantID
					break
				}
			}
		}
		delivered, err := s.repo.DeliveredQty(ctx, so.ID, pid, vid)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if delivered+noteBase[key]+1e-9 < ordered {
			shouldComplete = false
			break
		}
	}
	// Pre-check every line's availability first so the common short-stock
	// case fails before anything moves.
	for _, it := range full.Items {
		bal, err := s.stock.GetBalance(ctx, branchID, it.ProductID, it.VariantID, it.LocationID)
		if err != nil {
			return nil, err
		}
		if bal.Available() < it.QtyBase {
			return nil, apperror.Conflict("Stok tersedia tidak mencukupi")
		}
	}
	var moved []*contracts.Item
	for _, it := range full.Items {
		if err := s.stock.MoveOut(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
			it.QtyBase, "delivery", full.ID, actorID); err != nil {
			// Compensate moved lines back (saga rollback): balances end
			// correct, and the note stays draft for a clean retry.
			for _, done := range moved {
				_ = s.stock.MoveIn(ctx, branchID, done.ProductID, done.VariantID, done.LocationID,
					done.QtyBase, "delivery_rollback", full.ID, actorID)
			}
			return nil, err
		}
		moved = append(moved, it)
	}
	if err := s.repo.SetStatus(ctx, full.ID, contracts.StatusConfirmed, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.repo.SetDispatch(ctx, full.ID, &contracts.DeliveryNote{
		RecipientName:                   strings.TrimSpace(in.RecipientName),
		RecipientSignatureStatus:        strings.TrimSpace(in.RecipientSignatureStatus),
		RecipientSignatureMissingReason: strings.TrimSpace(in.RecipientSignatureMissingReason),
	}, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	if shouldComplete {
		if err := s.sales.SetStatus(ctx, branchID, so.ID, salescontracts.StatusCompleted, actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	return s.GetByID(ctx, full.ID)
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
// corrections flow through sales returns (L6).
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.DeliveryNote, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if n.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Surat jalan terkonfirmasi tidak dapat dibatalkan (gunakan retur)")
	}
	if err := s.repo.SetStatus(ctx, id, contracts.StatusCancelled, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.cancel", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: n.Number})
	return s.GetByID(ctx, id)
}

// Proof records a delivery photo under the note (bukti kirim).
func (s *Service) Proof(ctx context.Context, actorID, branchID, id int64, up mediacontracts.Upload) (string, error) {
	n, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return "", err
	}
	f, err := s.media.Upload(ctx, mediacontracts.OwnerDeliveryProof, n.ID, up, actorID)
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
	files, err := s.media.List(ctx, mediacontracts.OwnerDeliveryProof, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		url, err := s.media.GetURL(ctx, f)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		out = append(out, map[string]any{
			"id": f.ID, "originalName": f.OriginalName, "mime": f.MIME,
			"sizeBytes": f.SizeBytes, "url": url,
		})
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
	deliveryDate := strings.TrimSpace(in.DeliveryDate)
	if deliveryDate != "" {
		if _, err := timeutil.ParseDateInput(deliveryDate); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "deliveryDate", Message: "format tanggal harus YYYY-MM-DD"}})
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
	// Whole-note availability first: a short line aborts the entire
	// dispatch, never a partial shipment (D4 failure path).
	for _, it := range full.Items {
		bal, err := s.stock.GetBalance(ctx, branchID, it.ProductID, it.VariantID, it.LocationID)
		if err != nil {
			return nil, err
		}
		if bal.Available() < it.QtyBase {
			return nil, apperror.Conflict("Stok tersedia tidak mencukupi")
		}
	}
	var moved []*contracts.Item
	for _, it := range full.Items {
		if err := s.stock.MoveOut(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
			it.QtyBase, "delivery", full.ID, actorID); err != nil {
			for _, done := range moved {
				_ = s.stock.MoveIn(ctx, branchID, done.ProductID, done.VariantID, done.LocationID,
					done.QtyBase, "delivery_rollback", full.ID, actorID)
			}
			return nil, err
		}
		moved = append(moved, it)
	}
	if err := s.repo.SetStatus(ctx, full.ID, contracts.StatusConfirmed, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.repo.SetDispatch(ctx, full.ID, &contracts.DeliveryNote{
		RecipientName:                   strings.TrimSpace(in.RecipientName),
		RecipientSignatureStatus:        strings.TrimSpace(in.RecipientSignatureStatus),
		RecipientSignatureMissingReason: strings.TrimSpace(in.MissingReason),
	}, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	out, err := s.GetByID(ctx, full.ID)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.confirm", Entity: "delivery", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
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
