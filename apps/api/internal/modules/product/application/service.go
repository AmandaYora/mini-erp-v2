package application

import (
	"context"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/domain"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// Service implements contracts.ProductClient plus catalog administration.
type Service struct {
	repo *infrastructure.Repository
	// audit receives best-effort trail records after catalog mutations.
	audit auditcontracts.AuditClient
	// stock answers "does this product hold stock / have history", supplied by
	// the stock module after both are built (SetStockState). Nil means no
	// stock facts are known yet; the guards that need them are then inert,
	// which is only safe while no stock module is wired (tests of the module
	// alone). The composition root always wires it.
	stock contracts.StockStateProvider
}

// NewService wires catalog use cases.
func NewService(repo *infrastructure.Repository, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, audit: audit}
}

// SetStockState wires the stock provider (late-bound edge).
func (s *Service) SetStockState(p contracts.StockStateProvider) { s.stock = p }

func (s *Service) stockState(ctx context.Context, productID int64) (contracts.StockState, error) {
	if s.stock == nil {
		return contracts.StockState{}, nil
	}
	return s.stock.ProductStockState(ctx, productID)
}

func (s *Service) variantStates(ctx context.Context, productID int64) (map[int64]contracts.VariantStockState, error) {
	if s.stock == nil {
		return map[int64]contracts.VariantStockState{}, nil
	}
	return s.stock.VariantStockStates(ctx, productID)
}

// GetByID resolves a product with active variants and category name.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.Product, error) {
	p, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, nil
	}
	return s.enrich(ctx, p)
}

