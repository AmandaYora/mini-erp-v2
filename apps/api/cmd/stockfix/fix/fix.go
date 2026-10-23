// Package fix neutralizes stock adjustments proven to be wrong (temuan 6:
// corrections a user made because a POS hold bug hid stock) with a counter
// adjustment through the stock application service — the same path as
// Stok → Koreksi — so the stock card, cost ledger, audit trail and the daily
// close journal stay consistent. Rows are never edited or deleted.
//
// Each line names the movement it neutralizes (refMovementId). The counter
// adjustment carries the marker "[stockfix:<refMovementId>]" in its reason,
// which is how a later run recognises a line it already applied.
//
// It lives under cmd/ (a composition root) because it drives the stock
// service directly; the command in ../main.go is a thin wrapper.
package fix

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	productcontracts "mini-erp/internal/modules/product/contracts"
	stockapp "mini-erp/internal/modules/stock/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// Line is one counter adjustment to make. Product, variant and location may be
// omitted: they then come from the referenced movement (and must match it when
// given).
type Line struct {
	ProductCode   string  `json:"productCode"`
	ProductID     int64   `json:"productId"`
	VariantID     int64   `json:"variantId"`
	LocationID    int64   `json:"locationId"`
	Direction     string  `json:"direction"` // in | out
	Qty           float64 `json:"qty"`
	Reason        string  `json:"reason"`
	RefMovementID int64   `json:"refMovementId"`
}

// Options select the branch, the acting user and the approver.
type Options struct {
	BranchID int64
	// Actor is the username recorded on the movement and audit entry: an
	// active user with stock.adjust and access to the branch.
	Actor string
	// Approver/ApproverPassword are verified by the stock service exactly as
	// for an adjustment made in the web app (strict approval is the default).
	Approver         string
	ApproverPassword string
	Commit           bool
}

// Deps are the services the tool drives (production wiring via bootstrap).
type Deps struct {
	DB       *sql.DB
	Stock    *stockapp.Service
	Products productcontracts.ProductClient
	Users    usercontracts.UserClient
	Roles    usercontracts.RoleClient
	// Sleep waits between retries when the approver rate limit trips (the
	// service allows 5 approvals a minute per actor+approver); nil = time.Sleep.
	Sleep func(time.Duration)
}

// Status of one line in the report.
const (
	StatusPlanned  = "rencana"
	StatusApplied  = "diterapkan"
	StatusDone     = "sudah dinetralkan"
	StatusRejected = "ditolak"
)

// Result reports one line.
type Result struct {
	Line          int     `json:"line"`
	RefMovementID int64   `json:"refMovementId"`
	RefSummary    string  `json:"ref"`
	ProductID     int64   `json:"productId"`
	ProductCode   string  `json:"productCode"`
	ProductName   string  `json:"productName"`
	VariantID     int64   `json:"variantId"`
	LocationID    int64   `json:"locationId"`
	Direction     string  `json:"direction"`
	Qty           float64 `json:"qty"`
	OnHandBefore  float64 `json:"onHandBefore"`
	OnHandAfter   float64 `json:"onHandAfter"`
	Reserved      float64 `json:"reserved"`
	Reason        string  `json:"reason"`
	Status        string  `json:"status"`
	Problem       string  `json:"problem,omitempty"`
}

// Report is the archivable outcome of a run.
type Report struct {
	Commit   bool      `json:"commit"`
	BranchID int64     `json:"branchId"`
	Actor    string    `json:"actor"`
	Approver string    `json:"approver,omitempty"`
	At       time.Time `json:"at"`
	Lines    []Result  `json:"lines"`
	Planned  int       `json:"planned"`
	Applied  int       `json:"applied"`
	Done     int       `json:"alreadyDone"`
	Rejected int       `json:"rejected"`
}

// Marker is the tag a counter adjustment carries for refMovementID.
func Marker(refMovementID int64) string { return fmt.Sprintf("[stockfix:%d]", refMovementID) }

type refMovement struct {
	branchID, productID, variantID, locationID int64
	direction, movementType                    string
	qty                                        float64
	movedAt                                    time.Time
}

type posKey struct{ product, variant, location int64 }

// ErrRejected is returned when validation refused at least one line; nothing
// is written then, even with Commit.
var ErrRejected = errors.New("ada baris yang ditolak; tidak ada yang ditulis")

