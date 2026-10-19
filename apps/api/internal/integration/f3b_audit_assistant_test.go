package integration

import (
	"strings"
	"testing"

	assistantapp "mini-erp/internal/modules/assistant/application"
	assistantcontracts "mini-erp/internal/modules/assistant/contracts"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
)

// TestAssistantAuditCarriesBeforeAfter (F3b-8/9): authorization and config
// changes record before/after; phone numbers are masked in the diff.
func TestAssistantAuditCarriesBeforeAfter(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	const phone = "6281100000077"

	a, err := svc.CreateAuthorization(ctx(), 5, assistantapp.AuthorizationInput{
		Phone: phone, Name: "Operator", AccessLevel: assistantcontracts.AccessAuthorizedParty,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.UpdateAuthorization(ctx(), 5, a.ID, assistantapp.AuthorizationInput{
		Phone: phone, Name: "Operator Baru", AccessLevel: assistantcontracts.AccessAuthorizedParty,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := svc.UpdateConfig(ctx(), 5, assistantcontracts.ModeRule, 33); err != nil {
		t.Fatalf("config: %v", err)
	}

	recs, _, err := testMods.audit.Service().Query(ctx(), auditcontracts.ListFilter{AllBranches: true, Action: "assistant.", Page: 1, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]auditcontracts.Record{}
	for _, r := range recs {
		seen[r.Action] = r
		if strings.Contains(string(r.Before)+string(r.After)+r.Note, phone) {
			t.Fatalf("full phone leaked in %s", r.Action)
		}
	}
	upd := seen["assistant.authorization_update"]
	if !strings.Contains(string(upd.Before), "Operator") || !strings.Contains(string(upd.After), "Operator Baru") {
		t.Fatalf("update diff: before=%s after=%s", upd.Before, upd.After)
	}
	cfg := seen["assistant.config_update"]
	if !strings.Contains(string(cfg.After), "33") || len(cfg.Before) == 0 {
		t.Fatalf("config diff: before=%s after=%s", cfg.Before, cfg.After)
	}
	if upd.ActorID != 5 {
		t.Fatalf("actor = %d", upd.ActorID)
	}
}
