package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/health"
)

// Statuses: draft → confirmed (stock out) → completed? No — confirmed is
// final for stock; cancelled voids drafts or reverses confirmed docs.
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// Item is one SJ line (location chosen at SJ time).
type Item struct {
	ID         int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	UOMFactor  float64
	Qty        float64
	QtyBase    float64
	// ReservationKey names the stock hold this line consumes (POS cart).
	ReservationKey string
}

// Origins of a delivery note (ADR-0022). An order note is drafted against a
// confirmed SO; a direct note ("SJ Langsung") is drafted for a customer BEFORE
// any order exists — SalesOrderID stays 0 until the sales module builds the
// order from it (LinkDirect). After that it behaves like an order note.
const (
	OriginOrder  = "order"
	OriginDirect = "direct"
)

// DeliveryNote is one surat jalan against an SO (or, for a direct note,
// against a customer until its order exists).
type DeliveryNote struct {
	ID       int64
	Number   string
	BranchID int64
	// SalesOrderID is 0 for a direct note that has no order yet.
	SalesOrderID int64
	OrderNumber  string
	// Origin: OriginOrder | OriginDirect.
	Origin string
	// PartyID / PartyName / ShipTo* are the customer snapshot of a DIRECT note
	// (an order note reads them from its SO). PartyName is filled best-effort
	// on reads through the party contract.
	PartyID         int64
	PartyName       string
	ShipToLabel     string
	ShipToRecipient string
	ShipToPhone     string
	ShipToAddress   string
	// LinkedAt / LinkedBy: when and by whom a direct note received its order.
	LinkedAt                        string
	LinkedBy                        int64
	DeliveryDate                    string
	DriverName                      string
	VehiclePlate                    string
	WarehouseStaffName              string
	RecipientName                   string
	RecipientSignatureStatus        string
	RecipientSignatureMissingReason string
	DropLocationNote                string
	DispatchedAt                    string
	DispatchedBy                    int64
	ConfirmedAt                     string
	ConfirmedBy                     int64
	// DocumentKind distinguishes order shipments from replacement
	// shipments for exchanges (D3); SalesReturnID links the latter.
	DocumentKind  string
	SalesReturnID int64
	Status        string
	Notes         string
	ClientRef     string
	// CancelReason / CancelledAt / CancelledBy record why, when and by whom a
	// draft was voided (empty for a note that was never cancelled, or one
	// cancelled before the reason was recorded).
	CancelReason string
	CancelledAt  string
	CancelledBy  int64
	Items        []*Item
	// Warnings are soft notices computed when a draft is created (DL-04, ADR-0018
	// D2): never persisted, only returned by the create call.
	Warnings []string
}

// Document kinds for delivery notes.
const (
	DocumentKindOrder       = "order"
	DocumentKindReplacement = "replacement"
)

// ReplacementLine is one replacement-goods line (same shape as an SJ
// line; priced downstream by finance at moving average, not stored here).
type ReplacementLine struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
}

// ReplacementDraft carries everything needed to draft a replacement
// shipment. SalesOrderID keeps the original order as context (it does NOT
// count toward delivered qty — DeliveredQty filters kind='order').
type ReplacementDraft struct {
	SalesOrderID   int64
	SalesReturnID  int64
	DeliveryDate   string
	Notes          string
	DriverName     string
	VehiclePlate   string
	WarehouseStaff string
	DropNote       string
	// AtCounter marks a replacement handed over at the POS counter in the same
	// step as the return (RT-02): it has no crew, so the driver/staff rule
	// (RT-03) does not apply.
	AtCounter bool
	Items     []ReplacementLine
}

// ReplacementConfirm carries receipt evidence for a replacement shipment and,
// optionally, the money collected on the spot for an exchange that costs more
// than the returned goods (K-04e).
type ReplacementConfirm struct {
	RecipientName            string
	RecipientSignatureStatus string
	MissingReason            string
	// Settlement is the pay_now surcharge collected with the hand-over. The
	// salesreturn module validates it (charge kind, cap) before calling and
	// passes the resolved Amount; nil = nothing collected now.
	Settlement *ReplacementSettlement
}

// ReplacementSettlement is the surcharge payment of an exchange recorded when
// its replacement shipment is confirmed. The payment is allocated to the
// SALES RETURN (charge kind), not to the source order.
type ReplacementSettlement struct {
	// Amount is the money collected (must be > 0; salesreturn defaults it to
	// the whole open surcharge before calling).
	Amount         int64
	Method         string
	PaymentDate    string
	Reference      string
	AmountTendered int64
}

// OpenDirectSummary counts the confirmed direct notes still waiting for an
// order and names the oldest delivery date (YYYY-MM-DD, empty when none).
type OpenDirectSummary struct {
	Count      int64  `json:"count"`
	OldestDate string `json:"oldestDate"`
}

