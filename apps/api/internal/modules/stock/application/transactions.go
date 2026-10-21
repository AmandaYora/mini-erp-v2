package application

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
)

// TransferItemInput is one transfer line (base UOM quantities).
type TransferItemInput struct {
	ProductID int64
	VariantID int64
	Qty       float64
	Notes     string
	// ToLocationID overrides the document destination for this line (0 = none).
	ToLocationID int64
}

// prepareTransfer validates a transfer request and builds the (unnumbered)
// document. Shared by the draft flow and the one-step location move.
func (s *Service) prepareTransfer(ctx context.Context, fromBranchID, toBranchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput) (*infrastructure.Transfer, error) {
	if len(items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	fromLoc, err := s.resolveLocation(ctx, fromBranchID, fromLocationID)
	if err != nil {
		return nil, err
	}
	toLoc, err := s.resolveLocation(ctx, toBranchID, toLocationID)
	if err != nil {
		return nil, err
	}
	if fromLoc.ID == toLoc.ID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "toLocationId", Message: "tujuan harus berbeda dari asal"}})
	}
	if _, _, err := s.ensureDefaults(ctx, fromBranchID); err != nil {
		return nil, err
	}
	docItems := make([]infrastructure.TransferItem, 0, len(items))
	productIDs := make([]int64, 0, len(items))
	for _, it := range items {
		productIDs = append(productIDs, it.ProductID)
	}
	products, err := s.loadProducts(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	for i, it := range items {
		if it.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "qty baris " + strconv.Itoa(i+1) + " harus lebih dari 0"}})
		}
		if _, _, err := pickLiveVariant(products[it.ProductID], it.VariantID); err != nil {
			return nil, err
		}
		if it.ToLocationID != 0 && it.ToLocationID != toLocationID {
			dest, err := s.resolveLocation(ctx, toBranchID, it.ToLocationID)
			if err != nil {
				return nil, err
			}
			if fromBranchID == toBranchID && dest.ID == fromLoc.ID {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "tujuan baris " + strconv.Itoa(i+1) + " harus berbeda dari asal"}})
			}
		}
		docItems = append(docItems, infrastructure.TransferItem{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty, Notes: it.Notes,
			ToLocation: it.ToLocationID,
		})
	}
	return &infrastructure.Transfer{
		FromBranchID: fromBranchID, ToBranchID: toBranchID,
		FromLocation: fromLocationID, ToLocation: toLocationID, Notes: notes, Items: docItems,
	}, nil
}

// CreateTransfer drafts a transfer document with an allocated number.
// Stock does not move until dispatch.
func (s *Service) CreateTransfer(ctx context.Context, actorID, fromBranchID, toBranchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput) (*infrastructure.Transfer, error) {
	return s.CreateTransferWith(ctx, actorID, fromBranchID, toBranchID, fromLocationID, toLocationID, notes, items, TransferMeta{})
}

// TransferMeta is the document face of a transfer (ST-02): who drives, in what,
// and the day it moves. Enforced reports whether the driver rule applies (the
// HTTP create path; internal callers keep the relaxed behavior).
type TransferMeta struct {
	DriverName   string
	VehiclePlate string
	// TransferDate is YYYY-MM-DD (WIB); empty = today.
	TransferDate string
	Enforced     bool
}

// validateMeta checks lengths and the date, and requires a driver on a
// transfer between branches (BR-20): goods leaving the premises need a named
// carrier on the paper.
func validateMeta(m *TransferMeta, interBranch bool) error {
	m.DriverName, m.VehiclePlate = strings.TrimSpace(m.DriverName), strings.TrimSpace(m.VehiclePlate)
	var fields []apperror.FieldError
	if len(m.DriverName) > 150 {
		fields = append(fields, apperror.FieldError{Field: "driverName", Message: "maksimal 150 karakter"})
	}
	if len(m.VehiclePlate) > 30 {
		fields = append(fields, apperror.FieldError{Field: "vehiclePlate", Message: "maksimal 30 karakter"})
	}
	if strings.TrimSpace(m.TransferDate) == "" {
		m.TransferDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	} else if _, err := timeutil.ParseDateInput(strings.TrimSpace(m.TransferDate)); err != nil {
		fields = append(fields, apperror.FieldError{Field: "transferDate", Message: "format tanggal harus YYYY-MM-DD"})
	}
	if m.Enforced && interBranch && m.DriverName == "" {
		fields = append(fields, apperror.FieldError{Field: "driverName", Message: "wajib diisi untuk transfer antar cabang"})
	}
	if len(fields) > 0 {
		return apperror.Validation("", fields)
	}
	return nil
}

