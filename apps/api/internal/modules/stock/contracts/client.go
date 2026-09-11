package contracts

import (
	"context"

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
	Status   string // active | archived
}

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

// StockClient is the public surface of the stock module. Every balance
// change the stock module makes is paired with a movement row (contract §7.5).
type StockClient interface {
	// GetBalance resolves a position; missing rows read as zero, never error.
	GetBalance(ctx context.Context, branchID, productID, variantID, locationID int64) (*Balance, error)
	// Hold reserves qty under key until ttlMinutes pass. Returns the
	// reservation id. Over-holds are rejected, never partial.
	Hold(ctx context.Context, actorID, branchID, productID, variantID, locationID int64, qty float64, key string, ttlMinutes int) (int64, error)
	// Release frees a reservation by key (idempotent: unknown keys succeed).
	Release(ctx context.Context, actorID int64, key string) error
	// Consume converts reserved qty into a sale outflow (partial allowed).
	Consume(ctx context.Context, key string, qty float64, refType string, refID int64, actorID int64) error
	// MoveIn records an inflow with its movement (receipts, openings, returns).
	MoveIn(ctx context.Context, branchID, productID, variantID, locationID int64, qty float64, refType string, refID int64, actorID int64) error
	// MoveOut records an outflow with its movement (deliveries, write-offs).
	MoveOut(ctx context.Context, branchID, productID, variantID, locationID int64, qty float64, refType string, refID int64, actorID int64) error
	// CheckLocation validates a posting target (exists, active, leaf, same
	// branch) without writing. Callers pre-validate locations before
	// recording documents so stock failures never strand a document row.
	CheckLocation(ctx context.Context, branchID, locationID int64) error
	// ListLocations lists a branch's locations (return context pickers).
	ListLocations(ctx context.Context, branchID int64, status string) ([]*Location, error)
	// ListCostMovements streams movement rows after an id (ordered, capped)
	// for HPP costing. The finance module resumes from its own cursor.
	ListCostMovements(ctx context.Context, branchID, afterID int64, limit int) ([]*CostMovement, error)
	// CriticalStock lists tracked products whose total available stock is at
	// or below minimum, scarcest first (assistant bot tool). Capped.
	CriticalStock(ctx context.Context, branchID int64, limit int) ([]*CriticalItem, error)
}

// CriticalItem is one product at or below its minimum stock level.
type CriticalItem struct {
	ProductID   int64
	ProductCode string
	ProductName string
	Available   float64
	MinStock    float64
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
	CreatedAt string
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "stock.view", Name: "Stok: Lihat"},
		{Code: "stock.manage", Name: "Stok: Kelola"},
		{Code: "stock.adjust", Name: "Stok: Koreksi"},
		{Code: "stock.approve", Name: "Stok: Setujui Koreksi"},
	}
}
