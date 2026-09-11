package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Category groups products (optionally nested via parent).
type Category struct {
	ID       int64
	Code     string
	Name     string
	ParentID *int64
	Status   string // active | archived
}

// Variant is one sellable form of a product. Every product owns at least one
// variant (the hidden default when not configured variantly — SYSTEM_DESIGN §7.11).
type Variant struct {
	ID        int64
	ProductID int64
	Code      string
	Name      string
	Barcode   string
	IsDefault bool
	Status    string // active | archived
}

// Product is the catalog row other modules snapshot (name/prices frozen into
// order lines at order time — never live-joined).
type Product struct {
	ID              int64
	Code            string
	Name            string
	CategoryID      *int64
	CategoryName    string
	Type            string // barang | jasa
	Tracked         bool
	BaseUOM         string
	PurchaseUOM     string
	SalesUOM        string
	PurchaseFactor  float64
	SalesFactor     float64
	PurchasePrice   int64
	SellingPrice    int64
	MinSellingPrice int64
	MinStock        float64
	Status          string // active | archived
	Variants        []*Variant
}

// ProductClient is the public surface of the product module.
type ProductClient interface {
	// GetByID resolves a product with its active variants, or nil when missing.
	GetByID(ctx context.Context, id int64) (*Product, error)
	// GetByCode resolves a product by code (spreadsheet flows address
	// products by code, APIs by ID), or nil when missing.
	GetByCode(ctx context.Context, code string) (*Product, error)
	// GetVariantByBarcode resolves an active variant by barcode for scan
	// flows (stock/POS), or nil when unknown.
	GetVariantByBarcode(ctx context.Context, barcode string) (*Variant, error)
	// ResolveUOM maps a transaction UOM to its base-UOM factor. Only the
	// product's own base/purchase/sales UOMs resolve — anything else is rejected,
	// never guessed.
	ResolveUOM(ctx context.Context, productID int64, uom string) (factor float64, baseUOM string, err error)
	// ListStocked lists tracked, active products with their minimum stock
	// (assistant critical-stock tool). Small catalog read, code-ordered.
	ListStocked(ctx context.Context) ([]*StockedProduct, error)
}

// StockedProduct is one tracked product with its minimum stock level.
type StockedProduct struct {
	ID       int64
	Code     string
	Name     string
	MinStock float64
}

// PermissionCatalog lists this module's permission codes for the seed.
// It reuses the user module's seed pair type (contracts-to-contracts only).
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "products.view", Name: "Produk: Lihat"},
		{Code: "products.create", Name: "Produk: Tambah"},
		{Code: "products.update", Name: "Produk: Ubah"},
		{Code: "products.archive", Name: "Produk: Arsipkan"},
		{Code: "product_categories.manage", Name: "Kategori Produk: Kelola"},
		{Code: "product_import.manage", Name: "Impor Produk: Kelola"},
	}
}