// CreateTransferWith is CreateTransfer carrying the document face (ST-02).
func (s *Service) CreateTransferWith(ctx context.Context, actorID, fromBranchID, toBranchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput, meta TransferMeta) (*infrastructure.Transfer, error) {
	if err := validateMeta(&meta, fromBranchID != toBranchID); err != nil {
		return nil, err
	}
	doc, err := s.prepareTransfer(ctx, fromBranchID, toBranchID, fromLocationID, toLocationID, notes, items)
	if err != nil {
		return nil, err
	}
	doc.DriverName, doc.VehiclePlate, doc.TransferDate = meta.DriverName, meta.VehiclePlate, meta.TransferDate
	// Allocated last, right before the insert, so a validation failure never
	// burns a document number.
	number, err := s.branches.NextDocumentNumber(ctx, fromBranchID, branchcontracts.DocStockTransfer)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	doc.Number = number
	id, err := s.repo.CreateTransfer(ctx, doc, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	t, err := s.repo.GetTransfer(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.create", Entity: "stock_transfer", EntityID: t.ID, BranchID: fromBranchID, ActorID: actorID, Note: t.Number})
	return t, nil
}

// GetTransfer loads a document with lines. Branch-scoped: a document is
// visible only from its source or destination branch; anything else reads
// as missing (NotFound, never Forbidden — existence must not leak).
func (s *Service) GetTransfer(ctx context.Context, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.repo.GetTransfer(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if t == nil || (t.FromBranchID != branchID && t.ToBranchID != branchID) {
		return nil, apperror.NotFound("Transfer")
	}
	return t, nil
}

// transferLegs builds the stock legs of a transfer document.
func transferOutLeg(t *infrastructure.Transfer) ([]*postLine, error) {
	lines := make([]contracts.PostLine, 0, len(t.Items))
	for _, it := range t.Items {
		lines = append(lines, contracts.PostLine{ProductID: it.ProductID, VariantID: it.VariantID, LocationID: t.FromLocation, Qty: it.Qty})
	}
	return mergeLines("out", lines)
}

func transferInLeg(t *infrastructure.Transfer) ([]*postLine, error) {
	lines := make([]contracts.PostLine, 0, len(t.Items))
	for _, it := range t.Items {
		to := t.ToLocation
		if it.ToLocation != 0 {
			to = it.ToLocation
		}
		lines = append(lines, contracts.PostLine{ProductID: it.ProductID, VariantID: it.VariantID, LocationID: to, Qty: it.Qty})
	}
	return mergeLines("in", lines)
}

// transferStep runs one status transition of a transfer as a single retried
// transaction: the document row is locked, the origin status is checked inside
// the transaction, the movements are written (idempotent, sorted), and the
// status flips with a CAS. A repeat of a finished step is a no-op success
// (replayed=true) — two clicks, a retry, or a lost response cannot move stock
// twice.
func (s *Service) transferStep(ctx context.Context, id, actorID int64, from []string, to string, work func(tx *sql.Tx, t *infrastructure.Transfer) error, t *infrastructure.Transfer) (replayed bool, err error) {
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		replayed = false
		status, err := s.repo.LockTransferStatusTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if status == "" {
			return apperror.NotFound("Transfer")
		}
		if status == to {
			replayed = true
			return nil
		}
		allowed := false
		for _, f := range from {
			if f == status {
				allowed = true
			}
		}
		if !allowed {
			return apperror.Conflict("Transfer berstatus '" + status + "', aksi ini tidak dapat dilakukan")
		}
		if work != nil {
			if err := work(tx, t); err != nil {
				return err
			}
		}
		won, err := s.repo.TransitionTransferTx(ctx, tx, id, from, to, actorID)
		if err != nil {
			return err
		}
		if !won {
			return apperror.Conflict("Status transfer berubah, muat ulang")
		}
		return nil
	})
	return replayed, asAppErrorOrNil(err)
}

func asAppErrorOrNil(err error) error {
	if err == nil {
		return nil
	}
	return asAppError(err)
}

// DispatchTransfer deducts source stock (out-movements) and marks the
// document dispatched, atomically. Short lines abort the whole dispatch —
// partial dispatch does not exist. Only the source branch may dispatch.
func (s *Service) DispatchTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.FromBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	if t.Status == "draft" {
		out, err := transferOutLeg(t)
		if err != nil {
			return nil, err
		}
		if err := s.validateLegs(ctx, []leg{{branchID: t.FromBranchID, direction: "out", lines: out}}, false); err != nil {
			return nil, err
		}
	}
	ref := legRef{refType: contracts.RefTransfer, refID: t.ID, actorID: actorID, note: "Kirim " + t.Number}
	replayed, err := s.transferStep(ctx, t.ID, actorID, []string{"draft"}, "dispatched",
		func(tx *sql.Tx, t *infrastructure.Transfer) error {
			out, err := transferOutLeg(t)
			if err != nil {
				return err
			}
			_, err = s.applyTx(ctx, tx, ref, []leg{{branchID: t.FromBranchID, direction: "out", lines: out}})
			return err
		}, t)
	if err != nil {
		return nil, err
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.dispatch", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.FromBranchID, ActorID: actorID, Note: done.Number})
	}
	return done, nil
}

