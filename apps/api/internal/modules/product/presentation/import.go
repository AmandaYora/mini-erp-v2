package presentation

import (
	"net/http"

	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Template handles GET /api/v1/product-imports/template (workbook download).
func (h *Handler) Template(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-produk.xlsx"`)
	if err := h.svc.Template(w); err != nil {
		// Headers already sent; the download is truncated. Log-grade failure
		// only — generation has no inputs to fail on.
		return
	}
}

// Preview handles POST /api/v1/product-imports/preview (multipart dry-run).
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	fh, _, appErr := httpx.MultipartFile(r, maxBytes)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	prev, err := h.svc.PreviewImport(r.Context(), fh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau impor", prev)
}

// Commit handles POST /api/v1/product-imports/commit (multipart, resumable).
func (h *Handler) Commit(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	fh, _, appErr := httpx.MultipartFile(r, maxBytes)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	res, err := h.svc.CommitImport(r.Context(), actorID(r), fh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Impor selesai", res)
}
