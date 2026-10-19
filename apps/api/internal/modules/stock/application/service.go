package application

import (
	"context"
	"database/sql"
	"errors"
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
	"mini-erp/internal/shared/ratelimit"
)

// System location codes, stable per branch.
const (
	DefaultLocationCode = "GDG"
	DamagedLocationCode = "RUSAK"
)

// ApprovalThresholdSetting is the company setting (base units) above which
// adjustments need an approver. Default 100 when unset/unparseable.
const ApprovalThresholdSetting = "stock_adjust_approval_threshold"

// StrictAdjustSetting, when "true", makes every adjustment need an approver
// regardless of size (legacy parity for stricter companies).
const StrictAdjustSetting = "stock_adjust_always_approve"

// ApprovalModeSetting = "strict" forbids approving one's own adjustment.
const ApprovalModeSetting = "stock_adjust_approval_mode"

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
	// approverLim bounds approver-password guesses per actor+approver pair;
	// approverUserLim bounds them per approver username (F1-8: the
	// adjustment form otherwise lets any stock.adjust holder guess another
	// user's password, with bcrypt pinning a pool connection per guess).
	approverLim     *ratelimit.Limiter
	approverUserLim *ratelimit.Limiter
	// gate is the late-bound finance side (opening guard, cost basis).
	gate FinanceGate
}

// FinanceGate is the slice of finance the stock module consults: whether the
// cutover balance is already posted, and what a position costs on average.
type FinanceGate interface {
	OpeningPosted(ctx context.Context, branchID int64) (bool, error)
	AverageUnitCost(ctx context.Context, branchID, productID, variantID int64) (int64, error)
}

// SetFinanceGate wires finance (built after stock).
func (s *Service) SetFinanceGate(g FinanceGate) { s.gate = g }

// ServiceOption customizes the service (tests inject tight limiters).
type ServiceOption func(*Service)