// ReceiveTransfer books destination stock (in-movements) and closes the
// document, atomically. Transfer is two movements (out + in), never a
// `transfer` type of its own (contract §7.6). Only the destination branch may
// receive.
func (s *Service) ReceiveTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.ToBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	if t.Status == "dispatched" {
		in, err := transferInLeg(t)
		if err != nil {
			return nil, err
		}
		if err := s.validateLegs(ctx, []leg{{branchID: t.ToBranchID, direction: "in", lines: in}}, false); err != nil {
			return nil, err
		}
	}
	ref := legRef{refType: contracts.RefTransfer, refID: t.ID, actorID: actorID, note: "Terima " + t.Number}
	replayed, err := s.transferStep(ctx, t.ID, actorID, []string{"dispatched"}, "received",
		func(tx *sql.Tx, t *infrastructure.Transfer) error {
			in, err := transferInLeg(t)
			if err != nil {
				return err
			}
			_, err = s.applyTx(ctx, tx, ref, []leg{{branchID: t.ToBranchID, direction: "in", lines: in}})
			return err
		}, t)
	if err != nil {
		return nil, err
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.receive", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.ToBranchID, ActorID: actorID, Note: done.Number})
	}
	return done, nil
}

// CancelTransfer voids a draft outright, or reverses a dispatched document
// with return movements to source, atomically. Received documents are final.
// Only the source branch may cancel (it owns the stock while in transit).
func (s *Service) CancelTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.FromBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	if t.Status == "received" {
		return nil, apperror.Conflict("Transfer yang sudah diterima tidak dapat dibatalkan")
	}
	replayed, err := s.transferStep(ctx, t.ID, actorID, []string{"draft", "dispatched"}, "cancelled",
		func(tx *sql.Tx, t *infrastructure.Transfer) error {
			// A draft has no movements, so reversing is a no-op for it; a
			// dispatched document gets its stock back at the source.
			_, err := s.reverseTx(ctx, tx, t.FromBranchID, contracts.RefTransfer, t.ID, actorID)
			return err
		}, t)
	if err != nil {
		return nil, err
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.cancel", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.FromBranchID, ActorID: actorID, Note: done.Number})
	}
	return done, nil
}

// ListTransfersResult is a paginated transfer page.
type ListTransfersResult struct {
	Transfers []*infrastructure.Transfer
	Total     int64
}

// ListTransfers lists documents involving a branch.
func (s *Service) ListTransfers(ctx context.Context, branchID int64, status string, page, limit int) (*ListTransfersResult, error) {
	return s.ListTransfersDir(ctx, branchID, status, "", page, limit)
}

