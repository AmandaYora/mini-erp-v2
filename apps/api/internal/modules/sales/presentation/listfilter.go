package presentation

import (
	"net/http"

	"mini-erp/internal/modules/sales/application"
	"mini-erp/internal/shared/listquery"
)

// orderSort whitelists the sortable columns of the order list.
var orderSort = &listquery.SortSpec{
	Allowed: map[string]string{
		"id":     "id",
		"date":   "order_date",
		"number": "number",
		"total":  "grand_total",
		"status": "status",
	},
	Default:     "id",
	DefaultDesc: true,
}

// parseListFilter reads the filter shared by GET /sales-orders and
// GET /sales-orders/export: search, status (CSV), channel, paymentTerms,
// partyId, from, to, sort, dir, page, limit. Malformed values are a 400 that
// names the field instead of silently widening the list.
func parseListFilter(r *http.Request) (application.ListFilter, listquery.Common, error) {
	q := r.URL.Query()
	c, err := listquery.ParseCommon(q, orderSort)
	if err != nil {
		return application.ListFilter{}, c, err
	}
	f := application.ListFilter{
		Search:        c.Search,
		Statuses:      c.Statuses,
		Channel:       q.Get("channel"),
		PaymentTerms:  q.Get("paymentTerms"),
		PaymentStatus: q.Get("paymentStatus"),
		PartyID:       c.PartyID,
		From:          c.From,
		To:            c.To,
	}
	if c.Sort != "id" || !c.Desc {
		f.OrderBy = c.OrderBy("id DESC")
	}
	return f, c, nil
}
