package contracts

import (
	"context"
	"strconv"

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

// Payment methods. "offset" is internal (credit offsets), never user-chosen.
const (
	MethodCash     = "cash"
	MethodTransfer = "transfer"
	MethodCheck    = "check"
	MethodOffset   = "offset"
	// MethodAdvance is the bill leg of a saldo (uang muka) use (N-41): no cash
	// moves, the party's advance account pays the bill. Internal, never
	// user-chosen on a plain payment.
	MethodAdvance = "advance"
)

// Payment is one money movement with its allocations.
type Payment struct {
	ID        int64
	Number    string
	BranchID  int64
	PartyID   int64
	PartyName string
	PartyType string
	// PartyPhone / PartyAddress: the party's contact, detail read only (receipts).
	PartyPhone   string
	PartyAddress string
	Direction    string
	Amount       int64
	Method       string
	PaidAt       string
	Notes        string
	Status       string
	// ClientRef makes creation idempotent (unique per branch).
	ClientRef string
	// AmountTendered is the cash handed over (cash only); change = tendered - amount.
	AmountTendered int64
	// Reference is the transfer/check reference number.
	Reference string
	// PairID points at the opposite leg of an offset (0 when unpaired).
	PairID int64
	// CreditSourceType/ID name the return whose saldo a use-credit pair spends
	// (both legs carry it; empty/0 on every other payment, N-41).
	CreditSourceType string
	CreditSourceID   int64
	Allocations      []*Allocation
}

// OffsetInput offsets a confirmed return against an open bill of the same
// party with no cash moving (two paired contra legs). Idempotent per ClientRef.
type OffsetInput struct {
	ActorID    int64
	BranchID   int64
	PartyID    int64
	ReturnType string // sales_return | purchase_return
	ReturnID   int64
	OrderType  string // sales | purchase
	OrderID    int64
	Amount     int64
	ClientRef  string
	Notes      string
}

// RecordInput records ONE payment in full against ONE order (a delivery
// settlement or a POS checkout). Idempotent per ClientRef.
type RecordInput struct {
	ActorID        int64
	BranchID       int64
	OrderType      string // sales | purchase | sales_return (exchange surcharge only)
	OrderID        int64
	Amount         int64 // 0 = the order's whole outstanding
	Method         string
	PaidAt         string
	Reference      string
	AmountTendered int64
	ClientRef      string
	Notes          string
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
	// RecordForOrder records one payment against one order and reports whether
	// it was a replay of an earlier call with the same ClientRef. The party
	// and direction come from the order itself; walk-in orders (party 0) are
	// serialized per order, never on a shared "party 0" lock.
	RecordForOrder(ctx context.Context, in RecordInput) (p *Payment, replayed bool, err error)
	// OffsetReturn offsets a return against an order bill atomically; the call
	// replays (replayed=true) when ClientRef was already served.
	OffsetReturn(ctx context.Context, in OffsetInput) (p *Payment, replayed bool, err error)
	// PaidForOrders sums active allocations per order id of one order type in
	// one grouped read.
	PaidForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error)
	// OffsetForOrders is the part of PaidForOrders made by offset legs (a
	// return set against the bill): no money moved for it.
	OffsetForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error)
	// PaidForOrdersAsOf is PaidForOrders counting only payments (offset legs
	// included) dated on or before asOf, a YYYY-MM-DD WIB day — aging "per
	// tanggal" must not subtract money that arrived later.
	PaidForOrdersAsOf(ctx context.Context, orderType string, ids []int64, asOf string) (map[int64]int64, error)
	// CancelledAmong returns which of ids are cancelled. Finance uses it to
	// find journals that outlived their cancelled source document.
	CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error)
	// ListActiveAfter pages ACTIVE payments by ascending id after a cursor
	// (offsets included: finance filters them by Method).
	ListActiveAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*Payment, error)
	// OrderIDsByPayments maps payment id -> the sales order ids it settles,
	// in one grouped read. A payment may settle SEVERAL orders, so the
	// value is a slice: callers must not assume one order per payment.
	OrderIDsByPayments(ctx context.Context, ids []int64) (map[int64][]int64, error)
	// PaymentInfoByOrders labels orders of one type with how they were paid and
	// by whom, in one grouped read (active payments, offset legs excluded).
	// Orders without a cash payment are absent.
	PaymentInfoByOrders(ctx context.Context, orderType string, ids []int64) (map[int64]OrderPaymentInfo, error)
}

// OrderPaymentInfo summarises the cash payments of one order. Methods are the
// distinct payment methods in payment order; PayerIDs the distinct users who
// recorded those payments (resolve names through the user contract);
// LastPayerID the user who recorded the most recent one (the receipt's
// cashier, as legacy printed it).
type OrderPaymentInfo struct {
	Methods     []string
	PayerIDs    []int64
	LastPayerID int64
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "payment.view", Name: "Pembayaran: Lihat"},
		{Code: "payment.create", Name: "Pembayaran: Tambah"},
		{Code: "payment.archive", Name: "Pembayaran: Batalkan"},
	}
}

// LockKey is the in-process serialization key (shared/doclock) every money
// movement against an order takes: the party, or — for a walk-in order with
// no party — the order itself. An order module that cancels or reprices the
// order takes the same key inside its own document lock (document lock first,
// then this one, the order every payment-making flow already uses), so a
// payment can never land between its "nothing is paid" check and its write.
func LockKey(partyID int64, orderType string, orderID int64) string {
	if partyID != 0 || orderID == 0 {
		return "party:" + strconv.FormatInt(partyID, 10)
	}
	if orderType == "" {
		orderType = OrderSales
	}
	return "wk:" + orderType + "/" + strconv.FormatInt(orderID, 10)
}
