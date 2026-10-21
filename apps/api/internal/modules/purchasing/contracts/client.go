package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Order statuses: fixed state machine in code (DB_SCHEMA §6 — the legacy
// configurable statuses never worked fully, KI-35).
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Item is one snapshotted order line: product name/price/UOM frozen at order
// time (contract §7.4) — master changes never rewrite history.
type Item struct {
	ID              int64
	ProductID       int64
	VariantID       int64
	ProductCode     string
	ProductName     string
	UOM             string
	UOMFactor       float64
	Qty             float64
	QtyBase         float64
	UnitPrice       int64
	DiscountPct     float64
	DiscountNominal int64
	LineTotal       int64
	// Snapshot of what applied when the order was written (F2a-7/F2a-10).
	StockTracked   bool   // false: a service/non-stock line, never moves stock
	ItemType       string // barang | jasa
	VariantName    string
	VariantCode    string
	BaseUOM        string
	DiscountReason string
	Note           string
	// Taxable false: the line ignores the order's tax; TaxBase/TaxAmount are
	// stored so returns and journals never recompute them.
	Taxable bool
	// TaxRate is the rate this line was taxed at (PO-03): the line's own rate
	// when it set one, else the order rate; 0 when exempt or the order is untaxed.
	TaxRate   float64
	TaxBase   int64
	TaxAmount int64
}

// PurchaseOrder is the full document with lines and party/branch names.
type PurchaseOrder struct {
	ID            int64
	Number        string
	BranchID      int64
	PartyID       int64
	PartyName     string
	OrderDate     string
	DueDate       string
	PaymentTerms  string
	TaxType       string
	TaxRate       float64
	Subtotal      int64
	DiscountTotal int64
	TaxTotal      int64
	GrandTotal    int64
	Status        string
	Notes         string
	// Supplier invoice (C3): printed on SPT working papers.
	SupplierInvoiceNumber string
	SupplierInvoiceDate   string
	// TaxInvoiceNumber/Date is the supplier's tax invoice (faktur pajak, PO-03).
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	// AssignedUserID is the person in charge of the order (0 = none);
	// AssignedUserName is resolved for display only. CreatedBy/CreatedByName
	// say who wrote the document (0 / "" for seeded or migrated rows).
	AssignedUserID   int64
	AssignedUserName string
	CreatedBy        int64
	CreatedByName    string
	// PaymentTermDays is the supplier's credit period (0 = none given); the
	// due date derives from the supplier invoice date plus these days.
	PaymentTermDays int
	// Cancel / short close stamps (000034): why, when (UTC wire datetime) and
	// by whom the order was cancelled or closed short ("tutup sisa order");
	// the names are resolved for display only.
	CancelReason      string
	CancelledAt       string
	CancelledBy       int64
	CancelledByName   string
	ShortCloseReason  string
	ShortClosedAt     string
	ShortClosedBy     int64
	ShortClosedByName string
	Items             []*Item
}

// Correction kinds (purchase_order_corrections.kind).
const (
	// CorrectionPrice re-prices lines after receipt; finance journals the value
	// difference of the received part (source purchase_price_correction).
	CorrectionPrice = "price"
	// CorrectionShortClose cuts every line to its received qty; no journal (the
	// payable was booked per receipt).
	CorrectionShortClose = "short_close"
)

// CorrectionLine is one order line before and after a correction. ReceivedQty
// (line UOM) is what the posted receipts had brought in at that moment: those
// receipts stay valued at the OLD economics (OldQty/OldTaxBase/OldTaxAmount),
// later ones at the new. ValueDelta/TaxDelta are the received part's change
// (whole rupiah): Prorate(new, received, qty) − Prorate(old, received, qty).
type CorrectionLine struct {
	ID                 int64
	OrderItemID        int64
	ProductID          int64
	VariantID          int64
	ProductName        string
	UOM                string
	StockTracked       bool
	ReceivedQty        float64
	ReceivedQtyBase    float64
	OldQty             float64
	NewQty             float64
	OldUnitPrice       int64
	NewUnitPrice       int64
	OldDiscountPct     float64
	NewDiscountPct     float64
	OldDiscountNominal int64
	NewDiscountNominal int64
	OldTaxBase         int64
	NewTaxBase         int64
	OldTaxAmount       int64
	NewTaxAmount       int64
	OldLineTotal       int64
	NewLineTotal       int64
	ValueDelta         int64
	TaxDelta           int64
}

// Correction is one correction of a confirmed or completed order.
type Correction struct {
	ID            int64
	BranchID      int64
	OrderID       int64
	OrderNumber   string
	PartyID       int64
	PartyName     string
	Kind          string
	Date          string // YYYY-MM-DD (WIB day of the correction)
	Reason        string
	OldGrandTotal int64
	NewGrandTotal int64
	// ValueDelta is the received part's tax-base change (inventory or, for
	// service lines, expense); TaxDelta its PPN Masukan change.
	ValueDelta    int64
	TaxDelta      int64
	CreatedAt     string
	CreatedBy     int64
	CreatedByName string
	Lines         []*CorrectionLine
}

// PurchaseOrderClient is the public surface of the purchasing module.
type PurchaseOrderClient interface {
	// GetByID resolves a document with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*PurchaseOrder, error)
	// ListByParty lists a party's documents in a branch for balance
	// computation (payment module). Lightweight rows, newest first.
	ListByParty(ctx context.Context, branchID, partyID int64) ([]*OrderSummary, error)
	// ListOrders lists lightweight rows for operational readers (assistant
	// bot tools). Empty statuses = all non-cancelled; empty from/to =
	// unbounded (order_date range, YYYY-MM-DD). Newest first, capped.
	ListOrders(ctx context.Context, branchID int64, statuses []string, from, to string, limit int) ([]*OrderSummary, error)
	// GetByNumber resolves one document by exact number, branch-scoped
	// (assistant order-status tool), or nil when missing.
	GetByNumber(ctx context.Context, branchID int64, number string) (*PurchaseOrder, error)
	// SetStatus moves confirmed→completed (goods receipt) or any open
	// document→cancelled. Anything else is rejected. Branch-scoped: cross-branch
	// ids read as missing, so L5 callers cannot mutate foreign documents.
	SetStatus(ctx context.Context, branchID, id int64, status string, actorID int64) error
	// SwitchToNet moves a confirmed cash-terms order (cod/prepaid) to net with a
	// due date. Idempotent for an order already net with that due date.
	SwitchToNet(ctx context.Context, branchID, id int64, dueDate string, actorID int64) error
	// OrderParties maps order ids to their party ids in one grouped read
	// (A3) for balance computation over many returns. Unknown ids absent.
	OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error)
	// SummariesByIDs resolves many purchase orders to lightweight rows in one
	// read (payment ledger labels documents with it). Unknown ids are absent.
	SummariesByIDs(ctx context.Context, ids []int64) (map[int64]*OrderSummary, error)
	// SearchOrderIDs returns up to limit ids of the branch's orders whose number
	// or supplier (name/code/phone/email/address) matches term, newest first —
	// the order half of a return list search (RT-01). Short terms match nothing.
	SearchOrderIDs(ctx context.Context, branchID int64, term string, limit int) ([]int64, error)
	// GetCorrection resolves one correction with its lines, order number and
	// supplier, or nil when missing (finance builds the price correction
	// journal from it).
	GetCorrection(ctx context.Context, id int64) (*Correction, error)
	// ListPriceCorrectionsAfter pages the branch's PRICE corrections that move
	// the books (a nonzero value or tax delta) by ascending id after a cursor —
	// the finance posting queue.
	ListPriceCorrectionsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*Correction, error)
	// CorrectionsByOrder lists every correction of an order with its lines,
	// oldest first. Finance values a receipt covered by a correction at the
	// line economics it was booked at (LineAt), so a receipt journal or cost
	// row rebuilt after a correction never double-counts it.
	CorrectionsByOrder(ctx context.Context, orderID int64) ([]*Correction, error)
}