// WithApproverLimiter overrides the default approver guess limiters.
func WithApproverLimiter(perPair, perApprover *ratelimit.Limiter) ServiceOption {
	return func(s *Service) {
		if perPair != nil {
			s.approverLim = perPair
		}
		if perApprover != nil {
			s.approverUserLim = perApprover
		}
	}
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
	opts ...ServiceOption,
) *Service {
	s := &Service{repo: repo, products: products, branches: branches,
		users: users, roles: roles, company: company, audit: audit,
		approverLim:     ratelimit.New(5, time.Minute),
		approverUserLim: ratelimit.New(10, time.Minute)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ensureDefaults guarantees the branch's system locations exist
// ("pastikan ada"). Called at the head of every write path.
func (s *Service) ensureDefaults(ctx context.Context, branchID int64) (*contracts.Location, *contracts.Location, error) {
	// The system default location is named by the branch's label (F3b-6) and
	// follows later label edits with one idempotent rename.
	label := "Gudang Utama"
	if b, err := s.branches.GetByID(ctx, branchID); err == nil && b != nil && b.DefaultLocationLabel != "" {
		label = b.DefaultLocationLabel
	}
	def, err := s.repo.EnsureSystemLocation(ctx, branchID, "default", DefaultLocationCode, label)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	if def.Name != label {
		if err := s.repo.RenameLocation(ctx, def.ID, label); err != nil {
			return nil, nil, apperror.Internal(err)
		}
		def.Name = label
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
	return pickLiveVariant(p, variantID)
}

// pickLiveVariant is resolveProduct's rule over an already loaded product.
func pickLiveVariant(p *productcontracts.Product, variantID int64) (*productcontracts.Product, *productcontracts.Variant, error) {
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

// loadProducts reads every product of a document in one batch (3 statements
// whatever the line count) for per-line validation.
func (s *Service) loadProducts(ctx context.Context, ids []int64) (map[int64]*productcontracts.Product, error) {
	m, err := s.products.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return m, nil
}

// resolveLocation validates a posting target: exists, active, leaf,
// same branch.
func (s *Service) resolveLocation(ctx context.Context, branchID, locationID int64) (*contracts.Location, error) {
	return s.resolveLocationKind(ctx, branchID, locationID, false)
}

// resolveLocationKind is resolveLocation with an explicit stance on the
// damaged location (only returns of damaged goods may target it).
func (s *Service) resolveLocationKind(ctx context.Context, branchID, locationID int64, allowDamaged bool) (*contracts.Location, error) {
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
	if l.Kind == contracts.KindDamaged && !allowDamaged {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "locationId", Message: "lokasi barang rusak tidak dapat dipakai untuk transaksi ini"}})
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

// DamagedLocation returns the branch damaged-goods location.
func (s *Service) DamagedLocation(ctx context.Context, branchID int64) (*contracts.Location, error) {
	_, damaged, err := s.ensureDefaults(ctx, branchID)
	return damaged, err
}

// CheckLocation validates a posting target without writing.
func (s *Service) CheckLocation(ctx context.Context, branchID, locationID int64) error {
	_, err := s.resolveLocation(ctx, branchID, locationID)
	return err
}

// ListRecentCostMovements returns the movements of the last few minutes.
func (s *Service) ListRecentCostMovements(ctx context.Context, branchID int64, minutes, limit int) ([]*contracts.CostMovement, error) {
	if minutes <= 0 || minutes > 60 {
		minutes = 10
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	out, err := s.repo.RecentCostMovements(ctx, branchID, minutes, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if out == nil {
		out = []*contracts.CostMovement{}
	}
	return out, nil
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
// The caller holds a validated position; the balance row is locked here so
// before/after on the movement are exact. refID 0 means "no document"
// (adjustments, openings) and is exempt from the unique document key.
func postLocked(ctx context.Context, tx *sql.Tx, repo *infrastructure.Repository, branchID, productID, variantID, locationID int64, onHandDelta float64, direction, movementType, refType string, refID int64, notes string, actorID int64) error {
	if err := repo.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
		return err
	}
	bal, err := repo.GetBalanceTx(ctx, tx, branchID, productID, variantID, locationID)
	if err != nil {
		return err
	}
	if err := repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, onHandDelta, 0); err != nil {
		return err
	}
	qty := onHandDelta
	if qty < 0 {
		qty = -qty
	}
	return repo.InsertMovementTx(ctx, tx, infrastructure.MovementRow{
		BranchID: branchID, ProductID: productID, VariantID: variantID, LocationID: locationID,
		Direction: direction, MovementType: movementType, Qty: qty,
		Before: bal.OnHand, After: bal.OnHand + onHandDelta,
		RefType: refType, RefID: refID, Notes: notes, ActorID: actorID,
	})
}

// Hold reserves qty under key (upsert). The same key on the same position
// changes the held quantity in place (the delta is checked against what is
// available) and extends the TTL, so a POS cart can follow its basket; a key
// owned by another position/branch, or already consumed/released, conflicts.
// An expired hold of the same position is revived.
func (s *Service) Hold(ctx context.Context, actorID, branchID, productID, variantID, locationID int64, qty float64, key string, ttlMinutes int) (int64, error) {
	key = strings.TrimSpace(key)
	if qty <= 0 {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0"}})
	}
	if key == "" || len(key) > 100 {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "wajib diisi, maksimal 100 karakter"}})
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 20
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
	var id int64
	var changed bool
	for attempt := 0; attempt < 3; attempt++ {
		var err error
		id, changed, err = s.holdOnce(ctx, actorID, branchID, productID, variantID, locationID, qty, key, ttlMinutes)
		if err == nil {
			break
		}
		if attempt < 2 && isDuplicateKey(err) {
			continue // another request created the key meanwhile: update path
		}
		return 0, err
	}
	if changed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_reservations.hold", Entity: "stock_reservation",
			EntityID: id, BranchID: branchID, ActorID: actorID, Note: key})
	}
	return id, nil
}

