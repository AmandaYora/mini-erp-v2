package integration

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"mini-erp/internal/bootstrap"
)

// Dokumen cetak pelanggan meniru sistem lama: nota & surat jalan jatuh ke
// HP/alamat pelanggan bila order tanpa alamat kirim, bukti pembayaran mencetak
// kontak pihak, dan kop bukti pembayaran memuat alamat cabang sesi. Respons
// detail yang dibaca halaman cetak harus membawa ketiganya.
func TestPrintDocumentsCarryContacts(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	if _, err := m.db.Exec(`UPDATE parties SET address_text = 'Jl. Pelanggan 2' WHERE id = ?`, fx.customerID); err != nil {
		t.Fatalf("party address: %v", err)
	}
	if _, err := m.db.Exec(`UPDATE branches SET address = 'Jl. Cabang 1', city = 'Blora' WHERE id = ?`, fx.branchID); err != nil {
		t.Fatalf("branch address: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	so, err := m.sales.Service().GetByID(ctx(), soID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", so.GrandTotal, payAll(soID, so.GrandTotal))
	if err != nil {
		t.Fatalf("payment: %v", err)
	}

	if err := bootstrap.EnsureAuthz(ctx(), m.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	h := routeServer(t).Handler()
	tok := twoBranchUser(t, "pencetak", []string{"sales.view", "payment.view"}, fx.branchID)
	data := func(path string) map[string]any {
		t.Helper()
		rec := doReq(h, http.MethodGet, path, tok, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return env.Data
	}

	order := data("/api/v1/sales-orders/" + strconv.FormatInt(soID, 10))
	if order["partyPhone"] != "081234567890" || order["partyAddress"] != "Jl. Pelanggan 2" {
		t.Fatalf("order contact = %v / %v", order["partyPhone"], order["partyAddress"])
	}
	payment := data("/api/v1/payments/" + strconv.FormatInt(pay.ID, 10))
	if payment["partyPhone"] != "081234567890" || payment["partyAddress"] != "Jl. Pelanggan 2" {
		t.Fatalf("payment contact = %v / %v", payment["partyPhone"], payment["partyAddress"])
	}
	me := data("/api/v1/auth/me")
	branch, _ := me["branch"].(map[string]any)
	if branch["address"] != "Jl. Cabang 1" || branch["city"] != "Blora" {
		t.Fatalf("session branch = %v", branch)
	}
}
