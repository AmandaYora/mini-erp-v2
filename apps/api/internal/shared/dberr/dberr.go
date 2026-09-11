package dberr

import (
	"errors"

	"github.com/go-sql-driver/mysql"

	"mini-erp/internal/shared/apperror"
)

// Map translates storage errors into user-facing failures. Integrity
// violations (duplicate keys from double-submits or races that slipped past
// pre-checks) become 409 with a non-leaking Indonesian message instead of a
// bare 500 — the last line of defense behind application-level checks.
func Map(err error) *apperror.AppError {
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return apperror.Conflict("Data duplikat, periksa kembali isian")
		case 1451, 1452:
			return apperror.Conflict("Data masih dipakai dan tidak dapat diubah")
		}
	}
	return apperror.Internal(err)
}
