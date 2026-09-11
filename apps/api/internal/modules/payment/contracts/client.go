package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Directions: in = customer pays us, out = we pay a supplier.
const (
	DirectionIn  = "in"
	DirectionOut = "out"
)

// Order types an allocation may point at. Return types settle refunds and
// credit offsets; normal types settle bills.
const (
	OrderPurchase       = "purchase"
	OrderSales          = "sales"
	OrderSalesReturn    = "sales_return"
	OrderPurchaseReturn = "purchase_return"
)

// Allocation links part of a payment to one document.
type Allocation struct {
	OrderType   string
	OrderID     int64
	OrderNumber string
	Amount      int64
}

// Payment is one money movement with its allocations.
type Payment struct {
	ID          int64
	Number      string
	BranchID    int64
	PartyID     int64
	PartyName   string
	PartyType   string
	Direction   string
	Amount      int64
	Method      string
	PaidAt      string
	Notes       string
	Status      string
	Allocations []*Allocation
}

// OrderBalance is one document's outstanding position.
type OrderBalance struct {
	OrderType   string `json:"orderType"`
	OrderID     int64  `json:"orderId"`
	Number      string `json:"number"`
	GrandTotal  int64  `json:"grandTotal"`
	Paid        int64  `json:"paid"`
	Outstanding int64  `json:"outstanding"`
}

// PartyBalance is the full position of one party. Normal orders bill one
// way, confirmed returns bill the other; Outstanding is the net.
type PartyBalance struct {
	PartyID       int64          `json:"partyId"`
	PartyName     string         `json:"partyName"`
	TotalBilled   int64          `json:"totalBilled"`
	TotalPaid     int64          `json:"totalPaid"`
	TotalReturned int64          `json:"totalReturned"`
	TotalRefunded int64          `json:"totalRefunded"`
	Outstanding   int64          `json:"outstanding"`
	Orders        []OrderBalance `json:"orders"`
	Returns       []OrderBalance `json:"returns"`
}

// PaymentClient is the public surface of the payment module.
type PaymentClient interface {
	// GetByID resolves a payment with allocations, or nil when missing.
	GetByID(ctx context.Context, id int64) (*Payment, error)
	// GetPartyBalance computes the live outstanding position. Cancelled
	// payments and cancelled orders never count.
	GetPartyBalance(ctx context.Context, branchID, partyID int64) (*PartyBalance, error)
	// HasActiveAllocations reports whether any active payment allocates to
	// the document. Returns modules use it to block cancelling a refunded
	// return (money would silently detach from reports).
	HasActiveAllocations(ctx context.Context, orderType string, orderID int64) (bool, error)
	// ListActive lists active payments newest-first, bounded by limit.
	// Finance reads it for the derived posting queue (no posting table).
	ListActive(ctx context.Context, branchID int64, limit int) ([]*Payment, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "payment.view", Name: "Pembayaran: Lihat"},
		{Code: "payment.create", Name: "Pembayaran: Tambah"},
		{Code: "payment.archive", Name: "Pembayaran: Batalkan"},
	}
}
