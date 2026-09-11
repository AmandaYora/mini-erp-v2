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
}

// SalesOrder is the full document with lines and names.
type SalesOrder struct {
	ID            int64
	Number        string
	BranchID      int64
	PartyID       int64
	PartyName     string
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
	Items            []*Item
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
		{Code: "sales.view", Name: "Penjualan: Lihat"},
		{Code: "sales.create", Name: "Penjualan: Tambah"},
		{Code: "sales.update", Name: "Penjualan: Ubah"},
		{Code: "sales.approve", Name: "Penjualan: Persetujuan Kredit"},
		{Code: "sales.archive", Name: "Penjualan: Batalkan"},
	}
}
