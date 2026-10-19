# Analisis Paritas Fitur — Legacy `mini-erp` vs Revamp `mini-erp-revamp`

**Tanggal:** 2026-10-16 · **Metode:** audit independen, baca-saja, dari **kode** (bukan dokumen) oleh 8 auditor paralel
(per kelompok modul legacy). Klaim `docs/PARITY_AUDIT.md` diuji **sesudah** enumerasi kode, bukan dipercaya.
**Verifikasi tambahan:** 4 temuan berdampak tertinggi (F-01…F-04) saya periksa ulang langsung di kode revamp.
**Batas:** temuan lainnya berasal dari laporan auditor dengan rujukan `file:baris`; belum semuanya diuji ulang satu per satu.

**Rencana tindak lanjut + checkpoint kelengkapan:** [`docs/plan/penyempurnaan-paritas/PLAN.md`](plan/penyempurnaan-paritas/PLAN.md) (`npm run plan:check`).

Yang BUKAN temuan: struktur, teknologi, skema, dan algoritma yang berbeda secara sengaja (modular monolith Go, tanpa multi-tenant,
status order tetap, tanpa PWA/Knowledge-RAG/AI). Yang dinilai adalah **kapabilitas yang dilihat atau dibutuhkan pengguna/bisnis**.

---

## 1. Kesimpulan keseluruhan

**Tidak ada modul yang hilang.** Seluruh 21 modul legacy punya padanan, dan ±98% kapabilitas operasional dan pembukuan setara atau
lebih baik (idempotensi, penguncian, audit dalam transaksi, keamanan sesi, observability, CI, backup, ETL, ±2× jumlah test).
Klaim "paritas ±98%" di `PARITY_AUDIT.md` **secara umum benar**, tetapi **terlalu longgar di banyak baris ✅**: audit ini menemukan
**±70 celah** (HILANG atau SEBAGIAN) yang tidak tercatat di dokumen itu.

| Kategori | Jumlah | Arti |
|---|---|---|
| **Tinggi** | 6 | berisiko salah data/akuntansi/keamanan atau memblokir proses bisnis |
| **Sedang** | ±24 | fitur kerja harian berkurang; ada jalan memutar |
| **Rendah** | ±40 | kenyamanan, kolom/filter, aset, detail |
| Dibuang sengaja (tertulis) | 11 | status order konfigurabel, PWA, multi-tenant, Knowledge/RAG/AI, feature flag, dll. |

**Pola yang berulang** (bukan kebetulan, tetapi akibat cara dokumen paritas disusun):
1. **Backend dibawa, UI/isi belum.** Kemampuan ada di API tetapi tidak sampai ke layar: PPN di form SO, status bayar di daftar,
   izin per role, `trackedCount` dashboard, `receivedBy` penerimaan.
2. **Daftar punya kolom kosong.** Daftar retur (kolom "Order") dan daftar penerimaan ("No. PO", "Baris") selalu `-`/0 karena
   data pengayaan hanya diisi di halaman detail.
3. **Keputusan tertulis yang tidak diwujudkan.** `A-3` (jurnal saldo kredit), `A-19` (tambah-bayar tukar saat SJ pengganti),
   dan `DB_SCHEMA.md:300` (abaikan/pulihkan sumber posting) tertulis "dibawa/diputuskan", tetapi kode belum mengerjakan.
4. **Isi ekspor lebih tipis** dari legacy walau mekanismenya (streaming, mengikuti filter) lebih baik.

---

## 2. Temuan prioritas tinggi (6)

