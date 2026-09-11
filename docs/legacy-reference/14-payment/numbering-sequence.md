# Numbering Sequence — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format nomor bayar.

---

## 1. Format nomor bayar

```
PAY-{KODECABANG}/{TAHUN}/{5 digit}
Contoh: PAY-BLR/2026/01187 (pola dari KI-45 modul 04)
```

| Aspek | Aturan |
|---|---|
| Prefix | Dari baris sequence `payment` cabang (seed awal `PAY-JKT`/`PAY-BDG` untuk cabang 1/2! — fosil data awal, kini dari kode cabang); fallback `PAY` (lalu `PAY-{id}-{Date.now()}` tanpa baris — darurat) |
| Periode | Tahun berjalan (server); **tanpa reset** (counter global; `reset_policy='yearly'` dekoratif — KI-45) |
| Konkuren | Lock `pessimistic_write` per baris cabang (dipakai juga oleh COD serah/terima/konfirmasi via generator milik order — 3 implementasi seidenya, perilaku identik) |
| Abadi | Tak ditulis ulang; dipakai cetak + referensi + audit |

## 2. Nomor terkait

- Referensi: `REF-...` bebas-ganda (KI-87). Alokasi/settlement: id internal. SJ/retur: modul masing-masing.
- Generator kembar: `OrderService.generatePaymentNumber` + `PaymentService.generatePaymentNumber` (+ pakai di terima/serah/konfirmasi) — perilaku identik, kode ganda.

## 3. Yang bukan penomoran

ID bayar/alokasi; `amount_tendered`; bukti-path; tanggal.
