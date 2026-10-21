package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Order statuses: fixed state machine in code (DB_SCHEMA §6).
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Channels: regular sales flow vs POS cashier flow (no backend module of its
// own — SYSTEM_DESIGN §3, ADR-0007).
const (
	ChannelRegular = "regular"
	ChannelPOS     = "pos"
)

// Item is one snapshotted order line (contract §7.4).
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
	StockTracked bool   // false: a service/non-stock line, never moves stock
	ItemType     string // barang | jasa
	VariantName  string
	VariantCode  string
	BaseUOM      string
	// PricingSource says where UnitPrice came from; BasePrice is the catalog
	// price it started from (the floor check never applies to member prices).
	PricingSource  string
	BasePrice      int64
	DiscountReason string
	Note           string
	// Taxable false: the line ignores the order's tax; TaxBase/TaxAmount are
	// stored so returns and journals never recompute them.
	Taxable   bool
	TaxBase   int64
	TaxAmount int64
}

// Pricing sources of an order line.
const (
	PricingCatalog    = "catalog"
	PricingMemberRule = "member_rule"
	PricingManual     = "manual"
)

// SalesOrder is the full document with lines and names.
type SalesOrder struct {
	ID        int64
	Number    string
	BranchID  int64
	PartyID   int64
	PartyName string
	// PartyPhone / PartyAddress are the customer's own contact, filled by the
	// detail read only (printed documents fall back to them without a ship-to).
	PartyPhone    string
	PartyAddress  string
	Channel       string
	MemberCode    string
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
	// Ship-to snapshot (C2): frozen at write time, never follows the
	// address master afterwards.
	ShipToAddressID int64
	ShipToLabel     string
	ShipToRecipient string
	ShipToPhone     string
	ShipToAddress   string
	// Tax invoice (C3): printed on SPT working papers.
	TaxInvoiceNumber string
	TaxInvoiceDate   string
	// ClientRef is the client-generated idempotency key of the order (POS cart).
	ClientRef string
	// Who is behind the order (SL-09). IDs are primitives; the names resolve
	// through the user contract on detail/list reads (best-effort, empty when
	// the user is unknown). AssignedUserID is the person in charge (0 = none).
	CreatedAt        string
	CreatedBy        int64
	CreatedByName    string
	AssignedUserID   int64
	AssignedUserName string
	// Credit approval stamped when the order switched to net terms.
	CreditApprovedBy     int64
	CreditApprovedByName string
	CreditApprovedAt     string
	// CashierName is the user who recorded the order's payment (POS receipt /
	// export "Kasir"); empty when nothing was paid yet. Filled on detail reads.
	CashierName string
	// Corrections (filled by detail reads only, see Service.DecorateDetail):
	// why/who/when the order was cancelled, or short-closed ("tutup sisa
	// order": the lines were cut to what confirmed shipments delivered).
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

// SalesOrderClient is the public surface of the sales module.
type SalesOrderClient interface {
	// GetByID resolves a document with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*SalesOrder, error)
	// ListByParty lists a party's documents in a branch for balance
	// computation (payment module). Lightweight rows, newest first.
	ListByParty(ctx context.Context, branchID, partyID int64) ([]*OrderSummary, error)
	// ListOrders lists lightweight rows for operational readers (assistant
	// bot tools). Empty statuses = all non-cancelled; empty from/to =
	// unbounded (order_date range, YYYY-MM-DD). Newest first, capped.
	ListOrders(ctx context.Context, branchID int64, statuses []string, from, to string, limit int) ([]*OrderSummary, error)
	// GetByNumber resolves one document by exact number, branch-scoped
	// (assistant order-status tool), or nil when missing.
	GetByNumber(ctx context.Context, branchID int64, number string) (*SalesOrder, error)
	// SetStatus moves confirmed→completed (delivery) or any open
	// document→cancelled. Branch-scoped: cross-branch ids read as missing.
	SetStatus(ctx context.Context, branchID, id int64, status string, actorID int64) error
	// SwitchToNet moves a confirmed cash-terms order (cod/prepaid) to net
	// with a due date. Idempotent: an order that already is net with that due
	// date replays successfully. Branch-scoped.
	SwitchToNet(ctx context.Context, branchID, id int64, dueDate string, actorID int64) error
	// OrderParties maps order ids to their party ids in one grouped read
	// (A3) for balance computation over many returns. Unknown ids absent.
	OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error)
	// SummariesByIDs resolves many orders to lightweight rows in one read.
	// Finance uses it to label the gross-turnover working paper without a
	// GetByID per order. Unknown ids are absent.
	SummariesByIDs(ctx context.Context, ids []int64) (map[int64]*OrderSummary, error)
	// SearchOrderIDs returns up to limit ids of the branch's orders whose number
	// or customer (name/code/phone/email/address) matches term, newest first —
	// the order half of a return list search (RT-01). Empty term matches nothing.
	SearchOrderIDs(ctx context.Context, branchID int64, term string, limit int) ([]int64, error)
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
	// BranchID and the tax invoice pair let finance's tax working paper label
	// many journal entries from one batch read (no GetByID per order).
	BranchID         int64  `json:"branchId"`
	TaxInvoiceNumber string `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string `json:"taxInvoiceDate"`
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "sales.view", Name: "Penjualan: Lihat"},
		{Code: "sales.create", Name: "Penjualan: Tambah"},
		{Code: "sales.update", Name: "Penjualan: Ubah"},
		{Code: "sales.approve", Name: "Penjualan: Persetujuan Kredit"},
		{Code: "sales.archive", Name: "Penjualan: Batalkan"},
		{Code: "sales.export", Name: "Penjualan: Ekspor"},
	}
}
