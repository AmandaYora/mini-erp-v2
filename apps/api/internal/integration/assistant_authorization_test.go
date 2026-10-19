package integration

import (
	"testing"

	assistantapp "mini-erp/internal/modules/assistant/application"
	assistantcontracts "mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/shared/apperror"
)

// T-02: whitelist (nomor pengakses BOT) rules at the service level, the
// counterpart of legacy e2e 19-whatsapp-authorization-management (add / edit
// / delete a number) plus the duplicate, revoke and primary-owner rules.

func asstAppErr(t *testing.T, err error, code apperror.Code) *apperror.AppError {
	t.Helper()
	ae, ok := err.(*apperror.AppError)
	if !ok || ae.Code != code {
		t.Fatalf("error = %v, want AppError %s", err, code)
	}
	return ae
}

func asstIn(phone, name, level string, primary bool) assistantapp.AuthorizationInput {
	return assistantapp.AuthorizationInput{Phone: phone, Name: name, AccessLevel: level, IsPrimaryOwner: primary}
}

func TestAssistantAuthorizationAddEditDelete(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()

	// Add: phone is normalized to digits, name trimmed.
	a, err := svc.CreateAuthorization(ctx(), 0, asstIn("+62 811-0000-0001", "  Operator BOT  ", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	if a.Phone != "6281100000001" || a.Name != "Operator BOT" || a.Status != assistantcontracts.AuthActive || a.ID == 0 {
		t.Fatalf("created = %+v", a)
	}
	list, _ := svc.ListAuthorizations(ctx(), false)
	if len(list) != 1 || list[0].Phone != "6281100000001" {
		t.Fatalf("list = %+v", list)
	}

	// Edit name and phone.
	up, err := svc.UpdateAuthorization(ctx(), 0, a.ID, asstIn("6282-1000-0002", "Operator Edit", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil || up.Phone != "628210000002" || up.Name != "Operator Edit" {
		t.Fatalf("update = %+v %v", up, err)
	}
	list, _ = svc.ListAuthorizations(ctx(), false)
	if len(list) != 1 || list[0].Name != "Operator Edit" || list[0].Phone != "628210000002" {
		t.Fatalf("list after edit = %+v", list)
	}

	// Delete (soft revoke): gone from the active list, kept in history.
	if err := svc.RevokeAuthorization(ctx(), 0, a.ID); err != nil {
		t.Fatal(err)
	}
	if active, _ := svc.ListAuthorizations(ctx(), false); len(active) != 0 {
		t.Fatalf("active after revoke = %+v", active)
	}
	all, _ := svc.ListAuthorizations(ctx(), true)
	if len(all) != 1 || all[0].Status != assistantcontracts.AuthRevoked {
		t.Fatalf("history after revoke = %+v", all)
	}
	// Empty lists are non-nil (JSON [] not null).
	freshDB(t)
	if l, err := svc.ListAuthorizations(ctx(), true); err != nil || l == nil || len(l) != 0 {
		t.Fatalf("empty list = %v %v", l, err)
	}
}

func TestAssistantAuthorizationValidation(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	cases := map[string]assistantapp.AuthorizationInput{
		"phone too short": asstIn("1234567", "A", assistantcontracts.AccessOwner, false),
		"phone too long":  asstIn("1234567890123456", "A", assistantcontracts.AccessOwner, false),
		"phone letters":   asstIn("abc", "A", assistantcontracts.AccessOwner, false),
		"blank name":      asstIn("6281100000010", "   ", assistantcontracts.AccessOwner, false),
		"unknown level":   asstIn("6281100000010", "A", "superuser", false),
	}
	for name, in := range cases {
		if _, err := svc.CreateAuthorization(ctx(), 0, in); err == nil {
			t.Errorf("%s: create must fail", name)
		} else {
			asstAppErr(t, err, apperror.CodeValidation)
		}
	}
	if n := count(t, "SELECT COUNT(*) FROM assistant_authorizations"); n != 0 {
		t.Fatalf("rejected input persisted %d rows", n)
	}

	a, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000010", "Ok", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateAuthorization(ctx(), 0, a.ID, asstIn("12", "Ok", assistantcontracts.AccessAuthorizedParty, false)); err == nil {
		t.Fatal("update with invalid phone must fail")
	} else {
		asstAppErr(t, err, apperror.CodeValidation)
	}
}

func TestAssistantAuthorizationDuplicateAndReactivate(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	const phone = "6281100000020"

	a, err := svc.CreateAuthorization(ctx(), 0, asstIn(phone, "Satu", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate of an active number (even with different formatting) = 409.
	_, err = svc.CreateAuthorization(ctx(), 0, asstIn("+62 811-0000-0020", "Dua", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeConflict)
	if n := count(t, "SELECT COUNT(*) FROM assistant_authorizations"); n != 1 {
		t.Fatalf("duplicate persisted: %d rows", n)
	}

	// Editing another row onto a taken number = 409; editing onto itself is fine.
	b, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000021", "Tiga", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateAuthorization(ctx(), 0, b.ID, asstIn(phone, "Tiga", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeConflict)
	if _, err := svc.UpdateAuthorization(ctx(), 0, b.ID, asstIn("6281100000021", "Tiga Baru", assistantcontracts.AccessAuthorizedParty, false)); err != nil {
		t.Fatalf("self update: %v", err)
	}

	// Revoked number: re-adding reactivates the same row in place with new data.
	if err := svc.RevokeAuthorization(ctx(), 0, a.ID); err != nil {
		t.Fatal(err)
	}
	// A revoked row still blocks edits onto its number (it is "taken").
	_, err = svc.UpdateAuthorization(ctx(), 0, b.ID, asstIn(phone, "Tiga", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeConflict)

	re, err := svc.CreateAuthorization(ctx(), 0, asstIn(phone, "Satu Lagi", assistantcontracts.AccessOwner, false))
	if err != nil {
		t.Fatal(err)
	}
	if re.ID != a.ID || re.Name != "Satu Lagi" || re.Status != assistantcontracts.AuthActive || re.AccessLevel != assistantcontracts.AccessOwner {
		t.Fatalf("reactivated = %+v (want id %d reused)", re, a.ID)
	}
	if n := count(t, "SELECT COUNT(*) FROM assistant_authorizations"); n != 2 {
		t.Fatalf("reactivation created a new row: %d rows", n)
	}
	active, _ := svc.ListAuthorizations(ctx(), false)
	if len(active) != 2 {
		t.Fatalf("active = %+v", active)
	}
}

func TestAssistantAuthorizationRevokeAndMissing(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	a, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000030", "X", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeAuthorization(ctx(), 0, a.ID); err != nil {
		t.Fatal(err)
	}
	// Second revoke, unknown id and non-positive id all read as missing.
	for _, id := range []int64{a.ID, 99999, 0, -3} {
		asstAppErr(t, svc.RevokeAuthorization(ctx(), 0, id), apperror.CodeNotFound)
	}
	// A revoked row cannot be edited; the number must be re-added.
	_, err = svc.UpdateAuthorization(ctx(), 0, a.ID, asstIn("6281100000030", "Y", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeNotFound)
	_, err = svc.UpdateAuthorization(ctx(), 0, 0, asstIn("6281100000030", "Y", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeNotFound)
	_, err = svc.UpdateAuthorization(ctx(), 0, 99999, asstIn("6281100000031", "Y", assistantcontracts.AccessAuthorizedParty, false))
	asstAppErr(t, err, apperror.CodeNotFound)
}

func primaries(t *testing.T) []string {
	t.Helper()
	rows, err := testMods.db.Query(`SELECT phone FROM assistant_authorizations WHERE is_primary_owner = 1 AND status = 'active' ORDER BY phone`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestAssistantAuthorizationPrimaryOwnerRules(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()

	// An authorized party never becomes primary, flag or not.
	if _, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000040", "Staf", assistantcontracts.AccessAuthorizedParty, true)); err != nil {
		t.Fatal(err)
	}
	if p := primaries(t); len(p) != 0 {
		t.Fatalf("authorized party flagged primary: %v", p)
	}

	// First owner without an active primary becomes primary automatically.
	o1, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000041", "Owner 1", assistantcontracts.AccessOwner, false))
	if err != nil || !o1.IsPrimaryOwner {
		t.Fatalf("first owner = %+v %v, want auto primary", o1, err)
	}
	// A second owner without the flag stays non-primary.
	o2, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000042", "Owner 2", assistantcontracts.AccessOwner, false))
	if err != nil || o2.IsPrimaryOwner {
		t.Fatalf("second owner = %+v %v", o2, err)
	}
	// Promoting the second demotes the first: exactly one primary.
	if _, err := svc.UpdateAuthorization(ctx(), 0, o2.ID, asstIn("6281100000042", "Owner 2", assistantcontracts.AccessOwner, true)); err != nil {
		t.Fatal(err)
	}
	if p := primaries(t); len(p) != 1 || p[0] != "6281100000042" {
		t.Fatalf("primaries after promote = %v", p)
	}
	// Creating a third owner flagged primary also moves the flag.
	if _, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000043", "Owner 3", assistantcontracts.AccessOwner, true)); err != nil {
		t.Fatal(err)
	}
	if p := primaries(t); len(p) != 1 || p[0] != "6281100000043" {
		t.Fatalf("primaries after create = %v", p)
	}
	// The primary is listed first.
	list, _ := svc.ListAuthorizations(ctx(), false)
	if len(list) == 0 || list[0].Phone != "6281100000043" || !list[0].IsPrimaryOwner {
		t.Fatalf("primary must sort first: %+v", list)
	}
	// Downgrading the primary to authorized party clears the flag.
	if _, err := svc.UpdateAuthorization(ctx(), 0, list[0].ID, asstIn("6281100000043", "Owner 3", assistantcontracts.AccessAuthorizedParty, true)); err != nil {
		t.Fatal(err)
	}
	if p := primaries(t); len(p) != 0 {
		t.Fatalf("downgraded primary kept the flag: %v", p)
	}
	// Revoking the primary drops the flag; the next owner added takes it.
	if _, err := svc.UpdateAuthorization(ctx(), 0, o1.ID, asstIn("6281100000041", "Owner 1", assistantcontracts.AccessOwner, true)); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeAuthorization(ctx(), 0, o1.ID); err != nil {
		t.Fatal(err)
	}
	if p := primaries(t); len(p) != 0 {
		t.Fatalf("revoked primary kept the flag: %v", p)
	}
	o4, err := svc.CreateAuthorization(ctx(), 0, asstIn("6281100000044", "Owner 4", assistantcontracts.AccessOwner, false))
	if err != nil || !o4.IsPrimaryOwner {
		t.Fatalf("owner after primary revoked = %+v %v, want auto primary", o4, err)
	}
}

// A sender that is unknown, or whose number was revoked, never gets a run:
// the whitelist is the gate in front of the engine (inbound path).
func TestAssistantInboundRespectsWhitelist(t *testing.T) {
	freshDB(t)
	svc := testMods.assistant.Service()
	const phone = "6281100000050"

	runs := func() int { return count(t, "SELECT COUNT(*) FROM assistant_runs") }
	svc.HandleInbound(phone, "stok") // unknown number
	if runs() != 0 || count(t, "SELECT COUNT(*) FROM assistant_messages") != 0 {
		t.Fatal("unknown sender must not create runs or messages")
	}

	a, err := svc.CreateAuthorization(ctx(), 0, asstIn(phone, "Op", assistantcontracts.AccessAuthorizedParty, false))
	if err != nil {
		t.Fatal(err)
	}
	svc.HandleInbound(phone, "bantuan")
	if runs() != 1 {
		t.Fatalf("authorized sender should create one run, got %d", runs())
	}
	if _, err := svc.Simulate(ctx(), 0, 0, "6281100000051", "bantuan"); err == nil {
		t.Fatal("simulate for a non-whitelisted number must be forbidden")
	} else {
		asstAppErr(t, err, apperror.CodeForbidden)
	}

	if err := svc.RevokeAuthorization(ctx(), 0, a.ID); err != nil {
		t.Fatal(err)
	}
	before := runs()
	svc.HandleInbound(phone, "bantuan")
	if runs() != before {
		t.Fatal("revoked sender must not create runs")
	}
	if _, err := svc.Simulate(ctx(), 0, 0, phone, "bantuan"); err == nil {
		t.Fatal("simulate for a revoked number must be forbidden")
	} else {
		asstAppErr(t, err, apperror.CodeForbidden)
	}
}