| ID | Modul | Temuan | Bukti | Dampak |
|---|---|---|---|---|
| **F-01** ✔ | Penjualan (UI) | **PPN tidak bisa diatur dari UI.** Backend menghitung PPN, tetapi form SO/POS tidak punya kontrol "kena pajak / harga termasuk pajak / tarif". `taxType` selalu `none` untuk SO baru. | `SalesOrderFormPage.tsx:156-157` (state `taxType`/`taxRate`, hanya diisi saat edit); legacy `order-form-page.tsx:916-946` | Usaha PKP tidak bisa membuat order ber-PPN; laporan PPN keluaran kosong untuk order baru |
| **F-02** ✔ | User/Role | **Izin yang dimiliki sebuah role tidak bisa dibaca.** Tidak ada endpoint baca izin per role; layar peran mulai dengan centang kosong, dan menyimpan **menimpa seluruh izin**. | `RolesPage.tsx:162,359`; `user/presentation/routes.go:23-24` (hanya `PUT` dan `GET /permissions`) | Admin bisa menghapus izin role tanpa sengaja; tidak ada matriks hak akses |
| **F-03** ✔ | Auth/Sesi | **Akses cabang yang dicabut tidak mematikan sesi berjalan.** `GetBranchAccess` hanya dicek saat login dan ganti cabang, tidak saat resolve sesi. | `auth/application/service.go:257,537`; legacy `jwt.strategy.ts:59-68` | User yang dikeluarkan dari cabang tetap membaca/menulis cabang itu hingga 30 hari (rotasi refresh) |
| **F-04** ✔ | Retur / Keuangan | **Saldo kredit retur tidak dijurnal.** Penyelesaian `customer_credit`/`supplier_credit` hanya memo. Akun `customer_advance`/`supplier_advance` ada di pemetaan tetapi tidak dipakai builder jurnal mana pun. | `finance/application/service.go:31,69,387-388` (tak ada di `builders.go`); legacy `finance-posting.service.ts:1454,1704`; keputusan A-3 `PLAN.md:56` | Kredit retur menyelinap ke Piutang (bisa negatif); neraca tidak punya kewajiban/uang muka terpisah |
| F-05 | Keuangan | **Saldo awal persediaan tidak terpandu.** Jurnal saldo awal diketik manual; nilai persediaan tidak dihitung dari stok awal dan tidak direkonsiliasi dengan ledger costing. | `opening.go:73-103`, `OpeningPage.tsx`; legacy `finance.service.ts:485-760,837,893` | Akun Persediaan di buku bisa berbeda dari laporan nilai persediaan; risiko akurasi cutover |
| F-06 | Stok | **Transfer antar-cabang tidak bisa memilih lokasi tujuan di cabang lain.** `GET /stock/locations` hanya mengembalikan cabang sesi; server menolak lokasi yang bukan milik cabang tujuan. | `stock/presentation/handler.go:55-56`, `TransferNewPage.tsx:136-149`; legacy `stock-location.service.ts:46-52` | Transfer antar cabang praktis tidak bisa dibuat dari UI |

✔ = diverifikasi ulang langsung di kode oleh saya.

---

## 3. Per modul

Status: ✅ paritas · ➕ lebih baik · ⚠️ sebagian · ❌ hilang · 🚫 dibuang sengaja (tertulis).

### 3.1 Auth, user, role, cabang, perusahaan, audit, storage — **kuat, 2 celah tinggi**
- ➕ Login/refresh (hash + rotasi + deteksi replay), rate limit login, jejak login gagal, last-admin guard, audit dalam transaksi,
  URL media bertanda tangan, kode cabang terkunci setelah ada dokumen, pengaturan perusahaan bertipe + retensi.
- ❌ **F-02** izin per role tidak terbaca · ❌ **F-03** akses cabang dicabut tidak memutus sesi.
- ⚠️ Daftar user tanpa kolom Role/Cabang/Jabatan dan `lastLoginAt` · ⚠️ Audit log menampilkan kunci mentah (`users.create`),
  legacy punya ±90 label aksi dan 30 label entitas berbahasa Indonesia · ⚠️ galat 5xx tidak masuk audit (hanya log/metrics).
