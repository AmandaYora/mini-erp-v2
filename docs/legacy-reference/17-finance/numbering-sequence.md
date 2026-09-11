# Numbering & Sequence — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

Berbeda dari modul 04 Branch yang penomorannya **per cabang**, penomoran finance adalah
**per perusahaan**. Keduanya memakai tabel yang berbeda dan tidak pernah bertabrakan.

---

## 1. Tiga Sequence Ter-seed — Hanya Dua Terpakai

| Kunci | Prefix | Kebijakan reset | Pola | Dipakai? |
|---|---|---|---|---|
| `journal` | `JRN` | `yearly` | `{prefix}/{year}/{seq:05}` | ✅ **Ya** — semua jurnal |
| `business_expense` | `EXP` | `yearly` | `{prefix}/{year}/{seq:05}` | ✅ **Ya** — nomor biaya usaha |
| `opening_balance` | `OB` | `none` | `{prefix}/{seq:05}` | ❌ **Tidak pernah dibaca** |

Jurnal saldo awal memakai sequence **`journal`**, bukan `opening_balance`. Sequence `OB` adalah
sisa rancangan yang tidak pernah dipakai — kandidat dibuang di sistem baru.

---

## 2. Contoh Nomor Nyata

| Dokumen | Contoh | Kapan terbit |
|---|---|---|
| Jurnal penjualan | `JRN/2026/00001` | Posting `sales_completed` |
| Jurnal HPP | `JRN/2026/00002` | Posting `stock_issue_cogs` |
| Jurnal saldo awal | `JRN/2026/00003` | Kunci saldo awal |
| Jurnal lengkapi stok awal | `JRN/2026/00004` | Lengkapi stok awal |
| Jurnal biaya usaha | `JRN/2026/00005` | Catat biaya |
| Jurnal pembalik | `JRN/2026/00006` | Balik jurnal / batal posting / batal biaya |
| Nomor biaya usaha | `EXP/2026/00001` | Catat biaya |

**Satu transaksi biaya usaha menghabiskan dua nomor sekaligus** — satu `EXP` dan satu `JRN`.

---

## 3. Format & Cara Pembentukannya

Satu fungsi tunggal membentuk semua nomor finance. Pola diambil dari kolom `format_template` pada
barisnya — **berbeda dari modul Branch yang mengabaikan kolom itu**.

| Token | Diisi dengan |
|---|---|
| `{prefix}` | Kolom prefix |
| `{year}` | Tahun kalender **WIB** dari tanggal acuan |
| `{month}` | Bulan 2 digit kalender **WIB** |
| `{seq:N}` | Nilai berjalan, diberi nol di depan sampai N digit |
| `{seq}` | Nilai berjalan apa adanya |

### 3.1 Aturan yang paling penting: tanggal acuan = tanggal **transaksi**

| Dokumen | Tanggal acuan |
|---|---|
| Jurnal otomatis | `journalDate` — tanggal peristiwa bisnisnya |
| Jurnal saldo awal | **Tanggal cutover**, bukan hari mengunci |
| Jurnal lengkapi stok awal | **Tanggal cutover** saldo awal itu |
| Nomor & jurnal biaya usaha | **Tanggal biaya**, bukan hari mencatat |
| Jurnal pembalik biaya usaha | **Hari ini** — lihat §5 |

Sebelum disatukan, empat salinan logika ini semuanya memakai jam server, sehingga posting mundur
menghasilkan nomor bulan **saat diposting**, bukan bulan transaksinya. Perbaikan ini tercatat
sebagai keputusan sadar di kode dan **wajib dipertahankan**.

### 3.2 Zona waktu

Tahun dan bulan diambil dari **kalender Asia/Jakarta**, bukan UTC. Transaksi 1 Agustus pukul 01:00
WIB (= 31 Juli 18:00 UTC) tetap mendapat nomor bulan **08**.

Ini konsisten dengan aturan zona waktu finance lainnya — dan **berbeda dari modul Branch** yang
penomorannya masih memakai waktu lokal server.

---

## 4. Kebijakan Reset Tahunan — Sama Seperti Branch: Tidak Pernah Dijalankan

Kedua sequence yang terpakai bertanda `yearly`, tetapi **tidak ada satu pun kode yang mereset
nilai berjalan saat tahun berganti**. Pembentuk nomor hanya menambah 1 lalu memformat.

Yang akan dilihat pengguna:

