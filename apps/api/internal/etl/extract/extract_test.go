package extract_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/registry"
)

func TestExtractCountsAndDeterminism(t *testing.T) {
	env := etltest.Setup(t)
	ldb, err := legacydb.Open(env.DSN, etltest.Prefix, etltest.CompanyID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir1, dir2 := t.TempDir(), t.TempDir()
	m1, err := extract.Run(context.Background(), ldb, dir1, false, log)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := extract.Run(context.Background(), ldb, dir2, false, log)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range registry.All {
		a, b := m1.Tables[ent.Name], m2.Tables[ent.Name]
		if a.SHA256 != b.SHA256 {
			t.Errorf("%s: sha berbeda antar lari", ent.Name)
		}
		var n int64
		q := ldb.Q("SELECT COUNT(*) FROM {" + ent.Name + "} WHERE (" + ent.Tenant + ")")
		if err := env.DB.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", ent.Name, err)
		}
		if a.Rows != n {
			t.Errorf("%s: manifest %d baris, COUNT(*) %d", ent.Name, a.Rows, n)
		}
	}
	// tenant filter: the other company's branch/order/user are not extracted.
	rows, err := extract.ReadTable(dir1, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("orders: want 9 (company 2 excluded), got %d", len(rows))
	}
	if rows[0].Str("order_number") == "" || rows[0].Str("order_date") != "2025-12-10T03:00:00.000000Z" {
		t.Fatalf("order_date format: %v", rows[0]["order_date"])
	}
	if _, ok := m1.Tables["users"]; !ok || m1.Tables["users"].Rows != 4 {
		t.Fatalf("users rows = %d, want 4", m1.Tables["users"].Rows)
	}

	// resume: a finished extraction is skipped and keeps identical files.
	before, _ := os.ReadFile(filepath.Join(extract.RawDir(dir1), "orders.jsonl.gz"))
	if _, err := extract.Run(context.Background(), ldb, dir1, true, log); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(extract.RawDir(dir1), "orders.jsonl.gz"))
	if string(before) != string(after) {
		t.Fatal("resume mengubah berkas yang sudah selesai")
	}
}
