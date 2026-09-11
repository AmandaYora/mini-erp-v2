# Algorithms Legacy — Modul 08 Order Purchasing

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — Nomor PO bulanan (hasil: unik per cabang-jenis-bulan tanpa job)

**Hasil yang diharapkan:** `ORD-{kode}/PB/YYYY/MM/00001` naik per bulan, independen dari penjualan,
atomik-konkuren, tak pernah ditulis ulang, bulan = server (bukan tanggal dokumen). **Bebas
diubah:** kunci sequence, fallback darurat.

## A-02 — Total rupiah utuh (hasil: tak ada sen di dokumen)

**Hasil yang diharapkan:** qty boleh pecahan (meter/kg), uang selalu rupiah utuh; pajak inklusif
memecah (`dasar = total/(1+r)`) vs eksklusif menambah; persen/tarif 4 desimal; toleransi sisa
0.0001. Contoh terkunci: pecahan × harga tanpa sisa sen. **Bebas diubah:** pustaka bulat.

## A-03 — Jatuh tempo neto (hasil: utang selalu bertanggal)

**Hasil yang diharapkan:** net purchase selalu punya tempo (eksplisit → lama → invoice+hari);
non-net selalu null; FE menghitung pratinjau yang sama dengan server. **Bebas diubah:** urutan
fallback, widget tanggal.

## A-04 — Penerimaan bertahap idempoten-sisa (hasil: mobil ke-n tak bisa ganda)

**Hasil yang diharapkan:** tiap penerimaan ≤ sisa (kumulatif, toleransi); kosong = sisa-semua;
final = tanggal + selesai + history otomatis; parsial = tanpa ketiganya; non-stok tanpa movement;
tanggal boleh masa lalu dan dipakai movement; header seragam-vs-null; respons bedakan final vs
parsial vs bayar vs tempo. **Bebas diubah:** struktur transaksi, selama penguncian sisa bertahan.

## A-05 — Keputusan COD akhir (hasil: utang tak menggantung tanpa keputusan)

**Hasil yang diharapkan:** final-COD-berutang wajib pilih: bayar-penuh-kini (butuh izin bayar,
bukti opsional tak membatalkan) atau jadi-tempo (tercatat sebagai persetujuan kredit). Non-final
tanpa keputusan. **Bebas diubah:** alur dialog, selama titik-final-menuntut-keputusan bertahan.

## A-06 — Guard tiga-lapis perubahan (hasil: yang bergerak tak bisa diubah diam-diam)

**Hasil yang diharapkan:** baris/batal/arsip ditolak bila movement ATAU bayar-aktif ATAU sumber
finance (5 status) — dengan pesan yang menunjuk jalan keluar (retur/adjustment/refund/reversal);
header tetap bisa; terminal menolak semua. **Bebas diubah:** query cek, selama tiga lapis + pesan
berarah bertahan.

## A-07 — Status bertahap terkunci (hasil: selesai = barang-diterima + lunas-bila-bayar-dulu)

**Hasil yang diharapkan:** target wajib transisi aktif; purchase selesai wajib tanggal-terima;
pasca-terima hanya selesai; batal pasca-gerak ditolak; selesai-terminal prabayar/COD wajib lunas,
net bebas; FE menyembunyikan tombol yang pasti ditolak. **Bebas diubah:** definisi seed, selama
rantai syarat bertahan.

## A-08 — Angka live + snapshot abadi (hasil: daftar jujur, histori beku)

**Hasil yang diharapkan:** dibayar/terutang/status dihitung live (bayar + alokasi − retur
completed); badge retur batch-tunggal; `createdBy` tak bocor; snapshot baris/penerimaan/pajak
final; export memakai snapshot + batas 366 hari/5000. **Bebas diubah:** query agregat, selama
rumus BR-36/BR-12 bertahan (dan badge daftar diperbaiki — E-30).

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Nomor bulanan + contoh terkunci (A-01) | Sequence, fallback |
| Rupiah utuh + rumus pajak + toleransi (A-02) | Round, presisi |
| Tempo-netto + derivasi invoice (A-03) | Fallback, widget |
| Sisa-kumulatif + final-vs-parsial (A-04) | Transaksi, alias |
| Keputusan-final-COD (A-05) | Dialog, bukti |
| Tiga lapis + pesan berarah (A-06) | Cek, teks |
| Rantai selesai purchase (A-07) | Seed, tombol |
| Live + snapshot + batas export (A-08) | Agregat, sheet |
