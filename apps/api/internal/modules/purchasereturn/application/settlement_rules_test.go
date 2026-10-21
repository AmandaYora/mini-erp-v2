package application

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/shared/apperror"
)

func conflictMsg(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to contain %q", err, want)
	}
}

// The return value first reduces what we owe the supplier; only the excess can
// be refunded or kept as supplier balance (K-04b/K-04c).
func TestPurchaseSettlementExcessOnly(t *testing.T) {
	p := positionOf(100000, 60000, nil)
	if got := p.ExternalRemaining(); got != 40000 {
		t.Fatalf("external remaining = %d, want 40000", got)
	}
	if want := []string{contracts.SettleRefund, contracts.SettleSupplierCredit}; !reflect.DeepEqual(p.Allowed(), want) {
		t.Fatalf("allowed = %v, want %v", p.Allowed(), want)
	}
	if err := p.validate(contracts.SettleSupplierCredit, 40000); err != nil {
		t.Fatalf("credit the whole excess: %v", err)
	}
	conflictMsg(t, p.validate(contracts.SettleRefund, 40001), "sisa 40000")
	conflictMsg(t, p.validate(contracts.SettleReducePayable, 1), "dilakukan otomatis")
}

// A return the open bill fully absorbs has no excess at all.
func TestPurchaseSettlementFullyOffset(t *testing.T) {
	p := positionOf(50000, 50000, nil)
	if len(p.Allowed()) != 0 {
		t.Fatalf("allowed = %v, want none", p.Allowed())
	}
	conflictMsg(t, p.validate(contracts.SettleRefund, 1), "sepenuhnya mengurangi hutang")
	conflictMsg(t, p.validate(contracts.SettleSupplierCredit, 1), "sepenuhnya mengurangi hutang")
}

// The sales vocabulary no longer exists on the purchase side (K-04b).
func TestPurchaseSettlementRejectsSalesTypes(t *testing.T) {
	p := positionOf(100000, 0, nil)
	for _, typ := range []string{"collect_payment", "reduce_receivable", "customer_credit", "bogus"} {
		var ae *apperror.AppError
		err := p.validate(typ, 1000)
		if !errors.As(err, &ae) || ae.Code != apperror.CodeValidation || len(ae.Fields) != 1 ||
			!strings.Contains(ae.Fields[0].Message, "tidak dikenal") {
			t.Fatalf("%s: err = %v, want a validation error on field type", typ, err)
		}
	}
}

func TestPurchaseSettlementConsumesRemaining(t *testing.T) {
	rows := []*contracts.Settlement{{Type: contracts.SettleRefund, Amount: 30000}}
	p := positionOf(100000, 20000, rows)
	if got := p.ExternalRemaining(); got != 50000 {
		t.Fatalf("remaining = %d, want 50000", got)
	}
}

func TestPurchaseOffsetAmount(t *testing.T) {
	for _, c := range []struct{ grand, paid, total, want int64 }{
		{100, 0, 30, 30},
		{100, 80, 30, 20},
		{100, 100, 30, 0},
		{100, 130, 30, 0},
	} {
		if got := offsetAmount(c.grand, c.paid, c.total); got != c.want {
			t.Errorf("offsetAmount(%d,%d,%d) = %d, want %d", c.grand, c.paid, c.total, got, c.want)
		}
	}
}
