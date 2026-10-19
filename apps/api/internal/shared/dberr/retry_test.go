package dberr

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func mysqlErrNum(num uint16) error {
	return &mysql.MySQLError{Number: num, Message: "tiruan"}
}

// TestIsRetryable pins the transient set: only 1213/1205 retry.
func TestIsRetryable(t *testing.T) {
	for num, want := range map[uint16]bool{1213: true, 1205: true, 1062: false, 1451: false, 2006: false} {
		if got := IsRetryable(mysqlErrNum(num)); got != want {
			t.Fatalf("IsRetryable(%d) = %v, want %v", num, got, want)
		}
	}
	if IsRetryable(errors.New("biasa")) {
		t.Fatal("non-MySQL error must not retry")
	}
	if IsRetryable(nil) {
		t.Fatal("nil must not retry")
	}
}

// TestSleepBeforeRetry: first attempt never waits; a cancelled context aborts
// the backoff instead of sleeping through it.
func TestSleepBeforeRetry(t *testing.T) {
	if err := sleepBeforeRetry(context.Background(), 0); err != nil {
		t.Fatalf("attempt 0 must not wait: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepBeforeRetry(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ctx: got %v, want context.Canceled", err)
	}
}
