package application

import "testing"

func ids(c []fifoCand) []int64 {
	out := make([]int64, 0, len(c))
	for _, x := range c {
		out = append(out, x.id)
	}
	return out
}

// F2a-6c: bills are paid oldest-DUE first, not oldest-created first.
func TestSortFIFOByDueDate(t *testing.T) {
	c := []fifoCand{
		{id: 1, orderDate: "2026-09-01", dueDate: "2026-10-30"}, // created first, due last
		{id: 2, orderDate: "2026-09-10", dueDate: "2026-09-20"}, // created later, due first
		{id: 3, orderDate: "2026-09-05", dueDate: "2026-10-05"},
	}
	sortFIFO(c)
	want := []int64{2, 3, 1}
	for i, id := range ids(c) {
		if id != want[i] {
			t.Fatalf("order = %v, want %v", ids(c), want)
		}
	}
}

// Without a due date (cash/COD) the order date ranks the bill, mixed freely
// with dated net bills.
func TestSortFIFOFallsBackToOrderDate(t *testing.T) {
	c := []fifoCand{
		{id: 1, orderDate: "2026-09-15"},                        // COD, ranked by 09-15
		{id: 2, orderDate: "2026-09-01", dueDate: "2026-10-01"}, // net, due 10-01
		{id: 3, orderDate: "2026-09-02"},                        // COD, ranked by 09-02
	}
	sortFIFO(c)
	want := []int64{3, 1, 2}
	for i, id := range ids(c) {
		if id != want[i] {
			t.Fatalf("order = %v, want %v", ids(c), want)
		}
	}
}

// Ties break by order date and then by id, deterministically.
func TestSortFIFOTieBreak(t *testing.T) {
	c := []fifoCand{
		{id: 9, orderDate: "2026-09-02", dueDate: "2026-10-01"},
		{id: 4, orderDate: "2026-09-01", dueDate: "2026-10-01"},
		{id: 3, orderDate: "2026-09-01", dueDate: "2026-10-01"},
	}
	sortFIFO(c)
	want := []int64{3, 4, 9}
	for i, id := range ids(c) {
		if id != want[i] {
			t.Fatalf("order = %v, want %v", ids(c), want)
		}
	}
}
