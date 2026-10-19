package housekeeping

import "testing"

func TestResolvePrecedence(t *testing.T) {
	settings := map[string]string{KeyAudit: "365", KeySession: "abc", KeyReservation: "-3", KeyAssistant: "45"}
	got := Resolve(settings, Retention{AssistantDays: 10})
	want := Retention{AuditDays: 365, SessionDays: 30, ReservationDays: 7, AssistantDays: 10, IdleThreadDays: 180}
	if got != want {
		t.Fatalf("Resolve = %+v, want %+v", got, want)
	}
	if d := Resolve(nil, Retention{}); d != Defaults() {
		t.Fatalf("no settings/env must give defaults, got %+v", d)
	}
}
