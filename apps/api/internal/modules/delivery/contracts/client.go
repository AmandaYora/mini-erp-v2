package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Statuses: draft → confirmed (stock out) → completed? No — confirmed is
// final for stock; cancelled voids drafts or reverses confirmed docs.
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// Item is one SJ line (location chosen at SJ time).
type Item struct {
	ID         int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	UOMFactor  float64
	Qty        float64
	QtyBase    float64
}

// DeliveryNote is one surat jalan against an SO.
type DeliveryNote struct {
	ID                              int64
	Number                          string
	BranchID                        int64
	SalesOrderID                    int64
	OrderNumber                     string
	DeliveryDate                    string
	DriverName                      string
	VehiclePlate                    string
	WarehouseStaffName              string
	RecipientName                   string
	RecipientSignatureStatus        string
	RecipientSignatureMissingReason string
	DropLocationNote                string
	DispatchedAt                    string
	DispatchedBy                    int64
	ConfirmedAt                     string
	ConfirmedBy                     int64
	// DocumentKind distinguishes order shipments from replacement
	// shipments for exchanges (D3); SalesReturnID links the latter.
	DocumentKind  string
	SalesReturnID int64
	Status        string
	Notes         string
	Items         []*Item
}

// Document kinds for delivery notes.
const (
	DocumentKindOrder       = "order"
	DocumentKindReplacement = "replacement"
)

// ReplacementLine is one replacement-goods line (same shape as an SJ
// line; priced downstream by finance at moving average, not stored here).
type ReplacementLine struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
}

// ReplacementDraft carries everything needed to draft a replacement
// shipment. SalesOrderID keeps the original order as context (it does NOT
// count toward delivered qty — DeliveredQty filters kind='order').
type ReplacementDraft struct {
	SalesOrderID   int64
	SalesReturnID  int64
	DeliveryDate   string
	Notes          string
	DriverName     string
	VehiclePlate   string
	WarehouseStaff string
	DropNote       string
	Items          []ReplacementLine
}

// ReplacementConfirm carries receipt evidence for a replacement shipment.
type ReplacementConfirm struct {
	RecipientName            string
	RecipientSignatureStatus string
	MissingReason            string
}

// DeliveryClient is the public surface of the delivery module.
type DeliveryClient interface {
	// GetByID resolves a note with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*DeliveryNote, error)
	// DeliveredQty sums confirmed-delivered base qty for one SO line
	// position. Returns cap the source of every sales return.
	DeliveredQty(ctx context.Context, soID, productID, variantID int64) (float64, error)
	// CreateReplacement drafts a replacement shipment for an exchange
	// return. No SO-status check: the order is already completed.
	CreateReplacement(ctx context.Context, actorID, branchID int64, in ReplacementDraft) (*DeliveryNote, error)
	// ConfirmReplacement confirms a draft replacement shipment (stock out).
	ConfirmReplacement(ctx context.Context, actorID, branchID, id int64, in ReplacementConfirm) (*DeliveryNote, error)
	// ListConfirmed lists confirmed notes newest-first, bounded by limit.
	// Finance reads it for the derived posting queue (no posting table).
	ListConfirmed(ctx context.Context, branchID int64, limit int) ([]*DeliveryNote, error)
	// OrderIDsByNotes maps note id -> sales order id in one grouped read.
	// Finance uses it to attribute journal entries to the economic
	// transaction behind them; a per-note GetByID loop would be the N+1
	// this exists to avoid.
	OrderIDsByNotes(ctx context.Context, ids []int64) (map[int64]int64, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "delivery.view", Name: "Pengiriman: Lihat"},
		{Code: "delivery.create", Name: "Pengiriman: Tambah"},
		{Code: "delivery.archive", Name: "Pengiriman: Batalkan"},
	}
}