- ⚠️ Bukti pengiriman tidak bertipe (lihat 3.5).
- 🚫 timezone/currency/locale/kode perusahaan (ADR-0006/0009), feature flag, kuota AI.

### 3.2 Produk & pihak (customer/supplier/member) — **kuat**
- ➕ Guard stok/riwayat sama untuk form dan impor, validasi harga, siklus kategori, kode pihak anti-race, buku alamat terkunci untuk
  pelanggan arsip, template impor `.xlsx` server-side, izin lebih granular.
- ❌ **Tambah pelanggan cepat** di form order dan **di POS** hilang (kasir harus keluar dari POS; Bayar Nanti mewajibkan pelanggan ber-HP).
- ❌ Peringkat relevansi pencarian produk (kode persis → prefix kode → prefix nama) dan `productId` hidrasi di `search-options`;
  peringkat stok-tersedia-dulu sengaja dibuang (F3b §3.4) tanpa pengganti di POS/form.
- ⚠️ Daftar produk/pelanggan default **"Semua"** (arsip bercampur), padahal rencana F3b dan legacy default "Aktif" + tab Sampah.
- ⚠️ Daftar produk tanpa stok/harga beli/indikator minimum · ⚠️ `sort_order` kategori tidak bisa diedit · ⚠️ foto tidak bisa
  diunggah saat membuat produk · ⚠️ impor mengabaikan kolom inline "Info tambahan:" tanpa peringatan (legacy memberi error).
- 🚫→❓ Guard "jenis member masih dipakai pelanggan aktif tidak boleh diarsipkan" dibuang (KI-72); akibatnya harga pelanggan turun diam-diam
  ke harga standar. **Layak ditinjau ulang** (minimal peringatan jumlah pelanggan terdampak).

### 3.3 Penjualan & POS — **alur inti kuat, 1 celah tinggi**
- ➕ Idempotensi lengkap (`clientRef`, `by-client-ref/state`), kunci per order, draf keranjang bertahan, ekspor streaming mengikuti filter.
- ❌ **F-01** PPN di UI · ❌ alamat kirim **ad-hoc** (teks bebas) · ❌ `assigned_user` · ❌ kolom "Dibuat Oleh".
- ⚠️ Daftar SO tanpa kolom/filter **status bayar & jatuh tempo** (data sudah di API) · ⚠️ ringkasan keuangan order mengabaikan retur
  (UI menulis "retur dikelola terpisah") · ⚠️ kredit "disetujui oleh/kapan" hanya di audit.
- ⚠️ **Ekspor SO** satu sheet 12 kolom vs legacy: sheet Ringkasan (total/dibayar/sisa/pajak/margin) + Daftar 22 kolom + Detail Item.
- POS: ❌ bunyi scan sukses, catatan transaksi, alamat kirim, scan per baris di form SO · ⚠️ toggle **"Sembunyikan diskon" tidak
  memengaruhi struk yang dicetak** (hanya pratinjau) · ⚠️ layar sukses hanya Cetak Struk + Lihat Order (nota/bukti bayar dari halaman order)
  · ⚠️ kasir di struk = pengguna yang mencetak ulang.
- 🚫 status order konfigurabel + riwayat status (D4), PWA/offline (D4), `deliver-goods` diganti alur idempoten.

### 3.4 Pembelian, penerimaan, retur — **inti kuat, area retur punya celah akuntansi**
- ➕ Penerimaan dua-tahap (pending→posted, posting ulang/batal idempoten), PO batal menolak bila ada dependen, retur draft→confirm→cancel
  dengan jurnal dibalik, qty retur dibatasi terkirim, izin refund per jenis.
- ❌ **F-04** jurnal saldo kredit · ⚠️ **retur pembelian memakai kamus tipe penjualan** (`collect_payment`, `customer_credit`;
  `supplier_credit`/`reduce_payable` tidak ada; UI menawarkan "Terima pembayaran"/"Kredit supplier" bernilai `customer_credit`).
