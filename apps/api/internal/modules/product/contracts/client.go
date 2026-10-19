package contracts

import (
	"context"
	"strings"
	"time"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Category groups products (optionally nested via parent).
type Category struct {
	ID       int64
	Code     string
	Name     string
	ParentID *int64
	// SortOrder orders siblings in lists and pickers (ascending, then name).
	SortOrder int
	Status    string // active | archived
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
	Status    string // active | inactive | archived
	// IsHidden marks the implicit default variant of a product that has no
	// explicit variants (legacy has_variants=false). Hidden variants are
	// never shown in forms but stay scannable through the product code.
	IsHidden   bool
	SortOrder  int
	Attributes map[string]string
}

// Product is the catalog row other modules snapshot (name/prices frozen into
// order lines at order time — never live-joined).
type Product struct {
	ID              int64
	Code            string
	Name            string
	CategoryID      *int64
	CategoryName    string
	// ArchivedAt is when an archived product was archived (nil otherwise).
	// Archived products are frozen (updates refused), so it is the row's last
	// write; only the list query fills it.
	ArchivedAt *time.Time
	Type            string // barang | jasa | paket | non_stok
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
	Status          string // active | inactive | archived
	Summary         string
	Attributes      map[string]string
	Variants        []*Variant
}

// Product types. Only TypeBarang may be stock-tracked.
const (
	TypeBarang  = "barang"
	TypeJasa    = "jasa"
	TypePaket   = "paket"
	TypeNonStok = "non_stok"
)

// ValidType reports whether t is a known product type.
func ValidType(t string) bool {
	switch t {
	case TypeBarang, TypeJasa, TypePaket, TypeNonStok:
		return true
	}
	return false
}

// Scan resolution routes (ScanMatch.Via).
const (
	ViaBarcode     = "barcode"
	ViaVariantCode = "variant_code"
	ViaProductCode = "product_code"
)

// ScanMatch is what a scanned/typed code means.
type ScanMatch struct {
	Product *Product
	// Variant is the resolved variant (nil when VariantRequired).
	Variant *Variant
	Via     string
	// VariantRequired: the code names a product with several visible
	// variants; the caller must pick one (Product.Variants lists them).
	VariantRequired bool
	// Inactive: the code is known but the product is inactive/archived, so
	// it cannot start a new transaction.
	Inactive bool
}

// StockState is the product module's view of stock-held facts, supplied by
// the stock module through StockStateProvider (stock depends on product,
// never the reverse).
type StockState struct {
	// HasHistory is true once any movement ever touched the product.
	HasHistory bool
	// Available is the on-hand quantity across ALL branches, base UOM.
	Available float64
	// HasHolds is true while active reservations or in-flight transfers
	// reference the product.
	HasHolds bool
}

// VariantStockState is one variant's stock facts.
type VariantStockState struct {
	OnHand     float64
	HasHistory bool
}

// StockStateProvider is implemented by the stock module and injected into
// product after both are built (late-bound edge, like payments).
type StockStateProvider interface {
	ProductStockState(ctx context.Context, productID int64) (StockState, error)
	// VariantStockStates answers for every variant of the product that has
	// any stock row or movement, in one grouped read.
	VariantStockStates(ctx context.Context, productID int64) (map[int64]VariantStockState, error)
	// AvailableByProducts answers the sellable quantity (on hand minus
	// reserved, base UOM, damaged locations excluded) per product in ONE
	// grouped read for one branch. Products without a stock row are absent
	// from the map (read as zero). Used by list pages: no per-row lookup.
	AvailableByProducts(ctx context.Context, branchID int64, productIDs []int64) (map[int64]float64, error)
}

// PricingInfo is the slice of a product member pricing needs, resolved for
// many products in one read.
type PricingInfo struct {
	ID              int64
	Code            string
	Name            string
	Status          string
	SellingPrice    int64
	MinSellingPrice int64
	PurchasePrice   int64
	PurchaseFactor  float64
	SalesFactor     float64
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
	// GetByIDs resolves many products (with active variants and category
	// name) in THREE reads regardless of how many ids: products, variants,
	// categories. Unknown ids are absent. Document pricing/validation paths
	// use this instead of GetByID per line (N+1).
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*Product, error)
	// ResolveUOM maps a transaction UOM to its base-UOM factor. Only the
	// product's own base/purchase/sales UOMs resolve — anything else is rejected,
	// never guessed.
	ResolveUOM(ctx context.Context, productID int64, uom string) (factor float64, baseUOM string, err error)
	// ResolveScan resolves what a scanner/typed code means: an active variant
	// barcode, else an active variant code, else a product code (its default
	// variant). Nil when nothing matches.
	ResolveScan(ctx context.Context, code string) (*Variant, error)
	// ResolveScanCode is ResolveScan with the full answer: the product, how
	// the code matched, and whether a variant must still be chosen or the
	// product is inactive. Nil when nothing matches.
	ResolveScanCode(ctx context.Context, code string) (*ScanMatch, error)
	// PricingByIDs resolves the pricing slice of many products in ONE read.
	// Unknown ids are absent.
	PricingByIDs(ctx context.Context, ids []int64) (map[int64]PricingInfo, error)
	// SearchIDs returns up to limit product ids whose code or name contain q.
	SearchIDs(ctx context.Context, q string, limit int) ([]int64, error)
	// ListStocked lists tracked, active products with their minimum stock
	// (assistant critical-stock tool). Small catalog read, code-ordered.
	ListStocked(ctx context.Context) ([]*StockedProduct, error)
	// ListStockedAfter pages tracked, active products by ascending id after a
	// cursor (keyset): callers that cannot hold the whole catalog walk it.
	ListStockedAfter(ctx context.Context, afterID int64, limit int) ([]*StockedProduct, error)
	// LabelsByIDs resolves code+name for many products in ONE grouped read.
	//
	// Reports decorate rows with a product's identity; doing that one row at
	// a time is an N+1 that grows with the catalog, which is exactly what a
	// report iterates. Unknown ids are simply absent from the map —
	// decoration is best-effort and never fails a report.
	LabelsByIDs(ctx context.Context, ids []int64) (map[int64]Label, error)
	// PurchasePricesByIDs resolves catalog purchase price for many products
	// in ONE grouped read. Cost ingestion prices opening movements from the
	// catalog, and a batch of movements is a batch of products. Unknown ids
	// are absent — the caller decides what a missing price means.
	PurchasePricesByIDs(ctx context.Context, ids []int64) (map[int64]int64, error)
	// PurchaseBaseCostsByIDs resolves the catalog purchase price per BASE unit
	// (the price is per purchase unit, so it is divided by the purchase→base
	// factor, legacy parity). Unknown ids and zero prices are absent.
	PurchaseBaseCostsByIDs(ctx context.Context, ids []int64) (map[int64]float64, error)
}

// UOMFactor maps a transaction UOM to its base factor from an already loaded
// product (same rules as ProductClient.ResolveUOM, without a read). ok=false
// means the unit is not one of the product's own base/purchase/sales UOMs.
func (p *Product) UOMFactor(uom string) (factor float64, baseUOM string, ok bool) {
	uom = strings.TrimSpace(uom)
	switch uom {
	case p.BaseUOM:
		return 1, p.BaseUOM, true
	case p.PurchaseUOM:
		return p.PurchaseFactor, p.BaseUOM, true
	case p.SalesUOM:
		return p.SalesFactor, p.BaseUOM, true
	}
	return 0, "", false
}

// Label is a product's identity for report decoration: the two fields a
// reader displays, without loading the catalog row and its variants behind
// them.
type Label struct {
	Code string
	Name string
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
