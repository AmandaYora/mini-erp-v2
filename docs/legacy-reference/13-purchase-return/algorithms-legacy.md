# Algorithms Legacy — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — Kurangi-dulu (hasil: hutang tak pernah minus oleh retur)

**Hasil yang diharapkan:** potong = min(retur, sisa-terutang-efektif); sisa → refund (izin) /
kredit-eksplisit; tanpa-sisa → potong-murni-tanpa-baris; paksa-salah ditolak. **Bebas diubah:**
ambang 0.009, default-refund.

## A-02 — Penuh-membatalkan (hasil: order habis = order tutup)

**Hasil yang diharapkan:** kumulatif-termasuk-dokumen-ini vs terima (terima-0 lewati);
penuh → flag + cancelled (kind-lalu-all; tanpa-cocok biarkan) + history + badge 3-tempat.
**Bebas diubah:** pencari status, selama non-gagal bertahan.

## A-03 — Lock-dulu (hasil: klik-ganda aman)

**Hasil yang diharapkan:** kunci baris-order (`pessimistic_write`) sebelum hitung-sisa —
pola yang TIDAK diwarisi sales (komentar kode eksplisit). **Bebas diubah:** cakupan lock;
jadikan standar terima/dispatch (KI-92/97).

## A-04 — Tanpa-id-di-metadata (hasil: margin sales kebal retur-beli)

**Hasil yang diharapkan:** movement retur-beli TIDAK membawa `idOrderItem` (id asli di kolom
`idOriginalOrderItem`); finance tak salah-akumulasi ke margin sales. **Wajib dipertahankan**
verbatim (aturan knowledge COMMERCE).

## A-05 — Lokasi-terakhir-default (hasil: barang kembali ke asalnya)

**Hasil yang diharapkan:** default = terima-terakhir per baris (timpa-manual; jatuh ke
default-cabang); non-tracked tanpa-lokasi; kurang-available ditolak (bukan on-hand!).
**Bebas diubah:** sumber default.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Kurangi-dulu + refund-izin (A-01) | Default, ambang |
| Penuh-membatalkan + non-gagal (A-02) | Status-cari |
| Lock-dulu (A-03) | Cakupan; standarkan |
| Tanpa-id-metadata (A-04) | — (verbatim!) |
| Lokasi-terakhir + available (A-05) | Sumber default |
