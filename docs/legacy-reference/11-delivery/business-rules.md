# Business Rules — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus sisi SJ. Mekanika order umum = modul 08; serah-langsung = 09. Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## 1. Terbit (BR-01…BR-07)

| ID | Aturan |
|---|---|
| BR-01 | Sales + non-terminal + ≥1 item (`Surat jalan hanya berlaku untuk sales order` / `Order sudah selesai, tidak bisa menerbitkan surat jalan baru` / `Minimal satu item harus disertakan dalam surat jalan`) |
| BR-02 | Prabayar berutang → tolak (`...harus lunas sebelum surat jalan diterbitkan.`); COD/net bebas (keputusan di konfirmasi) |
| BR-03 | Per baris: milik order (`Item dengan id {id} tidak ditemukan di order ini`); qty > 0 (`Qty item '{n}' harus lebih dari 0`); ≤ sisa-kirim + 0.0001 (`Item '{n}' melebihi sisa yang belum dikirim. Sisa: {s} {u}`); sisa = order − dispatch-non-arsip |
| BR-04 | **Tanpa cek transisi proses di API** (hanya terminal + prabayar) — syarat tahap hanya di tombol + antrean (→ KI baru) |
| BR-05 | Sopir + gudang teks bebas wajib (FE; server: kolom NOT NULL tanpa validasi isi — string kosong lolos API! → KI baru); plat/catatan opsional-null; tanggal-kirim `date` (parse `Tanggal pengiriman`, kosong → kini untuk movement) |
| BR-06 | Stok keluar saat terbit: alokasi manual (aturan §F-02.2) else otomatis (§F-02.1) else reservasi (§F-02.3); non-tracked tanpa gerak; movement `out` ref-SJ alasan-`Pengiriman dari SO {nomor}`; audit `delivery.create` |
| BR-07 | Respons `{id_delivery_note, sj_number, id_order, order_number}` + cetak-otomatis FE |

## 2. Konfirmasi (BR-08…BR-14)

| ID | Aturan |
|---|---|
| BR-08 | SJ ada-cabang-non-arsip (`Surat jalan tidak ditemukan`); belum-pernah (`...sudah dikonfirmasi sebelumnya`); TTD ∈ signed/not_signed (`Status tanda tangan penerima wajib dipilih`) |
| BR-09 | Arsip fisik ≥1 **sebelum** konfirmasi (`Foto atau scan surat jalan fisik wajib diunggah sebelum konfirmasi`); not_signed wajib alasan (`Alasan tanda tangan penerima kosong wajib diisi`); nama opsional (signed, null bila kosong; dibuang bila not_signed); drop opsional |
| BR-10 | Pengganti ditolak (`Surat jalan pengganti dikonfirmasi lewat menu Retur/Tukar`); order hilang → `Order terkait tidak ditemukan` |
| BR-11 | Auto-close bila: semua item terpenuhi-TERKONFIRMASI (±0.0001) + tanpa SJ aktif. Syarat: status-selesai ada + transisi aktif (`Order belum berada pada tahap yang bisa diselesaikan.`) → finansial: prepaid-berutang tolak (`...harus lunas sebelum barang dapat diserahkan sepenuhnya.`); COD-berutang wajib mode (pesan sama); `pay_now` (403 khusus; nomor; tanpa tender!) / `switch_to_net` (tempo + approve + audit sumber-konfirmasi) |
| BR-12 | Tanggal-serah = tanggal-kirim-SJ-penutup else kini; history `Order otomatis diselesaikan setelah semua surat jalan dikonfirmasi`; audit confirm kaya (autoClose, settlement, status-TTD, hitungan, flag) |
| BR-13 | File diunggah SEBELUM konfirmasi dalam satu alur FE (tandatangan lalu lokasi); gagal file → konfirmasi tak jalan (beda dengan terima-COD yang lanjut-tanpa-bukti) |
| BR-14 | Shortcut confirm sekali-pakai; SJ tak-aktif/tak-cocok → diam |

## 3. Batal & bukti (BR-15…BR-20)

