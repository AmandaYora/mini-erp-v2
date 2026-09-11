package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// TransferItemInput is one transfer line (base UOM quantities).
type TransferItemInput struct {
	ProductID int64
	VariantID int64
	Qty       float64
	Notes     string
}

// CreateTransfer drafts a transfer document with an allocated number.
// Stock does not move until dispatch.
func (s *Service) CreateTransfer(ctx context.Context, actorID, fromBranchID, toBranchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput) (*infrastructure.Transfer, error) {
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
	for i, it := range items {
		if it.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "qty baris " + strconv.Itoa(i+1) + " harus lebih dari 0"}})
		}
		if _, _, err := s.resolveProduct(ctx, it.ProductID, it.VariantID); err != nil {
			return nil, err
		}
		docItems = append(docItems, infrastructure.TransferItem{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty, Notes: it.Notes,
		})
	}
	number, err := s.branches.NextDocumentNumber(ctx, fromBranchID, branchcontracts.DocStockTransfer)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateTransfer(ctx, &infrastructure.Transfer{
		Number: number, FromBranchID: fromBranchID, ToBranchID: toBranchID,
		FromLocation: fromLocationID, ToLocation: toLocationID, Notes: notes, Items: docItems,
	}, actorID)
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

// DispatchTransfer deducts source stock (out-movements) and marks the
// document dispatched. Short lines abort the whole dispatch — partial
// dispatch does not exist. Only the source branch may dispatch.
func (s *Service) DispatchTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.FromBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	if t.Status != "draft" {
		return nil, apperror.Conflict("Transfer hanya bisa dikirim dari status draft")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for i, it := range t.Items {
		if err := s.repo.EnsureBalanceTx(ctx, tx, t.FromBranchID, it.ProductID, it.VariantID, t.FromLocation); err != nil {
			return nil, apperror.Internal(err)
		}
		if err := s.repo.ExpireReservationsTx(ctx, tx, t.FromBranchID, it.ProductID, it.VariantID, t.FromLocation); err != nil {
			return nil, apperror.Internal(err)
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, t.FromBranchID, it.ProductID, it.VariantID, t.FromLocation)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if bal.Available() < it.Qty {
			return nil, apperror.Conflict("Stok tersedia tidak mencukupi (baris " + strconv.Itoa(i+1) + ")")
		}
		if err := postLocked(ctx, tx, s.repo, t.FromBranchID, it.ProductID, it.VariantID, t.FromLocation,
			-it.Qty, "out", "out", "transfer", t.ID, "Kirim "+t.Number, actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	if err := s.repo.SetTransferStatusTx(ctx, tx, t.ID, "dispatched", actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperror.Internal(err)
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.dispatch", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.FromBranchID, ActorID: actorID, Note: done.Number})
	return done, nil
}

// ReceiveTransfer books destination stock (in-movements) and closes the
// document. Transfer is two movements (out + in), never a `transfer` type
// of its own (contract §7.6). Only the destination branch may receive.
func (s *Service) ReceiveTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.ToBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	if t.Status != "dispatched" {
		return nil, apperror.Conflict("Transfer hanya bisa diterima dari status terkirim")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, it := range t.Items {
		if err := postLocked(ctx, tx, s.repo, t.ToBranchID, it.ProductID, it.VariantID, t.ToLocation,
			it.Qty, "in", "in", "transfer", t.ID, "Terima "+t.Number, actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	if err := s.repo.SetTransferStatusTx(ctx, tx, t.ID, "received", actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperror.Internal(err)
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.receive", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.ToBranchID, ActorID: actorID, Note: done.Number})
	return done, nil
}

// CancelTransfer voids a draft outright, or reverses a dispatched document
// with return movements to source. Received documents are final. Only the
// source branch may cancel (it owns the stock while in transit).
func (s *Service) CancelTransfer(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Transfer, error) {
	t, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if t.FromBranchID != branchID {
		return nil, apperror.NotFound("Transfer")
	}
	switch t.Status {
	case "draft":
		if err := s.repo.SetTransferStatus(ctx, t.ID, "cancelled", actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	case "dispatched":
		tx, err := s.repo.Begin(ctx)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		defer func() { _ = tx.Rollback() }()
		for _, it := range t.Items {
			if err := postLocked(ctx, tx, s.repo, t.FromBranchID, it.ProductID, it.VariantID, t.FromLocation,
				it.Qty, "in", "in", "transfer_cancel", t.ID, "Batal "+t.Number, actorID); err != nil {
				return nil, apperror.Internal(err)
			}
		}
		if err := s.repo.SetTransferStatusTx(ctx, tx, t.ID, "cancelled", actorID); err != nil {
			return nil, apperror.Internal(err)
		}
		if err := tx.Commit(); err != nil {
			return nil, apperror.Internal(err)
		}
	default:
		return nil, apperror.Conflict("Transfer yang sudah diterima tidak dapat dibatalkan")
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_transfers.cancel", Entity: "stock_transfer", EntityID: done.ID, BranchID: t.FromBranchID, ActorID: actorID, Note: done.Number})
	return done, nil
}

// ListTransfersResult is a paginated transfer page.
type ListTransfersResult struct {
	Transfers []*infrastructure.Transfer
	Total     int64
}

// ListTransfers lists documents involving a branch.
func (s *Service) ListTransfers(ctx context.Context, branchID int64, status string, page, limit int) (*ListTransfersResult, error) {
	total, err := s.repo.CountTransfers(ctx, branchID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	transfers, err := s.repo.ListTransfers(ctx, branchID, status, limit, (page-1)*limit)
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
func (s *Service) Adjust(ctx context.Context, actorID, branchID int64, in AdjustInput) error {
	if _, _, err := s.resolveProduct(ctx, in.ProductID, in.VariantID); err != nil {
		return err
	}
	if _, err := s.resolveLocation(ctx, branchID, in.Location); err != nil {
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
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location); err != nil {
		return apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, in.Location)
	if err != nil {
		return apperror.Internal(err)
	}
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
	if abs > threshold {
		if err := s.checkApprover(ctx, in.Approver, in.ApproverP); err != nil {
			return err
		}
	}
	if delta < 0 && bal.Available() < -delta {
		return apperror.Conflict("Stok tersedia tidak mencukupi untuk koreksi")
	}
	direction, mtype := "in", "adjustment"
	if delta < 0 {
		direction = "out"
	}
	if err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, in.Location,
		delta, direction, mtype, "adjustment", 0, in.Reason, actorID); err != nil {
		return apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_adjustments.create", Entity: "stock_adjustment", EntityID: 0, BranchID: branchID, ActorID: actorID, Note: in.Reason})
	return nil
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

func (s *Service) checkApprover(ctx context.Context, username, password string) error {
	if strings.TrimSpace(username) == "" || password == "" {
		return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Koreksi melebihi ambang, perlu persetujuan"}
	}
	u, err := s.users.VerifyCredentials(ctx, username, password)
	if err != nil {
		return &apperror.AppError{Code: apperror.CodeForbidden, Message: "Persetujuan tidak valid"}
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

// MoveToDamaged records damaged stock (out of source, in to damaged).
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
	if _, _, err := s.resolveProduct(ctx, in.ProductID, in.VariantID); err != nil {
		return err
	}
	target, err := s.resolveLocation(ctx, branchID, in.LocationID)
	if err != nil {
		return err
	}
	_, damaged, err := s.ensureDefaults(ctx, branchID)
	if err != nil {
		return err
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
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, from); err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, in.ProductID, in.VariantID, from); err != nil {
		return apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, from)
	if err != nil {
		return apperror.Internal(err)
	}
	if bal.Available() < in.Qty {
		return apperror.Conflict("Stok tersedia tidak mencukupi")
	}
	if err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, from,
		-in.Qty, "out", "out", "damaged", 0, note, actorID); err != nil {
		return apperror.Internal(err)
	}
	if err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, to,
		in.Qty, "in", "in", "damaged", 0, note, actorID); err != nil {
		return apperror.Internal(err)
	}
	return tx.Commit()
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
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damaged.ID); err != nil {
		return apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, in.ProductID, in.VariantID, damaged.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if bal.OnHand < in.Qty {
		return apperror.Conflict("Stok rusak tidak mencukupi")
	}
	if err := postLocked(ctx, tx, s.repo, branchID, in.ProductID, in.VariantID, damaged.ID,
		-in.Qty, "out", "out", "write_off", 0, "Hapus: "+strings.TrimSpace(in.Reason), actorID); err != nil {
		return apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_damaged.write_off", Entity: "stock_damaged", EntityID: 0, BranchID: branchID, ActorID: actorID, Note: in.Reason})
	return nil
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