// LineEconomics is the (qty, tax base, tax) a receipt window of a line is
// valued against.
type LineEconomics struct {
	Qty       float64
	TaxBase   int64
	TaxAmount int64
}

// qtyEpsilon absorbs the decimal(24,12) rounding of stored quantities.
const qtyEpsilon = 1e-9

// LineAt returns the economics a receipt covering [prior, prior+qty] of the
// line (product/variant/uom) is valued at: the OLD economics of the earliest
// correction whose received qty already included that receipt, else the line
// as it stands now. corrections are oldest first.
func LineAt(corrections []*Correction, line *Item, prior, qty float64) LineEconomics {
	for _, c := range corrections {
		for _, l := range c.Lines {
			if l.ProductID != line.ProductID || l.VariantID != line.VariantID || l.UOM != line.UOM {
				continue
			}
			if prior+qty <= l.ReceivedQty+qtyEpsilon {
				return LineEconomics{Qty: l.OldQty, TaxBase: l.OldTaxBase, TaxAmount: l.OldTaxAmount}
			}
			break
		}
	}
	return LineEconomics{Qty: line.Qty, TaxBase: line.TaxBase, TaxAmount: line.TaxAmount}
}

// OrderSummary is a lightweight document row for balance computation
// and operational readers (bot tools read the date/party fields).
type OrderSummary struct {
	ID         int64  `json:"id"`
	Number     string `json:"number"`
	GrandTotal int64  `json:"grandTotal"`
	Status     string `json:"status"`
	PartyID    int64  `json:"partyId"`
	PartyName  string `json:"partyName"`
	OrderDate  string `json:"orderDate"`
	DueDate    string `json:"dueDate"`
	// BranchID and the supplier invoice pair serve finance's tax working
	// paper in one batch read (no GetByID per order).
	BranchID              int64  `json:"branchId"`
	SupplierInvoiceNumber string `json:"supplierInvoiceNumber"`
	SupplierInvoiceDate   string `json:"supplierInvoiceDate"`
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "purchasing.view", Name: "Pembelian: Lihat"},
		{Code: "purchasing.create", Name: "Pembelian: Tambah"},
		{Code: "purchasing.update", Name: "Pembelian: Ubah"},
		{Code: "purchasing.archive", Name: "Pembelian: Batalkan"},
		{Code: "purchasing.export", Name: "Pembelian: Ekspor"},
	}
}
