# User Flows — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard`. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Lihat seksi/saldo/ledger/URL/cetak | `payment.view` (+ `order.view` untuk cetak) | — |
| Catat + bukti + Edit-bukti | `payment.create` | Tombol hilang |
| Hapus bayar | `payment.archive` (sembunyi bila terminal + non-net) | Tombol hilang |
| COD-serah/terima bayar-kini | + `payment.create` di sesi (403) | Opsi hilang (terima) / 403 (serah) |

## UF-01 — Bayar per order (cicilan/DP/pelunasan)

Detail → seksi → **+ Catat Pembayaran** → tanggal + jumlah (≤ sisa, bulat) + metode +
referensi + catatan + bukti opsional → **Simpan** → toast + tabel + badge terbarui. Over →
`400 Maksimal`; desimal → `400 bulat`; lunas/batal → `400`.

## UF-02 — Bayar gabungan FIFO (E2E 07 terkunci)

5 order × 10rb (satu pihak) → saldo 50rb → bayar 27rb (tanpa alokasi) → FIFO 10+10+7 (3
order pertama!) → saldo 23rb → ledger memuat alokasi persis → lunasi 23rb transfer → saldo 0.
Supplier cermin (payable). Tanpa-pihak-tanpa-order → `400 Pilih...`.

## UF-03 — Alokasi manual

Pilih baris + jumlah (tiap ≤ sisa-baris; total = bayar) → `manual` tersimpan per alokasi;
salah → 4 pesan (§F-02.2). UI-nya di mana? — **tanpa UI** (hanya API/E2E/helper! → KI baru:
FIFO satu-satunya jalan UI).

## UF-04 — Bukti & arsip & cetak

Catat + bukti (kompres; gagal tak membatalkan) → `Lihat` overlay → `Edit` timpa (notice) →
`Cetak` (akumulasi + sisa-sesudah) → `Hapus` (dialog; terminal-non-net-berutang ditolak;
saldo kembali — E2E 07). Bukti supplier E2E: path memuat id + URL signed.

## UF-05 — Saldo & ledger (kolektor)

Finance → kartu Piutang/Utang (cari + total global + paginasi) → `Rincian {nama}` (tagihan
terbuka + umur-bucket + bayar + alokasi) → bayar gabungan → saldo turun. Arsip tetap tertagih
(E2E 22).

## UF-06 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Nol/negatif/desimal/over/tanpa-target | `400` buat (4 pesan) |
| Batal/lunas/hilang | `400` order |
| Pihak hilang/sisi-bentrokan/tanpa-tunggakan | `400` pihak (5 pesan) |
| Manual salah | `400` alokasi (4 pesan) |
| Arsip terminal-non-net-berutang | `400` arsip |
| Bukti hilang/tanpa-path/tipe | `404/400` bukti |