// GetByIDs resolves many products with active variants and category names in
// three reads (products, variants, categories) whatever the id count.
func (s *Service) GetByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.Product, error) {
	products, err := s.repo.ProductsByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(products) == 0 {
		return products, nil
	}
	pids := make([]int64, 0, len(products))
	var catIDs []int64
	for id, p := range products {
		pids = append(pids, id)
		if p.CategoryID != nil {
			catIDs = append(catIDs, *p.CategoryID)
		}
	}
	variants, err := s.repo.VariantsByProducts(ctx, pids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	names, err := s.repo.CategoryNamesByIDs(ctx, catIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for id, p := range products {
		p.Variants = variants[id]
		if p.Variants == nil {
			p.Variants = []*contracts.Variant{}
		}
		if p.CategoryID != nil {
			p.CategoryName = names[*p.CategoryID]
		}
	}
	return products, nil
}

func (s *Service) enrich(ctx context.Context, p *contracts.Product) (*contracts.Product, error) {
	variants, err := s.repo.VariantsByProduct(ctx, p.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if variants == nil {
		variants = []*contracts.Variant{}
	}
	p.Variants = variants
	if p.CategoryID != nil {
		if c, err := s.repo.CategoryByID(ctx, *p.CategoryID); err != nil {
			return nil, apperror.Internal(err)
		} else if c != nil {
			p.CategoryName = c.Name
		}
	}
	return p, nil
}

// GetByCode resolves a product by code with variants, or nil when missing.
func (s *Service) GetByCode(ctx context.Context, code string) (*contracts.Product, error) {
	p, err := s.repo.ProductByCode(ctx, code)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, nil
	}
	return s.enrich(ctx, p)
}

// GetVariantByBarcode resolves an active variant by barcode, or nil.
func (s *Service) GetVariantByBarcode(ctx context.Context, barcode string) (*contracts.Variant, error) {
	if barcode == "" {
		return nil, nil
	}
	v, err := s.repo.VariantByBarcode(ctx, barcode)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return v, nil
}

// ResolveScan resolves a scanned/typed code to one variant, or nil when it is
// unknown, ambiguous, needs a variant choice, or names an inactive product.
func (s *Service) ResolveScan(ctx context.Context, code string) (*contracts.Variant, error) {
	m, err := s.ResolveScanCode(ctx, code)
	if err != nil || m == nil || m.Inactive || m.VariantRequired {
		return nil, err
	}
	return m.Variant, nil
}

// ResolveScanCode resolves what a code means, in the legacy order: variant
// barcode, then variant code, then product code. A product code with several
// visible variants asks the caller to choose (VariantRequired); a product
// that is not active answers Inactive instead of silently resolving.
func (s *Service) ResolveScanCode(ctx context.Context, code string) (*contracts.ScanMatch, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	var hit *contracts.Variant
	via := ""
	if v, err := s.repo.VariantByBarcode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if v != nil {
		hit, via = v, contracts.ViaBarcode
	}
	if hit == nil {
		vs, err := s.repo.ActiveVariantsByCode(ctx, strings.ToUpper(code), 2)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(vs) == 1 {
			hit, via = vs[0], contracts.ViaVariantCode
		} else if len(vs) > 1 {
			return nil, nil // the same variant code on several products: ambiguous, never guess
		}
	}
	if hit != nil {
		p, err := s.repo.GetProduct(ctx, hit.ProductID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if p == nil {
			return nil, nil
		}
		ep, err := s.enrich(ctx, p)
		if err != nil {
			return nil, err
		}
		return &contracts.ScanMatch{Product: ep, Variant: hit, Via: via, Inactive: p.Status != "active"}, nil
	}
	p, err := s.repo.ProductByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, nil
	}
	ep, err := s.enrich(ctx, p)
	if err != nil {
		return nil, err
	}
	m := &contracts.ScanMatch{Product: ep, Via: contracts.ViaProductCode, Inactive: p.Status != "active"}
	var visible []*contracts.Variant
	for _, v := range ep.Variants {
		if !v.IsHidden {
			visible = append(visible, v)
		}
	}
	switch {
	case len(visible) > 1:
		m.VariantRequired = true
	case len(visible) == 1:
		m.Variant = visible[0]
	default:
		def, err := s.repo.DefaultVariant(ctx, p.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if def == nil {
			return nil, nil
		}
		m.Variant = def
	}
	return m, nil
}

// PricingByIDs resolves the member-pricing slice of many products in ONE read.
func (s *Service) PricingByIDs(ctx context.Context, ids []int64) (map[int64]contracts.PricingInfo, error) {
	m, err := s.repo.PricingByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return m, nil
}

// SearchIDs returns up to limit product ids matching a code/name fragment.
func (s *Service) SearchIDs(ctx context.Context, q string, limit int) ([]int64, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	ids, err := s.repo.SearchProductIDs(ctx, q, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
}

// ListStockedAfter pages tracked, active products by ascending id (keyset).
func (s *Service) ListStockedAfter(ctx context.Context, afterID int64, limit int) ([]*contracts.StockedProduct, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.repo.ListStockedAfter(ctx, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.StockedProduct{}
	}
	return rows, nil
}

// ListStocked lists tracked, active products with minimum stock
// (assistant critical-stock tool).
func (s *Service) ListStocked(ctx context.Context) ([]*contracts.StockedProduct, error) {
	rows, err := s.repo.ListStocked(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.StockedProduct{}
	}
	return rows, nil
}

// LabelsByIDs resolves code+name for many products in one grouped read.
// This is what report decoration should call: GetByID also loads variants,
// which no report row displays.
func (s *Service) LabelsByIDs(ctx context.Context, ids []int64) (map[int64]contracts.Label, error) {
	labels, err := s.repo.LabelsByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return labels, nil
}

// PurchasePricesByIDs resolves catalog purchase prices in one grouped read.
func (s *Service) PurchaseBaseCostsByIDs(ctx context.Context, ids []int64) (map[int64]float64, error) {
	costs, err := s.repo.PurchaseBaseCostsByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return costs, nil
}

// PurchasePricesByIDs resolves catalog purchase prices in one grouped read.
func (s *Service) PurchasePricesByIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	prices, err := s.repo.PurchasePricesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return prices, nil
}

// ResolveUOM maps a transaction UOM to its base factor. Only the product's
// own base/purchase/sales UOMs resolve — unknown units are rejected, never
// guessed (a wrong factor silently corrupts stock and HPP).
func (s *Service) ResolveUOM(ctx context.Context, productID int64, uom string) (float64, string, error) {
	p, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return 0, "", apperror.Internal(err)
	}
	if p == nil {
		return 0, "", apperror.NotFound("Produk")
	}
	uom = strings.TrimSpace(uom)
	switch uom {
	case p.BaseUOM:
		return 1, p.BaseUOM, nil
	case p.PurchaseUOM:
		return p.PurchaseFactor, p.BaseUOM, nil
	case p.SalesUOM:
		return p.SalesFactor, p.BaseUOM, nil
	default:
		return 0, "", apperror.Validation("", []apperror.FieldError{
			{Field: "uom", Message: "satuan '" + uom + "' tidak dikenal untuk produk ini"},
		})
	}
}

// --- categories -------------------------------------------------------------

// checkSortOrder bounds the category sort key (non-negative, sane upper limit).
func checkSortOrder(n int) string {
	if n < 0 || n > 9999 {
		return "harus antara 0 dan 9999"
	}
	return ""
}

// CreateCategory inserts a category (code immutable once issued).
func (s *Service) CreateCategory(ctx context.Context, actorID int64, code, name string, parentID *int64, sortOrder *int) (*contracts.Category, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.CategoryByCode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if dup != nil {
		return nil, apperror.Conflict("Kode kategori '" + code + "' sudah digunakan")
	}
	if parentID != nil {
		if parent, err := s.repo.CategoryByID(ctx, *parentID); err != nil {
			return nil, apperror.Internal(err)
		} else if parent == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "kategori induk tidak ditemukan"}})
		}
	}
	order := 0
	if sortOrder != nil {
		if msg := checkSortOrder(*sortOrder); msg != "" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "sortOrder", Message: msg}})
		}
		order = *sortOrder
	}
	id, err := s.repo.CreateCategory(ctx, &contracts.Category{Code: code, Name: strings.TrimSpace(name), ParentID: parentID, SortOrder: order}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	cat, err := s.repo.CategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "categories.create", Entity: "category", EntityID: cat.ID, BranchID: 0, ActorID: actorID, Note: cat.Code})
	return cat, nil
}

