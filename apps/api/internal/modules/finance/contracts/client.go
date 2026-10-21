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
	// Scope is 0 for commercial books, 1 for fiscal-correction journals (and
	// their mirrors). Commercial reports read scope 0 only.
	Scope int
	// Numbering, when set and Number is empty, makes the repository allocate the
	// number INSIDE the insert transaction: a failed insert then burns no number.
	Numbering *Numbering
	Lines     []*JournalLine
}

// Numbering describes a yearly document counter: PREFIX-BRANCHCODE/YEAR/00001.
type Numbering struct {
	Prefix     string
	BranchCode string
	Year       string
}

// Standard chart codes (EnsureChart defaults). Public so dashboard/reporting
// readers can resolve key balances without knowing finance internals.
const (
	CodeCash       = "1000"
	CodeBank       = "1010"
	CodeReceivable = "1100"
	CodeInventory  = "1200"
	CodePayable    = "2000"
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
// SourceReverser reverses the journal of a source document that was
// cancelled after posting (F2b-5). Idempotent: a document with no live journal
// reports (false, nil). The reversal does NOT re-queue the document.
type SourceReverser interface {
	ReverseForSource(ctx context.Context, actorID, branchID int64, docType string, docID int64, reason string) (reversed bool, err error)
}

type FinanceClient interface {
	// OpeningPosted reports whether the branch already posted its cutover
	// balance (stock opening must then go through adjustments).
	OpeningPosted(ctx context.Context, branchID int64) (bool, error)
	// AverageUnitCost is the moving-average unit cost (whole rupiah) of one
	// position from the cost ledger; 0 when nothing is known yet.
	AverageUnitCost(ctx context.Context, branchID, productID, variantID int64) (int64, error)
	// AccountBalance returns lifetime debit/credit footing for one account.
	//
	// Costs a full-history footing of the branch's books. Asking for several
	// accounts means paying for that history once per account — use
	// AccountBalances instead, which pays once for all of them.
	AccountBalance(ctx context.Context, branchID int64, code string) (debit, credit int64, err error)
	// AccountBalances returns lifetime net balances (debit − credit) for
	// several accounts in ONE grouped read, keyed by code. Codes with no
	// account read as zero, matching AccountBalance's not-found handling for
	// an unseeded chart. This is the call dashboards want: four key figures
	// at the price of one, not four.
	AccountBalances(ctx context.Context, branchID int64, codes []string) (map[string]int64, error)
	// BalancesByKey is AccountBalances addressed by account MAPPING KEY (cash,
	// bank, receivable, payable, inventory…) instead of chart code: a renumbered
	// or re-mapped chart keeps the dashboard right. Unmapped keys read zero.
	BalancesByKey(ctx context.Context, branchID int64, keys []string) (map[string]int64, error)
	// NetProfit returns revenue/expense/profit for a date range (inclusive,
	// YYYY-MM-DD). Contra accounts net naturally (debits offset credits).
	NetProfit(ctx context.Context, branchID int64, from, to string) (revenue, expense, profit int64, err error)
	// InventoryValue values every cost position at moving average, decorated
	// with product code/name. Callers that only need the money should use
	// InventoryTotal — naming every position costs a product read per row.
	InventoryValue(ctx context.Context, branchID int64) ([]InventoryPosition, int64, error)
	// InventoryTotal values every cost position and returns only the sum,
	// skipping product-name resolution entirely. One query, no decoration.
	InventoryTotal(ctx context.Context, branchID int64) (int64, error)
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
	// UncostedCount counts the products whose cost ledger still holds estimated
	// (catalog-price) rows: their margin is indicative until a real purchase
	// cost is known (DB-01 cost-basis status). One grouped read.
	UncostedCount(ctx context.Context, branchID int64) (int64, error)
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
	// CostIncomplete is true when part of the product's cost is an estimate
	// (catalog price, no recorded receipt cost): the margin is indicative.
	CostIncomplete bool `json:"costIncomplete"`
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "finance.view", Name: "Keuangan: Lihat"},
		{Code: "finance.manage", Name: "Keuangan: Kelola"},
		{Code: "finance.close.force", Name: "Keuangan: Tutup Periode Tanpa Gerbang"},
		{Code: "finance.journal.reverse", Name: "Keuangan: Balik Jurnal"},
		{Code: "finance.report.all_branches", Name: "Keuangan: Laporan Semua Cabang"},
		{Code: "finance.posting.view", Name: "Keuangan: Lihat Posting"},
		{Code: "finance.posting.post", Name: "Keuangan: Posting Jurnal"},
		{Code: "finance.journal.view", Name: "Keuangan: Lihat Jurnal"},
		{Code: "finance.report.view", Name: "Keuangan: Lihat Laporan"},
		{Code: "finance.tax_report.view", Name: "Keuangan: Lihat Laporan Pajak"},
		{Code: "finance.expense.view", Name: "Keuangan: Lihat Beban"},
		{Code: "finance.expense.create", Name: "Keuangan: Catat Beban"},
		{Code: "finance.expense.cancel", Name: "Keuangan: Batalkan Beban"},
		{Code: "finance.tax_adjustment.view", Name: "Keuangan: Lihat Penyesuaian Pajak"},
		{Code: "finance.tax_adjustment.manage", Name: "Keuangan: Kelola Penyesuaian Pajak"},
	}
}
