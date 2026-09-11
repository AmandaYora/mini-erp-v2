# Numbering Sequence — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format nomor SJ.

---

## 1. Format nomor SJ

```
SJ-{KODECABANG}/{TAHUN}/{5 digit}
Contoh: SJ-BLR/2026/00042 (lihat KI-46 modul 04 untuk contoh nyata)
```

| Aspek | Aturan (dari `generateSjNumber`) |
|---|---|
| Prefix | Dari baris sequence `sj` cabang; fallback `SJ` (lalu `SJ-{idBranch}-{Date.now()}` bila baris tak ada — nomor darurat, sekelas KI-82) |
| Periode | Tahun berjalan (server); **tanpa reset** (counter global cabang; kebijakan `reset_policy` tersimpan tetapi tak dibaca — KI-45) |
| Konkuren | Lock `pessimistic_write` per baris cabang |
| Abadi | Tak ditulis ulang; SJ pengganti memakai deret yang sama (modul 12) |

## 2. Yang bukan penomoran

ID SJ/baris/alokasi (internal); tanggal-kirim vs dispatch (dua waktu, bukan nomor); karbon
(teks tetap); detention? — tidak ada.
