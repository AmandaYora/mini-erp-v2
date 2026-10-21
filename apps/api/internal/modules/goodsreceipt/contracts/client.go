package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Item is one received line (snapshot UOM/qty, location chosen at receipt).
type Item struct {
	ID         int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	UOMFactor  float64
	Qty        float64
	QtyBase    float64
}

// Receipt lifecycle: pending -> posted | void. Only posted receipts count as
// received; pending ones hold PO quota but have not (fully) moved stock.
const (
	StatusPending = "pending"
	StatusPosted  = "posted"
	StatusVoid    = "void"
)

// GoodsReceipt is one receipt against a PO.
type GoodsReceipt struct {
	ID                     int64
	Number                 string
	BranchID               int64
	PurchaseOrderID        int64
	OrderNumber            string
	ReceivedAt             string
	Notes                  string
	SupplierDeliveryNumber string
	Status                 string
	ClientRef              string
	// CreatedBy is the user who recorded the receipt (0 = seeded/migrated);
	// ReceivedByName is resolved for display on reads that decorate it.
	CreatedBy      int64
	ReceivedByName string
	// CancelReason / CancelledAt / CancelledBy record why, when and by whom a
	// pending receipt was discarded (empty when never discarded, or discarded
	// before the reason was recorded).
	CancelReason string
	CancelledAt  string
	CancelledBy  int64
	// LineCount is the number of lines; filled by the list read (GR-01),
	// otherwise len(Items) tells it.
	LineCount int
	Items     []*Item
	// Payment is the supplier payment recorded by a pay_now settlement in THIS
	// call (GR-02), so the caller can attach a transfer slip to it afterwards.
	Payment *SettlementPayment
}

// SettlementPayment identifies the payment a receipt's pay_now settlement made.
type SettlementPayment struct {
	ID     int64
	Number string
}

// ReceiptHints are the facts a purchase return needs from the order's POSTED
// receipts (RT-04).
type ReceiptHints struct {
	// Locations maps "productId/variantId" to the location of the latest
	// posted receipt line of that position.
	Locations map[string]int64
	// FirstReceivedAt is the earliest posted receive date (YYYY-MM-DD HH:MM:SS),
	// empty when nothing was posted yet.
	FirstReceivedAt string
}

// GoodsReceiptClient is the public surface of the goodsreceipt module.
type GoodsReceiptClient interface {
	// GetByID resolves a receipt with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*GoodsReceipt, error)
	// PostedQtyBefore sums, per "product/variant/uom", the line-UOM qty of the
	// order's POSTED receipts with an id below receiptID (rounding-remainder
	// split in finance).
	PostedQtyBefore(ctx context.Context, poID, receiptID int64) (map[string]float64, error)
	// ReceivedQty sums POSTED received base qty for one PO line position.
	// Returns cap the source of every purchase return.
	ReceivedQty(ctx context.Context, poID, productID, variantID int64) (float64, error)
	// PurchaseOrderIDsByReceipts maps receipt id -> purchase order id in one
	// grouped read (finance's tax working paper; no GetByID per receipt).
	PurchaseOrderIDsByReceipts(ctx context.Context, ids []int64) (map[int64]int64, error)
	// ListPostedAfter pages POSTED receipts by ascending id after a cursor.
	ListPostedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*GoodsReceipt, error)
	// ActiveReceiptCount counts the order's live (pending + posted)
	// receipts. Purchasing uses it to refuse cancelling an order under them.
	ActiveReceiptCount(ctx context.Context, poID int64) (int64, error)
	// ListRecent lists POSTED receipts newest-first, bounded by limit. A
	// pending or void receipt moved no (complete) stock and is never
	// journalled. Finance reads it for the derived posting queue.
	ListRecent(ctx context.Context, branchID int64, limit int) ([]*GoodsReceipt, error)
	// ReceivedQtyByOrders sums POSTED received base qty per order and
	// "productId/variantId" in one grouped read (PO list return badge).
	ReceivedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error)
	// PostedReceiptHints reports the latest receipt location per position and
	// the first receive date of the order's posted receipts (RT-04).
	PostedReceiptHints(ctx context.Context, poID int64) (*ReceiptHints, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "goodsreceipt.view", Name: "Penerimaan: Lihat"},
		{Code: "goodsreceipt.create", Name: "Penerimaan: Tambah"},
	}
}
