# Test Cases — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`delivery.service.spec.ts` (601 baris), `delivery-proof.service.spec.ts` (243),
`delivery-note-print-page.test.tsx`, E2E 06 (4 skenario) + 18 (pengganti). **GAP** = belum ada.

---

## TC-D — SJ (TC-D-01…TC-D-16)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-D-01 | Antrean: dua daftar tanpa audit | Tepat | Ya (spec) |
| TC-D-02 | Terbit + stok kurang-saat itu | Tepat | Ya (spec) |
| TC-D-03 | Terbit prepaid-berutang / lunas | `400` / lolos | Ya (spec ×2) |
| TC-D-04 | Bukan-sales / terminal / over-sisa / stok-kurang / hilang | `400`/`404` | Ya (spec ×5) |
| TC-D-05 | Tempo net 10+2 → SJ → stok −10 → cetak nama → confirm tutup (tanpa double) → piutang > 0 → nota Tagihan | E2E penuh | Ya (E2E 06-1) |
| TC-D-06 | COD 3 + `pay_now` tunai → id bayar + saldo 0 + Lunas + bukti-bayar | Tepat | Ya (E2E 06-2) |
| TC-D-07 | Prepaid 2 lunas → terbit → confirm tanpa bayar (`payment_id` null) | Tepat | Ya (E2E 06-3) |
| TC-D-08 | Parsial 1 → batal → stok kembali → 4 (tanpa tutup) → 6 (tutup + tanggal) | Tepat | Ya (E2E 06-4) |
| TC-D-09 | Pengganti: pending (stok 5) → dispatch (stok 3) → delivered | Via modul 12, bernomor SJ | Ya (E2E 18) |
| TC-D-10 | Confirm tanpa tutup (SJ-aktif-lain) / tutup-net / tunda-tak-penuh | Tepat | Ya (spec ×3) |
| TC-D-11 | Tanpa-arsip / TTD-kosong-tanpa-alasan / tanpa-alasan-lolos / ganda / hilang | Tepat | Ya (spec ×5) |
| TC-D-12 | Batal ok (stok kembali) / terkonfirmasi / hilang | Tepat | Ya (spec ×3) |
| TC-D-13 | Tender murni (null/pas/bulat/tolak) | Tepat | Ya (spec ×3) |
| TC-D-14 | Terbit tanpa-transisi-proses (API lolos, UI/antrean saring) | Lolos API | GAP → KI baru |
| TC-D-15 | Sopir/gudang kosong via API; dispatch-konkuren ganda | Lolos / tanpa-lock | GAP → KI baru |
| TC-D-16 | Pengganti di list/cetak-order; drop-note di cetak; cetak-tanpa-`order.view` | Tak-tampil / tak-tampil / lolos-login | GAP → batas/KI baru |

## TC-B/W — Bukti & antrean/UI (TC-B-01…TC-W-06)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-B-01 | Bukti: unggah + URL + purpose + optimasi + audit (243 baris spec) | Tepat | Ya (spec) |
| TC-B-02 | Cetak: render + karbon + status + ganjal-5 + preprinted | Tepat | Ya (FE test) |
| TC-W-01 | Antrean: cari/tanggal/umur/limit + dua-tabel + aksi + shortcut | Tepat | GAP (kode + E2E implisit) |
| TC-W-02 | Modal konfirmasi: arsip-syarat + radio + alasan + lokasi + catatan | Tepat | GAP |
| TC-W-03 | Modal bukti: tombol-purpose + pratinjau + notice-tanpa-TTD | Tepat | GAP |
| TC-W-04 | Dialog batal: retry-terbuka + toast-info | Tepat | GAP |
| TC-W-05 | Alokasi manual/reservasi/otomatis-urutan | Tepat | GAP (E2E selalu manual) |
| TC-W-06 | `deliveries/list` array-mentah + status-hitung + tanpa-URL | Tepat | GAP |

## GAP (G-01…G-06)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Terbit longgar (transisi, sopir-kosong) | Mengunci KI baru (ketatkan atau kunci) |
| G-02 | Dispatch konkuren | Sekelas KI-92 |
| G-03 | Pengganti di list/cetak + drop di cetak | Batas vs bug |
| G-04 | UI antrean/modal/bukti/batal | Tanpa jaring FE khusus SJ |
| G-05 | Alokasi otomatis-urutan + reservasi | Jalur paling-rumit tanpa jaring langsung |
| G-06 | Cetak-tanpa-izin + umur-badge + karbon | Keputusan tercetak tanpa jaring |
