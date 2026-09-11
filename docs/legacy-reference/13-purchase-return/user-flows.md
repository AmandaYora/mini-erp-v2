# User Flows — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard`. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Daftar/detail/konteks | `purchase_return.view` | Rute diblokir |
| Preview/buat | `purchase_return.create` | Rute diblokir |
| Opsi refund | `purchase_return.refund` (opsi hilang; paksa → 403) | Opsi hilang |
| Buka dari detail PO | `purchase_return.create` + sudah-terima | Tombol hilang |

## UF-01 — Retur sebagian potong-hutang

PO terima 10 → buat (qty 3 + lokasi-terima + alasan) → preview (potong-penuh, tanpa kas) →
simpan → stok −3 + movement + detail (tanpa settlement). Sisa 7 tetap bisa diretur.

## UF-02 — Retur penuh membatalkan order

Retur 10/10 (kumulatif, boleh multi-dokumen!) → `cancels_order` + order → Dibatalkan + history
+ badge kuning 3 tempat + badge `Diretur penuh` di daftar order. Tanpa status-cancelled →
retur tetap, order dibiarkan.

## UF-03 — Sisa refund/kredit

Retur > sisa-hutang (lunas-sebagian) → preview (sisa-refund-kredit) → pilih refund (izin +
metode) / kredit → simpan → baris settlement + kas di luar sistem (seperti sales E-16!).

## UF-04 — Lokasi & nilai

Default = terima-terakhir (timpa manual); non-tracked tanpa lokasi; nilai = rasio × harga
bersih + pajak proporsional; tanggal ≥ terima & ≤ kini+1mnt.

## UF-05 — Preview-mengikat

Tanpa preview → warning; tiap ubah → invalidasi; tipe-dari-preview; sukses → detail baru.

## UF-06 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| PO belum-terima/non-purchase/tak-ada | Toast + `400/404` |
| Over-sisa/duplikat/qty-0 | `400` baris |
| Tanggal < terima / masa-depan | `400` tanggal |
| Lokasi hilang/non-daun/stok-kurang | `400` lokasi/stok |
| Refund tanpa izin / paksa-salah | `403` / `400` |
