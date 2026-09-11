# Numbering & Sequence — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Modul Cabang adalah **pemilik
tunggal** seluruh penomoran dokumen operasional. Modul lain hanya *memakai* penghitung yang
disiapkan di sini. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [business-rules.md](business-rules.md) §2 ·
[algorithms-legacy.md](algorithms-legacy.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan

Setiap cabang punya sekumpulan **penghitung nomor dokumen**. Satu penghitung = satu jenis dokumen
di satu cabang, menyimpan: prefix, nilai berjalan, kebijakan reset, dan pola format.

| Sifat | Nilai |
|---|---|
| Cakupan penghitung | Per **cabang**, bukan per perusahaan |
| Jumlah penghitung wajib | **5**, dibuat otomatis saat cabang lahir |
| Penghitung tambahan | 1 (`stock_transfer`) dibuat malas oleh modul Stok saat dipakai pertama kali |
| Prefix | Diturunkan dari **kode cabang huruf besar** |
| Kunci unik | Kombinasi cabang + jenis dokumen |
| Perilaku saat kode cabang berubah | Prefix ditulis ulang, **nilai berjalan dipertahankan** |

Penomoran **jurnal keuangan** dan **biaya usaha** memakai sistem terpisah per-perusahaan
(bukan per-cabang) dan bukan milik modul ini.

---

## 2. Format Nomor Per Jenis Dokumen

### 2.1 Order penjualan & pembelian — pola khusus

```
<PREFIX>/<PJ|PB>/<TAHUN>/<BULAN>/<URUT 5 DIGIT>
```

| Bagian | Isi |
|---|---|
| `PREFIX` | `ORD-<KODE CABANG>`, mis. `ORD-BLR` |
| `PJ` / `PB` | `PJ` untuk penjualan, `PB` untuk pembelian |
| `TAHUN` | 4 digit, tahun saat nomor dibuat |
| `BULAN` | 2 digit dengan nol di depan |
| `URUT` | 5 digit dengan nol di depan, dimulai dari `00001` |

Contoh nyata: `ORD-BLR/PJ/2026/08/00001`

**Aturan yang menyertainya:**

- **Penjualan dan pembelian punya deret nomor yang terpisah.** `.../PJ/.../00001` dan
  `.../PB/.../00001` bisa hidup berdampingan.
- **Deret direset ke `00001` setiap bulan**, otomatis, tanpa job terjadwal — bulan baru
  menciptakan penghitung baru.
- Nomor order yang sudah tersimpan **tidak pernah ditulis ulang**. Order lama berformat lama tetap
  apa adanya.
- Prefix diambil dari penghitung `order` cabang; bila entah bagaimana tidak ada, dipakai
  `ORD-<nomor internal cabang>` sebagai cadangan.

### 2.2 Pembayaran, surat jalan, retur — pola bersama

```
<PREFIX>/<TAHUN>/<URUT 5 DIGIT>
```

| Dokumen | Prefix | Contoh |
|---|---|---|
| Pembayaran | `PAY-<KODE>` | `PAY-BLR/2026/00001` |
| Surat jalan | `SJ-<KODE>` | `SJ-BLR/2026/00042` |
| Retur penjualan | `RTR-<KODE>` | `RTR-BLR/2026/00003` |
| Retur pembelian | `RTB-<KODE>` | `RTB-BLR/2026/00002` |

Tahun diambil dari waktu pembuatan dokumen. **Nomor urut tidak pernah direset** — lihat §4.

### 2.3 Mutasi stok antar-cabang — pola menyimpang

```
TRF-<NOMOR INTERNAL CABANG>/<TAHUN>/<URUT 5 DIGIT>
```

Contoh: `TRF-2/2026/00001`

Ini satu-satunya nomor dokumen yang **tidak memakai kode cabang**. Penghitungnya dibuat oleh modul
Stok saat mutasi pertama, bukan saat cabang dibuat, dan tidak pernah diperbarui saat kode cabang
berubah. Lihat KI-46.

---

## 3. Kapan Penghitung Dibuat & Diperbarui

| Kejadian | Yang terjadi pada penghitung |
|---|---|
| Cabang **dibuat** | Lima penghitung wajib dibuat sekaligus, nilai berjalan `0` |
| Cabang **disimpan ulang** (ubah apa pun) | Lima penghitung "dipastikan ada": yang hilang dibuat, prefix yang ada disesuaikan kode cabang saat ini |
| **Kode cabang berubah** | Kelima prefix ditulis ulang; **nilai berjalan tidak disentuh** |
| Order pertama bulan berjalan | Penghitung bulanan baru muncul otomatis |
| Mutasi stok pertama cabang | Penghitung mutasi dibuat malas oleh modul Stok |
| Migrasi fitur baru (retur) | Migrasi menyisipkan penghitung untuk **semua cabang yang sudah ada** |

**Sifat penting untuk rebuild:** langkah "pastikan ada" ini adalah mekanisme yang membuat cabang
lama otomatis mendapat jenis penomoran baru — cukup dengan pernah disimpan ulang. Kalau tidak
pernah disimpan ulang, ia mengandalkan migrasi.

---

## 4. Kebijakan Reset — yang tertulis vs yang benar-benar terjadi

Setiap penghitung menyimpan kebijakan reset dan pola format:

| Penghitung | Kebijakan tersimpan | Pola tersimpan |
|---|---|---|
| `order` (dasar) | `yearly` | `{prefix}/{year}/{seq:05}` |
| `payment`, `sj`, `sales_return`, `purchase_return` | `yearly` | `{prefix}/{year}/{seq:05}` |
| `order_<jenis>_<tahun-bulan>` (bulanan) | `monthly` | `ORD-{code}/{kind}/{year}/{month}/{seq:05}` |
| `stock_transfer` | `none` | kosong |

**Kedua kolom itu tidak pernah dibaca oleh satu pun pembuat nomor.** Semua pembuat nomor menyusun
teksnya secara langsung di kode, dan tidak satu pun memeriksa pergantian tahun.

Akibat nyata:

| Dokumen | Reset yang benar-benar terjadi |
|---|---|
| Order | **Bulanan** — bukan karena kebijakan, tapi karena kunci penghitungnya memuat tahun-bulan |
| Pembayaran, surat jalan, retur penjualan, retur pembelian | **Tidak pernah reset.** Angka terus naik lintas tahun sementara bagian tahun pada nomor berganti |
| Mutasi stok | Tidak pernah reset (sesuai kebijakannya) |

Contoh perilaku yang akan dilihat pengguna pada pergantian tahun:

```
PAY-BLR/2026/01187      (pembayaran terakhir tahun 2026)
PAY-BLR/2027/01188      (pembayaran pertama tahun 2027 — bukan 00001)
```

Lihat KI-45. **[PERLU KONFIRMASI]** — apakah nomor pembayaran/surat jalan/retur memang diharapkan
kembali ke `00001` tiap 1 Januari? Untuk kebutuhan rekap dan SPT ini biasanya diharapkan, tapi
mengubahnya berarti perilaku sistem baru berbeda dari yang selama ini terjadi.

---

## 5. Perilaku Saat Penghitung Tidak Ada (nomor darurat)

Bila pembuat nomor tidak menemukan penghitung cabangnya, ia **tidak menggagalkan transaksi** —
ia menerbitkan nomor darurat:

| Dokumen | Nomor darurat |
|---|---|
| Pembayaran | `PAY-<nomor internal cabang>-<cap waktu milidetik>` |
| Surat jalan | `SJ-<nomor internal cabang>-<cap waktu milidetik>` |
| Retur penjualan | `RTR-<nomor internal cabang>-<cap waktu milidetik>` |
| Retur pembelian | `RTB-<nomor internal cabang>-<cap waktu milidetik>` |

Contoh: `PAY-2-1755168000000`

Bentuknya jelas berbeda dari nomor normal dan mustahil dihasilkan alur biasa, jadi kemunculannya
di data adalah **penanda bahwa cabang itu tidak diprovisioning dengan benar**.

Untuk mutasi stok perilakunya berbeda: penghitung yang hilang **dibuat**, bukan dilewati.

**[PERLU KONFIRMASI]** Apakah nomor berbentuk `PAY-<angka>-<angka panjang>` pernah terlihat di
data production? Bila ya, ada cabang yang lahir di luar alur normal.

---

## 6. Jaminan Keunikan & Perilaku Bersamaan

| Aspek | Perilaku |
|---|---|
| Keunikan penghitung | Dijamin kunci unik cabang + jenis dokumen di basis data |
| Order (jalur normal) | Kenaikan nomor dilakukan dalam satu operasi tulis atomik, aman untuk order pertama tiap bulan yang dibuat bersamaan |
| Pembayaran, surat jalan, retur, mutasi | Baris penghitung **dikunci** selama transaksi, sehingga dua dokumen bersamaan tidak bisa mendapat nomor yang sama |
| Nomor dokumen setelah gagal | Bila transaksi dokumen gagal setelah nomor diambil, kenaikan penghitung **ikut dibatalkan** (ada di transaksi yang sama) — tidak ada nomor bolong |

**Yang tidak dijamin:** keunikan nomor **lintas cabang** setelah kode cabang diubah. Bila cabang A
berkode `JKT` menerbitkan `ORD-JKT/PJ/2026/08/00003`, lalu kode cabang A diubah ke `SBY` dan cabang
B kemudian diberi kode `JKT`, cabang B bisa menerbitkan nomor yang sama persis. Skenarionya
menuntut dua perubahan kode berturut-turut dan belum tentu pernah terjadi — lihat KI-40.

---

## 7. Nilai Bawaan Data Seed

Cabang bawaan sistem (`BLR` — Cabang Blora, ditandai pusat) datang dengan:

| Penghitung | Prefix | Nilai awal |
|---|---|---|
| `order` | `ORD-BLR` | 0 |
| `payment` | `PAY-BLR` | 0 |
| `sj` | `SJ-BLR` | 0 |
| `sales_return` | `RTR-BLR` | 0 |
| `purchase_return` | `RTB-BLR` | 0 |

Dan satu lokasi stok default berkode `GDG-BLR` bernama `Gudang Pusat Blora`.

**Catatan kesenjangan perkakas:** skrip *reset* basis data pengembangan hanya menyiapkan **tiga**
penghitung (order, pembayaran, surat jalan) — dua penghitung retur tidak ikut. Setelah reset,
retur pertama akan menerbitkan **nomor darurat** (§5) sampai cabang disimpan ulang. Ini hanya
memengaruhi lingkungan pengembangan.

---

## 8. Ringkasan untuk Rebuild

Yang **wajib dipertahankan**:

1. Prefix nomor diturunkan dari kode cabang, huruf besar, dengan awalan tetap
   `ORD-`/`PAY-`/`SJ-`/`RTR-`/`RTB-`.
2. Order memakai pola bertingkat dengan penanda jenis (`PJ`/`PB`), tahun, bulan, dan reset bulanan.
3. Dokumen lain memakai pola `prefix/tahun/urut-5-digit`.
4. Nomor urut selalu 5 digit dengan nol di depan.
5. Cabang baru langsung punya penomoran lengkap tanpa langkah manual.
6. Mengubah kode cabang mengubah prefix, **tidak** mereset nomor, **tidak** mengubah dokumen lama.

Yang **perlu diputuskan ulang** (bukan disalin buta):

1. Reset tahunan yang dijanjikan tapi tidak pernah berjalan (§4, KI-45).
2. Prefix mutasi stok yang memakai nomor internal cabang (§2.3, KI-46).
3. Boleh tidaknya kode cabang diubah setelah ada dokumen terbit (KI-40).
4. Nasib kolom kebijakan reset dan pola format yang tidak pernah dibaca.
