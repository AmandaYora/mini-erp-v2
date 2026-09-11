# Numbering Sequence — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

---

## 1. Format nomor SO

```
ORD-{KODECABANG}/PJ/YYYY/MM/00001
Contoh: ORD-BLR/PJ/2026/08/00001
```

Mekanika = modul 08 (kunci `order_sales_YYYY-MM`, atomik, abadi, bulan-server, fallback
`ORD-{idBranch}`, counter independen dari PB). Contoh terkunci: sales → `.../PJ/...`.

## 2. Nomor terkait

- **Bayar COD-serah**: `PAY-{prefix}/{tahun}/{5 digit}` (+ `amount_tendered` tak-bernomor).
- **Referensi bayar manual**: `REF-YYYYMMDD-XXXX` (FE, bisa-ubah, tanpa cek-unik — → KI baru:
  tabrakan acak 36^4 + edit-bebas; [PERLU KONFIRMASI] perlu unik?).
- **SJ supplier n/a** (sisi beli). **SJ keluar** milik 11.

## 3. Yang bukan penomoran

ID, tender/kembalian, Notes cetak, `discount_as_price` (parameter tampil).
