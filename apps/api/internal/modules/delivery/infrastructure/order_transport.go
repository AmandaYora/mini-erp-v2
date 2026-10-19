package infrastructure

import (
	"context"
	"strings"

	"mini-erp/internal/modules/delivery/contracts"
)

// TransportByOrders joins the distinct driver names and vehicle plates of each
// order's live shipments (one grouped read, chunked by 500 ids).
func (r *Repository) TransportByOrders(ctx context.Context, orderIDs []int64) (map[int64]contracts.OrderTransport, error) {
	out := map[int64]contracts.OrderTransport{}
	for start := 0; start < len(orderIDs); start += 500 {
		end := start + 500
		if end > len(orderIDs) {
			end = len(orderIDs)
		}
		rows, err := r.q.OrderTransportByOrders(ctx, orderIDs[start:end])
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			t := out[row.SalesOrderID]
			t.Drivers = joinDistinct(t.Drivers, row.DriverName.String)
			t.Plates = joinDistinct(t.Plates, row.VehiclePlate.String)
			if t.Drivers != "" || t.Plates != "" {
				out[row.SalesOrderID] = t
			}
		}
	}
	return out, nil
}

// joinDistinct appends v to a ", "-joined list unless blank or already there.
func joinDistinct(list, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return list
	}
	if list == "" {
		return v
	}
	for _, x := range strings.Split(list, ", ") {
		if x == v {
			return list
		}
	}
	return list + ", " + v
}