// DeliveryClient is the public surface of the delivery module.
type DeliveryClient interface {
	// GetByID resolves a note with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*DeliveryNote, error)
	// GetByIDs resolves many notes with lines in two grouped reads (headers,
	// lines); unknown ids are absent. Sales reads the source notes of an
	// order-from-SJ this way.
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*DeliveryNote, error)
	// ListOpenDirect lists the branch's confirmed direct notes that have no
	// order yet, oldest first, optionally for one customer (partyID 0 = all),
	// without lines, bounded by limit.
	ListOpenDirect(ctx context.Context, branchID, partyID int64, limit int) ([]*DeliveryNote, error)
	// OpenDirectSummary counts those notes (partyID 0 = all) in one read.
	OpenDirectSummary(ctx context.Context, branchID, partyID int64) (OpenDirectSummary, error)
	// LinkDirect attaches confirmed, unordered direct notes of one customer to
	// the order the sales module built from them (ADR-0022). It only links and
	// checks — completing the order stays with the caller. Invariants, all
	// checked under the order's lock: the order is confirmed, of origin
	// "delivery", in branchID, with the same customer as every note; Σ base qty
	// of the notes per product/variant equals the order's stock lines; every
	// note is a confirmed direct order-kind note that is unordered OR already
	// linked to this very order (replay). All notes link in one transaction or
	// none; a note that belongs to another order is a Conflict.
	LinkDirect(ctx context.Context, actorID, branchID, soID int64, noteIDs []int64) error
	// DeliveredQty sums confirmed-delivered base qty for one SO line
	// position. Returns cap the source of every sales return.
	DeliveredQty(ctx context.Context, soID, productID, variantID int64) (float64, error)
	// ListBySO lists the order's shipments (all statuses, oldest first).
	ListBySO(ctx context.Context, branchID, soID int64) ([]*DeliveryNote, error)
	// ConfirmedQtyBefore sums, per "product/variant/uom", the line-UOM qty of
	// the order's confirmed order-kind notes with an id below noteID. Finance
	// uses it to split a line's value so that the notes add up to the line
	// exactly (the note that completes a line takes the rounding remainder).
	ConfirmedQtyBefore(ctx context.Context, soID, noteID int64) (map[string]float64, error)
	// ActiveNoteCount counts the order's live (draft + confirmed) order-kind
	// shipments. Sales uses it to refuse cancelling an order under them.
	ActiveNoteCount(ctx context.Context, soID int64) (int64, error)
	// CreateReplacement drafts a replacement shipment for an exchange
	// return. No SO-status check: the order is already completed.
	CreateReplacement(ctx context.Context, actorID, branchID int64, in ReplacementDraft) (*DeliveryNote, error)
	// ConfirmReplacement confirms a draft replacement shipment (stock out).
	ConfirmReplacement(ctx context.Context, actorID, branchID, id int64, in ReplacementConfirm) (*DeliveryNote, error)
	// CancelReplacement voids a DRAFT replacement shipment of an exchange,
	// reversing any stock a failed confirm already moved, and records the
	// reason. It accepts only document_kind='replacement' drafts (the generic
	// cancel refuses those) and replays as success on a note already
	// cancelled. Like ConfirmReplacement it takes no lock: the caller
	// (salesreturn) serializes per return.
	CancelReplacement(ctx context.Context, actorID, branchID, id int64, reason string) (*DeliveryNote, error)
	// ListConfirmed lists confirmed notes newest-first, bounded by limit.
	// Finance reads it for the derived posting queue (no posting table).
	ListConfirmed(ctx context.Context, branchID int64, limit int) ([]*DeliveryNote, error)
	// ListConfirmedAfter pages confirmed notes by ascending id after a cursor.
	// Finance walks the whole set with it (no silent cap) to find what is
	// still unjournalled.
	ListConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*DeliveryNote, error)
	// OrderIDsByNotes maps note id -> sales order id in one grouped read.
	// Finance uses it to attribute journal entries to the economic
	// transaction behind them; a per-note GetByID loop would be the N+1
	// this exists to avoid.
	OrderIDsByNotes(ctx context.Context, ids []int64) (map[int64]int64, error)
	// TransportByOrders names the drivers and vehicle plates of the orders'
	// live (draft + confirmed) order-kind shipments in one grouped read. Orders
	// without a named driver/plate are absent.
	TransportByOrders(ctx context.Context, orderIDs []int64) (map[int64]OrderTransport, error)
}

// OrderTransport is the driver/plate label of one order: the distinct names in
// shipment order, joined with ", ".
type OrderTransport struct {
	Drivers string
	Plates  string
}

// HealthClient is the delivery module's read-only data-health surface (ADR-0025).
type HealthClient interface {
	// UnsignedSummary counts confirmed regular notes with no recipient-signature
	// status (A11); AmountRp = linked orders' grand total, OldestAt = oldest
	// confirmation day.
	UnsignedSummary(ctx context.Context, branchID int64, sample int) (health.Result, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "delivery.view", Name: "Pengiriman: Lihat"},
		{Code: "delivery.create", Name: "Pengiriman: Tambah"},
		{Code: "delivery.create_direct", Name: "Pengiriman: Surat Jalan Langsung (tanpa order)"},
		{Code: "delivery.archive", Name: "Pengiriman: Batalkan"},
	}
}
