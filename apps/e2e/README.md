# apps/e2e — suite E2E Playwright (F8-5)

Spec E01..E18 dan V00..V02 (E18 = `/sw.js` pembersih service worker legacy: tersaji sebagai JavaScript `no-store`, dan service worker tiruan legacy beserta cache `mini-erp-pwa-*` hilang setelah browser mengambil berkas v2 (cutover domain, DEPLOYMENT §7); E17 = SJ Langsung (ADR-0022) lewat API: SJ tanpa order → konfirmasi → order dari dua SJ → tutup harian → batal SJ belum diorder; E16 = label QR produk: PNG `/products/{id}/qr` didekode lalu dikenali `scan-product`, dan kamera POS — umpan video palsu Chromium berisi label QR — memasukkan barangnya ke keranjang; E15 = tiga alur back-office + lompat ke dokumen dikerjakan **keyboard saja** — tanpa `.click()`, `tabTo` membuktikan tiap kontrol terjangkau Tab (PLAN redesain-ui U7-2); V02 = tanpa gulir menyamping per template pada 390/768/1366/1920, mode kartu baris barang di 390, POS landscape 1024/1280/1340 (U7-3); E11 = ketahanan operasional: aset `public/`, pemulihan chunk-load; E12 = matriks akses role seed x menu/endpoint, dibangkitkan dari `registry.tsx` dan gagal bila ada menu/izin baru tanpa deklarasi; E13 = volume 50 PO + 100 penjualan via API + integritas relasi pelanggan; E14 = ekspor SO/PO/paket pajak lewat UI dan baca isi xlsx) menjalankan **binari Go sungguhan** yang menyajikan build web
(`apps/web/dist`, `PUBLIC_DIR`) di port 18080 — sama dengan produksi satu-kontainer, bukan dev server Vite.
Setup data lewat REST (`helpers/api.ts`, satu token per peran); UI dipakai pada interaksi kritis
(login, POS, CRUD pelanggan, halaman cetak, smoke menu). Teks Indonesia terpusat di `helpers/labels.ts`.

## Menjalankan

```bash
# sekali: unduh browser
npm run install:browsers -w apps/e2e

# dari akar repo (memegang kunci database + advisory lock)
npm run test:e2e
# satu spec / opsi Playwright:
npm run test -w apps/e2e -- E03
# lewati build web bila dist sudah segar:
E2E_SKIP_WEB_BUILD=1 npm run test -w apps/e2e
```

Prasyarat: MySQL hidup (`node scripts/test-db.mjs up`), Go, Node 22+ (`.nvmrc`).
`global-setup` menolak jalan bila port 18080 masih dipakai (mis. sisa server run sebelumnya) — tanpa penjaga itu spec
diam-diam menguji binari lama. `global-teardown` menghentikan server lintas platform (`taskkill` di Windows, `SIGTERM`
di macOS/Linux). Laporan HTML: `npm run report -w apps/e2e`.

## Database bersama (PENTING)

Satu-satunya database yang tersedia adalah `mini_erp_test`, **dipakai bersama test integrasi Go yang
TRUNCATE-nya**. Karena itu:

1. `npm run test` dibungkus `node scripts/db-lock.mjs --` (kunci direktori antar-proses).
2. `global-setup.ts` juga memegang `GET_LOCK('mini_erp_test_db', 3000)` di koneksi khusus selama seluruh run
   (server + seed + spec) — kunci yang sama dengan harness integrasi Go.
3. `go run ./cmd/seed -profile=e2e` (kode: `apps/api/cmd/seed/e2e.go`) menolak database selain `*_test`,
   membangun ulang skema bila hash migrasi berubah (penanda `_it_schema` yang sama dengan harness Go),
   TRUNCATE semua tabel aplikasi, lalu mengisi data tetap. Jadi tiap run mulai dari keadaan yang sama.

Bila nanti ada database `mini_erp_e2e` terpisah: set `E2E_DB_DSN` dan longgarkan `dsnguard` (suffix `_e2e`).

## Data seed (deterministik)

| Item | Isi |
|---|---|
| Cabang | `HQ` (pusat, id 1) dan `CB2` |
| Pengguna (password `E2e-Rahasia-2026`) | `admin` (owner, kedua cabang), `kasir` (peran kasir bawaan, sudah termasuk payment.create/view), `gudang`, `akuntan` (peran khusus) |
| Lokasi | per cabang: lokasi bawaan + `HQ-GA`/`HQ-GB`, `CB2-GA`/`CB2-GB` |
| Produk | `P001..P020`, satu varian terlihat dengan barcode `8990000000NN`, stok awal 100 di `HQ-GA` |
| Pelanggan | `C001` "Tunai" + `C002..C006`; pemasok `S001..S003` |
| Pengaturan | `delivery_require_evidence=false` (tanpa foto bukti); persetujuan koreksi stok memakai default (wajib) |

Spec saling terisolasi dengan memakai produk/pihak berbeda dan asersi berbasis selisih (bukan angka absolut).

## Konvensi

- 1 worker, `retries: 0`: selector yang flaky diperbaiki, bukan diulang. Selector berbasis role/label.
- Tidak ada jam beku; tanggal relatif hari ini (Asia/Jakarta).
- `test.fixme` = bug produk yang terdokumentasi — dihidupkan saat diperbaiki. Saat ini tidak ada (K1, K2 sudah ditutup).

## Temuan produk yang dicatat

- **K1 — ditutup 2026-10-21.** Masalahnya: role `kasir` bawaan tanpa `payment.create`. Akibatnya POS tunai
  meninggalkan order yatim, karena konfirmasi SJ order tunai mewajibkan pembayaran (legacy BR-18 punya cacat
  yang sama).
  - Perbaikan backend: role kasir bawaan kini membawa `payment.create` + `payment.view` (`DefaultGrants`,
    `internal/bootstrap/legacymap.go`), dan seed e2e tidak lagi menambahkannya.
  - Perbaikan POS: akun tanpa izin itu ditolak sebelum dokumen apa pun dibuat; Bayar Nanti tetap bisa.
  - Bukti: E02 "kasir tanpa izin pembayaran…" dan tes tunai E02 yang memakai izin bawaan.
- **K2 — ditutup.** Pembayaran piutang pihak terarsip kembali diterima seperti legacy BR-05/E-16; arsip hanya
  menghentikan order baru. Bukti: E07 "pembayaran piutang pihak terarsip diterima (K2)".
