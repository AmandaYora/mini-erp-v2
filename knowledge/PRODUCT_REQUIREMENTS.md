# Product Requirements — mini-erp

Ringkasan gateway. Dokumen produk penuh (latar belakang, prinsip cakupan, keputusan bisnis
terbuka, urutan fase) ada di **[../docs/PRD.md](../docs/PRD.md)** — baca itu sebelum mendesain
modul apa pun, jangan hanya file ini.

## Ringkasan Cakupan

- Satu perusahaan, multi-cabang. Peran: owner, admin, kasir, staf gudang, staf keuangan.
- 20 modul bisnis (lihat [MODULE_MAP.md](MODULE_MAP.md)), dibangun urut berdasarkan dependensi —
  bukan berdasarkan prioritas fitur semata.
- Prinsip cakupan per modul: **replikasi fitur + alur bisnis dari sistem lama, bukan replikasi
  bug-nya** — lihat [../docs/PRD.md §4](../docs/PRD.md#4-prinsip-cakupan-replikasi-perilaku-bukan-replikasi-cacat).
- Assistant AI/WhatsApp/Knowledge (modul 21 lama) **di luar cakupan tahap ini** — keputusan
  terpisah, lihat [../docs/PRD.md §3](../docs/PRD.md#3-cakupan-produk-peta-modul-bisnis).

## Sebelum Mendesain Modul Manapun

1. Baca bagian modul itu di [../docs/PRD.md §5](../docs/PRD.md#5-keputusan-bisnis-yang-perlu-dikonfirmasi-sebelumselagi-desain-modul) —
   apakah ada keputusan bisnis (Kelompok A) yang harus dikonfirmasi ke pemilik sistem dulu.
2. Baca `docs/legacy-reference/known-issues.md` bagian modul itu — putuskan eksplisit per known
   issue: perbaiki atau replikasi. Jangan diam-diam mewarisi salah satunya.
3. Baca dokumen legacy modul terkait (tabel routing di [MODULE_MAP.md](MODULE_MAP.md) §"Referensi
   Sistem Lama per Modul") untuk memahami fitur & alur yang sudah berjalan.
