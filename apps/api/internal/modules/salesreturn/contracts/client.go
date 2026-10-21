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

// Item conditions: where the returned goods end up.
//   - normal       back into sellable stock at the chosen location;
//   - damaged      into the branch damaged location (never sellable);
//   - not_returned the customer keeps the goods: value is returned, no stock.
const (
	ConditionNormal      = "normal"
	ConditionDamaged     = "damaged"
	ConditionNotReturned = "not_returned"
)

// Kinds of money a return settles (see Return).
const (
	KindRefund = "refund" // we owe the customer
	KindCharge = "charge" // the customer owes us (exchange costs more)
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
	// Condition is normal | damaged | not_returned.
	Condition string
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
	Reason                    string
	// Exchange money: replacement goods priced with the order's tax;
	// DifferenceAmount = ReplacementTotal - Total (signed).
	ReplacementSubtotal int64
	ReplacementTaxTotal int64
	ReplacementTotal    int64
	DifferenceAmount    int64
	// ReceivableReduction is the part of the return automatically offset
	// against the order's open bill at confirmation.
	ReceivableReduction int64
	Items               []*Item
	ReplacementItems    []*ReplacementItem
	Settlements         []*Settlement
	// Settlement position of a CONFIRMED return (filled on detail reads, K-04c):
	// what can still be refunded / kept as customer balance, the exchange
	// surcharge not yet collected, and the settlement types that are still
	// legal. Derived by the same rules AddSettlement enforces.
	ExternalRemaining      int64
	ChargeRemaining        int64
	AllowedSettlementTypes []string
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
	TaxBase    int64
	TaxAmount  int64
	// LineTotal includes tax (TaxBase + TaxAmount).
	LineTotal int64
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
	// Auto marks rows the system wrote (automatic offset), not the operator.
	Auto bool
}

// CreditSettlement is one customer_credit memo row of a return: the part of
// the return value kept as the customer's balance. Finance journals it as the
// reclassification Dr Piutang / Cr Uang Muka Pelanggan (K-04a); the row id is
// the journal's source document id (source type sales_return_settlement).
type CreditSettlement struct {
	ID           int64
	ReturnID     int64
	ReturnNumber string
	BranchID     int64
	// OrderID is the source sales order (its party owns the balance).
	OrderID int64
	Date    string
	Amount  int64
	// ReturnStatus is the parent return's status (confirmed | cancelled | draft).
	ReturnStatus string
}

// Return is the payment-facing projection (refund validation + outstanding).
// Total is the money the return can settle through payments: for a plain
// return the whole return value (Kind refund); for an exchange the absolute
// difference between replacement and return value, refund when the customer
// is owed, charge when the customer owes.
type Return struct {
	ID      int64
	Number  string
	Branch  int64
	OrderID int64
	Total   int64
	Kind    string
	Date    string
	Status  string
	// CreditSettled is the part already kept as customer balance (journalled
	// as Uang Muka Pelanggan, N-40). Payments subtract it from what a refund
	// or a credit offset may still take from the return. Set by GetReturn only.
	CreditSettled int64
}

// SalesReturnClient is the public surface of the salesreturn module.
type SalesReturnClient interface {
	// GetByID resolves a return with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*SalesReturn, error)
	// CancelledAmong returns which of ids are cancelled. Finance uses it to
	// find journals that outlived their cancelled source document.
	CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error)
	// ListConfirmedAfter pages CONFIRMED returns by ascending id after a cursor.
	ListConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*Return, error)
	// ReturnedByProduct sums the returned base qty per product over the
	// confirmed returns dated within an inclusive range (best-seller netting).
	ReturnedByProduct(ctx context.Context, branchID int64, from, to string) (map[int64]float64, error)
	// NetAdjustment is the effect of the confirmed returns dated within a range
	// on the sales figure: a return-only return subtracts its total, an exchange
	// adds (replacement total - returned total).
	NetAdjustment(ctx context.Context, branchID int64, from, to string) (int64, error)
	// GetCreditSettlement resolves one customer_credit settlement row with its
	// parent return, or nil when missing or of another type.
	GetCreditSettlement(ctx context.Context, id int64) (*CreditSettlement, error)
	// ListCreditSettlementsAfter pages the customer_credit settlements of
	// CONFIRMED returns by ascending id after a cursor (finance posting queue).
	ListCreditSettlementsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*CreditSettlement, error)
	// CancelledSettlementsAmong returns which settlement ids belong to a
	// CANCELLED return: their journals must not outlive the return.
	CancelledSettlementsAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error)
	// ActiveReturnCount counts the order's live (draft + confirmed) returns.
	// Sales uses it to refuse cancelling an order under them.
	ActiveReturnCount(ctx context.Context, soID int64) (int64, error)
	// OrderEffects sums the money effect of the CONFIRMED returns per source
	// order in two grouped reads (SL-03): the sales order summary shows the net
	// total and the remaining balance from it. Orders without returns are absent.
	OrderEffects(ctx context.Context, branchID int64, orderIDs []int64) (map[int64]OrderEffect, error)
	// OrderIDsByReturns maps return id -> originating sales order id in one
	// grouped read, for journal-to-order attribution in finance.
	OrderIDsByReturns(ctx context.Context, ids []int64) (map[int64]int64, error)
}

// OrderEffect is what the confirmed returns of one order did to its money
// (legacy salesReturnFinancialEffect). Net order total = grand total + Adjustment.
type OrderEffect struct {
	// Adjustment is signed: a return-only return subtracts its total, an
	// exchange adds replacement - returned (positive = surcharge).
	Adjustment int64
	// Offset is the part already offset against the order's open bill; the
	// payment module counts it inside the order's paid amount.
	Offset int64
	// Refunded / Credit / Collected sum the operator settlement memos: cash
	// handed back, value kept as customer balance, exchange surcharge collected.
	Refunded  int64
	Credit    int64
	Collected int64
}

// ReturnsClient is the narrow projection consumed by the payment module.
type ReturnsClient interface {
	// GetReturn resolves refund-relevant fields, or nil when missing.
	GetReturn(ctx context.Context, id int64) (*Return, error)
	// ConfirmedByOrders lists the CONFIRMED returns of the given orders (the
	// party balance reads only its own orders' returns, never the whole branch).
	ConfirmedByOrders(ctx context.Context, branchID int64, orderIDs []int64) ([]*Return, error)
	// CreditSettledByReturns sums the part of each return kept as customer
	// balance (customer_credit settlements) in one grouped read; returns
	// without any are absent (saldo list, N-41).
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
		{Code: "salesreturn.view", Name: "Retur Jual: Lihat"},
		{Code: "salesreturn.create", Name: "Retur Jual: Tambah"},
		{Code: "salesreturn.archive", Name: "Retur Jual: Batalkan"},
		{Code: "salesreturn.refund", Name: "Retur Jual: Refund"},
	}
}