// Run validates every line (a dry run writes nothing) and, with Commit and no
// rejected line, applies the counter adjustments one by one.
func Run(ctx context.Context, d Deps, o Options, lines []Line) (*Report, error) {
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	rep := &Report{Commit: o.Commit, BranchID: o.BranchID, Actor: o.Actor, Approver: o.Approver, At: time.Now()}
	if o.BranchID <= 0 {
		return rep, errors.New("--branch-id wajib")
	}
	actor, err := checkActor(ctx, d, o)
	if err != nil {
		return rep, err
	}
	positions := map[posKey]*struct{ onHand, reserved float64 }{}
	seenRef := map[int64]int{}
	for i, ln := range lines {
		r := Result{Line: i + 1, RefMovementID: ln.RefMovementID, Direction: strings.ToLower(strings.TrimSpace(ln.Direction)), Qty: ln.Qty}
		reject := func(format string, a ...any) {
			r.Status, r.Problem = StatusRejected, fmt.Sprintf(format, a...)
		}
		r.Reason = strings.TrimSpace(ln.Reason)
		switch {
		case ln.RefMovementID <= 0:
			reject("refMovementId wajib")
		case seenRef[ln.RefMovementID] > 0:
			reject("refMovementId %d sudah dipakai di baris %d", ln.RefMovementID, seenRef[ln.RefMovementID])
		case r.Direction != "in" && r.Direction != "out":
			reject("direction harus in atau out")
		case !(ln.Qty > 0) || math.IsInf(ln.Qty, 0):
			reject("qty harus lebih dari 0")
		case r.Reason == "":
			reject("reason wajib diisi")
		}
		if r.Status == "" {
			seenRef[ln.RefMovementID] = i + 1
			r.Reason = r.Reason + " " + Marker(ln.RefMovementID)
			if len(r.Reason) > 500 {
				reject("reason terlalu panjang (maks. 500 karakter termasuk penanda)")
			}
		}
		if r.Status == "" {
			resolveLine(ctx, d, o, ln, &r, reject)
		}
		done := false
		if r.Status == "" {
			if done, err = alreadyApplied(ctx, d.DB, o.BranchID, ln.RefMovementID); err != nil {
				return rep, err
			}
		}
		if r.Status == "" {
			k := posKey{r.ProductID, r.VariantID, r.LocationID}
			p := positions[k]
			if p == nil {
				p = &struct{ onHand, reserved float64 }{}
				bals, err := d.Stock.BalancesByProduct(ctx, o.BranchID, r.ProductID)
				if err != nil {
					return rep, err
				}
				for _, b := range bals {
					if b.VariantID == r.VariantID && b.LocationID == r.LocationID {
						p.onHand, p.reserved = b.OnHand, b.Reserved
					}
				}
				positions[k] = p
			}
			delta := r.Qty
			if r.Direction == "out" {
				delta = -delta
			}
			r.OnHandBefore, r.Reserved = p.onHand, p.reserved
			r.OnHandAfter = p.onHand + delta
			switch {
			case done: // applied by an earlier run: shown at the current balance, never twice
				r.OnHandAfter, r.Status = p.onHand, StatusDone
			case r.OnHandAfter < -1e-9:
				reject("saldo sesudah negatif (%s)", num(r.OnHandAfter))
			case delta < 0 && p.onHand-p.reserved < -delta-1e-9:
				reject("stok tersedia %s (ditahan %s) kurang dari %s", num(p.onHand-p.reserved), num(p.reserved), num(-delta))
			default:
				p.onHand = r.OnHandAfter
				r.Status = StatusPlanned
			}
		}
		rep.Lines = append(rep.Lines, r)
	}
	count(rep)
	if rep.Rejected > 0 {
		return rep, ErrRejected
	}
	if !o.Commit {
		return rep, nil
	}
	for i := range rep.Lines {
		r := &rep.Lines[i]
		if r.Status != StatusPlanned {
			continue
		}
		// Re-checked right before writing: a concurrent run of the same file
		// must not neutralize a movement twice.
		done, err := alreadyApplied(ctx, d.DB, o.BranchID, r.RefMovementID)
		if err != nil {
			return rep, err
		}
		if done {
			r.Status = StatusDone
			continue
		}
		in := stockapp.AdjustInput{ProductID: r.ProductID, VariantID: r.VariantID, Location: r.LocationID,
			Mode: r.Direction, QtyDelta: r.Qty, Reason: r.Reason, Approver: o.Approver, ApproverP: o.ApproverPassword}
		err = adjustPaced(ctx, d, actor.ID, o.BranchID, in)
		if err != nil {
			r.Status, r.Problem = StatusRejected, err.Error()
			count(rep)
			return rep, fmt.Errorf("baris %d: %w (baris sebelumnya sudah diterapkan; jalankan ulang setelah diperbaiki, yang sudah diterapkan dilewati)", r.Line, err)
		}
		r.Status = StatusApplied
	}
	count(rep)
	return rep, nil
}

