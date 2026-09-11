package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Return modes: return_only refunds value; exchange ships replacement
// goods tracked by replacement_delivery_status.
const (
	ReturnModeReturnOnly = "return_only"
	ReturnModeExchange   = "exchange"
)

// Replacement delivery states for exchange returns.
const (
	ReplacementNotRequired = "not_required"
	ReplacementPending     = "pending"
	ReplacementDispatched  = "dispatched"
	ReplacementConfirmed   = "confirmed"
)

// Settlement types record HOW a return's value was settled in the real
// world. Memo-level (like legacy): money itself still moves through the
// payment module; rows here make the unsettled remainder visible.
const (
	SettleCollectPayment   = "collect_payment"
	SettleReduceReceivable = "reduce_receivable"
	SettleRefund           = "refund"
	SettleCustomerCredit   = "customer_credit"
)

// Statuses: draft → confirmed (stock back in). Cancel voids drafts and
// reverses confirmed notes (KI-99/KI-101: the cancel path exists and is
// permission-guarded, unlike legacy).
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// Item is one return line. Value follows the source order economics
// (D1): discounts pro-rate in, tax flips with it. tax_base is the
// post-discount base, so lineTotal = tax_base + tax_amount always.
type Item struct {
	ID              int64
	ProductID       int64
	VariantID       int64
	LocationID      int64
	UOM             string
	UOMFactor       float64
	Qty             float64
	QtyBase         float64
	UnitPrice       int64
	DiscountPct     float64
	DiscountNominal int64
	TaxBase         int64
	TaxAmount       int64
	LineTotal       int64
}

// SalesReturn is one customer return against an SO.
type SalesReturn struct {
	ID            int64
	Number        string
	BranchID      int64
	SalesOrderID  int64
	OrderNumber   string
	ReturnDate    string
	Subtotal      int64
	DiscountTotal int64
	TaxTotal      int64
	TaxType       string
	TaxRate       float64
	Total         int64
	Status        string
	Notes         string
	// Exchange mode (D): return-only refunds value; exchange additionally
	// ships replacement goods tracked below.
	ReturnMode                string
	ReplacementDeliveryStatus string
	Items                     []*Item
	ReplacementItems          []*ReplacementItem
	Settlements               []*Settlement
}

// ReplacementItem is one replacement-goods line of an exchange return.
// Priced once at creation from the live catalog (a new sale in effect);
// totals derive on read, never stored.
type ReplacementItem struct {
	ID         int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	UOMFactor  float64
	Qty        float64
	QtyBase    float64
	UnitPrice  int64
	LineTotal  int64
}

// Settlement is one memo row recording how return value was settled.
type Settlement struct {
	ID              int64
	Type            string
	Date            string
	Amount          int64
	PaymentMethod   string
	ReferenceNumber string
	Notes           string
}

// Return is the payment-facing projection (refund validation + outstanding).
type Return struct {
	ID      int64
	Number  string
	Branch  int64
	OrderID int64
	Total   int64
	Status  string
}

// SalesReturnClient is the public surface of the salesreturn module.
type SalesReturnClient interface {
	// GetByID resolves a return with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*SalesReturn, error)
	// ListByBranch lists the branch's returns (any status), newest first.
	// Finance reads it for the derived posting queue (no posting table).
	ListByBranch(ctx context.Context, branchID int64) ([]*Return, error)
}

// ReturnsClient is the narrow projection consumed by the payment module.
type ReturnsClient interface {
	// GetReturn resolves refund-relevant fields, or nil when missing.
	GetReturn(ctx context.Context, id int64) (*Return, error)
	// ListByBranch lists the branch's returns (any status), newest first.
	ListByBranch(ctx context.Context, branchID int64) ([]*Return, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "salesreturn.view", Name: "Retur Jual: Lihat"},
		{Code: "salesreturn.create", Name: "Retur Jual: Tambah"},
		{Code: "salesreturn.archive", Name: "Retur Jual: Batalkan"},
	}
}
