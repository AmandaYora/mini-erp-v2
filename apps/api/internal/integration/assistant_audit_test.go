package integration

import (
	"strings"
	"testing"

	assistantapp "mini-erp/internal/modules/assistant/application"
	assistantcontracts "mini-erp/internal/modules/assistant/contracts"
)

func auditNotes(t *testing.T, action, entity string) []string {
	t.Helper()
	records, _, err := testMods.audit.Service().List(ctx(), 0, action, entity, 1, 50)
	if err != nil {
		t.Fatalf("audit list %s: %v", action, err)
	}
	var notes []string
	for _, r := range records {
		notes = append(notes, r.Note)
	}
	return notes
}

// TestAssistantSensitiveActsAudited (F1-11): channel/auth/config writes leave
// one trail entry each (action/entity names per F3b-9); phone numbers never
// appear in full.
func TestAssistantSensitiveActsAudited(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	const phone = "6281100000005"

	a, err := svc.CreateAuthorization(ctx(), 0, assistantapp.AuthorizationInput{
		Phone: phone, Name: "Operator", AccessLevel: assistantcontracts.AccessAuthorizedParty,
	})
	if err != nil {
		t.Fatalf("create auth: %v", err)
	}
	if _, err := svc.UpdateAuthorization(ctx(), 0, a.ID, assistantapp.AuthorizationInput{
		Phone: phone, Name: "Operator 2", AccessLevel: assistantcontracts.AccessAuthorizedParty,
	}); err != nil {
		t.Fatalf("update auth: %v", err)
	}
	if err := svc.RevokeAuthorization(ctx(), 0, a.ID); err != nil {
		t.Fatalf("revoke auth: %v", err)
	}
	if _, err := svc.UpdateConfig(ctx(), 0, assistantcontracts.ModeRule, 20); err != nil {
		t.Fatalf("update config: %v", err)
	}
	if err := svc.Disconnect(ctx(), 0); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	for action, entity := range map[string]string{
		"assistant.authorization_create": "assistant_authorization",
		"assistant.authorization_update": "assistant_authorization",
		"assistant.authorization_revoke": "assistant_authorization",
		"assistant.config_update":        "assistant_config",
		"assistant.channel_disconnect":   "assistant_channel",
	} {
		if got := len(auditNotes(t, action, entity)); got != 1 {
			t.Fatalf("%s/%s: %d entri, want 1", action, entity, got)
		}
	}
	for _, note := range auditNotes(t, "assistant.authorization_create", "assistant_authorization") {
		if strings.Contains(note, phone) {
			t.Fatalf("catatan memuat nomor penuh: %q", note)
		}
		if !strings.Contains(note, "0005") {
			t.Fatalf("catatan harus memuat 4 digit terakhir: %q", note)
		}
	}
}
