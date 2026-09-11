# Algorithms Legacy — Modul 10 Goods Receipt

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**. Inti penerimaan (sisa, tahap, settlement) = modul 08 A-04/A-05.

---

## A-01 — Sisa kumulatif (hasil: tak ada barang ganda)

**Hasil yang diharapkan:** sisa = order − Σ batch lama (toleransi 0,0001); kosong = isi-otomatis;
final menutup (tanggal + selesai + history); parsial membiarkan. **Bebas diubah:** agregasi,
selama tanpa-lock dievaluasi (E-18).

## A-02 — Lokasi bertingkat (hasil: pilah fisik tercatat benar)

**Hasil yang diharapkan:** baris → header → default-cabang (aktif-daun); header seragam-vs-null;
tampil header → item-pertama → `-`. **Bebas diubah:** default cabang, cache baris.

## A-03 — Saldo bertambah (hasil: stok masuk = saldo + jejak)

**Hasil yang diharapkan:** saldo dibuat-0 bila belum ada; `on_hand += base`;
`available = on_hand − reserved` (reservasi utuh); movement `in` bertanggal-terima + alasan-PO +
metadata konversi; non-stok tanpa movement. **Bebas diubah:** upsert, presisi.

## A-04 — Alias kompatibel (hasil: dua nama, satu perilaku)

**Hasil yang diharapkan (bila dipertahankan):** nama-alternatif dinormalkan lalu delegasi penuh
— hasil identik dengan inti. **Bebas diubah:** pertahankan + uji (G-01) atau buang (KI-88).

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Sisa + final-vs-parsial (A-01) | Agregat, lock |
| Bertingkat + seragam-vs-null (A-02) | Default, tampil |
| Bertambah + jejak + non-stok-tanpa-gerak (A-03) | Upsert, alasan |
| Identik-bila-ada (A-04) | Alias vs buang |

