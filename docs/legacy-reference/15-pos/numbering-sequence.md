# Numbering Sequence — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

---

## 1. Keputusan: modul ini TIDAK punya penomoran sendiri

Kasir memakai nomor order `.../PJ/...` + nomor bayar `PAY-...` (milik 08/14;
`source='pos'` penanda, migrasi 015). Tanpa nomor struk/kasir/shift terpisah.

## 2. Yang bukan penomoran

ID cart (kunci-sementara) · kunci-reservasi (`pos-...` acak!) · meterai (hash-isi) ·
`REF-...` (bayar-manual saja; POS tunai tanpa-referensi!) · `/ting.mp3` (aset!).
