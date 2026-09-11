# Data Model Legacy — Modul 14 Payment / Pembayaran

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi 009/012/033/047.

---

## 1. Tabel milik modul (2, lahir 009 + 033 + 047)

### `payments` — baris bayar

Satu baris = satu pembayaran (langsung per order ATAU gabungan per pihak). Kolom: id ·
`id_order` (**null sejak 033**; FK) · cabang · `id_business_party` (null; FK; backfill dari
order) · `financial_side` (null; backfill dari jenis) · nomor (100) · tanggal (bebas!) ·
`amount` `(18,2)` · `amount_tendered` (null; tunai-serah saja, 047) · metode (30, tanpa
cek!) · referensi (100) · catatan · `proof_file_path` (500, timpa; 012) · pembuat +
created + arsip (tanpa `updated_at`!). Index: (order), (cabang), (cabang, pihak, sisi,
tanggal).

### `payment_allocations` — pecahan gabungan (033)

Satu baris = satu order × satu bayar-gabungan. Kolom: id-bayar + id-order + cabang + pihak
+ sisi (wajib!) + jumlah `(18,2)` + metode (`fifo`/`manual`) + `allocated_at` (= tanggal-bayar!)
+ created + arsip. Index: (bayar), (cabang, order), (cabang, pihak, sisi).

## 2. Relasi (konsep)

```
orders 1──* payments (langsung; null-boleh sejak 033)
business_parties 1──* payments (gabungan; arsip-boleh)
payments 1──* allocations ──> orders (tampil-campur di daftar)
branches (sequence PAY); users (pembuat); storage (bukti-path)
```

Cabang = batas + guard; perusahaan via cabang/order (tanpa kolom langsung! — seperti SJ).

## 3. Jejak baca-tulis

Buat (transaksi): validasi → nomor → baris (+ alokasi) + audit. Arsip (transaksi): cek per
order → bayar + alokasi + audit. Bukti: optimasi → path-timpa + audit. Baca: daftar-campur,
saldo-agregat, ledger-dua-paginasi, URL-signed.

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Tanpa `id_company` + tanpa `updated_at`** (seperti SJ; batas via cabang).
2. **Tanpa cek metode** (string sampah tersimpan — KI baru).
3. **Timpa-bukti yatim-storage** (objek lama tanpa referensi).
4. **Backfill 033** (pihak + sisi dari order — data lama direkonstruksi, bukan asli).
5. **`allocated_at` = tanggal-bayar** (bukan kini — backdate konsisten, disengaja).
6. **Presisi cocok** (`(18,2)` dua sisi — tanpa drift; acuan benar).
