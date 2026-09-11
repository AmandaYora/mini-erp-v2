package application

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	companycontracts "mini-erp/internal/modules/company/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// System location codes, stable per branch.
const (
	DefaultLocationCode = "GDG"
	DamagedLocationCode = "RUSAK"
)

// ApprovalThresholdSetting is the company setting (base units) above which
// adjustments need an approver. Default 100 when unset/unparseable.
const ApprovalThresholdSetting = "stock_adjust_approval_threshold"

// Service implements contracts.StockClient plus stock administration.
type Service struct {
	repo     *infrastructure.Repository
	products productcontracts.ProductClient
	branches branchcontracts.BranchClient
	users    usercontracts.UserClient
	roles    usercontracts.RoleClient
	company  companycontracts.CompanyClient
	// audit receives best-effort trail records after stock mutations.
	audit auditcontracts.AuditClient
}

// NewService wires stock use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	products productcontracts.ProductClient,
	branches branchcontracts.BranchClient,
	users usercontracts.UserClient,
	roles usercontracts.RoleClient,
	company companycontracts.CompanyClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, products: products, branches: branches,
		users: users, roles: roles, company: company, audit: audit}
}

// ensureDefaults guarantees the branch's system locations exist
// ("pastikan ada"). Called at the head of every write path.
func (s *Service) ensureDefaults(ctx context.Context, branchID int64) (*contracts.Location, *contracts.Location, error) {
	def, err := s.repo.EnsureSystemLocation(ctx, branchID, "default", DefaultLocationCode, "Gudang Utama")
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	damaged, err := s.repo.EnsureSystemLocation(ctx, branchID, "damaged", DamagedLocationCode, "Barang Rusak")
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	return def, damaged, nil
}

// resolveProduct validates product + variant liveness for postings.
// Archived products freeze history: nothing posts to them.
func (s *Service) resolveProduct(ctx context.Context, productID, variantID int64) (*productcontracts.Product, *productcontracts.Variant, error) {
	p, err := s.products.GetByID(ctx, productID)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	if p == nil || p.Status != "active" {
		return nil, nil, apperror.NotFound("Produk")
	}
	for _, v := range p.Variants {
		if v.ID == variantID {
			return p, v, nil
		}
	}
	return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "variantId", Message: "varian tidak dikenal"}})
}

// resolveLocation validates a posting target: exists, active, leaf,
// same branch.
func (s *Service) resolveLocation(ctx context.Context, branchID, locationID int64) (*contracts.Location, error) {
	l, err := s.repo.LocationByID(ctx, locationID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if l == nil || l.BranchID != branchID {
		return nil, apperror.NotFound("Lokasi")
	}
	if l.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "locationId", Message: "lokasi sudah diarsipkan"}})
	}
	leaf, err := s.repo.IsLeaf(ctx, locationID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !leaf {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "locationId", Message: "stok hanya tercatat di lokasi daun"}})
	}
	return l, nil
}

// --- StockClient ------------------------------------------------------------

// GetBalance resolves a position; missing rows read as zero.
func (s *Service) GetBalance(ctx context.Context, branchID, productID, variantID, locationID int64) (*contracts.Balance, error) {
	b, err := s.repo.GetBalance(ctx, branchID, productID, variantID, locationID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return b, nil
}

// CheckLocation validates a posting target without writing.
func (s *Service) CheckLocation(ctx context.Context, branchID, locationID int64) error {
	_, err := s.resolveLocation(ctx, branchID, locationID)
	return err
}

// ListCostMovements streams movement rows for HPP costing.
func (s *Service) ListCostMovements(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CostMovement, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	movements, err := s.repo.ListCostMovements(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if movements == nil {
		movements = []*contracts.CostMovement{}
	}
	return movements, nil
}

// postLocked writes one balance delta plus its movement inside tx.
// The caller holds the locked balance row and a validated position.
func postLocked(ctx context.Context, tx *sql.Tx, repo *infrastructure.Repository, branchID, productID, variantID, locationID int64, onHandDelta float64, direction, movementType, refType string, refID int64, notes string, actorID int64) error {
	if err := repo.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return err
	}
	if err := repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, onHandDelta, 0); err != nil {
		return err
	}
	qty := onHandDelta
	if qty < 0 {
		qty = -qty
	}
	return repo.InsertMovementTx(ctx, tx, branchID, productID, variantID, locationID,
		direction, movementType, qty, refType, refID, notes, actorID)
}

