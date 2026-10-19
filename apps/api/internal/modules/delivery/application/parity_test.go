package application

import (
	"strings"
	"testing"
	"time"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

func TestMovedAtFollowsDeliveryDate(t *testing.T) {
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	if got := movedAt(today.Format("2006-01-02") + " 00:00:00"); !got.IsZero() {
		t.Fatalf("today stays 'now' (zero), got %v", got)
	}
	if got := movedAt(today.AddDate(0, 0, 3).Format("2006-01-02")); !got.IsZero() {
		t.Fatalf("a future date stays 'now', got %v", got)
	}
	past := today.AddDate(0, 0, -5)
	got := movedAt(past.Format("2006-01-02") + " 14:30:00")
	want := time.Date(past.Year(), past.Month(), past.Day(), 0, 0, 0, 0, timeutil.Jakarta)
	if !got.Equal(want) {
		t.Fatalf("backdated SJ = start of its Jakarta day: want %v got %v", want, got)
	}
	if !movedAt("").IsZero() || !movedAt("garbage-date").IsZero() {
		t.Fatal("unparseable dates fall back to now")
	}
}

func TestExplainShortage(t *testing.T) {
	if explainShortage(nil, "SJ-1") != nil {
		t.Fatal("nil passes through")
	}
	other := apperror.Validation("x", nil)
	if explainShortage(other, "SJ-1") != other {
		t.Fatal("non-shortage errors pass through untouched")
	}
	err := explainShortage(apperror.Conflict("Stok tersedia tidak mencukupi (produk 1, lokasi 2)"), "SJ-1")
	ae, ok := err.(*apperror.AppError)
	if !ok || ae.Code != apperror.CodeConflict {
		t.Fatalf("still a conflict, got %v", err)
	}
	for _, want := range []string{"SJ-1", "tidak mencukupi (produk 1", "tetap draf", "batalkan"} {
		if !strings.Contains(ae.Message, want) {
			t.Fatalf("message must mention %q: %s", want, ae.Message)
		}
	}
}

func TestProofOwnerByPurpose(t *testing.T) {
	for purpose, want := range map[string]string{
		"":                          "delivery_proof",
		PurposeLocationPhoto:        "delivery_proof",
		PurposeSignedDocument:       "delivery_document",
		" delivery_signed_document": "delivery_document",
	} {
		got, err := proofOwner(purpose)
		if err != nil || got != want {
			t.Fatalf("purpose %q -> %q (%v), want %q", purpose, got, err, want)
		}
	}
	if _, err := proofOwner("selfie"); err == nil {
		t.Fatal("unknown purpose is rejected")
	}
}
