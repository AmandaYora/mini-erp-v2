package presentation

import (
	"net/http"
	"strconv"
	"strings"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// MediaList handles GET /api/v1/products/{id}/media.
func (h *Handler) MediaList(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	if err := h.requireProduct(r, id); err != nil {
		response.FailErr(w, err)
		return
	}
	files, err := media.List(r.Context(), mediacontracts.OwnerProduct, id)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(files))
	for _, f := range files {
		url, err := media.GetURL(r.Context(), f)
		if err != nil {
			response.FailErr(w, err)
			return
		}
		items = append(items, mediaView(url, f))
	}
	response.OK(w, "Foto produk", items)
}

// MediaUpload handles POST /api/v1/products/{id}/media (multipart, images only).
func (h *Handler) MediaUpload(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	if err := h.requireWritableProduct(r, id); err != nil {
		response.FailErr(w, err)
		return
	}
	fh, header, appErr := httpx.MultipartFile(r, 6<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	// Optional form fields: sort_order (gallery position, 0-9999) and
	// is_primary (make this photo the primary one; the first photo of a
	// product is primary anyway). Both are validated before any byte is stored.
	sortOrder := 0
	if raw := strings.TrimSpace(r.FormValue("sort_order")); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 0 || n > 9999 {
			response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "sort_order", Message: "harus angka 0-9999"}}))
			return
		}
		sortOrder = n
	}
	wantPrimary := false
	if raw := strings.ToLower(strings.TrimSpace(r.FormValue("is_primary"))); raw != "" {
		switch raw {
		case "1", "true":
			wantPrimary = true
		case "0", "false":
		default:
			response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "is_primary", Message: "harus true atau false"}}))
			return
		}
	}
	f, err := media.Upload(r.Context(), mediacontracts.OwnerProduct, id, mediacontracts.Upload{
		OriginalName: header.Filename, MIME: header.Header.Get("Content-Type"), Content: fh,
		Accept: []string{"image/jpeg", "image/png", "image/webp"}, SortOrder: sortOrder,
	}, authcontracts.ActorID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if wantPrimary && !f.IsPrimary {
		if err := media.SetPrimary(r.Context(), mediacontracts.OwnerProduct, id, f.ID, authcontracts.ActorID(r)); err != nil {
			response.FailErr(w, err)
			return
		}
		f.IsPrimary = true
	}
	url, err := media.GetURL(r.Context(), f)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Foto diunggah", mediaView(url, f))
}

// MediaSetPrimary handles POST /api/v1/products/{id}/media/{mediaId}/primary.
func (h *Handler) MediaSetPrimary(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	mediaID, err := httpx.PathInt(r, "mediaId")
	if err != nil {
		response.Fail(w, apperror.NotFound("File"))
		return
	}
	if err := h.requireWritableProduct(r, id); err != nil {
		response.FailErr(w, err)
		return
	}
	if err := media.SetPrimary(r.Context(), mediacontracts.OwnerProduct, id, mediaID, authcontracts.ActorID(r)); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Foto utama diganti", nil)
}

// MediaArchive handles DELETE /api/v1/products/{id}/media/{mediaId}.
func (h *Handler) MediaArchive(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	mediaID, err := httpx.PathInt(r, "mediaId")
	if err != nil {
		response.Fail(w, apperror.NotFound("File"))
		return
	}
	if err := h.requireWritableProduct(r, id); err != nil {
		response.FailErr(w, err)
		return
	}
	if err := media.Archive(r.Context(), mediacontracts.OwnerProduct, id, mediaID, authcontracts.ActorID(r)); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Foto dihapus", nil)
}

func (h *Handler) requireProduct(r *http.Request, id int64) error {
	p, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		return err
	}
	if p == nil {
		return apperror.NotFound("Produk")
	}
	return nil
}

// requireWritableProduct additionally refuses archived products: history is
// frozen, so photos of an archived product can be listed but not changed.
func (h *Handler) requireWritableProduct(r *http.Request, id int64) error {
	p, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		return err
	}
	if p == nil {
		return apperror.NotFound("Produk")
	}
	if p.Status == "archived" {
		return apperror.Conflict("Produk sudah diarsipkan")
	}
	return nil
}

func mediaView(url string, f *mediacontracts.MediaFile) map[string]any {
	return map[string]any{
		"id": f.ID, "originalName": f.OriginalName, "mime": f.MIME,
		"sizeBytes": f.SizeBytes, "isPrimary": f.IsPrimary, "url": url,
		"width": f.Width, "height": f.Height, "originalSizeBytes": f.OriginalSizeBytes,
	}
}
