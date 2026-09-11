package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Statuses: draft → confirmed (stock back out). Cancel voids drafts and
// reverses confirmed notes.
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// Item is one return line, valued at the PO snapshot unit price.
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

// Settlement types record HOW a return's value was settled in the real
// world. Memo-level (like legacy and the sales side): money itself still
// moves through the payment module.
const (
	SettleCollectPayment   = "collect_payment"
	SettleReduceReceivable = "reduce_receivable"
	SettleRefund           = "refund"
	SettleCustomerCredit   = "customer_credit"
)

// PurchaseReturn is one supplier return against a PO.
type PurchaseReturn struct {
	ID              int64
	Number          string
	BranchID        int64
	PurchaseOrderID int64
	OrderNumber     string
	ReturnDate      string
	Subtotal        int64
	DiscountTotal   int64
	TaxTotal        int64
	TaxType         string
	TaxRate         float64
	Total           int64
	Status          string
	Notes           string
	Items           []*Item
	Settlements     []*Settlement
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

// PurchaseReturnClient is the public surface of the purchasereturn module.
type PurchaseReturnClient interface {
	// GetByID resolves a return with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*PurchaseReturn, error)
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
		{Code: "purchasereturn.view", Name: "Retur Beli: Lihat"},
		{Code: "purchasereturn.create", Name: "Retur Beli: Tambah"},
		{Code: "purchasereturn.archive", Name: "Retur Beli: Batalkan"},
	}
}
