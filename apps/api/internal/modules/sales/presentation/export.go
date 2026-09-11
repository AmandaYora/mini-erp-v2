package presentation

import (
	"net/http"

	"mini-erp/internal/shared/response"
)

// Export handles GET /api/v1/sales-orders/export?format=xlsx|pdf&search&
// status&channel — file download, same filters as the list.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	raw, name, err := h.svc.Export(r.Context(), branchID(r),
		q.Get("search"), q.Get("status"), q.Get("channel"), q.Get("format"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if len(name) > 4 && name[len(name)-4:] == ".pdf" {
		contentType = "application/pdf"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