// ListTransfersDir lists transfers filtered by status and direction relative to
// the session branch: "" all, "out", "in", "internal" (ST-06).
func (s *Service) ListTransfersDir(ctx context.Context, branchID int64, status, direction string, page, limit int) (*ListTransfersResult, error) {
	switch direction {
	case "", "out", "in", "internal":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "direction", Message: "harus out, in, atau internal"}})
	}
	total, err := s.repo.CountTransfersDir(ctx, branchID, status, direction)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	transfers, err := s.repo.ListTransfersDir(ctx, branchID, status, direction, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if transfers == nil {
		transfers = []*infrastructure.Transfer{}
	}
	return &ListTransfersResult{Transfers: transfers, Total: total}, nil
}

// --- adjustments ------------------------------------------------------------

// AdjustInput corrects stock. Exactly one of QtyAfter (mode "set") or
// QtyDelta (modes "in"/"out") applies.
type AdjustInput struct {
	ProductID int64
	VariantID int64
	Location  int64
	Mode      string // set | in | out
	QtyAfter  float64
	QtyDelta  float64
	Reason    string
	Approver  string
	ApproverP string
}

// Adjust posts an adjustment movement + balance change. Deltas above the
// company threshold need an approver holding stock.approve (username +
// password verified live — delegated trust, never stored).
//
// The approver credential is verified BEFORE the transaction opens (F1-8):
// bcrypt costs ~250 ms and must never pin a pooled connection (and its row
// locks) while hashing. Guesses are rate-limited per actor+approver and per
// approver, and failures are trailed without the password.
func (s *Service) Adjust(ctx context.Context, actorID, branchID int64, in AdjustInput) error {
	prod, _, err := s.resolveProduct(ctx, in.ProductID, in.VariantID)
	if err != nil {
		return err
	}
	if _, err := s.resolveLocation(ctx, branchID, in.Location); err != nil {
		return err
	}
	if err := s.assertCostBasis(ctx, branchID, prod, in.VariantID); err != nil {
		return err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}})
	}
	var delta float64
	switch in.Mode {
	case "in", "out":
		if in.QtyDelta <= 0 {
			return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
		}
		delta = in.QtyDelta
		if in.Mode == "out" {
			delta = -delta
		}
	case "set":
		if in.QtyAfter < 0 {
			return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "tidak boleh negatif"}})
		}
		delta = 0 // computed against live on-hand inside tx below
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "mode", Message: "harus set, in, atau out"}})
	}
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return err
	}
	threshold, err := s.adjustThreshold(ctx)
	if err != nil {
		return err
	}
	// Approver verification runs here — before Begin — so bcrypt never
	// holds the transaction's connection (F1-8). The result is a boolean
	// the tx body trusts; the credential itself is never rechecked inside.
	var approverOK bool
	if strings.TrimSpace(in.Approver) != "" {
		if !s.approverLim.Allow(strconv.FormatInt(actorID, 10)+"|"+in.Approver) ||
			!s.approverUserLim.Allow("a|"+in.Approver) {
			return apperror.RateLimited()
		}
		if err := s.checkApprover(ctx, actorID, in.Approver, in.ApproverP); err != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock.approver_failed", Entity: "stock_adjustment", BranchID: branchID, ActorID: actorID, Note: in.Approver})
			return err
		}
		approverOK = true
	}
	// The company settings are read before the transaction too, so no pooled
	// connection is pinned while they load.
	strict := s.strictAdjust(ctx)
	// One retried transaction (1213/1205), like Post: a deadlock with a
	// concurrent writer of the position is retried, never a 500.
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location); err != nil {
			return err
		}
		if _, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location); err != nil {
			return err
		}
		// Expired holds return their reserved qty first (as Post and Hold do),
		// so a reduction is not refused over a hold that already lapsed.
		if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location); err != nil {
			return err
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location)
		if err != nil {
			return err
		}
		delta := delta // per attempt: "set" recomputes it from the live on-hand
		if in.Mode == "set" {
			delta = in.QtyAfter - bal.OnHand
		}
		if delta == 0 {
			return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "tidak ada perubahan"}})
		}
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		if (abs > threshold || strict) && !approverOK {
			return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Koreksi melebihi ambang, perlu persetujuan"}
		}
		if delta < 0 && bal.Available() < -delta {
			return apperror.Conflict("Stok tersedia tidak mencukupi untuk koreksi")
		}
		direction, mtype := "in", "adjustment"
		if delta < 0 {
			direction = "out"
		}
		if _, err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, in.Location,
			delta, direction, mtype, "adjustment", 0, in.Reason, actorID); err != nil {
			return err
		}
		note := in.Reason
		if approverOK {
			note += " (approver: " + in.Approver + ")"
		}
		// The trail co-commits with the adjustment (ADR-0011); a failed write
		// rolls the stock change back.
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "stock_adjustments.create", Entity: "stock_adjustment", BranchID: branchID, ActorID: actorID, Note: note,
			Before: map[string]any{"productId": in.ProductID, "variantId": in.VariantID, "locationId": in.Location, "onHand": bal.OnHand},
			After:  map[string]any{"onHand": bal.OnHand + delta, "delta": delta, "approved": approverOK},
		})
	})
	return asAppErrorOrNil(err)
}

