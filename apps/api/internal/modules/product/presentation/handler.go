package presentation

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the product module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires product endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func categoryView(c *contracts.Category) map[string]any {
	return map[string]any{
		"id": c.ID, "code": c.Code, "name": c.Name,
		"parentId": c.ParentID, "sortOrder": c.SortOrder, "status": c.Status,
	}
}

func variantView(v *contracts.Variant) map[string]any {
	attrs := v.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	return map[string]any{
		"id": v.ID, "code": v.Code, "name": v.Name, "barcode": v.Barcode,
		"isDefault": v.IsDefault, "status": v.Status,
		"isHidden": v.IsHidden, "sortOrder": v.SortOrder, "attributes": attrs,
	}
}

func productView(p *contracts.Product) map[string]any {
	variants := make([]any, 0, len(p.Variants))
	for _, v := range p.Variants {
		variants = append(variants, variantView(v))
	}
	attrs := p.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	return map[string]any{
		"summary": p.Summary, "attributes": attrs,
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
		Code      string `json:"code" validate:"required"`
		Name      string `json:"name" validate:"required"`
		ParentID  *int64 `json:"parentId"`
		SortOrder *int   `json:"sortOrder"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), authcontracts.ActorID(r), in.Code, in.Name, in.ParentID, in.SortOrder)
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
		Name      string `json:"name" validate:"required"`
		ParentID  *int64 `json:"parentId"`
		SortOrder *int   `json:"sortOrder"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	c, appErr := h.svc.UpdateCategory(r.Context(), authcontracts.ActorID(r), id, in.Name, in.ParentID, in.SortOrder)
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
	if appErr := h.svc.ArchiveCategory(r.Context(), authcontracts.ActorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Kategori diarsipkan", nil)
}

type variantPayload struct {
	ID         int64             `json:"id"`
	Code       string            `json:"code"`
	Name       string            `json:"name"`
	Barcode    string            `json:"barcode"`
	IsDefault  bool              `json:"isDefault"`
	Status     string            `json:"status"`
	SortOrder  int               `json:"sortOrder"`
	Attributes map[string]string `json:"attributes"`
	IsHidden   *bool             `json:"isHidden"`
}

type productPayload struct {
	Code            string  `json:"code" validate:"required"`
	Name            string  `json:"name" validate:"required"`
	CategoryID      *int64  `json:"categoryId"`
	Type            string  `json:"type"`
	Tracked         *bool   `json:"tracked"`
	BaseUOM         string  `json:"baseUom" validate:"required"`
	PurchaseUOM     string  `json:"purchaseUom" validate:"required"`
	SalesUOM        string  `json:"salesUom" validate:"required"`
	PurchaseFactor  float64 `json:"purchaseFactor"`
	SalesFactor     float64 `json:"salesFactor"`
	PurchasePrice   int64   `json:"purchasePrice"`
	SellingPrice    int64   `json:"sellingPrice"`
	MinSellingPrice int64   `json:"minSellingPrice"`
	MinStock        float64 `json:"minStock"`
	// Status is active|inactive (archiving has its own endpoint).
	Status     string            `json:"status"`
	Summary    string            `json:"summary"`
	Attributes map[string]string `json:"attributes"`
	// Variants: omitted keeps the stored set, [] reverts to the implicit default.
	Variants []variantPayload `json:"variants"`
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
	var variants []application.VariantInput
	if in.Variants != nil {
		variants = make([]application.VariantInput, 0, len(in.Variants))
	}
	for _, v := range in.Variants {
		variants = append(variants, application.VariantInput{
			ID: v.ID, Code: v.Code, Name: v.Name, Barcode: v.Barcode, IsDefault: v.IsDefault,
			Status: v.Status, SortOrder: v.SortOrder, Attributes: v.Attributes, Hidden: v.IsHidden,
		})
	}
	return application.ProductInput{
		Code: in.Code, Name: in.Name, CategoryID: in.CategoryID, Type: typ, Tracked: tracked,
		BaseUOM: in.BaseUOM, PurchaseUOM: in.PurchaseUOM, SalesUOM: in.SalesUOM,
		PurchaseFactor: pf, SalesFactor: sf,
		PurchasePrice: in.PurchasePrice, SellingPrice: in.SellingPrice,
		MinSellingPrice: in.MinSellingPrice, MinStock: in.MinStock, Variants: variants,
		Status: in.Status, Summary: in.Summary, Attributes: in.Attributes,
	}
}

// filterFromQuery reads the shared product filter set from the query string.
func filterFromQuery(q url.Values) application.ListFilter {
	f := application.ListFilter{Search: q.Get("search"), Status: q.Get("status"), Type: q.Get("type")}
	if f.Search == "" {
		f.Search = q.Get("q")
	}
	if raw := q.Get("categoryId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			f.CategoryID = &id
		}
	}
	if q.Get("priceIssue") == application.PriceIssueMinAboveSelling {
		f.PriceIssue = application.PriceIssueMinAboveSelling
	}
	switch strings.ToLower(q.Get("duplicateName")) {
	case "true", "1":
		f.DuplicateName = true
	}
	switch strings.ToLower(q.Get("tracked")) {
	case "true", "1":
		t := true
		f.Tracked = &t
	case "false", "0":
		t := false
		f.Tracked = &t
	}
	return f
}

// List handles GET /api/v1/products. Each row carries its primary photo URL
// (one grouped read for the whole page, no per-row lookup).
func (h *Handler) List(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	f := filterFromQuery(q)
	f.Ranked = true // a typed search lists the best match first (PR-03)
	res, err := h.svc.List(r.Context(), f, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	ids := make([]int64, 0, len(res.Products))
	for _, p := range res.Products {
		ids = append(ids, p.ID)
	}
	// Stock needs stock.view + an active branch; cost needs a role that works
	// with cost (product maintenance or purchasing). One grouped stock read for
	// the whole page — never per row (PR-05).
	sess := authcontracts.SessionFromContext(r.Context())
	showStock := sess.HasPermission("stock.view") && sess.HasBranch()
	showCost := sess.HasPermission("products.update") || sess.HasPermission("purchasing.view")
	var avail map[int64]float64
	if showStock {
		// Stock is a bonus column: a failed lookup must not fail the list.
		if m, err := h.svc.StockAvailability(r.Context(), sess.BranchID, ids); err == nil {
			avail = m
		} else {
			showStock = false
		}
	}
	thumbs := map[int64]string{}
	if media != nil && len(ids) > 0 {
		// A broken thumbnail lookup must never fail the product list.
		if m, err := media.PrimaryByOwners(r.Context(), mediacontracts.OwnerProduct, ids); err == nil {
			thumbs = m
		}
	}
	items := make([]any, 0, len(res.Products))
	for _, p := range res.Products {
		v := productView(p)
		if p.ArchivedAt != nil {
			v["archivedAt"] = timeutil.WireTimeWIB(p.ArchivedAt)
		}
		if !showCost {
			delete(v, "purchasePrice")
		}
		if showStock && p.Tracked {
			base := avail[p.ID]
			factor := p.SalesFactor
			if factor <= 0 {
				factor = 1
			}
			v["stockAvailable"] = base / factor
			v["stockUom"] = p.SalesUOM
			v["belowMinStock"] = p.MinStock > 0 && base <= p.MinStock
		}
		if url, ok := thumbs[p.ID]; ok {
			v["primaryImageUrl"] = url
		} else {
			v["primaryImageUrl"] = nil
		}
		items = append(items, v)
	}
	response.Paginated(w, "Data produk", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// SearchOptions handles GET /api/v1/products/search-options: compact rows
// with units, prices, type and variants so order/PO forms need no per-pick
// detail call. Accepts q, limit (1-30), status, type, tracked, categoryId,
// productId (hydrate one option), page (1-based, scroll-to-load: every match is
// reachable) and preferStock=true (sellable stock first).
func (h *Handler) SearchOptions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 20
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = n
	}
	f := filterFromQuery(q)
	// productId hydrates one selected option without a new search (PR-03).
	if raw := q.Get("productId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			f.ID = &id
		}
	}
	opts := application.SearchOptionsOpts{}
	// page pages through every match (scroll-to-load pickers); absent = the
	// first rows only, as before.
	if raw := q.Get("page"); raw != "" {
		opts.Page, _ = pagination.Parse(raw, "")
	}
	if v := strings.ToLower(q.Get("preferStock")); v == "true" || v == "1" {
		opts.PreferStock = true
		opts.BranchID = authcontracts.BranchID(r)
	}
	products, err := h.svc.SearchOptionsFor(r.Context(), f, limit, opts)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(products))
	for _, p := range products {
		variants := make([]any, 0, len(p.Variants))
		for _, v := range p.Variants {
			variants = append(variants, variantView(v))
		}
		items = append(items, map[string]any{
			"id": p.ID, "code": p.Code, "name": p.Name, "sellingPrice": p.SellingPrice,
			"baseUom": p.BaseUOM, "purchaseUom": p.PurchaseUOM, "salesUom": p.SalesUOM,
			"purchaseFactor": p.PurchaseFactor, "salesFactor": p.SalesFactor,
			"purchasePrice": p.PurchasePrice, "minSellingPrice": p.MinSellingPrice,
			"type": p.Type, "tracked": p.Tracked, "status": p.Status, "variants": variants,
		})
	}
	response.OK(w, "Opsi produk", items)
}

// ScanLookup handles GET /api/v1/products/scan?code=: what a scanned/typed
// barcode, variant code or product code points to, for the product list's
// "Pindai" (open the product detail). Same resolution order as the POS scan
// but answered from the catalog alone, so it needs products.view, not
// stock.view; an inactive or archived product still resolves (the catalog may
// show it). Unknown or ambiguous codes are 404.
func (h *Handler) ScanLookup(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}}))
		return
	}
	m, err := h.svc.ResolveScanCode(r.Context(), code)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if m == nil || m.Product == nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	data := map[string]any{
		"id": m.Product.ID, "code": m.Product.Code, "name": m.Product.Name,
		"status": m.Product.Status, "via": m.Via, "variantRequired": m.VariantRequired,
	}
	if m.Variant != nil {
		data["variantId"] = m.Variant.ID
	}
	response.OK(w, "Produk ditemukan", data)
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
	p, err := h.svc.CreateProduct(r.Context(), authcontracts.ActorID(r), toInput(in))
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
	p, appErr := h.svc.UpdateProduct(r.Context(), authcontracts.ActorID(r), id, toInput(in))
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
	if appErr := h.svc.ArchiveProduct(r.Context(), authcontracts.ActorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Produk diarsipkan", nil)
}