// UpdateCategory rewrites name/parent/sort order (nil sortOrder keeps the stored one).
func (s *Service) UpdateCategory(ctx context.Context, actorID, id int64, name string, parentID *int64, sortOrder *int) (*contracts.Category, error) {
	c, err := s.repo.CategoryByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if c == nil {
		return nil, apperror.NotFound("Kategori")
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if parentID != nil {
		if *parentID == id {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "kategori tidak bisa menjadi induk dirinya sendiri"}})
		}
		if parent, err := s.repo.CategoryByID(ctx, *parentID); err != nil {
			return nil, apperror.Internal(err)
		} else if parent == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "kategori induk tidak ditemukan"}})
		}
		if cycle, err := s.repo.HasAncestor(ctx, *parentID, id); err != nil {
			return nil, apperror.Internal(err)
		} else if cycle {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "parentId", Message: "induk membentuk siklus"}})
		}
	}
	if sortOrder != nil {
		if msg := checkSortOrder(*sortOrder); msg != "" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "sortOrder", Message: msg}})
		}
		c.SortOrder = *sortOrder
	}
	c.Name = strings.TrimSpace(name)
	c.ParentID = parentID
	if err := s.repo.UpdateCategory(ctx, c, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.repo.CategoryByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "categories.update", Entity: "category", EntityID: updated.ID, BranchID: 0, ActorID: actorID, Note: updated.Code})
	return updated, nil
}