// Hold reserves qty under key. Same key + same params replays idempotently
// (safe retries); same key with different params is a conflict.
func (s *Service) Hold(ctx context.Context, actorID, branchID, productID, variantID, locationID int64, qty float64, key string, ttlMinutes int) (int64, error) {
	if qty <= 0 {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if strings.TrimSpace(key) == "" {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "wajib diisi"}})
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	if _, _, err := s.resolveProduct(ctx, productID, variantID); err != nil {
		return 0, err
	}
	if _, err := s.resolveLocation(ctx, branchID, locationID); err != nil {
		return 0, err
	}
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return 0, err
	}
	if existing, err := s.repo.ReservationByKey(ctx, key); err != nil {
		return 0, apperror.Internal(err)
	} else if existing != nil && existing.Status == "active" {
		if existing.BranchID == branchID && existing.ProductID == productID &&
			existing.VariantID == variantID && existing.Total == qty {
			return existing.ID, nil
		}
		return 0, apperror.Conflict("Kunci reservasi sudah dipakai")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return 0, apperror.Internal(err)
	}
	if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return 0, apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, productID, variantID, locationID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if bal.Available() < qty {
		return 0, apperror.Conflict("Stok tersedia tidak mencukupi")
	}
	expires := time.Now().UTC().Add(time.Duration(ttlMinutes) * time.Minute)
	id, err := s.repo.CreateReservationTx(ctx, tx, &infrastructure.Reservation{
		Key: key, BranchID: branchID, ProductID: productID, VariantID: variantID,
		LocationID: locationID, HasLocation: true, Total: qty, Remaining: qty,
		ExpiresAt: expires.Format("2006-01-02 15:04:05"),
	}, 0)
	if err != nil {
		return 0, dberr.Map(err)
	}
	if err := s.repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, 0, qty); err != nil {
		return 0, apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, apperror.Internal(err)
	}
	// Idempotent replays above return early without a log: no mutation
	// happened, same as the finance posting dedupe-hit rule.
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_reservations.hold", Entity: "stock_reservation",
		EntityID: id, BranchID: branchID, ActorID: actorID, Note: key})
	return id, nil
}

// Release frees a reservation by key. Unknown or finished keys succeed
// silently (idempotent — safe to retry and safe to call twice); only actual
// state changes are trailed.
func (s *Service) Release(ctx context.Context, actorID int64, key string) error {
	res, err := s.repo.ReservationByKey(ctx, strings.TrimSpace(key))
	if err != nil {
		return apperror.Internal(err)
	}
	if res == nil || res.Status != "active" {
		return nil
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := s.repo.GetReservationTx(ctx, tx, res.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if locked == nil || locked.Status != "active" {
		return nil // already finished concurrently — idempotent success
	}
	if !locked.HasLocation {
		if err := s.repo.UpdateReservationTx(ctx, tx, locked.ID, 0, "released"); err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Commit(); err != nil {
			return apperror.Internal(err)
		}
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_reservations.release", Entity: "stock_reservation",
			EntityID: locked.ID, BranchID: locked.BranchID, ActorID: actorID, Note: key})
		return nil
	}
	if err := s.repo.EnsureBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID); err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.AddBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID, 0, -locked.Remaining); err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.UpdateReservationTx(ctx, tx, locked.ID, 0, "released"); err != nil {
		return apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_reservations.release", Entity: "stock_reservation",
		EntityID: locked.ID, BranchID: locked.BranchID, ActorID: actorID, Note: key})
	return nil
}

