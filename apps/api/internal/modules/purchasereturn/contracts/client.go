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
// world (K-04b: the purchase vocabulary, not the sales one). Memo-level (like
// legacy): money itself still moves through the payment module.
//   - reduce_payable  the return reduced the open bill (automatic offset);
//   - refund          the supplier paid the excess back in cash/bank;
//   - supplier_credit the excess stays as our balance with the supplier
//     (journalled Dr Uang Muka Supplier / Cr Hutang, K-04a).
const (
	SettleReducePayable  = "reduce_payable"
	SettleRefund         = "refund"
	SettleSupplierCredit = "supplier_credit"
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
	Reason          string
	// CancelReason / CancelledAt / CancelledBy record why, when and by whom
	// the return was cancelled (empty when never cancelled, or cancelled
	// before the reason was recorded).
	CancelReason string
	CancelledAt  string
	CancelledBy  int64
	// PayableReduction is the part of the return automatically offset
	// against the order's open bill at confirmation.
	PayableReduction int64
	Items            []*Item
	Settlements      []*Settlement
	// Settlement position of a CONFIRMED return (filled on detail reads,
	// K-04c): what can still be refunded / kept as supplier balance and the
	// settlement types still legal.
	ExternalRemaining      int64
	AllowedSettlementTypes []string
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

// CreditSettlement is one supplier_credit memo row of a return: the excess kept
// as our balance with the supplier. Finance journals it as Dr Uang Muka
// Supplier / Cr Hutang (K-04a); the row id is the journal's source document id
// (source type purchase_return_settlement).
type CreditSettlement struct {
	ID           int64
	ReturnID     int64
	ReturnNumber string
	BranchID     int64
	// OrderID is the source purchase order (its supplier owns the balance).
	OrderID int64
	Date    string
	Amount  int64
	// ReturnStatus is the parent return's status (confirmed | cancelled | draft).
	ReturnStatus string
}

// Return is the payment-facing projection (refund validation + outstanding).
type Return struct {
	ID      int64
	Number  string
	Branch  int64
	OrderID int64
	Total   int64
	Kind    string
	Date    string
	Status  string
	// CreditSettled is the part already kept as supplier balance (journalled
	// as Uang Muka Supplier, N-40). Payments subtract it from what a refund or
	// a credit offset may still take from the return. Set by GetReturn only.
	CreditSettled int64
}

// PurchaseReturnClient is the public surface of the purchasereturn module.
type PurchaseReturnClient interface {
	// GetByID resolves a return with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*PurchaseReturn, error)
	// CancelledAmong returns which of ids are cancelled. Finance uses it to
	// find journals that outlived their cancelled source document.
	CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error)
	// ListConfirmedAfter pages CONFIRMED returns by ascending id after a cursor.
	ListConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*Return, error)
	// GetCreditSettlement resolves one supplier_credit settlement row with its
	// parent return, or nil when missing or of another type.
	GetCreditSettlement(ctx context.Context, id int64) (*CreditSettlement, error)
	// ListCreditSettlementsAfter pages the supplier_credit settlements of
	// CONFIRMED returns by ascending id after a cursor (finance posting queue).
	ListCreditSettlementsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*CreditSettlement, error)
	// CancelledSettlementsAmong returns which settlement ids belong to a
	// CANCELLED return: their journals must not outlive the return.
	CancelledSettlementsAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error)
	// ActiveReturnCount counts the order's live (draft + confirmed) returns.
	// Purchasing uses it to refuse cancelling an order under them.
	ActiveReturnCount(ctx context.Context, poID int64) (int64, error)
	// ReturnedQtyByOrders sums CONFIRMED returned base qty per order and
	// "productId/variantId" in one grouped read (PO list return badge).
	ReturnedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error)
}

// ReturnsClient is the narrow projection consumed by the payment module.
type ReturnsClient interface {
	// GetReturn resolves refund-relevant fields, or nil when missing.
	GetReturn(ctx context.Context, id int64) (*Return, error)
	// ConfirmedByOrders lists the CONFIRMED returns of the given orders (the
	// party balance reads only its own orders' returns, never the whole branch).
	ConfirmedByOrders(ctx context.Context, branchID int64, orderIDs []int64) ([]*Return, error)
	// CreditSettledByReturns sums the part of each return kept as supplier
	// balance in one grouped read; returns without any are absent (saldo list,
	// N-41).
	CreditSettledByReturns(ctx context.Context, ids []int64) (map[int64]int64, error)
	// ReleaseAutoOffset undoes the bookkeeping of the return's automatic offset
	// after the payment module cancelled that offset pair: the reduction goes
	// back to 0 (and the auto memo with it), so the order balance and the
	// refundable remainder read the same as the books. Idempotent.
	ReleaseAutoOffset(ctx context.Context, returnID int64) error
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "purchasereturn.view", Name: "Retur Beli: Lihat"},
		{Code: "purchasereturn.create", Name: "Retur Beli: Tambah"},
		{Code: "purchasereturn.archive", Name: "Retur Beli: Batalkan"},
		{Code: "purchasereturn.refund", Name: "Retur Beli: Refund"},
	}
}
