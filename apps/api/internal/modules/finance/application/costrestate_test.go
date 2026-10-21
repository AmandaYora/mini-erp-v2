package application

import (
	"testing"

	"mini-erp/internal/modules/finance/infrastructure"
)

// A cost-only row (supplement, revaluation) replays where it was written —
// after the real rows the ledger held then, before the ones costed after it —
// not at the end its synthetic movement id would put it.
func TestChainOrderPlacesCostOnlyRowsWhereTheyTookEffect(t *testing.T) {
	syn := infrastructure.SyntheticCostBase
	chain := []infrastructure.RestateRow{ // as read: by stock movement id
		{ID: 1, MovementID: 10},
		{ID: 3, MovementID: 11},
		{ID: 5, MovementID: 12},
		{ID: 2, MovementID: syn},     // written after movement 10 only
		{ID: 4, MovementID: syn + 1}, // written after movements 10 and 11
	}
	got := chainOrder(chain)
	want := []int64{1, 2, 3, 4, 5}
	for i, r := range got {
		if r.ID != want[i] {
			t.Fatalf("order = %v, want ledger ids %v", ledgerRowIDs(got), want)
		}
	}
	plain := []infrastructure.RestateRow{{ID: 2, MovementID: 5}, {ID: 1, MovementID: 7}}
	if out := chainOrder(plain); out[0].ID != 2 || out[1].ID != 1 {
		t.Fatalf("real rows reordered: %v", ledgerRowIDs(out))
	}
}

func ledgerRowIDs(rows []infrastructure.RestateRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
