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

// GoodsReceipt is one receipt against a PO.
type GoodsReceipt struct {
	ID              int64
	BranchID        int64
	PurchaseOrderID int64
	OrderNumber     string
	ReceivedAt      string
	Notes           string
	Items           []*Item
}

// GoodsReceiptClient is the public surface of the goodsreceipt module.
type GoodsReceiptClient interface {
	// GetByID resolves a receipt with lines, or nil when missing.
	GetByID(ctx context.Context, id int64) (*GoodsReceipt, error)
	// ReceivedQty sums received base qty for one PO line position.
	// Returns cap the source of every purchase return.
	ReceivedQty(ctx context.Context, poID, productID, variantID int64) (float64, error)
	// ListRecent lists receipts newest-first, bounded by limit. Receipts
	// carry no lifecycle of their own — every row is postable. Finance
	// reads it for the derived posting queue (no posting table).
	ListRecent(ctx context.Context, branchID int64, limit int) ([]*GoodsReceipt, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "goodsreceipt.view", Name: "Penerimaan: Lihat"},
		{Code: "goodsreceipt.create", Name: "Penerimaan: Tambah"},
	}
}