func (s *Service) holdOnce(ctx context.Context, actorID, branchID, productID, variantID, locationID int64, qty float64, key string, ttlMinutes int) (int64, bool, error) {
	var id int64
	var changed bool
	err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		id, changed = 0, false
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
			return err
		}
		// Balance lock first (same order as Post), then the hold row.
		if _, err := s.repo.GetBalanceTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
			return err
		}
		if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, productID, variantID, locationID); err != nil {
			return err
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, productID, variantID, locationID)
		if err != nil {
			return err
		}
		// Unlocked key probe: a locking read of a missing unique key would take
		// a gap lock and deadlock with a concurrent insert of the same key.
		probe, err := s.repo.ReservationProbeTx(ctx, tx, key)
		if err != nil {
			return err
		}
		expires := time.Now().UTC().Add(time.Duration(ttlMinutes) * time.Minute).Format("2006-01-02 15:04:05")
		if probe == 0 {
			if bal.Available() < qty {
				return apperror.Conflict("Stok tersedia tidak mencukupi")
			}
			newID, err := s.repo.CreateReservationTx(ctx, tx, &infrastructure.Reservation{
				Key: key, BranchID: branchID, ProductID: productID, VariantID: variantID,
				LocationID: locationID, HasLocation: true, Total: qty, Remaining: qty, ExpiresAt: expires,
			}, actorID)
			if err != nil {
				return err
			}
			if err := s.repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, 0, qty); err != nil {
				return err
			}
			id, changed = newID, true
			return nil
		}
		cur, err := s.repo.GetReservationTx(ctx, tx, probe)
		if err != nil {
			return err
		}
		if cur == nil || cur.BranchID != branchID || cur.ProductID != productID ||
			cur.VariantID != variantID || !cur.HasLocation || cur.LocationID != locationID {
			return apperror.Conflict("Kunci reservasi sudah dipakai")
		}
		switch cur.Status {
		case "active":
			delta := qty - cur.Remaining
			if delta > 0 && bal.Available() < delta {
				return apperror.Conflict("Stok tersedia tidak mencukupi")
			}
			if err := s.repo.RewriteReservationTx(ctx, tx, cur.ID, qty, qty, "active", expires); err != nil {
				return err
			}
			if delta != 0 {
				if err := s.repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, 0, delta); err != nil {
					return err
				}
			}
			id, changed = cur.ID, delta != 0
		case "expired":
			if bal.Available() < qty {
				return apperror.Conflict("Stok tersedia tidak mencukupi")
			}
			if err := s.repo.RewriteReservationTx(ctx, tx, cur.ID, qty, qty, "active", expires); err != nil {
				return err
			}
			if err := s.repo.AddBalanceTx(ctx, tx, branchID, productID, variantID, locationID, 0, qty); err != nil {
				return err
			}
			id, changed = cur.ID, true
		default:
			return apperror.Conflict("Reservasi sudah " + cur.Status)
		}
		return nil
	})
	if err != nil {
		var ae *apperror.AppError
		if errors.As(err, &ae) {
			return 0, false, ae
		}
		if isDuplicateKey(err) {
			return 0, false, err
		}
		return 0, false, apperror.Internal(err)
	}
	return id, changed, nil
}

// Release frees a reservation by key. Unknown or finished keys succeed
// silently (idempotent — safe to retry and safe to call twice); only actual
// state changes are trailed. branchID > 0 scopes the call to the session's
// branch: a key of another branch reads as unknown.
func (s *Service) Release(ctx context.Context, actorID, branchID int64, key string) error {
	key = strings.TrimSpace(key)
	res, err := s.repo.ReservationByKey(ctx, key)
	if err != nil {
		return apperror.Internal(err)
	}
	if res == nil || res.Status != "active" || (branchID > 0 && res.BranchID != branchID) {
		return nil
	}
	released := false
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		released = false
		if res.HasLocation {
			if err := s.repo.EnsureBalanceTx(ctx, tx, res.BranchID, res.ProductID, res.VariantID, res.LocationID); err != nil {
				return err
			}
			if _, err := s.repo.GetBalanceTx(ctx, tx, res.BranchID, res.ProductID, res.VariantID, res.LocationID); err != nil {
				return err
			}
		}
		locked, err := s.repo.GetReservationTx(ctx, tx, res.ID)
		if err != nil {
			return err
		}
		if locked == nil || locked.Status != "active" {
			return nil // already finished concurrently — idempotent success
		}
		if locked.HasLocation {
			if err := s.repo.AddBalanceTx(ctx, tx, locked.BranchID, locked.ProductID, locked.VariantID, locked.LocationID, 0, -locked.Remaining); err != nil {
				return err
			}
		}
		if err := s.repo.UpdateReservationTx(ctx, tx, locked.ID, 0, "released"); err != nil {
			return err
		}
		released = true
		return nil
	})
	if err != nil {
		return apperror.Internal(err)
	}
	if released {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_reservations.release", Entity: "stock_reservation",
			EntityID: res.ID, BranchID: res.BranchID, ActorID: actorID, Note: key})
	}
	return nil
}

