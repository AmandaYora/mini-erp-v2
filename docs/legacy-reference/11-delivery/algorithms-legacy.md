# Algorithms Legacy — Modul 11 Delivery / Pengiriman

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**. Serah-langsung = modul 09.

---

## A-01 — Alokasi petik-otomatis (hasil: gudang tak perlu pilih lokasi)

**Hasil yang diharapkan:** tanpa alokasi manual: ambil dari tersedia per varian, daun-aktif saja,
urut area-petik → primer → default → terbanyak → sort → id, sampai cukup; kurang ditolak dengan
angka. Manual: daun-aktif + cukup + unik + total-pas. Reservasi: milik-user + tak-kedaluwarsa +
cukup, lalu consumed. **Bebas diubah:** urutan prioritas, selama deterministik + pesan berangka.

## A-02 — Kurang-saat-terbit (hasil: stok tak pernah minus oleh SJ)

**Hasil yang diharapkan:** stok berkurang TEPAT saat terbit (bukan konfirmasi — tanpa
double-deduct); batal mengembalikan tepat-sasaran (+ movement pembatalan); terkonfirmasi tak
bisa batal. **Bebas diubah:** titik potong, selama invarian "terbit = kurang" bertahan.

## A-03 — Kembali-menutup (hasil: kertas kembali = kebenaran)

**Hasil yang diharapkan:** konfirmasi menuntut arsip-fisik + TTD/alasan; penutup hanya bila
semua-terpenuhi-terkonfirmasi + tanpa-aktif; finansial COD/prepaid di penutup; tanggal-serah =
kirim-penutup. **Bebas diubah:** syarat arsip, kecuali diputuskan.

## A-04 — Antrean dua-sisi (hasil: yang-tertua-dulu, dua arah)

**Hasil yang diharapkan:** buat = order-terlama (siap-kirim, non-pos, lunas-prepaid,
tanpa-proses, bersisa); tunggu = kirim-terlama (aktif, umur-merah >7, flag-arsip); cari/tanggal/
umur konsisten per sisi. **Bebas diubah:** query, selama urutan + syarat bertahan (dan selisih
`date_to` diseragamkan).

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Urutan petik + pesan (A-01) | Prioritas, reservasi |
| Kurang-terbit + kembali-batal (A-02) | Movement, alasan |
| Arsip-dulu + tutup-semua (A-03) | Modal, flag |
| Tertua-dulu dua-sisi (A-04) | Query, limit |
