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
	Items                 []*Item
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
	// OrderParties maps order ids to their party ids in one grouped read
	// (A3) for balance computation over many returns. Unknown ids absent.
	OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error)
}

// OrderSummary is a lightweight document row for balance computation
// and operational readers (bot tools read the date/party fields).
type OrderSummary struct {
	ID         int64
	Number     string
	GrandTotal int64
	Status     string
	PartyID    int64
	PartyName  string
	OrderDate  string
	DueDate    string
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "purchasing.view", Name: "Pembelian: Lihat"},
		{Code: "purchasing.create", Name: "Pembelian: Tambah"},
		{Code: "purchasing.update", Name: "Pembelian: Ubah"},
		{Code: "purchasing.archive", Name: "Pembelian: Batalkan"},
	}
}
