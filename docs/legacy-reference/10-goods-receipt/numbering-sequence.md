# Numbering Sequence — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

---

## 1. Keputusan: modul ini TIDAK punya penomoran

Penerimaan tidak menerbitkan nomor: tanpa sequence, tanpa kolom nomor, tanpa format. Identitas
operasional = tanggal terima + no SJ supplier (opsional, teks bebas supplier, boleh ganda) +
lokasi + id internal (`id_goods_receipt`, auto-increment, tak tampil sebagai nomor dokumen).

Konsekuensi tercatat (→ KI-89): merujuk penerimaan via telepon/catatan memakai kombinasi
tanggal + SJ + lokasi; dua batch sehari tanpa SJ hanya beda jam (`receivedAt` presisi 6).

## 2. Nomor terkait (milik modul lain)

- PO: `ORD-{cabang}/PB/...` (08). Bayar COD: `PAY-...` (08/14). SJ supplier: dokumen pihak
  ketiga (bebas).

