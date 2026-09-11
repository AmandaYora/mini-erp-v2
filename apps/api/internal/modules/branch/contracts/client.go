package contracts

import "context"

// Branch is the branch directory row. Status closes branches (KI-36) —
// branches are never hard-deleted once referenced.
type Branch struct {
	ID      int64
	Code    string
	Name    string
	Address string
	City    string
	Phone   string
	Status  string // active | inactive
	IsHead  bool
}

// DocKind identifies a document sequence owned by its own module but
// numbered through the branch (SYSTEM_DESIGN §7.7: prefix from branch code,
// allocated via BranchClient — never computed by hand in each module).
type DocKind string

const (
	DocPurchaseOrder  DocKind = "purchase_order"
	DocSalesOrder     DocKind = "sales_order"
	DocPayment        DocKind = "payment"
	DocDeliveryNote   DocKind = "delivery_note"
	DocSalesReturn    DocKind = "sales_return"
	DocPurchaseReturn DocKind = "purchase_return"
	DocStockTransfer  DocKind = "stock_transfer"
)

// BranchClient is the public surface of the branch module.
type BranchClient interface {
	GetByID(ctx context.Context, id int64) (*Branch, error)
	// GetByIDs resolves accessible branches; unknown IDs are skipped so
	// orphan access rows never break login (they are simply not offered).
	GetByIDs(ctx context.Context, ids []int64) ([]*Branch, error)
	// ListAll lists every branch, name-ordered (assistant bot aggregates
	// company-wide figures per branch). Small table, uncapped.
	ListAll(ctx context.Context) ([]*Branch, error)
	// NextDocumentNumber atomically allocates the next number for kind.
	NextDocumentNumber(ctx context.Context, branchID int64, kind DocKind) (string, error)
}
