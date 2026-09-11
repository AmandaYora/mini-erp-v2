# Business Rules — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus. Dari service 812 baris + controller + FE + E2E 07/22. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Buat (BR-01…BR-08)

| ID | Aturan |
|---|---|
| BR-01 | Jumlah > 0 (`Jumlah pembayaran harus lebih dari 0`); bulat (`...bilangan bulat rupiah (tanpa koma/desimal).` — anti-recehan) |
| BR-02 | Target: `id_order` → per-order; else `id_business_party` → gabungan; else `400 'Pilih order atau customer/supplier untuk mencatat pembayaran.'` (order menang bila keduanya dikirim!) |
| BR-03 | Per-order: ada-cabang-non-arsip (404); non-batal (`...sudah dibatalkan tidak dapat menerima pembayaran.`); sisa > 0 (`...sudah lunas...`); jumlah ≤ sisa (`...melebihi sisa saldo order. Maksimal {n}.`) |
| BR-04 | Hasil per-order: baris (pihak-dari-order + sisi-dari-jenis + nomor + tanggal-bebas + jumlah + metode-tanpa-validasi + referensi + catatan + pembuat) + audit (5 field) |
| BR-05 | Gabungan: pihak ada-perusahaan (**arsip boleh**, disengaja); sisi eksplisit-valid else tebak-tipe; cocok (`...hutang supplier harus memilih supplier.` / `...piutang customer harus memilih customer.`); sisi-sampah (`Jenis saldo harus piutang customer atau hutang supplier.`); tunggakan wajib (2 pesan sisi) |
| BR-06 | Alokasi: FIFO (tempo→tanggal→id; berhenti ≤0.009; lewati-nol) else manual (milik + >0 + ≤baris + total=bayar; 4 pesan); over-total (`...melebihi total saldo terbuka. Maksimal {n}.`); nol (`Tidak ada tagihan yang bisa dialokasikan.`) |
| BR-07 | Hasil gabungan: baris (order NULL) + N alokasi (fifo/manual + `allocated_at` = tanggal-bayar!) + audit (+ hitungan + mode) |
| BR-08 | Nomor `PAY-...` (lock, fallback darurat); tanggal-bebas (masa lalu/depan!); metode/referensi tanpa validasi (→ KI baru) |

## 2. Saldo & ledger (BR-09…BR-13)

| ID | Aturan |
|---|---|
| BR-09 | Sisi wajib-valid; cari kecil (kode/nama/telepon); agregat (hitung + sisa + tertua = tempo ?? tanggal); urut tertua + nama; total-global; paginasi-opt-in |
| BR-10 | Tanpa-pihak dilewati (walk-in tak tertagih!); batal dikecualikan; buka = sisa > 0.009 (vs bayar-order > 0! — E-11) |
| BR-11 | Ledger: pihak boleh-arsip; sisi else tebak; tagihan-terbuka (+ umur-hari + bucket) + bayar (+ alokasi-per-order) + total; paginasi-terpisah (total-global) |
| BR-12 | Daftar-order campur langsung + alokasi (jumlah = alokasi + flag); total = keduanya (arsip-kecuali) |
| BR-13 | Retur-completed menyesuaikan (total ± selisih; bayar ∓ collect/refund-kredit); tempo-lewat + berutang = overdue (net saja) |

## 3. Arsip & bukti (BR-14…BR-18)

| ID | Aturan |
|---|---|
| BR-14 | Arsip: ada-cabang-non-arsip (404); per-terdampak: terminal + non-net + berutang-pasca → tolak (2 label termin); else bebas (termasuk jadi-berutang non-terminal!) |
| BR-15 | Arsip serentak bayar + alokasi + audit (+ hitungan); saldo kembali (E2E 07) |
| BR-16 | Bukti: ada-non-arsip (404); file-wajib + 5MB + optimizer (400); timpa-path + audit-kompresi; URL butuh-path (`Bukti pembayaran belum tersedia`) |
| BR-17 | Gagal-bukti tak membatalkan (3 tempat: form, edit, terima-COD) |
| BR-18 | UI sembunyi-hapus terminal-non-net (cermin server); dialog tak-batal; toast hapus |

## 4. Cetak (BR-19…BR-20)

| ID | Aturan |
|---|---|
| BR-19 | Akumulasi urut-tanggal sampai-bayar-ini; sisa-sesudah = max(0, total − akumulasi); status = sisa-sesudah; pihak/store/raw fallback; tanpa-bayar → pesan khusus |
| BR-20 | Tanpa harga-baris (satu nominal + total); kalimat verifikasi tetap; kertas dua-mode |

## 5. Lintas modul (BR-21…BR-22)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-21 | Dibayar/terutang/status menjadi gerbang proses/selesai/SJ/terima + badge + antrean-prepaid | Order (08/09), Delivery (11) |
| BR-22 | Saldo/ledger menjadi kartu finance + penagihan-arsip + rekap | Finance (17), E2E 22 |

## 6. Aturan yang TIDAK ada (verifikasi)

Tidak ada: edit (selain bukti); halaman-daftar; desimal; over; batal/tanpa-tunggakan;
hapus-permanen; alokasi-edit; tagih-walk-in; overdue-non-net; kembalian-simpan; validasi
tanggal/metode/referensi; UI-alokasi-manual (KI baru!); unik-referensi (KI-87).