- ⚠️ Validasi penyelesaian retur longgar (hanya ≤ total; legacy: arah selisih, wajib pihak untuk credit, potong hanya sebesar sisa
  terutang) dan pratinjau tidak menampilkan selisih/penyelesaian.
- ⚠️ **Kolom kosong:** "Order jual/beli" di dua daftar retur, "No. PO" dan "Baris" di daftar penerimaan; pencarian retur hanya
  nomor/catatan · ⚠️ daftar PO tanpa status bayar/jatuh tempo dan badge "diretur" · ⚠️ **ekspor PO** tipis.
- ❌ Retur tukar di POS tidak langsung (semua tukar via SJ pengganti; supir/petugas gudang jadi opsional) · ❌ lokasi asal retur beli
  bawaan dari penerimaan terakhir · ❌ tanggal retur ≥ tanggal terima · ❌ lokasi default cabang saat terima · ⚠️ `receivedBy`,
  faktur pajak PO, penanggung jawab PO, catatan/bukti pada `pay_now` saat terima.

### 3.5 Pengiriman — **kuat, 4 celah sedang**
- ➕ SJ parsial dengan kuota draf + doclock, daftar SJ global + detail, antrian memulihkan order POS yatim, bukti bisa dimatikan lewat setting.
- ⚠️ **Stok keluar saat konfirmasi SJ** (sengaja: alur 2-status). Konsekuensi: selama SJ draf, stok belum berkurang dan tidak ditahan;
  penjualan lain bisa menghabiskannya lalu konfirmasi gagal ("Stok tidak mencukupi"; rollback utuh).
- ⚠️ Bukti satu jenis (legacy: scan SJ fisik **wajib** vs foto lokasi opsional) · ⚠️ antrian: filter tanggal hanya satu tab, pencarian
  disaring setelah `LIMIT`, order prepaid belum lunas tetap tampil, kartu tanpa jumlah item · ⚠️ tanggal mutasi stok = saat konfirmasi
  bukan tanggal kirim · ⚠️ alokasi lokasi hanya `ORDER BY location_id` (lihat F-07).

### 3.6 Pembayaran — **paling mendekati paritas penuh dan lebih baik**
- ➕ Idempotensi, lock per pihak, pembatalan CAS + jurnal dibalik, banyak bukti bayar, offset kredit retur, aging per dokumen + CSV.
- ⚠️ `party-balances` kini per satu pihak (legacy: daftar semua pihak berpiutang/berhutang dengan pencarian) — digantikan laporan finance.
- Kredit pelanggan/deposit **tidak ada di legacy** juga (bukan gap).

### 3.7 Stok & gudang — **1 celah tinggi, beberapa sedang**
- ➕ Transfer dengan cancel setelah dispatch + CAS, approval koreksi dengan ambang/rate-limit, saldo awal resumable, cek stok keluar pakai *available*.
- ❌ **F-06** lokasi cabang tujuan · ❌ **F-07 atribut lokasi** (area picking, utama, urutan, kode lokasi tak bisa diedit) dan prioritas alokasi ·
  ❌ field **sopir/kendaraan/tanggal transfer** dan **cetak Surat Transfer Barang** (tanda tangan 3 pihak) ·
  ❌ **estimasi kerugian** pada stok rusak (+ paginasi).
- ⚠️ **Kartu stok** tidak menampilkan saldo berjalan, catatan, tipe, dan filter tipe (API/DB punya; UI belum) ·
  ⚠️ impor saldo awal: sheet harus bernama persis `opening` dengan header Inggris; preview tanpa total qty/nilai; **UI mengiklankan `.xls`
  padahal backend hanya membaca `.xlsx`** (menyesatkan) · ⚠️ move-in rusak wajib lokasi asal · ⚠️ label dual-UOM, filter arah transfer, scan di halaman saldo.

