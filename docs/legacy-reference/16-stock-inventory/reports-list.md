# Reports List — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, halaman, dan E2E (04/08/12).

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas (tetapi 3 agregat kerja!)

Tanpa export/rekap/PDF/Excel-keluar (satu-satunya Excel = template-**masuk**-buka!):
daftar + kartu + wizard adalah operasional. Yang mirip-laporan:

| Agregat | Sumber | Logika |
|---|---|---|
| `{n} barang/mutasi ditemukan` + kritis-badge (`Kritis`/`Aman`) | `balances` (E-01) / `movements` (E-25) + `meta.total` | Hitung-filter-perusahaan-cabang (rusak-sembunyi-default!; kritis = `available ≤ min`, non-null!) |
| `total_estimated_loss` (global-lintas-halaman!) + 2-kartu-rugi (`Jenis Barang Rusak` + `Potensi Nilai`-Rp!) | `damaged/list` (E-11) | Σ(`onHand × purchasePrice`) lintas-**seluruh**-baris-rusak + `roundQty`; per-baris `roundQty(qty × beli)`-4-desimal! |
| Preview-buka (qty + nilai-`Rp` + 12-isu + lewati!) + konteks-4-kartu + status-`ready/needs_location/no_locations`! | `opening/*` (E-08/09/10) | Agregat-file (`total_quantity`-qty + `total_value`-uang!) + saran-lokasi (milik migrasi!) |

Konsumen: kartu finance, badge produk, rekap-SPT (milik 17 — angka modul ini!:
`average_cost` + `inventory_value` per-produk-cabang!).

## 2. Dokumen operasional (kontrak angka)

Mutasi (Waktu/Tipe-badge/Qty-+/−/Saldo-`→`/Catatan!) · saldo-dual-UOM (basis + jual + label +
step!) · pohon-gudang (relevan + tebal-berstok + varian-chip!) · surat-transfer-fisik
(`Surat Transfer Barang`: nomor-`TRF-`/tanggal/cabang-dari-tujuan/sopir/kendaraan/
tabel-Barang-Jumlah-Lokasi-Asal-Tujuan/catatan/tanda-3-`Gudang Asal`/`Sopir`/
`Gudang Tujuan` + `Nama & Tanda Tangan`!) · template-buka (7-kolom + Panduan + gaya!) ·
SJ/SO/PO (milik 08–13) · struk (milik 15).

## 3. Angka operasional

Tersedia/Fisik/Dipesan/Minimum · `Kritis` (≤!)/`Aman` · `Habis`? — tanpa-label-habis
(beda modul 08: `Diretur...` milik 08!) · umur? — **tanpa-umur** (FIFO-tanpa-tanggal!;
lokasi-tanpa-umur!) · lokasi-hitung (`locationCount` = berisi!) · varian-chip ·
`Draft`/`Sedang Dikirim`/`Diterima`/`Batal` · `Dalam Cabang`/`Keluar`/`Masuk` ·
`Sudah Diisi`/`Belum Diisi` · `Siap diproses`/`Perlu diperbaiki`/`Import selesai`.

## 4. Bahan modul lain

Saldo/mutasi/lokasi/saran/scan/reservasi (E-01/02/03/04/05/15/25!) + modal-guard
(`average_cost`, baca!) + buka-draf (`finance_opening_items`, gabung-aditif!) →
terima/serah/retur/kasir/lapor + posting + SPT (08–17). Kebenaran rekap = base-UOM +
tanggal-gerak-`moved_at` + ref-polimorfik (`referenceType` + `idReference`!).

## 5. Yang eksplisit BUKAN laporan

- Daftar/kartu/wizard (operasional + paginasi-20/25/30/100!) · pohon (navigasi-drill!) ·
  surat-fisik (dokumen-cetak, bukan-rekap!) · toast/notice (sesaat-21-pesan!) · audit
  (jejak-13-kunci, milik 20!) · template-Excel (masukan, bukan-keluaran!) · preview-buka
  (validasi, bukan-laporan!).
