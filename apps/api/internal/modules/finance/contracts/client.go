package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Account types with their normal balance side.
const (
	AccountAsset     = "aset"
	AccountLiability = "kewajiban"
	AccountEquity    = "modal"
	AccountIncome    = "pendapatan"
	AccountExpense   = "beban"
)

// Account is one chart-of-accounts row.
type Account struct {
	ID     int64
	Code   string
	Name   string
	Type   string
	IsCash bool
	Status string // active | archived
}

// JournalLine is one debit/credit leg. Exactly one side is nonzero.
//
// The dimension fields are what make the books answerable by product and by
// counterparty (margin per produk, piutang/utang per party). They are
// optional: only legs that genuinely carry the dimension set it — a cash leg
// has no product, a COGS leg has no party. Cross-module links are primitive
// IDs, never foreign keys.
type JournalLine struct {
	AccountID   int64
	AccountCode string
	AccountName string
	Debit       int64
	Credit      int64
	ProductID   int64  // 0 = leg is not product-attributable
	VariantID   int64  // 0 = leg is not product-attributable
	PartyID     int64  // 0 = leg has no counterparty
	Description string // per-line memo; entry memo stays the document-level one
}

// JournalEntry is one balanced compound entry.
type JournalEntry struct {
	ID         int64
	Number     string
	BranchID   int64
	Date       string
	Memo       string
	SourceType string
	SourceID   int64
	Status     string // posted | reversed
	ReversedBy int64
	Lines      []*JournalLine
}

// Standard chart codes (EnsureChart defaults). Public so dashboard/reporting
// readers can resolve key balances without knowing finance internals.
const (
	CodeCash       = "1100"
	CodeReceivable = "1200"
	CodeInventory  = "1300"
	CodePayable    = "2100"
)

// InventoryPosition is one valued cost position.
type InventoryPosition struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Qty         float64 `json:"qty"`
	AvgCost     float64 `json:"avgCost"`
	Value       int64   `json:"value"`
}

// FinanceClient is the public surface of the finance module (L8 consumers).
type FinanceClient interface {
	// AccountBalance returns lifetime debit/credit footing for one account.
	AccountBalance(ctx context.Context, branchID int64, code string) (debit, credit int64, err error)
	// NetProfit returns revenue/expense/profit for a date range (inclusive,
	// YYYY-MM-DD). Contra accounts net naturally (debits offset credits).
	NetProfit(ctx context.Context, branchID int64, from, to string) (revenue, expense, profit int64, err error)
	// InventoryValue values every cost position at moving average.
	InventoryValue(ctx context.Context, branchID int64) ([]InventoryPosition, int64, error)
	// DailyTrend returns per-day revenue/expense/profit for an inclusive
	// YYYY-MM-DD range, days with no postings included as zeroes.
	//
	// Query count: 3, constant in the range length (accounts + COGS mapping +
	// one GROUP BY DATE(entry_date) footing query). Call this instead of
	// looping NetProfit per day — the loop needs 3N+ queries for N days (P3).
	DailyTrend(ctx context.Context, branchID int64, from, to string) ([]DailyPoint, error)
	// MarginByProduct ranks products by realised gross margin (richest first)
	// over an inclusive YYYY-MM-DD range, capped at limit (0 = all).
	MarginByProduct(ctx context.Context, branchID int64, from, to string, limit int) ([]ProductMargin, error)
}

// DailyPoint is one calendar day of revenue/expense/profit, using the same
// semantics as NetProfit: revenue nets contra-income (4000 − 4100), expense
// excludes COGS, profit = revenue − cogs − expense.
type DailyPoint struct {
	Date    string `json:"date"`
	Revenue int64  `json:"revenue"`
	Expense int64  `json:"expense"`
	Profit  int64  `json:"profit"`
}

// ProductMargin is one product's realised margin over a range, from the
// books (not from order lines): revenue nets sales against sales returns and
// COGS nets deliveries against restocks, so a returned sale removes both its
// revenue and its cost instead of leaving a phantom margin.
type ProductMargin struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Revenue     int64   `json:"revenue"`
	Cogs        int64   `json:"cogs"`
	Gross       int64   `json:"gross"`
	Percent     float64 `json:"percent"`
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "finance.view", Name: "Keuangan: Lihat"},
		{Code: "finance.post", Name: "Keuangan: Posting"},
		{Code: "finance.manage", Name: "Keuangan: Kelola"},
	}
}