### 3.8 Keuangan, laporan, dashboard — **kuat, banyak celah sedang yang spesifik**
- ➕ Jurnal manual, aging per dokumen, integritas + rebuild, cutover per tipe dokumen, kunci periode saat safe-close, validasi tipe akun,
  dimensi produk/pihak, laporan order jual/beli terpisah, ekspor CSV semua laporan.
- ❌ **F-05** saldo awal persediaan · ❌ **abaikan/pulihkan sumber posting** (dokumen macet memblokir safe-close; jalan keluar hanya tutup paksa;
  `DB_SCHEMA.md:300` menulis "diputuskan" tetapi kolomnya tidak ada) · ⚠️ **ikhtisar tutup hari** (total hari ini, status hari, tugas "butuh data pemilik",
  backlog) · ⚠️ **laba rugi hanya 5 angka** (tanpa rincian per akun) · ⚠️ **paket berkas pajak**: tanpa PPN Masukan, baris retur, buku besar;
  hanya per bulan/cabang · ⚠️ **alasan buka kembali periode tidak dicatat** · ⚠️ ringkasan PPN tanpa DPP dan daftar "tanpa faktur" ·
  ⚠️ penyesuaian pajak jadi jurnal bertipe (tanpa draft/final, lampiran, arsip, kolom penghasilan final/non-objek; keputusan tertulis,
  tetapi alur konsultan pajak berubah) · ⚠️ akun uang muka tak dipakai (F-04) · ⚠️ buku besar wajib pilih akun · ⚠️ nilai persediaan/margin tanpa
  lintas cabang & by-branch · ⚠️ biaya usaha tanpa pihak lawan.
- Dashboard: ⚠️ kartu status WhatsApp dan "produk dipantau" (`trackedCount` ada di API, tidak dirender), prioritas hanya SO (legacy semua jenis),
  alasan prioritas/umur, status basis biaya margin. 🚫 widget SOP/Knowledge.
- 🚫 kas/rekening sebagai master (jadi flag `is_cash`, `DB_SCHEMA.md:302`), cron `daily_operational_metrics` (tak ada pembaca).

### 3.9 Asisten WhatsApp — **bot operasional paritas penuh, Knowledge/RAG dibuang**
- ➕ Cek status order per nomor, laba per cabang, pencarian produk lebih pintar, rate limit bisa diatur, reset sesi, console uji + simulasi di UI,
  statistik 7 hari, retensi, audit trail aksi sensitif, balasan ke pengirim tak berizin.
- ❌ **Pengenalan cabang dari teks pesan** ("penjualan Bandung hari ini": nama/kode/kota) — selalu semua cabang; console web hard-code `branchId: 0`.
- ⚠️ Info produk berkurang (tanpa minimum stok, satuan, status, kategori) · ⚠️ tool yang gagal tak tercatat per baris.
- 🚫 **Knowledge/RAG + mode AI** (tanya SOP/kebijakan via WA, kelola dokumen). Keputusan ada di **PRD §3/§9** (bukan ADR; tak ada ADR),
  dengan alasan kompleksitas dan frasa "keputusan bisnis terpisah". **Tidak ada bukti persetujuan pemilik** bahwa SOP-via-WA boleh hilang,
  padahal legacy menjadikannya intent kelas satu. **Layak dikonfirmasi.** (Nilai AI di luar RAG di legacy hanya satu kalimat penutup.)

### 3.10 Frontend lintas-modul, operasional, uji
- Rute/halaman: 65 → 84 file halaman; semua rute legacy punya padanan (stok digabung jadi tab, laporan keuangan dipecah).
- ❌ **Pemulihan chunk-load dan error boundary** — semua halaman `lazy()` dan `main.tsx` tidak memasang apa pun; setelah deploy, tab terbuka bisa
  **layar putih** (legacy punya auto-reload sekali). Tidak ada `errorElement` di kedua sistem.