// strictAdjust: every adjustment needs an approver unless the company turned
// that off explicitly (legacy parity: always required).
func (s *Service) strictAdjust(ctx context.Context) bool {
	settings, err := s.company.GetSettings(ctx)
	if err != nil {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(settings[StrictAdjustSetting]), "false")
}

// separateApprover: strict approval mode forbids approving one's own adjustment.
func (s *Service) separateApprover(ctx context.Context) bool {
	settings, err := s.company.GetSettings(ctx)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(settings[ApprovalModeSetting]), "strict")
}

// assertCostBasis refuses an adjustment for a product with no way to value it:
// neither a ledger average nor a catalog purchase price (legacy
// assertProductCostBasis).
func (s *Service) assertCostBasis(ctx context.Context, branchID int64, p *productcontracts.Product, variantID int64) error {
	if p.PurchasePrice > 0 {
		return nil
	}
	if s.gate != nil {
		avg, err := s.gate.AverageUnitCost(ctx, branchID, p.ID, variantID)
		if err != nil {
			return err
		}
		if avg > 0 {
			return nil
		}
	}
	return apperror.Conflict("Harga modal produk " + p.Code + " belum tersedia; isi harga beli di master produk dulu")
}

func (s *Service) adjustThreshold(ctx context.Context) (float64, error) {
	settings, err := s.company.GetSettings(ctx)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if raw, ok := settings[ApprovalThresholdSetting]; ok && strings.TrimSpace(raw) != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && f >= 0 {
			return f, nil
		}
	}
	return 100, nil
}

func (s *Service) checkApprover(ctx context.Context, actorID int64, username, password string) error {
	if strings.TrimSpace(username) == "" || password == "" {
		return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Koreksi melebihi ambang, perlu persetujuan"}
	}
	u, err := s.users.VerifyCredentials(ctx, username, password)
	if err != nil {
		return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Persetujuan tidak valid"}
	}
	if actorID != 0 && u.ID == actorID && s.separateApprover(ctx) {
		return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Penyetuju tidak boleh sama dengan pemohon"}
	}
	roleIDs, err := s.users.GetRoleIDs(ctx, u.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	codes, err := s.roles.GetPermissionCodes(ctx, roleIDs)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, c := range codes {
		if c == "stock.approve" {
			return nil
		}
	}
	return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Penyetuju tidak memegang izin stock.approve"}
}

// --- damaged ----------------------------------------------------------------

// DamagedInput moves qty between a location and the branch damaged location.
type DamagedInput struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	Qty        float64
	Reason     string
}

// MoveToDamaged records damaged stock: out of the source location and in to the
// damaged location. A zero LocationID records goods that ARRIVED damaged (no
// source position to deduct): only the damaged-side inflow is written (ST-06).
func (s *Service) MoveToDamaged(ctx context.Context, actorID, branchID int64, in DamagedInput) error {
	if err := s.moveDamaged(ctx, actorID, branchID, in, false); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_damaged.move", Entity: "stock_damaged", EntityID: 0, BranchID: branchID, ActorID: actorID, Note: in.Reason})
	return nil
}

