package application

import (
	"strings"

	"mini-erp/internal/shared/apperror"
)

// checkReturnDate refuses a return dated before the order's goods were first
// received (RT-04, legacy "tanggal retur tidak boleh lebih awal dari tanggal
// penerimaan"). Both sides compare by day; an empty receive date (nothing
// posted yet) never refuses — the quota check refuses that return anyway.
func checkReturnDate(returnDate, firstReceivedAt string) error {
	if len(firstReceivedAt) < 10 || len(returnDate) < 10 {
		return nil
	}
	day := strings.TrimSpace(firstReceivedAt)[:10]
	if strings.TrimSpace(returnDate)[:10] < day {
		return apperror.Validation("", []apperror.FieldError{{Field: "returnDate",
			Message: "tidak boleh lebih awal dari tanggal penerimaan barang (" + day + ")"}})
	}
	return nil
}