- ❌ Aset: bunyi scan POS, logo/favicon, `robots.txt` (`public/` kosong) · ⚠️ kompres gambar di klien · ⚠️ `cost:rebuild` (restatement ledger biaya,
  dry-run + `--commit`) tak punya padanan · ⚠️ APK `android-pos-shell` memakai `POS_URL` terkunci domain legacy (bangun ulang saat cutover bila domain berubah).
- Pengujian (dihitung ulang): backend legacy 57 spec/919 `it` vs revamp **146 file/503 `Test`** + integrasi MySQL nyata; frontend 53/382 vs **106/600**;
  E2E 27 spec/81 test vs **12/48**. **Alur yang diuji legacy tetapi tidak di revamp:** matriks akses role lengkap, otorisasi nomor WA/gateway,
  integritas relasional pelanggan, skenario volume (50 PO/100 penjualan), ekspor laporan order via UI; komponen tanpa test:
  `SalesOrderFormPage`, `ProductFormPage`, `RolesPage`, `UsersPage`, `SetupPage`, dll.
- ➕ Observability (`/healthz`, `/readyz`, metrics, request-id), CI, backup/restore/verify, Docker, ETL + gladi, seed admin tanpa password lemah,
  izin lebih granular (semua 59 kode legacy terpetakan di `bootstrap/legacymap.go`; kasir tak bisa membuka halaman Kategori karena
  `product_categories.manage` jadi syarat menu).

---

## 4. Klaim `PARITY_AUDIT.md` yang salah atau terlalu longgar

| Lokasi | Klaim | Realita |
|---|---|---|
| §3 modul 01/02 | ✅ / "✅ +" | tidak menyebut F-02 (izin role tak terbaca) dan F-03 (sesi cabang) |
| §3 modul 09 | "SO lengkap ✅" | UI tak bisa PPN; alamat ad-hoc, `assigned_user`, status bayar di daftar tidak ada |
| §3 modul 13/12 | "penyelesaian ✅ memo-level di kedua modul retur" | **salah untuk retur pembelian** (tipe milik penjualan); aturan arah & jurnal credit tak dibawa |
| §3 modul 14/15 | goods-receipt/PO ✅ | daftar penerimaan & retur punya kolom kosong; daftar PO tanpa status bayar |
| §4.3 / §0 #6 | "Ekspor ✅ selesai penuh, sisa nihil" | benar untuk mekanisme, **isi jauh lebih tipis** (SO dan PO) |
| §4.5 / `:154,329` | "toggle Sembunyikan diskon ✅"; "`PosScannerModal.tsx`"; "`PosPage.tsx` 744 baris" | toggle tak memengaruhi struk; file itu tak ada (kini `ScanDialog.tsx`); `PosPage` kini ±50 baris + komponen |
| §4.7 | "stok termasuk yang paling sehat, endpoint setara" | tidak mencakup F-06, F-07, sopir/kendaraan/cetak transfer, kerugian rusak, saldo di kartu stok |
| §4.1 vs §1/§3 | dashboard "❌ 8 field" vs "✅ paritas" | **kontradiksi internal**; kode mendukung ✅ operasional, §4.1/§0 basi |
| §4.2.3 / §9 | "abaikan/pulihkan tidak bermakna tanpa antrean tersimpan"; "opening … bukan penghambat cutover" | keliru: dokumen macet memblokir safe-close; saldo awal tanpa hitung persediaan **adalah** risiko akurasi cutover; bertentangan dengan `DB_SCHEMA.md:300` |
| §4.2.3 | "tax-package ✅", "tax-adjustments ✅" | tanpa PPN Masukan/buku besar/retur; tanpa draft/final, lampiran, arsip |
| §6 Knowledge/RAG | "tercatat di kode" (komentar `engine.go:19`) | sumber sebenarnya PRD §3/§9 (keputusan produk, bukan ADR); persetujuan pemilik tidak terbukti |
| §5 angka test | "10 E2E; 128 file Go/434 Test; 77 web" | sebenarnya 12 E2E; 146 file/503 Test; 106 web (V00 visual dan V01 a11y ikut terhitung) |
| §3 halaman | "71 `*Page.tsx`" | 84 |
| PLAN A-3 / A-19 | "dibawa"/"ditagih saat SJ pengganti" | belum diwujudkan di kode |