// RestoreDamaged returns salvaged stock to a location.
func (s *Service) RestoreDamaged(ctx context.Context, actorID, branchID int64, in DamagedInput) error {
	if err := s.moveDamaged(ctx, actorID, branchID, in, true); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_damaged.restore", Entity: "stock_damaged", EntityID: 0, BranchID: branchID, ActorID: actorID, Note: in.Reason})
	return nil
}

func (s *Service) moveDamaged(ctx context.Context, actorID, branchID int64, in DamagedInput, restore bool) error {
	if in.Qty <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}})
	}
	if _, _, err := s.resolveProduct(ctx, in.ProductID, in.VariantID); err != nil {
		return err
	}
	arrived := !restore && in.LocationID == 0
	var target *contracts.Location
	if !arrived {
		var err error
		if target, err = s.resolveLocation(ctx, branchID, in.LocationID); err != nil {
			return err
		}
	}
	_, damaged, err := s.ensureDefaults(ctx, branchID)
	if err != nil {
		return err
	}
	if arrived {
		return s.damagedArrival(ctx, actorID, branchID, damaged.ID, in)
	}
	from, to := target.ID, damaged.ID
	verb := "Rusak"
	if restore {
		from, to = damaged.ID, target.ID
		verb = "Pulih"
	}
	note := verb
	if strings.TrimSpace(in.Reason) != "" {
		note += ": " + strings.TrimSpace(in.Reason)
	}
	// The two legs share one ref_id — the out-leg's own movement id, written in
	// this same transaction — so finance pairs the in-leg with exactly its
	// out-leg (by ref_type + ref_id, like a transfer) even when two damaged
	// moves of the same product interleave.
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, from); err != nil {
			return err
		}
		if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, in.ProductID, in.VariantID, from); err != nil {
			return err
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, from)
		if err != nil {
			return err
		}
		if bal.Available() < in.Qty {
			return apperror.Conflict("Stok tersedia tidak mencukupi")
		}
		outID, err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, from,
			-in.Qty, "out", "out", contracts.RefDamaged, 0, note, actorID)
		if err != nil {
			return err
		}
		if err := s.repo.SetMovementRefTx(ctx, tx, outID, outID); err != nil {
			return err
		}
		_, err = postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, to,
			in.Qty, "in", "in", contracts.RefDamaged, outID, note, actorID)
		return err
	})
	return asAppErrorOrNil(err)
}

// damagedArrival books goods that came in already damaged: one inflow at the
// damaged location, no outflow anywhere. Its own ref type keeps it apart from
// the out+in pair of a damaged move: it adds value to the books (finance
// journals it like a stock surplus), while a move is value-neutral.
func (s *Service) damagedArrival(ctx context.Context, actorID, branchID, damagedID int64, in DamagedInput) error {
	err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damagedID); err != nil {
			return err
		}
		if _, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damagedID); err != nil {
			return err
		}
		_, err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, damagedID,
			in.Qty, "in", "in", contracts.RefDamagedArrival, 0, "Rusak (tanpa lokasi asal): "+strings.TrimSpace(in.Reason), actorID)
		return err
	})
	return asAppErrorOrNil(err)
}

// WriteOff destroys damaged stock (out of the damaged location, no return).
func (s *Service) WriteOff(ctx context.Context, actorID, branchID int64, in DamagedInput) error {
	if in.Qty <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}})
	}
	if _, _, err := s.resolveProduct(ctx, in.ProductID, in.VariantID); err != nil {
		return err
	}
	_, damaged, err := s.ensureDefaults(ctx, branchID)
	if err != nil {
		return err
	}
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damaged.ID); err != nil {
			return err
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damaged.ID)
		if err != nil {
			return err
		}
		if bal.OnHand < in.Qty {
			return apperror.Conflict("Stok rusak tidak mencukupi")
		}
		if _, err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, damaged.ID,
			-in.Qty, "out", "out", "write_off", 0, "Hapus: "+strings.TrimSpace(in.Reason), actorID); err != nil {
			return err
		}
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "stock_damaged.write_off", Entity: "stock_damaged", BranchID: branchID, ActorID: actorID, Note: in.Reason,
			Before: map[string]any{"productId": in.ProductID, "variantId": in.VariantID, "onHand": bal.OnHand},
			After:  map[string]any{"onHand": bal.OnHand - in.Qty, "written_off": in.Qty},
		})
	})
	return asAppErrorOrNil(err)
}

