# Numbering Sequence — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format nomor retur.

---

## 1. Format nomor retur

```
RTR-{KODECABANG}/{TAHUN}/{5 digit}
Contoh: RTR-BLR/2026/00007 (pola dari KI-40 modul 04)
```

| Aspek | Aturan (dari `generateReturnNumber`) |
|---|---|
| Prefix | Dari baris sequence `sales_return` cabang; fallback `RTR` (lalu `RTR-{idBranch}-{Date.now()}` bila baris tak ada — darurat, sekelas KI-82) |
| Periode | Tahun berjalan (server); **tanpa reset** (counter global cabang; `reset_policy` tersimpan tak dibaca — KI-45) |
| Konkuren | Lock `pessimistic_write` per baris cabang |
| Unik | `(id_branch, return_number)`; abadi tak ditulis ulang |

## 2. Nomor terkait

SJ pengganti memakai deret SJ normal (`SJ-...`, modul 11). Settlement tanpa nomor (id internal).
Bayar collect/refund tanpa nomor bayar (tanpa baris `payments` — E-16 feature).

## 3. Yang bukan penomoran

ID retur/baris/settlement; tanggal; alasan; `cancelled_*` mati.