---

## 5. Rekomendasi (berurut)

**Gelombang 1 — sebelum cutover (risiko data/keamanan/akuntansi):**
1. **F-03** validasi ulang akses cabang pada resolve sesi (atau reset `id_active_branch` saat akses dicabut) + test.
2. **F-02** endpoint baca izin per role + matriks hak akses di UI (cegah penimpaan izin tak sengaja).
3. **F-01** kontrol PPN di form SO (kena pajak, harga termasuk pajak, tarif) dan POS bila PKP.
4. **F-04** jurnal reklasifikasi saldo kredit retur (Piutang→Uang Muka / Hutang→Uang Muka Supplier) + perbaiki kamus tipe retur pembelian
   (`supplier_credit`, `reduce_payable`) + validasi arah/pihak/sisa terutang.
5. **F-05** hitung nilai persediaan awal dari stok awal + rekonsiliasi buku vs ledger costing di `integrity`/`readiness`.
6. **F-06** endpoint lokasi per cabang tujuan untuk transfer antar-cabang.

**Gelombang 2 — fitur kerja harian (sedang):**
tambah pelanggan cepat (form SO + POS), status bayar di daftar SO/PO, perbaiki kolom kosong (daftar retur/penerimaan), kartu stok (saldo/catatan/filter tipe),
sopir/kendaraan + cetak Surat Transfer, estimasi kerugian stok rusak, atribut lokasi picking/utama + prioritas alokasi, ekspor SO/PO diperkaya,
toggle "Sembunyikan diskon" berefek pada struk, pengenalan cabang di bot WA, label audit Indonesia + kolom Role/Cabang di daftar user,
abaikan/pulihkan sumber posting, ikhtisar tutup hari, rincian laba rugi per akun, PPN Masukan di paket pajak, alasan buka kembali periode.

**Gelombang 3 — ketahanan & rapi-rapi:**
`lazyWithRetry` + `errorElement` (cegah layar putih setelah deploy), aset (logo/favicon/bunyi scan), perbaiki teks `.xls` yang menyesatkan di impor saldo awal,
default daftar "Aktif" + tab arsip, peringatan saat mengarsipkan jenis member yang dipakai, tes untuk komponen form besar dan matriks role, `cost:rebuild`.

**Keputusan pemilik yang dibutuhkan:**
- Apakah **tanya SOP/kebijakan via WA (Knowledge/RAG)** memang boleh hilang? (jika tidak: fase terpisah + ADR; retrieval kata kunci tanpa AI cukup sebagai versi awal)
- **Stok keluar saat konfirmasi SJ**: terima konsekuensi "stok bisa terpakai selama SJ draf", atau tahan stok pada SJ draf?
- Apakah **guard arsip jenis member** yang dibuang (KI-72) tetap diterima?
- Apakah **usaha Anda PKP** (menentukan prioritas F-01 dan paket pajak)?
- Domain APK `android-pos-shell` saat cutover (bangun ulang bila berubah).

## 6. Yang revamp lakukan lebih baik (ringkas)
Idempotensi dan penguncian di seluruh alur transaksi · audit dalam transaksi · refresh token berotasi + rate limit · last-admin guard ·
penerimaan dua-tahap · retur draft→confirm→cancel dengan jurnal otomatis dibalik · aging per dokumen · jurnal manual · integritas/rebuild/cutover ·
kunci periode saat safe-close · ekspor server-side streaming · template impor server-side · URL media bertanda tangan · observability · CI/backup/Docker/ETL ·
izin granular · command palette, draf form, kepadatan UI · integrasi test MySQL nyata.
