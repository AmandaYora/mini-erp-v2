package application

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
)

func conflictMsg(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to contain %q", err, want)
	}
}

// A plain return owes the customer its whole value; the automatic offset eats
// into it first and only the rest can be refunded or kept as balance (K-04c).
func TestSalesSettlementReturnOnly(t *testing.T) {
	p := positionOf(contracts.ReturnModeReturnOnly, 100000, 0, 30000, nil, true)
	if got := p.ExternalRemaining(); got != 70000 {
		t.Fatalf("external remaining = %d, want 70000", got)
	}
	if want := []string{contracts.SettleRefund, contracts.SettleCustomerCredit}; !reflect.DeepEqual(p.Allowed(), want) {
		t.Fatalf("allowed = %v, want %v", p.Allowed(), want)
	}
	if err := p.validate(contracts.SettleRefund, 70000); err != nil {
		t.Fatalf("refund of the whole external part: %v", err)
	}
	conflictMsg(t, p.validate(contracts.SettleCustomerCredit, 70001), "Melebihi sisa")
	// collect_payment on a plain return is the K-04c headline rejection.
	conflictMsg(t, p.validate(contracts.SettleCollectPayment, 1000), "Tambah bayar hanya untuk retur tukar")
	// Reducing the receivable is the automatic offset, never a manual row.
	conflictMsg(t, p.validate(contracts.SettleReduceReceivable, 1000), "dilakukan otomatis")
}

func TestSalesSettlementCreditNeedsParty(t *testing.T) {
	p := positionOf(contracts.ReturnModeReturnOnly, 50000, 0, 0, nil, false)
	if want := []string{contracts.SettleRefund}; !reflect.DeepEqual(p.Allowed(), want) {
		t.Fatalf("walk-in allowed = %v, want %v", p.Allowed(), want)
	}
	conflictMsg(t, p.validate(contracts.SettleCustomerCredit, 1000), "data customer")
	if err := p.validate(contracts.SettleRefund, 50000); err != nil {
		t.Fatalf("walk-in refund: %v", err)
	}
}

// A return that the open bill fully absorbs leaves nothing to refund or keep.
func TestSalesSettlementFullyOffset(t *testing.T) {
	p := positionOf(contracts.ReturnModeReturnOnly, 40000, 0, 40000, nil, true)
	if len(p.Allowed()) != 0 || p.ExternalRemaining() != 0 {
		t.Fatalf("allowed = %v remaining = %d, want none", p.Allowed(), p.ExternalRemaining())
	}
	conflictMsg(t, p.validate(contracts.SettleRefund, 1), "sepenuhnya mengurangi sisa piutang")
	conflictMsg(t, p.validate(contracts.SettleCustomerCredit, 1), "sepenuhnya mengurangi sisa piutang")
}

// Settled rows consume the external part; the automatic reduce row does not.
func TestSalesSettlementConsumesRemaining(t *testing.T) {
	rows := []*contracts.Settlement{
		{Type: contracts.SettleReduceReceivable, Amount: 20000, Auto: true},
		{Type: contracts.SettleRefund, Amount: 30000},
	}
	p := positionOf(contracts.ReturnModeReturnOnly, 100000, 0, 20000, rows, true)
	if got := p.ExternalRemaining(); got != 50000 {
		t.Fatalf("remaining = %d, want 50000", got)
	}
	conflictMsg(t, p.validate(contracts.SettleCustomerCredit, 50001), "sisa 50000")
}

// An exchange that costs more is collected, never refunded, and the
// collection is capped at the difference (K-04c).
func TestSalesSettlementExchangeCostsMore(t *testing.T) {
	p := positionOf(contracts.ReturnModeExchange, 100000, 25000, 0, nil, true)
	if want := []string{contracts.SettleCollectPayment}; !reflect.DeepEqual(p.Allowed(), want) {
		t.Fatalf("allowed = %v, want %v", p.Allowed(), want)
	}
	if err := p.validate(contracts.SettleCollectPayment, 25000); err != nil {
		t.Fatalf("collect the whole surcharge: %v", err)
	}
	conflictMsg(t, p.validate(contracts.SettleCollectPayment, 25001), "selisih yang harus dibayar")
	conflictMsg(t, p.validate(contracts.SettleRefund, 1000), "tidak menyisakan nilai")
	conflictMsg(t, p.validate(contracts.SettleCustomerCredit, 1000), "tidak menyisakan nilai")
	// after a partial collection only the rest is open
	p = positionOf(contracts.ReturnModeExchange, 100000, 25000, 0, []*contracts.Settlement{{Type: contracts.SettleCollectPayment, Amount: 10000}}, true)
	if p.ChargeRemaining() != 15000 {
		t.Fatalf("charge remaining = %d, want 15000", p.ChargeRemaining())
	}
}

// An exchange that costs less owes the customer the difference only.
func TestSalesSettlementExchangeCostsLess(t *testing.T) {
	p := positionOf(contracts.ReturnModeExchange, 100000, -40000, 10000, nil, true)
	if p.Owed != 40000 || p.ExternalRemaining() != 30000 {
		t.Fatalf("owed %d remaining %d, want 40000 / 30000", p.Owed, p.ExternalRemaining())
	}
	conflictMsg(t, p.validate(contracts.SettleCollectPayment, 1000), "Tambah bayar hanya untuk retur tukar")
}

// A zero-difference exchange settles nothing.
func TestSalesSettlementExchangeEven(t *testing.T) {
	p := positionOf(contracts.ReturnModeExchange, 100000, 0, 0, nil, true)
	if len(p.Allowed()) != 0 {
		t.Fatalf("allowed = %v, want none", p.Allowed())
	}
	conflictMsg(t, p.validate(contracts.SettleRefund, 1), "tidak menyisakan nilai")
}

func TestSalesSettlementUnknownType(t *testing.T) {
	p := positionOf(contracts.ReturnModeReturnOnly, 1000, 0, 0, nil, true)
	var ae *apperror.AppError
	err := p.validate("bogus", 1)
	if !errors.As(err, &ae) || ae.Code != apperror.CodeValidation || len(ae.Fields) != 1 ||
		!strings.Contains(ae.Fields[0].Message, "tidak dikenal") {
		t.Fatalf("err = %v, want a validation error on field type", err)
	}
}

func TestSalesOffsetAmount(t *testing.T) {
	for _, c := range []struct{ grand, paid, owed, want int64 }{
		{100, 0, 30, 30},  // bill larger than the return: all of it offsets
		{100, 80, 30, 20}, // only 20 still open
		{100, 100, 30, 0}, // fully paid: nothing to offset
		{100, 120, 30, 0}, // over-paid never goes negative
	} {
		if got := offsetAmount(c.grand, c.paid, c.owed); got != c.want {
			t.Errorf("offsetAmount(%d,%d,%d) = %d, want %d", c.grand, c.paid, c.owed, got, c.want)
		}
	}
}
