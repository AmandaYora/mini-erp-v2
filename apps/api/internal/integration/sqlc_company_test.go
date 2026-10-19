package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"mini-erp/internal/modules/company/contracts"
	companyinfra "mini-erp/internal/modules/company/infrastructure"
)

// Characterization tests for the company Repository (F8-18).
func TestSqlcCompanyRepository(t *testing.T) {
	ctx := context.Background()
	freshDB(t)
	repo := companyinfra.NewRepository(testMods.db)

	if p, err := repo.GetProfile(ctx); err != nil || p != nil {
		t.Fatalf("empty profile: %v %v", p, err)
	}
	if s, err := repo.GetSettings(ctx); err != nil || len(s) != 0 {
		t.Fatalf("empty settings: %v %v", s, err)
	}

	// Insert: empty optionals become NULL and read back as "".
	hookID := int64(0)
	p, err := repo.SaveProfile(ctx, &contracts.Profile{Name: "Toko A", City: "Bandung"}, 5,
		func(_ context.Context, tx *sql.Tx, id int64) error {
			hookID = id
			var n int
			return tx.QueryRow("SELECT COUNT(*) FROM company_profile").Scan(&n) // visible inside the tx
		})
	if err != nil || p.ID == 0 || hookID != p.ID {
		t.Fatalf("insert: %+v hook=%d err=%v", p, hookID, err)
	}
	got, err := repo.GetProfile(ctx)
	if err != nil || got == nil || got.ID != p.ID || got.Name != "Toko A" || got.City != "Bandung" ||
		got.LegalName != "" || got.Address != "" || got.Phone != "" || got.Email != "" || got.TaxID != "" {
		t.Fatalf("get after insert: %+v %v", got, err)
	}
	var legalNull, createdBy, updatedBy sql.NullString
	if err := testMods.db.QueryRow("SELECT legal_name, created_by, updated_by FROM company_profile WHERE id = ?", p.ID).
		Scan(&legalNull, &createdBy, &updatedBy); err != nil || legalNull.Valid || createdBy.String != "5" || updatedBy.String != "5" {
		t.Fatalf("raw row: %+v %+v %+v %v", legalNull, createdBy, updatedBy, err)
	}

	// Hook error rolls the insert/update back.
	boom := errors.New("boom")
	if _, err := repo.SaveProfile(ctx, &contracts.Profile{Name: "X"}, 1,
		func(context.Context, *sql.Tx, int64) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hook err: %v", err)
	}
	if g, _ := repo.GetProfile(ctx); g.Name != "Toko A" {
		t.Fatalf("update must roll back: %+v", g)
	}

	// Update: full overwrite incl. clearing to NULL; singleton row retained.
	p2, err := repo.SaveProfile(ctx, &contracts.Profile{Name: "Toko B", LegalName: "PT B", Address: "Jl 1", Phone: "08", Email: "a@b.c", TaxID: "99"}, 6, nil)
	if err != nil || p2.ID != p.ID {
		t.Fatalf("update: %+v %v", p2, err)
	}
	got, _ = repo.GetProfile(ctx)
	if got.Name != "Toko B" || got.LegalName != "PT B" || got.Address != "Jl 1" || got.City != "" || got.Phone != "08" || got.Email != "a@b.c" || got.TaxID != "99" {
		t.Fatalf("after update: %+v", got)
	}
	var cnt int
	var updBy int64
	var createdAtOld, updatedAt sql.NullTime
	_ = testMods.db.QueryRow("SELECT COUNT(*), MAX(updated_by), MAX(created_at), MAX(updated_at) FROM company_profile").Scan(&cnt, &updBy, &createdAtOld, &updatedAt)
	if cnt != 1 || updBy != 6 || !updatedAt.Time.After(createdAtOld.Time) {
		t.Fatalf("singleton/updated_by/updated_at: cnt=%d by=%d c=%v u=%v", cnt, updBy, createdAtOld, updatedAt)
	}

	// Settings: upsert per key, merge keeps siblings, "" stores NULL and reads "".
	hooked := false
	if err := repo.SaveSettings(ctx, map[string]string{"a": `{"x":1}`, "b": "", "c": "3"}, 7,
		func(context.Context, *sql.Tx) error { hooked = true; return nil }); err != nil || !hooked {
		t.Fatalf("save settings: %v hooked=%v", err, hooked)
	}
	if err := repo.SaveSettings(ctx, map[string]string{"a": `{"x":2}`}, 8, nil); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := repo.GetSettings(ctx)
	if len(s) != 3 || s["a"] != `{"x":2}` || s["b"] != "" || s["c"] != "3" {
		t.Fatalf("settings: %v", s)
	}
	var bNull sql.NullString
	var by string
	_ = testMods.db.QueryRow("SELECT setting_value FROM company_settings WHERE setting_key='b'").Scan(&bNull)
	_ = testMods.db.QueryRow("SELECT updated_by FROM company_settings WHERE setting_key='a'").Scan(&by)
	if bNull.Valid || by != "8" {
		t.Fatalf("settings raw: b=%v by=%s", bNull, by)
	}
	// Empty map is a no-op that still runs the hook; hook error rolls back.
	if err := repo.SaveSettings(ctx, map[string]string{"c": "changed"}, 1,
		func(context.Context, *sql.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("settings hook err: %v", err)
	}
	if s, _ := repo.GetSettings(ctx); s["c"] != "3" {
		t.Fatalf("settings must roll back: %v", s)
	}
	if err := repo.SaveSettings(ctx, nil, 1, nil); err != nil {
		t.Fatalf("empty save: %v", err)
	}
}
