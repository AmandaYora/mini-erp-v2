package contracts

import (
	"context"
	"time"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Location is a warehouse/bin. Movements only ever touch leaf locations
// (no active children). System locations (default + damaged, auto-created
// per branch) cannot be archived.
type Location struct {
	ID       int64
	BranchID int64
	Code     string
	Name     string
	ParentID *int64
	IsSystem bool
	// Kind is "normal" or "damaged". Damaged stock never counts as available
	// and cannot be sold from, received into or suggested for allocation.
	Kind   string
	Status string // active | archived
	// IsPickingArea, IsPrimary and SortOrder are the warehouse attributes that
	// order stock allocation (ST-01): picking area first, then the primary
	// location, then the branch default (the system "normal" location), then the
	// largest available quantity, then SortOrder, then id.
	IsPickingArea bool
	IsPrimary     bool
	SortOrder     int
}

// KindDamaged marks the branch's damaged-goods location.
const KindDamaged = "damaged"

// Balance is one stock position. Available = OnHand − Reserved, always
// derived, never stored.
type Balance struct {
	BranchID   int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	OnHand     float64
	Reserved   float64
}

// Available returns the sellable quantity.
func (b Balance) Available() float64 {
	return b.OnHand - b.Reserved
}

// Suggestion is one allocation line (FIFO-ish across locations).
type Suggestion struct {
	LocationID int64   `json:"locationId"`
	Qty        float64 `json:"qty"`
}

// Standard ref_type values. A reversal uses RefType + "_cancel".
const (
	RefDelivery       = "delivery"
	RefGoodsReceipt   = "goods_receipt"
	RefSalesReturn    = "sales_return"
	RefPurchaseReturn = "purchase_return"
	RefTransfer       = "transfer"
	RefAdjustment     = "adjustment"
	RefDamaged        = "damaged"
	// RefDamagedArrival is goods that arrived damaged with no source position
	// (ST-06): a lone inflow, journaled like a stock surplus.
	RefDamagedArrival = "damaged_arrival"
	RefWriteOff       = "write_off"
	RefOpening        = "opening"

	CancelSuffix = "_cancel"
)

// PostLine is one position of a document posting.
type PostLine struct {
	ProductID, VariantID, LocationID int64
	// Qty is the base-unit quantity (> 0). Lines on the same position are summed.
	Qty float64
	// UnitCost is an explicit cost per base unit for an inflow (opening
	// balances with a known cost). 0 = unknown: finance falls back through
	// its cost chain.
	UnitCost float64
	// ReservationKey (out only) consumes that hold inside the same transaction.
	ReservationKey string
}

// PostRequest posts every line of one document atomically and idempotently:
// the same (RefType, RefID, position, direction) is never written twice.
type PostRequest struct {
	BranchID  int64
	Direction string // "in" | "out"
	RefType   string
	RefID     int64 // required > 0
	ActorID   int64
	MovedAt   time.Time // zero = now; effective time (backdated documents)
	Note      string
	// AllowDamagedLocation lets lines target the branch damaged location
	// (returns of damaged goods). Never set for sales or receipts.
	AllowDamagedLocation bool
	Lines                []PostLine
}

// PostResult reports how many positions were written vs already present.
type PostResult struct{ Posted, Replayed int }

// StockClient is the public surface of the stock module. Every balance
// change the stock module makes is paired with a movement row (contract §7.5).
type StockClient interface {
	// GetBalance resolves a position; missing rows read as zero, never error.
	GetBalance(ctx context.Context, branchID, productID, variantID, locationID int64) (*Balance, error)
	// Hold reserves qty under key until ttlMinutes pass. Returns the
	// reservation id. Over-holds are rejected, never partial.
	Hold(ctx context.Context, actorID, branchID, productID, variantID, locationID int64, qty float64, key string, ttlMinutes int) (int64, error)
	// Release frees a reservation by key (idempotent: unknown keys succeed).
	// branchID > 0 scopes it to that branch; 0 = internal, unscoped.
	Release(ctx context.Context, actorID, branchID int64, key string) error
	// Post writes a whole document's movements in one transaction, ordered
	// by position (deadlock-safe), retried on 1213/1205, idempotent per
	// document. A replay is a successful no-op (PostResult.Replayed).
	Post(ctx context.Context, req PostRequest) (*PostResult, error)
	// Reverse undoes every movement of a document once, writing opposite
	// movements under RefType+"_cancel". Idempotent.
	Reverse(ctx context.Context, branchID int64, refType string, refID, actorID int64) (*PostResult, error)
	// DamagedLocation returns the branch damaged-goods location, creating it
	// when missing.
	DamagedLocation(ctx context.Context, branchID int64) (*Location, error)
	// HasMovements reports whether the document wrote any movement.
	HasMovements(ctx context.Context, branchID int64, refType string, refID int64) (bool, error)
	// StockReport lists the tracked products with their total on hand, reserved
	// and available (damaged and inactive locations excluded), optionally only
	// those at or below their minimum, paginated with an exact total.
	StockReport(ctx context.Context, branchID int64, criticalOnly bool, page, limit int) (*StockReport, error)
	// CriticalSummary counts the tracked products at or below their minimum
	// stock and the tracked products overall (dashboard widgets).
	CriticalSummary(ctx context.Context, branchID int64) (critical, tracked int64, err error)
	// Allocate splits qty (base units) over the branch's pickable positions,
	// oldest location first, returning what could not be covered as shortfall.
	// Damaged locations are never suggested.
	Allocate(ctx context.Context, branchID, productID, variantID int64, qty float64) (lines []Suggestion, shortfall float64, err error)
	// CheckLocation validates a posting target (exists, active, leaf, same
	// branch) without writing. Callers pre-validate locations before
	// recording documents so stock failures never strand a document row.
	CheckLocation(ctx context.Context, branchID, locationID int64) error
	// ListLocations lists a branch's locations (return context pickers).
	ListLocations(ctx context.Context, branchID int64, status string) ([]*Location, error)
	// ListRecentCostMovements returns movements written in the last `minutes`
	// minutes, ordered by id. Costing uses it to pick up a lower id that
	// committed after a higher one was already ingested (cursor gaps).
	ListRecentCostMovements(ctx context.Context, branchID int64, minutes, limit int) ([]*CostMovement, error)
	// ListCostMovements streams movement rows after an id (ordered, capped)
	// for HPP costing. The finance module resumes from its own cursor.
	ListCostMovements(ctx context.Context, branchID, afterID int64, limit int) ([]*CostMovement, error)
	// TransferSourceBranch returns the sending branch of a transfer document
	// (0 when unknown), so costing can price the received leg at the sent one.
	TransferSourceBranch(ctx context.Context, transferID int64) (int64, error)
	// CriticalStock lists tracked products whose total available stock is at
	// or below minimum, scarcest first (assistant bot tool). Capped.
	CriticalStock(ctx context.Context, branchID int64, limit int) ([]*CriticalItem, error)
}

// CriticalItem is one product at or below its minimum stock level.
type CriticalItem struct {
	ProductID   int64   `json:"productId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Available   float64 `json:"available"`
	MinStock    float64 `json:"minStock"`
}

// CostMovement is one stock movement row for HPP costing.
type CostMovement struct {
	ID        int64
	BranchID  int64
	ProductID int64
	VariantID int64
	Direction string // in | out
	RefType   string
	RefID     int64
	QtyBase   float64
	// UnitCost is the explicit cost hint recorded with the movement (0 = none).
	UnitCost  float64
	CreatedAt string
	// MovedAt is the effective (accounting) time of the movement.
	MovedAt string
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "stock.view", Name: "Stok: Lihat"},
		{Code: "stock.update", Name: "Stok: Kelola Lokasi"},
		{Code: "stock.transfer", Name: "Stok: Transfer & Pindah Lokasi"},
		{Code: "stock.adjust", Name: "Stok: Koreksi"},
		{Code: "stock.approve", Name: "Stok: Setujui Koreksi"},
		{Code: "stock.reserve", Name: "Stok: Tahan (Keranjang POS)"},
	}
}