// ArchiveCategory archives an unused category. Categories holding active
// products are rejected — reassign the products first.
func (s *Service) ArchiveCategory(ctx context.Context, actorID, id int64) error {
	c, err := s.repo.CategoryByID(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if c == nil {
		return apperror.NotFound("Kategori")
	}
	if n, err := s.repo.CountActiveProductsByCategory(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Kategori masih dipakai produk dan tidak dapat diarsipkan")
	}
	if n, err := s.repo.CountActiveSubcategories(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if n > 0 {
		return apperror.Conflict("Kategori masih memiliki subkategori aktif")
	}
	if err := s.repo.SetCategoryStatus(ctx, id, "archived", actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "categories.archive", Entity: "category", EntityID: id, BranchID: 0, ActorID: actorID, Note: c.Code})
	return nil
}

// ListCategories returns all categories.
func (s *Service) ListCategories(ctx context.Context) ([]*contracts.Category, error) {
	cats, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if cats == nil {
		cats = []*contracts.Category{}
	}
	return cats, nil
}

// --- products ---------------------------------------------------------------

// ProductInput is the create/update payload (already structurally validated).
type ProductInput struct {
	Code            string
	Name            string
	CategoryID      *int64
	Type            string
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
	// Status is active|inactive; empty keeps the stored status (active on
	// create). Archiving has its own endpoint.
	Status  string
	Summary string
	// Attributes is the "Info Tambahan" block.
	Attributes map[string]string
	// Variants: nil keeps the stored set (a fresh product gets the hidden
	// default); a non-nil empty slice reverts to the hidden default.
	Variants []VariantInput
}

// VariantInput is one variant row.
type VariantInput struct {
	// ID matches a stored variant (so a code change is a rename, which stock
	// history forbids); 0 matches by code.
	ID        int64
	Code      string
	Name      string
	Barcode   string
	IsDefault bool
	// Status is active|inactive|archived ("" = active).
	Status     string
	SortOrder  int
	Attributes map[string]string
	// Hidden marks the implicit default variant; nil keeps what is stored
	// (a form echoing the stored variants back leaves it hidden).
	Hidden *bool
}

const (
	maxProductCode = 50
	maxVariantCode = 80
	maxName        = 255
	maxBarcode     = 100
)

// normalizeInput applies the type rules (legacy resolveProductUomConfig):
// only "barang" is stock-tracked; every other type is untracked with factor 1
// and one unit. Run before validation, by every entry point (form, import).
func normalizeInput(in ProductInput) ProductInput {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.BaseUOM = strings.TrimSpace(in.BaseUOM)
	in.PurchaseUOM = strings.TrimSpace(in.PurchaseUOM)
	in.SalesUOM = strings.TrimSpace(in.SalesUOM)
	if in.Type != contracts.TypeBarang && contracts.ValidType(in.Type) {
		in.Tracked = false
		in.PurchaseFactor, in.SalesFactor = 1, 1
		in.PurchaseUOM, in.SalesUOM = in.BaseUOM, in.BaseUOM
	}
	return in
}

// cleanAttrs trims an attribute block; problems are returned as messages.
func cleanAttrs(in map[string]string, field string) (map[string]string, []apperror.FieldError) {
	if in == nil {
		return nil, nil
	}
	var fields []apperror.FieldError
	if len(in) > 30 {
		return nil, []apperror.FieldError{{Field: field, Message: "maksimal 30 info tambahan"}}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key, val := strings.TrimSpace(k), strings.TrimSpace(v)
		if key == "" || len([]rune(key)) > 60 || val == "" || len([]rune(val)) > 255 {
			fields = append(fields, apperror.FieldError{Field: field, Message: "info tambahan '" + key + "': nama 1-60 dan isi 1-255 karakter"})
			continue
		}
		out[key] = val
	}
	return out, fields
}

func (s *Service) validateProduct(ctx context.Context, in ProductInput, excludeID int64) ([]apperror.FieldError, error) {
	return s.validateProductCore(ctx, in, excludeID, true)
}

// validateProductFields is the pure half of validation (no database): bulk
// import runs it per row and does the collision lookups in batches instead.
func (s *Service) validateProductFields(in ProductInput) []apperror.FieldError {
	f, _ := s.validateProductCore(context.Background(), in, 0, false)
	return f
}

func (s *Service) validateProductCore(ctx context.Context, in ProductInput, excludeID int64, db bool) ([]apperror.FieldError, error) {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	switch {
	case code == "":
		fail("code", "wajib diisi")
	case len([]rune(code)) > maxProductCode:
		fail("code", "maksimal 50 karakter")
	case db:
		dup, err := s.repo.ProductByCode(ctx, code)
		if err != nil {
			return nil, err
		}
		if dup != nil && dup.ID != excludeID {
			return nil, apperror.Conflict("Kode produk '" + code + "' sudah digunakan")
		}
	}
	if strings.TrimSpace(in.Name) == "" {
		fail("name", "wajib diisi")
	} else if len([]rune(strings.TrimSpace(in.Name))) > maxName {
		fail("name", "maksimal 255 karakter")
	}
	if in.CategoryID != nil && db {
		c, err := s.repo.CategoryByID(ctx, *in.CategoryID)
		if err != nil {
			return nil, err
		}
		if c == nil {
			fail("categoryId", "kategori tidak ditemukan")
		} else if c.Status != "active" {
			fail("categoryId", "kategori sudah diarsipkan")
		}
	}
	if !contracts.ValidType(in.Type) {
		fail("type", "harus barang, jasa, paket, atau non_stok")
	}
	if in.Status != "" && in.Status != "active" && in.Status != "inactive" {
		fail("status", "harus active atau inactive")
	}
	for _, uom := range []struct {
		name, value string
	}{{"baseUom", in.BaseUOM}, {"purchaseUom", in.PurchaseUOM}, {"salesUom", in.SalesUOM}} {
		if strings.TrimSpace(uom.value) == "" {
			fail(uom.name, "wajib diisi")
		} else if len([]rune(uom.value)) > 30 {
			fail(uom.name, "maksimal 30 karakter")
		}
	}
	if !domain.ValidFactor(in.PurchaseFactor) {
		fail("purchaseFactor", "harus lebih dari 0")
	}
	if !domain.ValidFactor(in.SalesFactor) {
		fail("salesFactor", "harus lebih dari 0")
	}
	for _, money := range []struct {
		name  string
		value int64
	}{{"purchasePrice", in.PurchasePrice}, {"sellingPrice", in.SellingPrice},
		{"minSellingPrice", in.MinSellingPrice}} {
		if money.value < 0 {
			fail(money.name, "tidak boleh negatif")
		}
	}
	if in.MinSellingPrice > in.SellingPrice {
		fail("minSellingPrice", "tidak boleh melebihi harga jual")
	}
	if in.MinStock < 0 {
		fail("minStock", "tidak boleh negatif")
	}
	if in.Tracked && in.PurchasePrice <= 0 {
		fail("purchasePrice", "harga beli wajib diisi untuk produk terpantau")
	}
	if _, af := cleanAttrs(in.Attributes, "attributes"); len(af) > 0 {
		fields = append(fields, af...)
	}

	// Variants: shape first, then scan-key collisions (one namespace for
	// product codes, variant codes and barcodes; legacy seenScanKeys).
	seen := map[string]bool{} // variant codes of the payload
	var codes, barcodes []string
	for i, v := range in.Variants {
		vc := strings.ToUpper(strings.TrimSpace(v.Code))
		if vc == "" {
			fail("variants", "kode varian baris "+strconv.Itoa(i+1)+" wajib diisi")
			continue
		}
		if len([]rune(vc)) > maxVariantCode {
			fail("variants", "kode varian '"+vc+"' maksimal 80 karakter")
		}
		if seen[vc] {
			fail("variants", "kode varian '"+vc+"' ganda")
		}
		seen[vc] = true
		if strings.TrimSpace(v.Name) == "" {
			fail("variants", "nama varian '"+vc+"' wajib diisi")
		} else if len([]rune(strings.TrimSpace(v.Name))) > maxName {
			fail("variants", "nama varian '"+vc+"' maksimal 255 karakter")
		}
		switch v.Status {
		case "", "active", "inactive", "archived":
		default:
			fail("variants", "status varian '"+vc+"' harus active, inactive, atau archived")
		}
		if _, af := cleanAttrs(v.Attributes, "variants"); len(af) > 0 {
			fields = append(fields, af...)
		}
		codes = append(codes, vc)
		bc := strings.TrimSpace(v.Barcode)
		if bc != "" {
			if len([]rune(bc)) > maxBarcode {
				fail("variants", "barcode '"+bc+"' maksimal 100 karakter")
			}
			barcodes = append(barcodes, bc)
		}
	}
	// Barcodes and variant codes of one payload share a namespace; two
	// variants may not use one value in different roles or twice as barcode.
	roles := map[string]string{}
	for _, vc := range codes {
		roles[vc] = "kode"
	}
	bseen := map[string]bool{}
	for _, bc := range barcodes {
		up := strings.ToUpper(bc)
		if bseen[up] {
			fail("variants", "barcode '"+bc+"' ganda")
		}
		bseen[up] = true
		if roles[up] == "kode" {
			// A variant may carry its own code as barcode; another's is a clash.
			own := false
			for _, v := range in.Variants {
				if strings.ToUpper(strings.TrimSpace(v.Code)) == up && strings.EqualFold(strings.TrimSpace(v.Barcode), bc) {
					own = true
				}
			}
			if !own {
				fail("variants", "barcode '"+bc+"' sama dengan kode varian lain")
			}
		}
	}
	if code != "" && db {
		// Product code vs OTHER products' variants (code or barcode).
		owners, err := s.repo.OtherVariantsUsingKeys(ctx, append([]string{code}, append(append([]string{}, codes...), barcodes...)...), excludeID)
		if err != nil {
			return nil, err
		}
		pcodes, err := s.repo.ProductCodesIn(ctx, append(append([]string{}, codes...), barcodes...), excludeID)
		if err != nil {
			return nil, err
		}
		payloadCode := map[string]bool{}
		for _, c := range codes {
			payloadCode[c] = true
		}
		payloadBarcode := map[string]bool{}
		for _, b := range barcodes {
			payloadBarcode[strings.ToUpper(b)] = true
		}
		for _, o := range owners {
			if strings.EqualFold(o.Code, code) || strings.EqualFold(o.Barcode, code) {
				return nil, apperror.Conflict("Kode produk '" + code + "' sudah digunakan sebagai kode atau barcode varian")
			}
			if o.Barcode != "" && (payloadCode[strings.ToUpper(o.Barcode)] || payloadBarcode[strings.ToUpper(o.Barcode)]) {
				fail("variants", "barcode '"+o.Barcode+"' sudah dipakai produk lain")
			}
			if payloadBarcode[strings.ToUpper(o.Code)] {
				fail("variants", "barcode '"+o.Code+"' sama dengan kode varian produk lain")
			}
		}
		for k := range pcodes {
			fail("variants", "'"+k+"' sudah digunakan sebagai kode produk lain")
		}
	}
	if len(fields) > 0 {
		return fields, nil
	}
	return nil, nil
}

// wrapErr passes validation/conflict failures through and wraps raw
// storage errors as internal (same rule as response.FailErr, for services).
func wrapErr(err error) *apperror.AppError {
	if ae, ok := err.(*apperror.AppError); ok {
		return ae
	}
	return apperror.Internal(err)
}

func (s *Service) toProduct(in ProductInput) *contracts.Product {
	attrs, _ := cleanAttrs(in.Attributes, "attributes")
	return &contracts.Product{
		Code: in.Code, Name: in.Name, CategoryID: in.CategoryID, Type: in.Type, Tracked: in.Tracked,
		BaseUOM: in.BaseUOM, PurchaseUOM: in.PurchaseUOM, SalesUOM: in.SalesUOM,
		PurchaseFactor: in.PurchaseFactor, SalesFactor: in.SalesFactor,
		PurchasePrice: in.PurchasePrice, SellingPrice: in.SellingPrice,
		MinSellingPrice: in.MinSellingPrice, MinStock: in.MinStock,
		Status: in.Status, Summary: strings.TrimSpace(in.Summary), Attributes: attrs,
	}
}

// CreateProduct validates and inserts product + variants atomically.
func (s *Service) CreateProduct(ctx context.Context, actorID int64, in ProductInput) (*contracts.Product, error) {
	in = normalizeInput(in)
	fields, err := s.validateProduct(ctx, in, 0)
	if err != nil {
		return nil, wrapErr(err)
	}
	if fields != nil {
		return nil, apperror.Validation("", fields)
	}
	p := s.toProduct(in)
	variants, err := planVariants(variantPlanInput{
		ProductCode: in.Code, ProductName: in.Name, Payload: in.Variants,
	})
	if err != nil {
		return nil, wrapErr(err)
	}
	p.Variants = variants
	id, err := s.repo.CreateProduct(ctx, p, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	out, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "products.create", Entity: "product", EntityID: out.ID, BranchID: 0, ActorID: actorID, Note: out.Code})
	return out, nil
}