```
JRN/2026/01847      (jurnal terakhir 2026)
JRN/2027/01848      (jurnal pertama 2027 — bukan 00001)
EXP/2026/00312
EXP/2027/00313
```

Ini **persis masalah yang sama dengan KI-45** di modul Branch, pada tabel yang berbeda. Keduanya
sebaiknya diputuskan bersama.

**Perbedaan penting dari Branch:** di sini kolom `format_template` **benar-benar dibaca**, jadi
hanya kolom `reset_policy` yang dekoratif. Di Branch, keduanya dekoratif.

---

## 5. Nomor Jurnal Pembalik

| Jalur pembalikan | Tanggal jurnal pembalik | Nomor mengikuti |
|---|---|---|
| Balik jurnal (`finance/journals/reverse`) | Tanggal jurnal **asli** | Bulan transaksi asli |
| Batal posting sumber | Tanggal jurnal **asli** | Bulan transaksi asli |
| **Batal biaya usaha** | **Hari ini** | Bulan berjalan |

Ketidakseragaman pada baris ketiga nyata di kode dan punya alasan akuntansi: pembatalan biaya
dianggap peristiwa baru di periode berjalan. Tetapi penjagaan periodenya justru memakai periode
**jurnal asli** — jadi membatalkan biaya dari bulan yang sudah dikunci **ditolak**, meski
pembaliknya akan jatuh di bulan berjalan yang masih terbuka.

**Cacat kecil yang menyertainya:** "hari ini" pada jalur ini dihitung dalam **UTC**, bukan WIB.
Pembatalan biaya antara pukul 00:00–07:00 WIB menghasilkan jurnal pembalik bertanggal
**hari sebelumnya**. Satu-satunya tempat di modul finance yang tidak WIB-aware. Lihat KI-146.

---

## 6. Keunikan & Perilaku Bersamaan

| Aspek | Perilaku |
|---|---|
| Keunikan baris sequence | Kunci (perusahaan + jenis) |
| Pengambilan nomor | Baris **dikunci** selama transaksi (`pessimistic_write`) |
| Nomor setelah gagal | Kenaikan ikut dibatalkan — tidak ada nomor bolong |
| Sequence hilang | Ditolak: `Sequence jurnal Finance belum dikonfigurasi` / `Sequence '{kunci}' belum dikonfigurasi` |

**Berbeda dari modul Branch, tidak ada "nomor darurat" berbasis cap waktu.** Bila sequence hilang,
transaksi **gagal** — pilihan yang lebih aman untuk dokumen akuntansi, dan layak ditiru.

---

## 7. Penomoran yang BUKAN Milik Modul Ini

| Nomor | Pemilik |
|---|---|
| `ORD-*`, `PAY-*`, `SJ-*`, `RTR-*`, `RTB-*`, `TRF-*` | Modul 04 Branch (per cabang) |
| Nomor faktur pajak (`tax_invoice_number`) | **Diketik manual** di modul Order — bukan dibangkitkan |
| Nomor invoice supplier | **Diketik manual** di modul Order |
| Kode periode & kode periode pajak | **Diketik manual** oleh pengguna |
| Kode akun & kode kas/rekening | **Diketik manual** |

Nomor faktur pajak penting untuk dicatat: sistem **tidak pernah membangkitkannya**, hanya
memeriksa apakah sudah terisi (masuk checklist tutup bulan dan ditandai di rincian PPN).

---

## 8. Ringkasan untuk Rebuild

Yang **wajib dipertahankan**:

1. Nomor jurnal memakai tahun/bulan **tanggal transaksi** dalam kalender WIB, bukan jam server.
2. Satu implementasi pembentuk nomor untuk seluruh dokumen finance (dulu ada 4 salinan yang
   berbeda perilaku).
3. Baris sequence dikunci selama transaksi; kegagalan membatalkan kenaikan.
4. Sequence hilang → transaksi gagal, bukan nomor darurat.
5. `format_template` benar-benar dibaca dari data.

Yang **perlu diputuskan ulang**:

1. Reset tahunan yang dijanjikan tapi tidak berjalan (§4) — putuskan bersama KI-45.
2. Sequence `opening_balance` yang tidak pernah dipakai (§1).
3. Tanggal jurnal pembalik biaya usaha: hari ini atau tanggal asli? (§5)
4. Perhitungan "hari ini" yang masih UTC pada jalur itu (KI-146).