// CriticalStock lists tracked products whose total available stock is at
// or below minimum, scarcest first (assistant bot tool). Names come from
// the product contract; quantities from own balances — never a cross-module
// join. Products with no balance row read as zero available.
func (s *Service) CriticalStock(ctx context.Context, branchID int64, limit int) ([]*contracts.CriticalItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	stocked, err := s.products.ListStocked(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	avail, err := s.repo.ProductAvailability(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := []*contracts.CriticalItem{}
	for _, p := range stocked {
		a := avail[p.ID]
		if a > p.MinStock {
			continue
		}
		out = append(out, &contracts.CriticalItem{
			ProductID: p.ID, ProductCode: p.Code, ProductName: p.Name,
			Available: a, MinStock: p.MinStock,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Available == out[j].Available {
			return out[i].ProductCode < out[j].ProductCode
		}
		return out[i].Available < out[j].Available
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Consume turns reserved qty into a sale outflow (partial allowed).
func (s *Service) Consume(ctx context.Context, key string, qty float64, refType string, refID int64, actorID int64) error {
	if qty <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	res, err := s.repo.ReservationByKey(ctx, strings.TrimSpace(key))
	if err != nil {
		return apperror.Internal(err)
	}
	if res == nil || res.Status != "active" {
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi tidak aktif"}})
	}
	if time.Now().UTC().After(parseTime(res.ExpiresAt)) {
		if uerr := s.repo.UpdateReservation(ctx, res.ID, res.Remaining, "expired"); uerr != nil {
			return apperror.Internal(uerr)
		}
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi kedaluwarsa"}})
	}
	if qty > res.Remaining {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "melebihi sisa reservasi"}})
	}
	if !res.HasLocation {
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi tanpa lokasi"}})
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := s.repo.GetReservationTx(ctx, tx, res.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if locked == nil || locked.Status != "active" {
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi tidak aktif"}})
	}
	if time.Now().UTC().After(parseTime(locked.ExpiresAt)) {
		if err := s.repo.UpdateReservationTx(ctx, tx, locked.ID, locked.Remaining, "expired"); err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Commit(); err != nil {
			return apperror.Internal(err)
		}
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi kedaluwarsa"}})
	}
	if qty > locked.Remaining {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "melebihi sisa reservasi"}})
	}
	if !locked.HasLocation {
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "reservasi tanpa lokasi"}})
	}
	if err := s.repo.EnsureBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID); err != nil {
		return apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID)
	if err != nil {
		return apperror.Internal(err)
	}
	if bal.OnHand < qty {
		return apperror.Conflict("Stok fisik tidak mencukupi")
	}
	if err := s.repo.AddBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID, -qty, -qty); err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.InsertMovementTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID,
		"out", "out", qty, refType, refID, "", actorID); err != nil {
		return apperror.Internal(err)
	}
	remaining := locked.Remaining - qty
	status := "active"
	if remaining <= 0 {
		remaining, status = 0, "consumed"
	}
	if err := s.repo.UpdateReservationTx(ctx, tx, locked.ID, remaining, status); err != nil {
		return apperror.Internal(err)
	}
	return tx.Commit()
}

func parseTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// MoveIn records an inflow with its movement (receipts, openings, returns).
func (s *Service) MoveIn(ctx context.Context, branchID, productID, variantID, locationID int64, qty float64, refType string, refID int64, actorID int64) error {
	if qty <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if _, _, err := s.resolveProduct(ctx, productID, variantID); err != nil {
		return err
	}
	if _, err := s.resolveLocation(ctx, branchID, locationID); err != nil {
		return err
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := postLocked(ctx, tx, s.repo, branchID, productID, variantID, locationID,
		qty, "in", "in", refType, refID, "", actorID); err != nil {
		return apperror.Internal(err)
	}
	return tx.Commit()
}

// MoveOut records an outflow with its movement, refusing to oversell.
func (s *Service) MoveOut(ctx context.Context, branchID, productID, variantID, locationID int64, qty float64, refType string, refID int64, actorID int64) error {
	if qty <= 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if _, _, err := s.resolveProduct(ctx, productID, variantID); err != nil {
		return err
	}
	if _, err := s.resolveLocation(ctx, branchID, locationID); err != nil {
		return err
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return apperror.Internal(err)
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, productID, variantID, locationID)
	if err != nil {
		return apperror.Internal(err)
	}
	if bal.Available() < qty {
		return apperror.Conflict("Stok tersedia tidak mencukupi")
	}
	if err := postLocked(ctx, tx, s.repo, branchID, productID, variantID, locationID,
		-qty, "out", "out", refType, refID, "", actorID); err != nil {
		return apperror.Internal(err)
	}
	return tx.Commit()
}

// --- locations --------------------------------------------------------------

// LocationInput is the location create/update payload.
type LocationInput struct {
	Code     string
	Name     string
	ParentID *int64
}

// CreateLocation inserts a leaf-capable location.
func (s *Service) CreateLocation(ctx context.Context, actorID, branchID int64, in LocationInput) (*contracts.Location, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.LocationByCode(ctx, branchID, code); err != nil {
		return nil, apperror.Internal(err)
	} else if dup != nil {
		return nil, apperror.Conflict("Kode lokasi '" + code + "' sudah digunakan")
	}
	if in.ParentID != nil {
		parent, err := s.repo.LocationByID(ctx, *in.ParentID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if parent == nil || parent.BranchID != branchID || parent.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "induk tidak valid"}})
		}
	}
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return nil, err
	}
	id, err := s.repo.CreateLocation(ctx, &contracts.Location{
		BranchID: branchID, Code: code, Name: strings.TrimSpace(in.Name), ParentID: in.ParentID,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	loc, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_locations.create", Entity: "stock_location", EntityID: loc.ID, BranchID: branchID, ActorID: actorID, Note: loc.Code})
	return loc, nil
}

// UpdateLocation rewrites name/parent (never into its own subtree).
func (s *Service) UpdateLocation(ctx context.Context, actorID, branchID, id int64, in LocationInput) (*contracts.Location, error) {
	existing, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil || existing.BranchID != branchID {
		return nil, apperror.NotFound("Lokasi")
	}
	if existing.IsSystem {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "lokasi sistem tidak dapat diubah"}})
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if in.ParentID != nil {
		if *in.ParentID == id {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "lokasi tidak bisa menjadi induk dirinya sendiri"}})
		}
		parent, err := s.repo.LocationByID(ctx, *in.ParentID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if parent == nil || parent.BranchID != existing.BranchID || parent.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "induk tidak valid"}})
		}
		if cyclic, err := s.isDescendant(ctx, id, *in.ParentID); err != nil {
			return nil, apperror.Internal(err)
		} else if cyclic {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "induk membentuk siklus"}})
		}
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.ParentID = in.ParentID
	if err := s.repo.UpdateLocation(ctx, existing, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_locations.update", Entity: "stock_location", EntityID: updated.ID, BranchID: existing.BranchID, ActorID: actorID, Note: updated.Code})
	return updated, nil
}

// isDescendant walks up from start looking for target (cycle guard).
func (s *Service) isDescendant(ctx context.Context, target, start int64) (bool, error) {
	current := start
	for i := 0; i < 20; i++ {
		l, err := s.repo.LocationByID(ctx, current)
		if err != nil || l == nil || l.ParentID == nil {
			return false, err
		}
		if *l.ParentID == target {
			return true, nil
		}
		current = *l.ParentID
	}
	return true, nil
}

