# Numbering Sequence — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format nomor retur-beli.

---

## 1. Format nomor

```
RTB-{KODECABANG}/{TAHUN}/{5 digit}
Contoh: RTB-BLR/2026/00003 (pola dari KI-40 modul 04)
```

| Aspek | Aturan (dari `generateReturnNumber` + migrasi 050) |
|---|---|
| Prefix | `RTB-{UPPER(kode)}` ditulis saat migrasi per cabang; fallback `RTB` (lalu `RTB-{id}-{Date.now()}` tanpa baris — darurat) |
| Periode | Tahun berjalan (server); **tanpa reset** (counter global; `reset_policy='yearly'` tersimpan tak dibaca — KI-45) |
| Konkuren | Lock `pessimistic_write` per baris cabang |
| Unik | `(id_branch, return_number)`; abadi |

## 2. Terkait

Settlement tanpa nomor (id internal). Bayar refund tanpa nomor bayar (tanpa baris payments).
Izin keempat `purchase_return.cancel` ada di matriks tetapi tanpa endpoint (→ KI baru).
