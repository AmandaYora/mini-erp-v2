package infrastructure

import (
	"context"
	"errors"
	"time"
)

// ErrLockBusy means another process held the advisory lock past the wait.
var ErrLockBusy = errors.New("advisory lock busy")

// WithAdvisoryLock runs fn while holding the MySQL named lock `name` (scoped
// to the current database, so the test and rehearsal databases on the same
// server never share one). shared/doclock serializes goroutines inside one
// process only; the cost ledger is also written by cmd/costrebuild, a
// separate process, so the server's costing and a restatement must meet at
// the database. The lock lives on one dedicated connection for fn's duration.
func (r *Repository) WithAdvisoryLock(ctx context.Context, name string, wait time.Duration, fn func() error) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var got *int64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(CONCAT(DATABASE(), ':', ?), ?)`, name, int(wait.Seconds())).Scan(&got); err != nil {
		return err
	}
	if got == nil || *got != 1 {
		return ErrLockBusy
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT RELEASE_LOCK(CONCAT(DATABASE(), ':', ?))`, name)
	}()
	return fn()
}