// DamagedList lists nonzero positions at the branch damaged location.
func (s *Service) DamagedList(ctx context.Context, branchID int64) ([]BalanceView, error) {
	_, damaged, err := s.ensureDefaults(ctx, branchID)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.BalancesByLocation(ctx, damaged.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]BalanceView, 0, len(balances))
	for _, b := range balances {
		out = append(out, BalanceView{BranchID: b.BranchID, ProductID: b.ProductID,
			VariantID: b.VariantID, LocationID: b.LocationID, OnHand: b.OnHand,
			Reserved: b.Reserved, Available: b.Available()})
	}
	return out, nil
}

// DamagedRow is one damaged position with its estimated loss (ST-03).
type DamagedRow struct {
	BalanceView
	// EstimatedUnitCost is the moving-average cost per base unit (whole rupiah),
	// falling back to the catalog purchase price while the books have no average.
	EstimatedUnitCost int64 `json:"estimatedUnitCost"`
	EstimatedLoss     int64 `json:"estimatedLoss"`
}

// DamagedPageResult is one page of damaged positions plus the figures of ALL of
// them, so the header total does not depend on the page shown.
type DamagedPageResult struct {
	Items []DamagedRow
	// Total counts the selected set (with stock, or emptied when Empty was asked);
	// WithStock/Empty count both sets for the tab labels.
	Total              int64
	WithStock, Empty   int64
	TotalEstimatedLoss int64
}

// DamagedListPage lists the branch's damaged positions with qty x average cost
// (the same average the books use for HPP) per row and in total, paginated by
// the server.
func (s *Service) DamagedListPage(ctx context.Context, branchID int64, page, limit int, emptied bool) (*DamagedPageResult, error) {
	all, err := s.DamagedList(ctx, branchID)
	if err != nil {
		return nil, err
	}
	positive := all[:0:0]
	var withStock, empty int64
	for _, b := range all {
		if b.OnHand > 0 {
			withStock++
		} else {
			empty++
		}
		if (b.OnHand > 0) != emptied {
			positive = append(positive, b)
		}
	}
	ids := make([]int64, 0, len(positive))
	for _, b := range positive {
		ids = append(ids, b.ProductID)
	}
	products, err := s.loadProducts(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]DamagedRow, 0, len(positive))
	var total int64
	for _, b := range positive {
		unit := int64(0)
		if s.gate != nil {
			if unit, err = s.gate.AverageUnitCost(ctx, branchID, b.ProductID, b.VariantID); err != nil {
				return nil, err
			}
		}
		if unit <= 0 {
			if p := products[b.ProductID]; p != nil {
				unit = p.PurchasePrice
			}
		}
		loss := int64(math.Round(b.OnHand * float64(unit)))
		total += loss
		rows = append(rows, DamagedRow{BalanceView: b, EstimatedUnitCost: unit, EstimatedLoss: loss})
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	from := (page - 1) * limit
	if from > len(rows) {
		from = len(rows)
	}
	to := from + limit
	if to > len(rows) {
		to = len(rows)
	}
	return &DamagedPageResult{Items: rows[from:to], Total: int64(len(rows)), WithStock: withStock, Empty: empty, TotalEstimatedLoss: total}, nil
}

// BalanceView is a serializable position.
type BalanceView struct {
	BranchID   int64   `json:"branchId"`
	ProductID  int64   `json:"productId"`
	VariantID  int64   `json:"variantId"`
	LocationID int64   `json:"locationId"`
	OnHand     float64 `json:"onHand"`
	Reserved   float64 `json:"reserved"`
	Available  float64 `json:"available"`
}