// productSnapshot is the audit view of a product (F3b-8): catalog fields and
// a compact variant list, no derived display data.
func productSnapshot(p *contracts.Product, variants []*contracts.Variant) map[string]any {
	vs := make([]map[string]any, 0, len(variants))
	for _, v := range variants {
		vs = append(vs, map[string]any{"code": v.Code, "name": v.Name, "barcode": v.Barcode, "status": v.Status, "isDefault": v.IsDefault})
	}
	var category any
	if p.CategoryID != nil {
		category = *p.CategoryID
	}
	return map[string]any{
		"code": p.Code, "name": p.Name, "categoryId": category, "type": p.Type, "tracked": p.Tracked,
		"baseUom": p.BaseUOM, "purchaseUom": p.PurchaseUOM, "salesUom": p.SalesUOM,
		"purchaseFactor": p.PurchaseFactor, "salesFactor": p.SalesFactor,
		"purchasePrice": p.PurchasePrice, "sellingPrice": p.SellingPrice, "minSellingPrice": p.MinSellingPrice,
		"minStock": p.MinStock, "status": p.Status, "variants": vs,
	}
}

// UpdateProduct validates and rewrites the product and its variants
// atomically. Stock facts guard it: UOM, factor and type lock once history
// exists; a tracked product cannot become untracked while it has history or
// stock; a variant holding stock or history is never archived or renamed.
func (s *Service) UpdateProduct(ctx context.Context, actorID, id int64, in ProductInput) (*contracts.Product, error) {
	existing, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil {
		return nil, apperror.NotFound("Produk")
	}
	in = normalizeInput(in)
	if existing.Status == "archived" {
		return nil, apperror.Conflict("Produk sudah diarsipkan")
	}
	if fields, err := s.validateProduct(ctx, in, id); err != nil {
		return nil, wrapErr(err)
	} else if fields != nil {
		return nil, apperror.Validation("", fields)
	}
	state, err := s.stockState(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ae := checkUpdateRules(existing, in, state); ae != nil {
		return nil, ae
	}
	all, err := s.repo.AllVariantsByProduct(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	states, err := s.variantStates(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Untracked products have no stock states to consult; the hidden default
	// follows the code freely.
	variants, err := planVariants(variantPlanInput{
		ProductCode: in.Code, OldProductCode: existing.Code, ProductName: in.Name,
		Existing: all, Payload: in.Variants, States: states,
	})
	if err != nil {
		return nil, wrapErr(err)
	}
	p := s.toProduct(in)
	p.ID = id
	if p.Status == "" {
		p.Status = existing.Status
	}
	p.Variants = variants
	if err := s.repo.UpdateProduct(ctx, p, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	out, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{
		Action: "products.update", Entity: "product", EntityID: out.ID, BranchID: 0, ActorID: actorID, Note: out.Code,
		Before: productSnapshot(existing, all), After: productSnapshot(out, out.Variants),
	})
	return out, nil
}

// lockedFieldsChange reports whether in changes what stock history locks
// (units, factors, type) or the tracked flag, i.e. whether the stock facts
// are needed at all to judge the update.
func lockedFieldsChange(existing *contracts.Product, in ProductInput) bool {
	return in.BaseUOM != existing.BaseUOM || in.PurchaseUOM != existing.PurchaseUOM ||
		in.SalesUOM != existing.SalesUOM || in.PurchaseFactor != existing.PurchaseFactor ||
		in.SalesFactor != existing.SalesFactor || in.Type != existing.Type ||
		in.Tracked != existing.Tracked || (in.Status == "inactive" && existing.Status != "inactive")
}

// checkUpdateRules applies the stock-history guards to an update (shared by
// the form and the bulk import, so the import can never bypass them).
func checkUpdateRules(existing *contracts.Product, in ProductInput, state contracts.StockState) *apperror.AppError {
	if state.HasHistory {
		if in.BaseUOM != existing.BaseUOM || in.PurchaseUOM != existing.PurchaseUOM ||
			in.SalesUOM != existing.SalesUOM ||
			in.PurchaseFactor != existing.PurchaseFactor || in.SalesFactor != existing.SalesFactor {
			return apperror.Conflict("Satuan dan faktor konversi terkunci karena sudah ada riwayat stok")
		}
		if in.Type != existing.Type {
			return apperror.Conflict("Tipe produk terkunci karena sudah ada riwayat stok")
		}
	}
	if existing.Tracked && !in.Tracked && (state.HasHistory || state.Available > 0) {
		return apperror.Conflict("Pantau stok tidak dapat dimatikan karena produk sudah punya riwayat atau saldo stok")
	}
	if in.Status == "inactive" && existing.Status != "inactive" && state.HasHolds {
		return apperror.Conflict("Produk masih punya reservasi berjalan dan tidak dapat dinonaktifkan")
	}
	return nil
}

// ArchiveProduct archives a product. Products with stock on hand anywhere or
// live holds are rejected (KI-57).
func (s *Service) ArchiveProduct(ctx context.Context, actorID, id int64) error {
	existing, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil {
		return apperror.NotFound("Produk")
	}
	state, err := s.stockState(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if state.Available > 0 || state.HasHolds {
		return apperror.Conflict("Produk masih memiliki stok atau reservasi berjalan")
	}
	if err := s.repo.SetProductStatus(ctx, id, "archived", actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "products.archive", Entity: "product", EntityID: id, BranchID: 0, ActorID: actorID, Note: existing.Code})
	return nil
}

// ListResult is a paginated product page.
type ListResult struct {
	Products []*contracts.Product
	Total    int64
}

// ListFilter is the product list filter set.
type ListFilter = infrastructure.ProductFilter

// PriceIssueMinAboveSelling is the list's priceIssue value for data-health A8.
const PriceIssueMinAboveSelling = infrastructure.PriceIssueMinAboveSelling

// List searches products per word with optional filters.
func (s *Service) List(ctx context.Context, f ListFilter, page, limit int) (*ListResult, error) {
	total, err := s.repo.CountProducts(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	products, err := s.repo.ListProducts(ctx, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if products == nil {
		products = []*contracts.Product{}
	}
	return &ListResult{Products: products, Total: total}, nil
}

// SearchOptionsOpts tunes SearchOptionsFor beyond the filter.
type SearchOptionsOpts struct {
	// PreferStock floats products with sellable stock in BranchID to the top
	// of an otherwise equal relevance rank (legacy prefer_stock_available).
	// Stock is another module's fact, so this re-orders a widened candidate
	// window through the stock provider instead of joining across modules.
	PreferStock bool
	BranchID    int64
}

// preferStockWindowFactor widens the candidate window PreferStock re-orders.
const (
	preferStockWindowFactor = 5
	preferStockWindowMax    = 150
)

// SearchOptions returns rows for dropdowns and POS search, variants included
// (one grouped read for the page) so forms need no per-pick detail call.
func (s *Service) SearchOptions(ctx context.Context, f ListFilter, limit int) ([]*contracts.Product, error) {
	return s.SearchOptionsFor(ctx, f, limit, SearchOptionsOpts{})
}

// SearchOptionsFor is SearchOptions with ranking options. A typed search is
// always ranked by relevance (exact code, code prefix, name prefix, rest); a
// ProductID hydration ignores the default active-only status so an already
// selected (possibly later deactivated) product can still be shown.
func (s *Service) SearchOptionsFor(ctx context.Context, f ListFilter, limit int, o SearchOptionsOpts) ([]*contracts.Product, error) {
	if limit <= 0 || limit > 30 {
		limit = 20
	}
	if f.Status == "" && f.ID == nil {
		f.Status = "active"
	}
	f.Ranked = true
	fetch := limit
	preferStock := o.PreferStock && o.BranchID > 0 && s.stock != nil && f.ID == nil
	if preferStock {
		fetch = min(limit*preferStockWindowFactor, preferStockWindowMax)
	}
	products, err := s.repo.ListProducts(ctx, f, fetch, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if products == nil {
		products = []*contracts.Product{}
	}
	if preferStock && len(products) > 0 {
		products, err = s.preferAvailable(ctx, o.BranchID, products, f.Search, limit)
		if err != nil {
			return nil, err
		}
	}
	ids := make([]int64, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ID)
	}
	vs, err := s.repo.VariantsByProducts(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, p := range products {
		p.Variants = vs[p.ID]
		if p.Variants == nil {
			p.Variants = []*contracts.Variant{}
		}
	}
	return products, nil
}

// preferAvailable stable-sorts the window by (relevance rank, stock rank) and
// cuts it to limit. Stock rank: tracked with sellable stock 0, untracked 1,
// tracked without stock 2 (legacy stock_rank_order).
func (s *Service) preferAvailable(ctx context.Context, branchID int64, ps []*contracts.Product, search string, limit int) ([]*contracts.Product, error) {
	ids := make([]int64, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	avail, err := s.stock.AvailableByProducts(ctx, branchID, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	term := strings.ToLower(strings.TrimSpace(search))
	relevance := func(p *contracts.Product) int {
		if term == "" {
			return 0
		}
		switch {
		case strings.ToLower(p.Code) == term:
			return 0
		case strings.HasPrefix(strings.ToLower(p.Code), term):
			return 1
		case strings.HasPrefix(strings.ToLower(p.Name), term):
			return 2
		}
		return 3
	}
	stockRank := func(p *contracts.Product) int {
		switch {
		case !p.Tracked:
			return 1
		case avail[p.ID] > 0:
			return 0
		}
		return 2
	}
	sort.SliceStable(ps, func(i, j int) bool {
		ri, rj := relevance(ps[i]), relevance(ps[j])
		if ri != rj {
			return ri < rj
		}
		return stockRank(ps[i]) < stockRank(ps[j])
	})
	if len(ps) > limit {
		ps = ps[:limit]
	}
	return ps, nil
}

// StockAvailability returns the sellable base-UOM quantity per product for one
// branch in a single grouped read (product list column). A missing stock
// module or branch reads as no data (nil map), never as an error.
func (s *Service) StockAvailability(ctx context.Context, branchID int64, productIDs []int64) (map[int64]float64, error) {
	if s.stock == nil || branchID <= 0 || len(productIDs) == 0 {
		return nil, nil
	}
	m, err := s.stock.AvailableByProducts(ctx, branchID, productIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return m, nil
}
