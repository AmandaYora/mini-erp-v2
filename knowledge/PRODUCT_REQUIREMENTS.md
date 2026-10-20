# Product Requirements — mini-erp

Ringkasan gateway. Dokumen produk penuh (latar belakang, prinsip cakupan, keputusan bisnis,
di luar cakupan) ada di **[../docs/PRD.md](../docs/PRD.md)** — baca itu sebelum mengubah perilaku
modul apa pun, jangan hanya file ini.

## Ringkasan Cakupan

- Satu perusahaan, multi-cabang. Peran bawaan: lihat [DOMAIN_GLOSSARY.md](DOMAIN_GLOSSARY.md).
- 20 modul bisnis (lihat [MODULE_MAP.md](MODULE_MAP.md)), dibangun urut berdasarkan dependensi.
- Prinsip cakupan per modul: **replikasi fitur + alur bisnis dari sistem lama, bukan replikasi
  bug-nya** — lihat [../docs/PRD.md §4](../docs/PRD.md#4-prinsip-cakupan-replikasi-perilaku-bukan-replikasi-cacat).
- Modul 21 lama (Assistant) dibawa **hanya sebagai bot WhatsApp + tooling operasional read-only**;
  AI generatif dan Knowledge/RAG di luar cakupan ([../docs/PRD.md §9](../docs/PRD.md#9-di-luar-cakupan-untuk-saat-ini),
  [ADR-0018](decisions/ADR-0018-keputusan-paritas.md) D1).
- Yang sengaja tidak dibawa dari sistem lama: [../docs/PARITY_AUDIT.md §6](../docs/PARITY_AUDIT.md).

## Sebelum Mengubah Perilaku Sebuah Modul

1. Cek apakah sudah ada keputusan: [decisions/](decisions/) (terutama ADR-0018) dan
   [../docs/PRD.md §5](../docs/PRD.md#5-keputusan-bisnis-yang-perlu-dikonfirmasi-sebelumselagi-desain-modul).
2. Baca `docs/legacy-reference/known-issues.md` bagian modul itu — putuskan eksplisit per known
   issue: perbaiki atau replikasi. Jangan diam-diam mewarisi salah satunya.
3. Baca dokumen legacy modul terkait (tabel di [MODULE_MAP.md](MODULE_MAP.md) §"Referensi
   Sistem Lama per Modul") untuk memahami fitur & alur yang sudah berjalan.
