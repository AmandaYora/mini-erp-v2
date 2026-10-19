package dberr

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Retryable MySQL codes: 1213 deadlock, 1205 lock wait timeout. Both mean
// "try the same transaction again" — never a user error.
const (
	maxAttempts = 4 // 1 try + 3 retries (F2a-0)
)

// IsRetryable reports whether err is a transient lock failure worth
// retrying (1213 deadlock, 1205 lock timeout).
func IsRetryable(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1213 || mysqlErr.Number == 1205
	}
	return false
}

var backoffs = []time.Duration{20 * time.Millisecond, 60 * time.Millisecond, 150 * time.Millisecond}

// sleepBeforeRetry waits the attempt's backoff plus up to 10 ms jitter so
// colliding transactions do not retry in lockstep. First attempt never sleeps.
func sleepBeforeRetry(ctx context.Context, attempt int) error {
	if attempt <= 0 {
		return nil
	}
	var wait time.Duration
	if attempt-1 < len(backoffs) {
		wait = backoffs[attempt-1]
	} else {
		wait = backoffs[len(backoffs)-1]
	}
	wait += time.Duration(rand.Int63n(int64(10 * time.Millisecond)))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// RetryTx runs fn in a fresh transaction, retrying the whole unit on lock
// failures (F2a-0). fn must be side-effect free outside tx: every attempt
// re-executes it from scratch. Non-retryable errors return immediately.
func RetryTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if sleepErr := sleepBeforeRetry(ctx, attempt); sleepErr != nil {
			return sleepErr
		}
		var tx *sql.Tx
		tx, err = db.BeginTx(ctx, nil)
		if err != nil {
			if IsRetryable(err) {
				continue
			}
			return err
		}
		err = func() error {
			defer func() { _ = tx.Rollback() }()
			if err := fn(tx); err != nil {
				return err
			}
			return tx.Commit()
		}()
		if err == nil || !IsRetryable(err) {
			return err
		}
	}
	return err
}

// IsDuplicate reports a MySQL duplicate-key error (1062): the unique key
// already holds the row, which idempotent flows read as "someone else won".
func IsDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
