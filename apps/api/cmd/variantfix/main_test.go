package main

import "testing"

func TestPlanOnlyTouchesETLDefaults(t *testing.T) {
	legacy := []legacyVariant{
		{ID: 1, Hidden: true},                 // "Standar" -> hide
		{ID: 2, Sort: 3},                      // colour with an order
		{ID: 3, Inactive: true, Hidden: true}, // switched off by a user in legacy
		{ID: 4, Hidden: true},                 // holds stock beside a visible colour: not hidden
		{ID: 7, Hidden: true},                 // holds stock, sole variant of its product: hidden
		{ID: 5, Sort: 2},                      // edited after go-live (sort 9): left alone
		{ID: 6, Hidden: true},                 // already fixed
		{ID: 99, Hidden: true},                // not migrated
	}
	idMap := map[int64]int64{1: 11, 2: 12, 3: 13, 4: 14, 5: 15, 6: 16, 7: 17}
	rows := map[int64]targetVariant{
		11: {ID: 11, ProductID: 100, Status: "active"},
		12: {ID: 12, ProductID: 100, Status: "active"},
		13: {ID: 13, ProductID: 101, Status: "archived"},
		14: {ID: 14, ProductID: 102, Status: "active", OnHand: 4},
		18: {ID: 18, ProductID: 102, Status: "active"},
		17: {ID: 17, ProductID: 105, Status: "active", OnHand: 9},
		15: {ID: 15, ProductID: 103, Status: "active", Sort: 9},
		16: {ID: 16, ProductID: 104, Status: "active", Hidden: true},
	}
	changes, r := plan(legacy, idMap, rows)
	if r.Mapped != 7 || r.Unmapped != 1 || r.Hide != 3 || r.Sort != 1 || r.Inactive != 1 || r.SkippedEdited != 1 || len(r.SkippedStock) != 1 || r.AlreadyDone != 1 {
		t.Fatalf("report = %+v", r)
	}
	if len(changes) != 4 || changes[0].VariantID != 11 || !changes[0].Hide || changes[1].Sort == nil || *changes[1].Sort != 3 ||
		!changes[2].Inactive || !changes[2].Hide || changes[3].VariantID != 17 || !changes[3].Hide {
		t.Fatalf("changes = %+v", changes)
	}
	// Applying the plan and planning again changes nothing (idempotent).
	rows[11] = targetVariant{ID: 11, ProductID: 100, Status: "active", Hidden: true}
	rows[12] = targetVariant{ID: 12, ProductID: 100, Status: "active", Sort: 3}
	rows[13] = targetVariant{ID: 13, ProductID: 101, Status: "inactive", Hidden: true}
	rows[17] = targetVariant{ID: 17, ProductID: 105, Status: "active", OnHand: 9, Hidden: true}
	if again, _ := plan(legacy, idMap, rows); len(again) != 0 {
		t.Fatalf("second run = %+v", again)
	}
}