// ArchiveLocation archives an empty, childless, non-system location.
func (s *Service) ArchiveLocation(ctx context.Context, actorID, branchID, id int64) error {
	existing, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil || existing.BranchID != branchID {
		return apperror.NotFound("Lokasi")
	}
	if existing.IsSystem {
		return apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "lokasi sistem tidak dapat diarsipkan"}})
	}
	if children, err := s.repo.HasActiveChildren(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if children {
		return apperror.Conflict("Lokasi masih memiliki sublokasi aktif")
	}
	if used, err := s.repo.LocationHasBalances(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if used {
		return apperror.Conflict("Lokasi masih memiliki saldo")
	}
	if err := s.repo.SetLocationStatus(ctx, id, "archived", actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_locations.archive", Entity: "stock_location", EntityID: id, BranchID: existing.BranchID, ActorID: actorID, Note: existing.Code})
	return nil
}

// ListLocations returns a branch's locations. This read deliberately ensures
// defaults first: a fresh branch must show its system locations on first
// view instead of an empty screen that invites duplicates.
func (s *Service) ListLocations(ctx context.Context, branchID int64, status string) ([]*contracts.Location, error) {
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return nil, err
	}
	locations, err := s.repo.ListLocations(ctx, branchID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if locations == nil {
		locations = []*contracts.Location{}
	}
	return locations, nil
}

// --- reads ------------------------------------------------------------------

// BalancesByProduct lists a product's nonzero positions in a branch.
func (s *Service) BalancesByProduct(ctx context.Context, branchID, productID int64) ([]*contracts.Balance, error) {
	balances, err := s.repo.BalancesByProduct(ctx, branchID, productID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if balances == nil {
		balances = []*contracts.Balance{}
	}
	return balances, nil
}

// ScanResult is the scan-product answer: identity plus live positions.
type ScanResult struct {
	Product  any           `json:"product"`
	Variant  any           `json:"variant"`
	Balances []BalanceView `json:"balances"`
}

// Scan resolves a barcode to variant + product + branch positions.
func (s *Service) Scan(ctx context.Context, branchID int64, barcode string) (*ScanResult, error) {
	v, err := s.products.GetVariantByBarcode(ctx, barcode)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if v == nil {
		return nil, apperror.NotFound("Produk")
	}
	p, err := s.products.GetByID(ctx, v.ProductID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, apperror.NotFound("Produk")
	}
	balances, err := s.repo.BalancesByProduct(ctx, branchID, p.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	views := make([]BalanceView, 0, len(balances))
	for _, b := range balances {
		views = append(views, BalanceView{BranchID: b.BranchID, ProductID: b.ProductID,
			VariantID: b.VariantID, LocationID: b.LocationID, OnHand: b.OnHand,
			Reserved: b.Reserved, Available: b.Available()})
	}
	return &ScanResult{
		Product:  map[string]any{"id": p.ID, "code": p.Code, "name": p.Name},
		Variant:  map[string]any{"id": v.ID, "code": v.Code, "name": v.Name},
		Balances: views,
	}, nil
}

// ListMovementsResult is a paginated movement page.
type ListMovementsResult struct {
	Movements []*infrastructure.Movement
	Total     int64
}

// ListMovements filters history newest-first (date bounds are inclusive,
// caller converts to UTC day bounds — OQ-A32).
func (s *Service) ListMovements(ctx context.Context, branchID, productID, variantID, locationID int64, movementType, dateFrom, dateTo string, page, limit int) (*ListMovementsResult, error) {
	total, err := s.repo.CountMovements(ctx, branchID, productID, variantID, locationID, movementType, dateFrom, dateTo)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	movements, err := s.repo.ListMovements(ctx, branchID, productID, variantID, locationID, movementType, dateFrom, dateTo, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if movements == nil {
		movements = []*infrastructure.Movement{}
	}
	return &ListMovementsResult{Movements: movements, Total: total}, nil
}

// Suggest greedily fills qty from locations with available stock (oldest
// locations first). Shortfall reports what could not be covered — callers
// decide whether partial is acceptable.
func (s *Service) Suggest(ctx context.Context, branchID, productID, variantID int64, qty float64) ([]contracts.Suggestion, float64, error) {
	if qty <= 0 {
		return nil, 0, apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	positions, err := s.repo.SuggestLocations(ctx, branchID, productID, variantID)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	var out []contracts.Suggestion
	remaining := qty
	for _, p := range positions {
		if remaining <= 0 {
			break
		}
		take := p.Available()
		if take > remaining {
			take = remaining
		}
		out = append(out, contracts.Suggestion{LocationID: p.LocationID, Qty: take})
		remaining -= take
	}
	if out == nil {
		out = []contracts.Suggestion{}
	}
	return out, remaining, nil
}
