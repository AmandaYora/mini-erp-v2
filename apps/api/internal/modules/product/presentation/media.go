package presentation

import (
	"net/http"
	"strings"

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
	if !strings.HasPrefix(header.Header.Get("Content-Type"), "image/") {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "hanya gambar (JPG/PNG/WebP)"}}))
		return
	}
	f, err := media.Upload(r.Context(), mediacontracts.OwnerProduct, id, mediacontracts.Upload{
		OriginalName: header.Filename, MIME: header.Header.Get("Content-Type"), Content: fh,
		Accept: []string{"image/"},
	}, actorID(r))
	if err != nil {
		response.FailErr(w, err)
		return
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
	if err := media.SetPrimary(r.Context(), mediacontracts.OwnerProduct, id, mediaID, actorID(r)); err != nil {
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
	if err := media.Archive(r.Context(), mediacontracts.OwnerProduct, id, mediaID, actorID(r)); err != nil {
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
	}
}