func checkActor(ctx context.Context, d Deps, o Options) (*usercontracts.User, error) {
	if strings.TrimSpace(o.Actor) == "" {
		return nil, errors.New("--actor wajib (username pengguna nyata)")
	}
	u, err := d.Users.GetByUsername(ctx, strings.TrimSpace(o.Actor))
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != "active" {
		return nil, fmt.Errorf("actor %q tidak ditemukan atau tidak aktif", o.Actor)
	}
	roleIDs, err := d.Users.GetRoleIDs(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	codes, err := d.Roles.GetPermissionCodes(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, c := range codes {
		allowed = allowed || c == "stock.adjust"
	}
	if !allowed {
		return nil, fmt.Errorf("actor %q tidak memegang izin stock.adjust", o.Actor)
	}
	access, err := d.Users.GetBranchAccess(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range access {
		if a.BranchID == o.BranchID {
			return u, nil
		}
	}
	return nil, fmt.Errorf("actor %q tidak punya akses ke cabang %d", o.Actor, o.BranchID)
}

// resolveLine checks the line against its referenced movement and fills the
// product, variant and location.
func resolveLine(ctx context.Context, d Deps, o Options, ln Line, r *Result, reject func(string, ...any)) {
	ref, err := loadRef(ctx, d.DB, ln.RefMovementID)
	if err != nil {
		reject("movement %d: %v", ln.RefMovementID, err)
		return
	}
	r.RefSummary = fmt.Sprintf("%s %s %s @%s", ref.movementType, ref.direction, num(ref.qty), ref.movedAt.In(jakarta).Format("2006-01-02 15:04"))
	switch {
	case ref.branchID != o.BranchID:
		reject("movement %d milik cabang %d, bukan %d", ln.RefMovementID, ref.branchID, o.BranchID)
	case ref.movementType != "adjustment":
		reject("movement %d bertipe %q; hanya koreksi stok (adjustment) yang dinetralkan", ln.RefMovementID, ref.movementType)
	case ref.direction == r.Direction:
		reject("arah %s sama dengan movement %d (%s); penyeimbang harus berlawanan", r.Direction, ln.RefMovementID, ref.direction)
	case ln.Qty > ref.qty+1e-9:
		reject("qty %s melebihi qty movement %d (%s)", num(ln.Qty), ln.RefMovementID, num(ref.qty))
	case ln.ProductID != 0 && ln.ProductID != ref.productID,
		ln.VariantID != 0 && ln.VariantID != ref.variantID,
		ln.LocationID != 0 && ln.LocationID != ref.locationID:
		reject("produk/varian/lokasi tidak cocok dengan movement %d (%d/%d/%d)", ln.RefMovementID, ref.productID, ref.variantID, ref.locationID)
	}
	if r.Status != "" {
		return
	}
	p, err := d.Products.GetByID(ctx, ref.productID)
	if err != nil || p == nil {
		reject("produk %d tidak ditemukan", ref.productID)
		return
	}
	if code := strings.TrimSpace(ln.ProductCode); code != "" && !strings.EqualFold(code, p.Code) {
		reject("productCode %s tidak cocok dengan movement %d (%s)", code, ln.RefMovementID, p.Code)
		return
	}
	r.ProductID, r.VariantID, r.LocationID = ref.productID, ref.variantID, ref.locationID
	r.ProductCode, r.ProductName = p.Code, p.Name
}

var jakarta = func() *time.Location {
	if l, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return l
	}
	return time.FixedZone("WIB", 7*3600)
}()

func loadRef(ctx context.Context, db *sql.DB, id int64) (*refMovement, error) {
	var m refMovement
	err := db.QueryRowContext(ctx, `SELECT branch_id, product_id, variant_id, location_id, direction, movement_type, qty_base, moved_at
		FROM stock_movements WHERE id = ?`, id).
		Scan(&m.branchID, &m.productID, &m.variantID, &m.locationID, &m.direction, &m.movementType, &m.qty, &m.movedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("tidak ditemukan")
	}
	return &m, err
}

// alreadyApplied: a counter adjustment for ref exists (its marker is in the
// movement notes, written from the adjustment reason).
func alreadyApplied(ctx context.Context, db *sql.DB, branchID, ref int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stock_movements
		WHERE branch_id = ? AND movement_type = 'adjustment' AND notes LIKE ?`, branchID, "%"+Marker(ref)+"%").Scan(&n)
	return n > 0, err
}

// adjustPaced calls the service and, when the approver rate limit trips,
// waits and retries instead of failing the batch.
func adjustPaced(ctx context.Context, d Deps, actorID, branchID int64, in stockapp.AdjustInput) error {
	for attempt := 0; ; attempt++ {
		err := d.Stock.Adjust(ctx, actorID, branchID, in)
		var ae *apperror.AppError
		if err == nil || !errors.As(err, &ae) || ae.Code != apperror.CodeRateLimited || attempt >= 8 {
			return err
		}
		d.Sleep(15 * time.Second)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func count(r *Report) {
	r.Planned, r.Applied, r.Done, r.Rejected = 0, 0, 0, 0
	for _, l := range r.Lines {
		switch l.Status {
		case StatusPlanned:
			r.Planned++
		case StatusApplied:
			r.Applied++
		case StatusDone:
			r.Done++
		case StatusRejected:
			r.Rejected++
		}
	}
}

func num(f float64) string {
	s := fmt.Sprintf("%.4f", f)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}