// SweepExpired returns the reserved quantity of up to limit expired holds to
// their positions (A6). Idempotent and safe to run concurrently with Hold,
// Post and itself: each hold flips active→expired once, under its position's
// balance lock. Returns how many holds were swept.
func (s *Service) SweepExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 500
	}
	list, err := s.repo.ExpiredReservations(ctx, limit)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	swept := 0
	for _, e := range list {
		var did bool
		err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
			did = false
			if e.HasLocation {
				if err := s.repo.EnsureBalanceTx(ctx, tx, e.BranchID, e.ProductID, e.VariantID, e.LocationID); err != nil {
					return err
				}
				if _, err := s.repo.GetBalanceTx(ctx, tx, e.BranchID, e.ProductID, e.VariantID, e.LocationID); err != nil {
					return err
				}
			}
			cur, err := s.repo.GetReservationTx(ctx, tx, e.ID)
			if err != nil {
				return err
			}
			if cur == nil || cur.Status != "active" {
				return nil
			}
			if cur.HasLocation {
				if err := s.repo.AddBalanceTx(ctx, tx, cur.BranchID, cur.ProductID, cur.VariantID, cur.LocationID, 0, -cur.Remaining); err != nil {
					return err
				}
			}
			did = true
			return s.repo.UpdateReservationTx(ctx, tx, cur.ID, cur.Remaining, "expired")
		})
		if err != nil {
			return swept, apperror.Internal(err)
		}
		if did {
			swept++
		}
	}
	return swept, nil
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
	if err := s.repo.InsertMovementTx(ctx, tx, infrastructure.MovementRow{
		BranchID: locked.BranchID, ProductID: locked.ProductID, VariantID: locked.VariantID, LocationID: locked.LocationID,
		Direction: "out", MovementType: "out", Qty: qty, Before: bal.OnHand, After: bal.OnHand - qty,
		RefType: refType, RefID: refID, ActorID: actorID}); err != nil {
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

// --- locations --------------------------------------------------------------

// LocationInput is the location create/update payload.
type LocationInput struct {
	Code     string
	Name     string
	ParentID *int64
	// Attributes (ST-01). On create nil = off / 0; on update nil = unchanged.
	IsPickingArea *bool
	IsPrimary     *bool
	SortOrder     *int
	// ForceTransfer lets a location be placed under a parent that still holds
	// stock: the parent's stock is moved into the new/moved location in the
	// same transaction (a parent must end up empty — stock lives in leaves).
	ForceTransfer bool
}

// parentHasStockMessage marks the conflict the UI turns into a confirmation.
const parentHasStockMessage = "PARENT_HAS_STOCK: lokasi induk masih memiliki stok; " +
	"kirim forceTransfer=true untuk memindahkan seluruh stok induk ke lokasi ini"

// guardParentStock rejects placing a location under a parent holding stock
// unless the caller asked to move that stock down.
func (s *Service) guardParentStock(ctx context.Context, parentID int64, force bool) (hasStock bool, err error) {
	has, err := s.repo.LocationHasBalances(ctx, parentID)
	if err != nil {
		return false, apperror.Internal(err)
	}
	if has && !force {
		return true, apperror.Conflict(parentHasStockMessage)
	}
	return has, nil
}

// relocateParentStockTx moves every nonzero position of parentID into childID
// inside tx, as one received transfer document (so the movements have a
// document, a number and an audit trail). Reserved stock blocks the move.
func (s *Service) relocateParentStockTx(ctx context.Context, tx *sql.Tx, actorID, branchID, parentID, childID int64, number string) error {
	balances, err := s.repo.BalancesByLocationTx(ctx, tx, parentID)
	if err != nil {
		return err
	}
	if len(balances) == 0 {
		return nil
	}
	items := make([]infrastructure.TransferItem, 0, len(balances))
	outLines := make([]contracts.PostLine, 0, len(balances))
	inLines := make([]contracts.PostLine, 0, len(balances))
	for _, b := range balances {
		if b.Reserved != 0 {
			return apperror.Conflict("Lokasi induk memiliki reservasi aktif; lepaskan dulu sebelum memindahkan stok")
		}
		if b.OnHand <= 0 {
			continue
		}
		items = append(items, infrastructure.TransferItem{ProductID: b.ProductID, VariantID: b.VariantID, Qty: b.OnHand})
		outLines = append(outLines, contracts.PostLine{ProductID: b.ProductID, VariantID: b.VariantID, LocationID: parentID, Qty: b.OnHand})
		inLines = append(inLines, contracts.PostLine{ProductID: b.ProductID, VariantID: b.VariantID, LocationID: childID, Qty: b.OnHand})
	}
	if len(items) == 0 {
		return nil
	}
	out, err := mergeLines("out", outLines)
	if err != nil {
		return err
	}
	in, err := mergeLines("in", inLines)
	if err != nil {
		return err
	}
	docID, err := s.repo.CreateTransferTx(ctx, tx, &infrastructure.Transfer{
		Number: number, FromBranchID: branchID, ToBranchID: branchID,
		FromLocation: parentID, ToLocation: childID, Notes: "Pindah stok induk ke sublokasi", Items: items,
	}, "received", actorID)
	if err != nil {
		return err
	}
	ref := legRef{refType: contracts.RefTransfer, refID: docID, actorID: actorID, note: "Pindah " + number}
	_, err = s.applyTx(ctx, tx, ref, []leg{
		{branchID: branchID, direction: "out", lines: out},
		{branchID: branchID, direction: "in", lines: in},
	})
	return err
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
	moveStock := false
	if in.ParentID != nil {
		has, err := s.guardParentStock(ctx, *in.ParentID, in.ForceTransfer)
		if err != nil {
			return nil, err
		}
		moveStock = has
	}
	var number string
	if moveStock {
		n, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocStockTransfer)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		number = n
	}
	var id int64
	err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		newID, err := s.repo.CreateLocationTx(ctx, tx, &contracts.Location{
			BranchID: branchID, Code: code, Name: strings.TrimSpace(in.Name), ParentID: in.ParentID,
			IsPickingArea: in.IsPickingArea != nil && *in.IsPickingArea,
			IsPrimary:     in.IsPrimary != nil && *in.IsPrimary,
			SortOrder:     intOr(in.SortOrder, 0),
		}, actorID)
		if err != nil {
			return err
		}
		id = newID
		if moveStock {
			return s.relocateParentStockTx(ctx, tx, actorID, branchID, *in.ParentID, newID, number)
		}
		return nil
	})
	if err != nil {
		return nil, mapLocationErr(err)
	}
	loc, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_locations.create", Entity: "stock_location", EntityID: loc.ID, BranchID: branchID, ActorID: actorID, Note: loc.Code})
	return loc, nil
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// UpdateLocation rewrites code/name/parent/attributes (never into its own
// subtree). A new code is refused when another location of the branch already
// holds it (archived ones included: the unique key spans them); the change is
// audited with the old and new code. System locations keep their identity
// (code/name/parent) but the default one may still carry the picking attributes.
func (s *Service) UpdateLocation(ctx context.Context, actorID, branchID, id int64, in LocationInput) (*contracts.Location, error) {
	existing, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil || existing.BranchID != branchID {
		return nil, apperror.NotFound("Lokasi")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	before := map[string]any{"code": existing.Code, "name": existing.Name, "isPickingArea": existing.IsPickingArea,
		"isPrimary": existing.IsPrimary, "sortOrder": existing.SortOrder}
	newCode := existing.Code
	if c := strings.ToUpper(strings.TrimSpace(in.Code)); c != "" {
		newCode = c
	}
	if existing.IsSystem {
		sameParent := (in.ParentID == nil && existing.ParentID == nil) ||
			(in.ParentID != nil && existing.ParentID != nil && *in.ParentID == *existing.ParentID)
		if newCode != existing.Code || strings.TrimSpace(in.Name) != existing.Name || !sameParent {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "lokasi sistem tidak dapat diubah"}})
		}
		if existing.Kind == contracts.KindDamaged && (in.IsPickingArea != nil || in.IsPrimary != nil || in.SortOrder != nil) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "isPickingArea", Message: "lokasi stok rusak tidak ikut alokasi"}})
		}
	}
	if newCode != existing.Code {
		if len(newCode) > 30 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "maksimal 30 karakter"}})
		}
		if dup, err := s.repo.LocationByCode(ctx, branchID, newCode); err != nil {
			return nil, apperror.Internal(err)
		} else if dup != nil && dup.ID != id {
			return nil, apperror.Conflict("Kode lokasi '" + newCode + "' sudah digunakan")
		}
		existing.Code = newCode
	}
	if in.IsPickingArea != nil {
		existing.IsPickingArea = *in.IsPickingArea
	}
	if in.IsPrimary != nil {
		existing.IsPrimary = *in.IsPrimary
	}
	if in.SortOrder != nil {
		existing.SortOrder = *in.SortOrder
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
	movedUnder := in.ParentID != nil && (existing.ParentID == nil || *existing.ParentID != *in.ParentID)
	existing.ParentID = in.ParentID
	moveStock := false
	if movedUnder {
		has, err := s.guardParentStock(ctx, *in.ParentID, in.ForceTransfer)
		if err != nil {
			return nil, err
		}
		moveStock = has
	}
	var number string
	if moveStock {
		n, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocStockTransfer)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		number = n
	}
	if err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateLocationTx(ctx, tx, existing, actorID); err != nil {
			return err
		}
		if moveStock {
			return s.relocateParentStockTx(ctx, tx, actorID, branchID, *in.ParentID, id, number)
		}
		return nil
	}); err != nil {
		return nil, mapLocationErr(err)
	}
	updated, err := s.repo.LocationByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_locations.update", Entity: "stock_location", EntityID: updated.ID, BranchID: existing.BranchID, ActorID: actorID, Note: updated.Code,
		Before: before, After: map[string]any{"code": updated.Code, "name": updated.Name, "isPickingArea": updated.IsPickingArea,
			"isPrimary": updated.IsPrimary, "sortOrder": updated.SortOrder}})
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

