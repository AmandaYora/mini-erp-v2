package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/tableexport"
)

// Export handles GET /api/v1/purchase-orders/export?format=xlsx|pdf plus every
// filter of the list (search, status, paymentTerms, partyId, from,
// to) — the file contains exactly the rows the list shows, all pages. The
// download is built by keyset paging and streamed (F4-11).
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	f, _, err := parseListFilter(r)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	release, err := tableexport.Acquire(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer release()
	tableexport.ExtendWriteDeadline(w)
	file, err := h.svc.Export(r.Context(), authcontracts.BranchID(r), f, r.URL.Query().Get("format"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	tableexport.Stream(w, file)
}
