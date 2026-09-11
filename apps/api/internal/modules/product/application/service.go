package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/domain"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// StockState is the product module's view of stock-held facts. The stock
// module supplies it once it exists; until then the hook stays nil and the
// history-dependent guards are inert — safe, because no stock row can exist
// before the stock module is implemented.
type StockState struct {
	// HasHistory is true once any movement ever touched the product.
	HasHistory bool
	// Available is the current sellable quantity in base UOM.
	Available float64
	// HasHolds is true while reservations or in-flight transfers reference it.
	HasHolds bool
}

// Service implements contracts.ProductClient plus catalog administration.
type Service struct {
	repo *infrastructure.Repository
	// audit receives best-effort trail records after catalog mutations.
	audit auditcontracts.AuditClient
	// StockCheck is wired by the composition root when the stock module
	// lands. Nil means "no history possible yet".
	StockCheck func(ctx context.Context, productID int64) (StockState, error)
}

// NewService wires catalog use cases.
func NewService(repo *infrastructure.Repository, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, audit: audit}
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

// CreateCategory inserts a category (code immutable once issued).
func (s *Service) CreateCategory(ctx context.Context, actorID int64, code, name string, parentID *int64) (*contracts.Category, error) {
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
	id, err := s.repo.CreateCategory(ctx, &contracts.Category{Code: code, Name: strings.TrimSpace(name), ParentID: parentID}, actorID)
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

// UpdateCategory rewrites name/parent.
func (s *Service) UpdateCategory(ctx context.Context, actorID, id int64, name string, parentID *int64) (*contracts.Category, error) {
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
	Variants        []VariantInput
}

// VariantInput is one variant row.
type VariantInput struct {
	Code      string
	Name      string
	Barcode   string
	IsDefault bool
}

func (s *Service) validateProduct(ctx context.Context, in ProductInput, excludeID int64) ([]apperror.FieldError, error) {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		fail("code", "wajib diisi")
	} else {
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
	}
	if in.CategoryID != nil {
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
	if in.Type != "barang" && in.Type != "jasa" {
		fail("type", "harus barang atau jasa")
	}
	for _, uom := range []struct {
		name, value string
	}{{"baseUom", in.BaseUOM}, {"purchaseUom", in.PurchaseUOM}, {"salesUom", in.SalesUOM}} {
		if strings.TrimSpace(uom.value) == "" {
			fail(uom.name, "wajib diisi")
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
	seen := map[string]bool{}
	for i, v := range in.Variants {
		vc := strings.ToUpper(strings.TrimSpace(v.Code))
		if vc == "" {
			fail("variants", "kode varian baris "+strconv.Itoa(i+1)+" wajib diisi")
			continue
		}
		if seen[vc] {
			fail("variants", "kode varian '"+vc+"' ganda")
		}
		seen[vc] = true
		if strings.TrimSpace(v.Name) == "" {
			fail("variants", "nama varian '"+vc+"' wajib diisi")
		}
		if bc := strings.TrimSpace(v.Barcode); bc != "" {
			if other, err := s.repo.VariantByBarcode(ctx, bc); err != nil {
				return nil, err
			} else if other != nil && other.ProductID != excludeID {
				fail("variants", "barcode '"+bc+"' sudah dipakai produk lain")
			}
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

// buildVariants injects the hidden default variant when none are supplied,
func buildVariants(in ProductInput) []*contracts.Variant {
	if len(in.Variants) == 0 {
		return []*contracts.Variant{{
			Code: strings.ToUpper(strings.TrimSpace(in.Code)), Name: strings.TrimSpace(in.Name),
			IsDefault: true,
		}}
	}
	out := make([]*contracts.Variant, 0, len(in.Variants))
	defaults := 0
	for _, v := range in.Variants {
		if v.IsDefault {
			defaults++
		}
		out = append(out, &contracts.Variant{
			Code: strings.ToUpper(strings.TrimSpace(v.Code)),
			Name: strings.TrimSpace(v.Name), Barcode: strings.TrimSpace(v.Barcode),
			IsDefault: v.IsDefault,
		})
	}
	if defaults == 0 {
		out[0].IsDefault = true
	}
	return out
}

// CreateProduct validates and inserts product + variants atomically.
func (s *Service) CreateProduct(ctx context.Context, actorID int64, in ProductInput) (*contracts.Product, error) {
	fields, err := s.validateProduct(ctx, in, 0)
	if err != nil {
		return nil, wrapErr(err)
	}
	if fields != nil {
		return nil, apperror.Validation("", fields)
	}
	id, err := s.repo.CreateProduct(ctx, &contracts.Product{
		Code: strings.ToUpper(strings.TrimSpace(in.Code)), Name: strings.TrimSpace(in.Name),
		CategoryID: in.CategoryID, Type: in.Type, Tracked: in.Tracked,
		BaseUOM: strings.TrimSpace(in.BaseUOM), PurchaseUOM: strings.TrimSpace(in.PurchaseUOM),
		SalesUOM:       strings.TrimSpace(in.SalesUOM),
		PurchaseFactor: in.PurchaseFactor, SalesFactor: in.SalesFactor,
		PurchasePrice: in.PurchasePrice, SellingPrice: in.SellingPrice,
		MinSellingPrice: in.MinSellingPrice, MinStock: in.MinStock,
		Variants: buildVariants(in),
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "products.create", Entity: "product", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

// UpdateProduct validates and rewrites product + variants atomically.
// UOM and type lock once stock history exists (guard inert until the stock
// module wires StockCheck — no history can exist before then).
func (s *Service) UpdateProduct(ctx context.Context, actorID, id int64, in ProductInput) (*contracts.Product, error) {
	existing, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil {
		return nil, apperror.NotFound("Produk")
	}
	if fields, err := s.validateProduct(ctx, in, id); err != nil {
		return nil, wrapErr(err)
	} else if fields != nil {
		return nil, apperror.Validation("", fields)
	}
	if s.StockCheck != nil {
		state, err := s.StockCheck(ctx, id)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if state.HasHistory {
			norm := func(u string) string { return strings.TrimSpace(u) }
			if norm(in.BaseUOM) != existing.BaseUOM || norm(in.PurchaseUOM) != existing.PurchaseUOM ||
				norm(in.SalesUOM) != existing.SalesUOM ||
				in.PurchaseFactor != existing.PurchaseFactor || in.SalesFactor != existing.SalesFactor {
				return nil, apperror.Conflict("Satuan dan faktor konversi terkunci karena sudah ada riwayat stok")
			}
			if in.Type != existing.Type {
				return nil, apperror.Conflict("Tipe produk terkunci karena sudah ada riwayat stok")
			}
		}
	}
	existing.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	existing.Name = strings.TrimSpace(in.Name)
	existing.CategoryID = in.CategoryID
	existing.Type = in.Type
	existing.Tracked = in.Tracked
	existing.BaseUOM = strings.TrimSpace(in.BaseUOM)
	existing.PurchaseUOM = strings.TrimSpace(in.PurchaseUOM)
	existing.SalesUOM = strings.TrimSpace(in.SalesUOM)
	existing.PurchaseFactor = in.PurchaseFactor
	existing.SalesFactor = in.SalesFactor
	existing.PurchasePrice = in.PurchasePrice
	existing.SellingPrice = in.SellingPrice
	existing.MinSellingPrice = in.MinSellingPrice
	existing.MinStock = in.MinStock
	existing.Variants = buildVariants(in)
	if err := s.repo.UpdateProduct(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "products.update", Entity: "product", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

// ArchiveProduct archives a product. Once the stock module wires StockCheck,
// products with sellable stock or live holds are rejected (KI-57).
func (s *Service) ArchiveProduct(ctx context.Context, actorID, id int64) error {
	existing, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil {
		return apperror.NotFound("Produk")
	}
	if s.StockCheck != nil {
		state, err := s.StockCheck(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if state.Available > 0 || state.HasHolds {
			return apperror.Conflict("Produk masih memiliki stok atau reservasi berjalan")
		}
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

// List searches products per word with optional filters.
func (s *Service) List(ctx context.Context, search string, categoryID *int64, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.CountProducts(ctx, search, categoryID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	products, err := s.repo.ListProducts(ctx, search, categoryID, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if products == nil {
		products = []*contracts.Product{}
	}
	return &ListResult{Products: products, Total: total}, nil
}

// SearchOptions returns compact rows for dropdowns and POS search.
func (s *Service) SearchOptions(ctx context.Context, search string) ([]*contracts.Product, error) {
	products, err := s.repo.ListProducts(ctx, search, nil, "active", 20, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if products == nil {
		products = []*contracts.Product{}
	}
	return products, nil
}