| ID | Aturan |
|---|---|
| BR-15 | Batal: ada + non-arsip (`Surat jalan tidak ditemukan`); belum-confirm (`...sudah dikonfirmasi tidak bisa dibatalkan`); stok kembali per alokasi (+ movement `in` alasan-`Pembatalan SJ {sj} dari SO {order}`, metadata cancellation); tanpa-alokasi → default-bila-ada-bersaldo (tanpa-saldo lewati); arsip + audit `delivery.archive` |
| BR-16 | Bukti: multipart (`order.update`); id-valid (`Surat jalan tidak valid`); purpose ∈ 2 (`Jenis bukti surat jalan tidak valid`); file wajib (`File bukti surat jalan wajib diunggah`); 5MB; optimasi profil-delivery; private; pertama primer + sortOrder naik; audit `delivery.proof_uploaded` (obyek + kompresi) |
| BR-17 | URL: media milik-perusahaan + purpose-dua-itu + SJ-ada-cabang-non-arsip → `{url, expires_in}`; else `404 'Bukti surat jalan tidak ditemukan'` |
| BR-18 | Arti status list: `confirmedAt? confirmed : active` (dihitung); badge FE: Aktif / Selesai-Tanpa-TTD / Selesai-TTD / Selesai |
| BR-19 | `deliveries/list` = array mentah (bukan envelope-items); urut dibuat-ASC; bukti tanpa URL; nama/kode/varian dari baris order (kosong bila baris hilang) |
| BR-20 | Pengganti tak tampil di list/cetak-order (id NULL) — batas modul 12 |

## 4. Antrean (BR-21…BR-25)

| ID | Aturan |
|---|---|
| BR-21 | Tanpa audit (read-only); limit 1–200 default 100; respons `{create_sj, waiting_return, limit}` |
| BR-22 | `create_sj`: sales non-pos + belum-serah + active-non-terminal + (non-prepaid / lunas ±0.01) + tanpa-transisi-proses + sisa > 0 (HAVING); urut order-terlama; hitungan total/dispatch/sisa + SJ-tunggu |
| BR-23 | `waiting_return`: SJ aktif-non-arsip sales-non-pos; urut kirim-terlama; hitungan item/qty/arsip + flag |
| BR-24 | Cari: buat (SJ? — TIDAK: order/pihak saja) vs tunggu (+ nomor-SJ); tanggal: buat = tanggal-order, tunggu = kirim (fallback dispatch); `date_to` panjang-10 → akhir-hari (beda `orders/list`!); umur hanya-tunggu (today/gt_2/gt_7 dari kirim) |
| BR-25 | Jatuh-tempo-bayar hanya info di antrean (bukan syarat, kecuali prepaid-lunas) |

## 5. Cetak & penomoran (BR-26…BR-28)

| ID | Aturan |
|---|---|
| BR-26 | SJ: `SJ-{KODECABANG}/{TAHUN}/{5 digit}` tanpa-reset, lock tulis, fallback `SJ-{id}-{Date.now()}` (detail numbering doc) |
| BR-27 | Status cetak dari status+termin order (Lunas/Sebagian/COD/Tempo/Belum Lunas); staff = pembuat-mentah; USER = pencetak; karbon terkunci |
| BR-28 | Cetak tanpa `order.view` (login saja); SJ dari daftar-SJ + info order-detail/store; tanpa-SJ → pesan khusus |

## 6. Lintas modul (BR-29…BR-30)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-29 | Kurang-saat-terbit + tanggal-kirim + alokasi-per-lokasi menjadi HPP/saldo/retur-jual | Stock (16), Finance (17), Sales Return (12) |
| BR-30 | Pengganti (stok + status + nomor-SJ) ditulis modul 12 ke tabel ini; tanggal-serah menutup SO | Sales Return (12), Dashboard net-sales (18) |

## 7. Aturan yang TIDAK ada (verifikasi)

Tidak ada: edit SJ; cek transisi di create (BR-04); validasi isi sopir/gudang di server; tender
di konfirmasi; cicilan COD; cetak massal; URL di list; audit antrean; SJ purchase; batas umur
SJ-tanpa-kembali (hanya badge merah); lock dispatch konkuren (berurutan aman — sisa menghitung
dispatch non-arsip termasuk yang belum-confirm; bersamaan tak terkunci, sekelas KI-92 → KI baru).
