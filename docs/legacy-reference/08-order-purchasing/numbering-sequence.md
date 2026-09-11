# Numbering Sequence — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format penomoran PO.

---

## 1. Format nomor PO

```
ORD-{KODECABANG}/PB/YYYY/MM/00001
Contoh: ORD-BLR/PB/2026/08/00001
```

| Aspek | Aturan (dari `generateOrderNumber`) |
|---|---|
| Prefix | `ORD-{kode}` dibaca dari baris sequence `order` cabang (ditulis saat cabang dibuat); fallback `ORD-{idBranch}` bila tak ada (→ KI baru: nomor darurat, sekelas KI di modul 04) |
| Tag jenis | `PB` (pembelian) vs `PJ` (penjualan) — counter **terpisah** |
| Periode | Tahun + bulan **waktu server** (bukan tanggal order — PO backdate tetap memakai bulan berjalan; zona waktu = lokal server, sejalan temuan shared §7) |
| Urut | 5 digit, mulai 00001 **setiap bulan** (reset tanpa job: kunci `order_purchase_YYYY-MM` per cabang; baris dibuat saat bulan berjalan) |
| Konkuren | Upsert atomik (`ON DUPLICATE KEY UPDATE`) + baca ulang; fallback ORM `pessimistic_write` (hanya test-double) |
| Abadi | Kolom tak pernah ditulis ulang (format lama tetap; ubah kode cabang tak mengubah nomor lama) |

Contoh terkunci (unit test): sales → `ORD-{cabang}/PJ/{tahun}/{bulan}/{00001}`; purchase →
`.../PB/...` independen; tanpa baris `order` → `ORD-{idBranch}/...`.

## 2. Nomor terkait (diterbitkan alur ini, pemilik di modul lain)

- **Bayar COD** (`pay_now`): `PAY-{prefix}/{tahun}/{5 digit}` tanpa reset bulanan (counter
  `payment` global cabang; fallback `PAY-{idBranch}-{Date.now()}` bila tak ada — → KI-46 area).
- **SJ supplier**: teks bebas dari supplier (bukan penomoran sistem).
- **ID penerimaan**: auto-increment internal (tak tampil sebagai nomor dokumen).

## 3. Yang bukan penomoran

ID order/item/penerimaan (FK internal); nomor invoice supplier & faktur pajak (dokumen pihak
ketiga, bebas ganda — BR-9-area).
