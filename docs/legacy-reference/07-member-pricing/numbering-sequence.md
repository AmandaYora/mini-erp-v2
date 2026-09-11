# Numbering Sequence — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

---

## 1. Keputusan: modul ini TIDAK punya penomoran dokumen

Tidak ada sequence, auto-number, maupun generator. `code` jenis member diisi manual (≤50 char,
disimpan **UPPERCASE**, unik per perusahaan di antara non-arsip). Contoh seed/test: `A`, `GROSIR`
— tanpa pola wajib.

## 2. Identitas teknis

`member_types.id_member_type` (auto-increment; FK oleh `business_parties.id_member_type` +
snapshot `order_items.id_member_type_snapshot`); `order_items` menyimpan juga nama rule sebagai
teks (tahan rename). Tidak tampil ke user.
