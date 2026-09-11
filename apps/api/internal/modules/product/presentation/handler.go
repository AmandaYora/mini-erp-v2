package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the product module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires product endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

func categoryView(c *contracts.Category) map[string]any {
	return map[string]any{
		"id": c.ID, "code": c.Code, "name": c.Name,
		"parentId": c.ParentID, "status": c.Status,
	}
}

func productView(p *contracts.Product) map[string]any {
	variants := make([]any, 0, len(p.Variants))
	for _, v := range p.Variants {
		variants = append(variants, map[string]any{
			"id": v.ID, "code": v.Code, "name": v.Name, "barcode": v.Barcode,
			"isDefault": v.IsDefault, "status": v.Status,
		})
	}
	return map[string]any{
		"id": p.ID, "code": p.Code, "name": p.Name,
		"categoryId": p.CategoryID, "categoryName": p.CategoryName,
		"type": p.Type, "tracked": p.Tracked,
		"baseUom": p.BaseUOM, "purchaseUom": p.PurchaseUOM, "salesUom": p.SalesUOM,
		"purchaseFactor": p.PurchaseFactor, "salesFactor": p.SalesFactor,
		"purchasePrice": p.PurchasePrice, "sellingPrice": p.SellingPrice,
		"minSellingPrice": p.MinSellingPrice, "minStock": p.MinStock,
		"status": p.Status, "variants": variants,
	}
}

// ListCategories handles GET /api/v1/product-categories.
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.ListCategories(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(cats))
	for _, c := range cats {
		items = append(items, categoryView(c))
	}
	response.OK(w, "Data kategori produk", items)
}

// CreateCategory handles POST /api/v1/product-categories.
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code     string `json:"code" validate:"required"`
		Name     string `json:"name" validate:"required"`
		ParentID *int64 `json:"parentId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), actorID(r), in.Code, in.Name, in.ParentID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Kategori dibuat", categoryView(c))
}

// UpdateCategory handles PUT /api/v1/product-categories/{id}.
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Kategori"))
		return
	}
	var in struct {
		Name     string `json:"name" validate:"required"`
		ParentID *int64 `json:"parentId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	c, appErr := h.svc.UpdateCategory(r.Context(), actorID(r), id, in.Name, in.ParentID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Kategori disimpan", categoryView(c))
}

// ArchiveCategory handles POST /api/v1/product-categories/{id}/archive.
func (h *Handler) ArchiveCategory(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Kategori"))
		return
	}
	if appErr := h.svc.ArchiveCategory(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Kategori diarsipkan", nil)
}

type variantPayload struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Barcode   string `json:"barcode"`
	IsDefault bool   `json:"isDefault"`
}

type productPayload struct {
	Code            string           `json:"code" validate:"required"`
	Name            string           `json:"name" validate:"required"`
	CategoryID      *int64           `json:"categoryId"`
	Type            string           `json:"type"`
	Tracked         *bool            `json:"tracked"`
	BaseUOM         string           `json:"baseUom" validate:"required"`
	PurchaseUOM     string           `json:"purchaseUom" validate:"required"`
	SalesUOM        string           `json:"salesUom" validate:"required"`
	PurchaseFactor  float64          `json:"purchaseFactor"`
	SalesFactor     float64          `json:"salesFactor"`
	PurchasePrice   int64            `json:"purchasePrice"`
	SellingPrice    int64            `json:"sellingPrice"`
	MinSellingPrice int64            `json:"minSellingPrice"`
	MinStock        float64          `json:"minStock"`
	Variants        []variantPayload `json:"variants"`
}

func toInput(in productPayload) application.ProductInput {
	tracked := true
	if in.Tracked != nil {
		tracked = *in.Tracked
	}
	typ := in.Type
	if typ == "" {
		typ = "barang"
	}
	pf, sf := in.PurchaseFactor, in.SalesFactor
	if pf == 0 {
		pf = 1
	}
	if sf == 0 {
		sf = 1
	}
	variants := make([]application.VariantInput, 0, len(in.Variants))
	for _, v := range in.Variants {
		variants = append(variants, application.VariantInput{
			Code: v.Code, Name: v.Name, Barcode: v.Barcode, IsDefault: v.IsDefault,
		})
	}
	return application.ProductInput{
		Code: in.Code, Name: in.Name, CategoryID: in.CategoryID, Type: typ, Tracked: tracked,
		BaseUOM: in.BaseUOM, PurchaseUOM: in.PurchaseUOM, SalesUOM: in.SalesUOM,
		PurchaseFactor: pf, SalesFactor: sf,
		PurchasePrice: in.PurchasePrice, SellingPrice: in.SellingPrice,
		MinSellingPrice: in.MinSellingPrice, MinStock: in.MinStock, Variants: variants,
	}
}

// List handles GET /api/v1/products.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var categoryID *int64
	if raw := q.Get("categoryId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			categoryID = &id
		}
	}
	res, err := h.svc.List(r.Context(), q.Get("search"), categoryID, q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Products))
	for _, p := range res.Products {
		items = append(items, productView(p))
	}
	response.Paginated(w, "Data produk", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// SearchOptions handles GET /api/v1/products/search-options.
func (h *Handler) SearchOptions(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.SearchOptions(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(products))
	for _, p := range products {
		items = append(items, map[string]any{
			"id": p.ID, "code": p.Code, "name": p.Name, "sellingPrice": p.SellingPrice,
		})
	}
	response.OK(w, "Opsi produk", items)
}

// Get handles GET /api/v1/products/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	p, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if p == nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	response.OK(w, "Data produk", productView(p))
}

// QR handles GET /api/v1/products/{id}/qr — PNG label bytes (J2). The
// payload choice (variant barcode → code, else default barcode → first
// barcode → code) matches the scan loop, so a printed label scans straight
// into the cart. Optional ?variantId= pins one variant, ?size= sets pixels
// (128–1024, default 256).
func (h *Handler) QR(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	q := r.URL.Query()
	var variantID int64
	if raw := q.Get("variantId"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			variantID = v
		}
	}
	size := 0
	if raw := q.Get("size"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			size = n
		}
	}
	png, _, appErr := h.svc.QRLabel(r.Context(), id, variantID, size)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// QRExport handles GET /api/v1/products/qr-codes/export — the whole
// printable catalog as a ZIP of QR PNGs, foldered per category (J2).
func (h *Handler) QRExport(w http.ResponseWriter, r *http.Request) {
	targets, appErr := h.svc.QRTargets(r.Context())
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+application.QRExportFileName+`"`)
	w.WriteHeader(http.StatusOK)
	_ = application.QRZip(targets, w)
}

// Create handles POST /api/v1/products.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in productPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), actorID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Produk dibuat", productView(p))
}

// Update handles PUT /api/v1/products/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	var in productPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, appErr := h.svc.UpdateProduct(r.Context(), actorID(r), id, toInput(in))
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Produk disimpan", productView(p))
}

// Archive handles POST /api/v1/products/{id}/archive.
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	if appErr := h.svc.ArchiveProduct(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Produk diarsipkan", nil)
}