// AuthorizeBranch lets a request address a branch other than its session
// branch (K-06: transfer destination pickers). The session branch is always
// allowed; any other needs a live branch-access link for the actor, else
// Forbidden. The check reads the user's access each time, so a revoked link
// takes effect immediately and the request body/query is never trusted.
func (s *Service) AuthorizeBranch(ctx context.Context, actorID, sessionBranchID, targetBranchID int64) error {
	if targetBranchID == sessionBranchID {
		return nil
	}
	access, err := s.users.GetBranchAccess(ctx, actorID)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, a := range access {
		if a.BranchID == targetBranchID {
			return nil
		}
	}
	return apperror.Forbidden()
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

// BalanceQuery narrows the branch-wide balance list.
type BalanceQuery struct {
	ProductID      int64
	LocationID     int64
	Search         string // product code or name fragment
	CriticalOnly   bool   // only products at or below their minimum stock
	Negative       bool
	IncludeDamaged bool
	Page, Limit    int
}

// BalanceRow is one listed position, decorated with the product identity.
type BalanceRow struct {
	BalanceView
	ProductCode string `json:"productCode"`
	ProductName string `json:"productName"`
}

// ListBalances pages the branch-wide positions. Text and critical filters are
// resolved through the product contract (never a cross-module join).
func (s *Service) ListBalances(ctx context.Context, branchID int64, q BalanceQuery) ([]BalanceRow, int64, error) {
	f := infrastructure.BalanceFilter{ProductID: q.ProductID, LocationID: q.LocationID,
		Negative: q.Negative, IncludeDamaged: q.IncludeDamaged, Page: q.Page, Limit: q.Limit}
	var restrict map[int64]bool
	narrow := func(ids []int64) {
		set := map[int64]bool{}
		for _, id := range ids {
			if restrict == nil || restrict[id] {
				set[id] = true
			}
		}
		restrict = set
	}
	if strings.TrimSpace(q.Search) != "" {
		ids, err := s.products.SearchIDs(ctx, q.Search, 200)
		if err != nil {
			return nil, 0, apperror.Internal(err)
		}
		narrow(ids)
	}
	if q.CriticalOnly {
		stocked, err := s.products.ListStocked(ctx)
		if err != nil {
			return nil, 0, apperror.Internal(err)
		}
		avail, err := s.repo.ProductAvailability(ctx, branchID)
		if err != nil {
			return nil, 0, apperror.Internal(err)
		}
		var ids []int64
		for _, p := range stocked {
			if avail[p.ID] <= p.MinStock {
				ids = append(ids, p.ID)
			}
		}
		narrow(ids)
	}
	if restrict != nil {
		f.RestrictIDs = true
		for id := range restrict {
			f.ProductIDs = append(f.ProductIDs, id)
		}
	}
	list, total, err := s.repo.ListBalances(ctx, branchID, f)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	ids := make([]int64, 0, len(list))
	for _, b := range list {
		ids = append(ids, b.ProductID)
	}
	labels, err := s.products.LabelsByIDs(ctx, ids)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	out := make([]BalanceRow, 0, len(list))
	for _, b := range list {
		l := labels[b.ProductID]
		out = append(out, BalanceRow{
			BalanceView: BalanceView{BranchID: b.BranchID, ProductID: b.ProductID, VariantID: b.VariantID,
				LocationID: b.LocationID, OnHand: b.OnHand, Reserved: b.Reserved, Available: b.Available()},
			ProductCode: l.Code, ProductName: l.Name})
	}
	return out, total, nil
}

// ScanResult is the scan-product answer: identity plus live positions.
// Status is ok, variant_required (the code names a product with several
// variants: pick one from Variants) or inactive (known but not sellable).
type ScanResult struct {
	Status   string        `json:"status"`
	Via      string        `json:"via,omitempty"`
	Product  any           `json:"product"`
	Variant  any           `json:"variant"`
	Variants []any         `json:"variants,omitempty"`
	Balances []BalanceView `json:"balances"`
}

// Scan resolves a barcode, variant code or product code to variant + product
// + branch positions.
func (s *Service) Scan(ctx context.Context, branchID int64, barcode string) (*ScanResult, error) {
	m, err := s.products.ResolveScanCode(ctx, barcode)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if m == nil || m.Product == nil {
		return nil, apperror.NotFound("Produk")
	}
	p := m.Product
	res := &ScanResult{Status: "ok", Via: m.Via, Balances: []BalanceView{},
		Product: map[string]any{"id": p.ID, "code": p.Code, "name": p.Name}}
	switch {
	case m.Inactive:
		res.Status = "inactive"
		return res, nil
	case m.VariantRequired:
		res.Status = "variant_required"
		for _, v := range p.Variants {
			if !v.IsHidden {
				res.Variants = append(res.Variants, map[string]any{"id": v.ID, "code": v.Code, "name": v.Name, "barcode": v.Barcode})
			}
		}
		return res, nil
	}
	v := m.Variant
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
	res.Variant = map[string]any{"id": v.ID, "code": v.Code, "name": v.Name}
	res.Balances = views
	return res, nil
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

// StockReport pages the tracked products with their totals. The catalog read
// and the balance totals are two grouped reads; filtering and paging happen
// over the (bounded) tracked catalog in memory.
func (s *Service) StockReport(ctx context.Context, branchID int64, criticalOnly bool, page, limit int) (*contracts.StockReport, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}
	stocked, err := s.products.ListStocked(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	totals, err := s.repo.ProductTotals(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows := make([]contracts.StockReportRow, 0, len(stocked))
	for _, p := range stocked {
		t := totals[p.ID]
		avail := t[0] - t[1]
		critical := avail <= p.MinStock
		if criticalOnly && !critical {
			continue
		}
		rows = append(rows, contracts.StockReportRow{ProductID: p.ID, ProductCode: p.Code, ProductName: p.Name,
			OnHand: t[0], Reserved: t[1], Available: avail, MinStock: p.MinStock, Critical: critical})
	}
	total := int64(len(rows))
	start := (page - 1) * limit
	if start > len(rows) {
		start = len(rows)
	}
	end := start + limit
	if end > len(rows) {
		end = len(rows)
	}
	return &contracts.StockReport{Rows: rows[start:end], Total: total}, nil
}

// CriticalSummary counts critical and tracked products without naming them.
func (s *Service) CriticalSummary(ctx context.Context, branchID int64) (int64, int64, error) {
	avail, err := s.repo.ProductAvailability(ctx, branchID)
	if err != nil {
		return 0, 0, apperror.Internal(err)
	}
	var critical, tracked int64
	var after int64
	for {
		page, err := s.products.ListStockedAfter(ctx, after, 2000)
		if err != nil {
			return 0, 0, apperror.Internal(err)
		}
		for _, p := range page {
			tracked++
			if avail[p.ID] <= p.MinStock {
				critical++
			}
		}
		if len(page) < 2000 {
			break
		}
		after = page[len(page)-1].ID
	}
	return critical, tracked, nil
}

// Allocate implements the contract: the same picking order as Suggest.
func (s *Service) Allocate(ctx context.Context, branchID, productID, variantID int64, qty float64) ([]contracts.Suggestion, float64, error) {
	return s.Suggest(ctx, branchID, productID, variantID, qty)
}

// mapLocationErr keeps typed errors and maps duplicate-key races to conflicts.
func mapLocationErr(err error) error {
	var ae *apperror.AppError
	if errors.As(err, &ae) {
		return ae
	}
	return dberr.Map(err)
}
