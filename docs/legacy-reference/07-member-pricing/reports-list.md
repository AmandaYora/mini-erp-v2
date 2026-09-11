# Reports List — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari controller,
service, halaman FE, dan test.

---

## 1. Keputusan: modul ini TIDAK punya laporan

Tidak ada endpoint rekap, halaman rekap, ekspor rekap, maupun agregasi milik modul ini. Satu-satunya
angka — `"{n} jenis member tersedia."` — adalah `meta.total` list.

## 2. Data modul ini sebagai BAHAN laporan modul lain

| Laporan (pemilik) | Field yang dipakai | Kontrak |
|---|---|---|
| Export order + nota/cetak (08–11) | 7 snapshot pricing per baris (`pricing_source`, member, basis, aturan, `basis_price`) | Snapshot final; hemat member = `basis − unit` hanya bila `member_rule` |
| Margin/laba (17 Finance) | `unit_price` + snapshot (margin dihitung dari harga jual aktual = harga member) | Harga member adalah harga jual sah, bukan diskon |
| Daftar pelanggan (06) | badge/nama member | Relasi live |
| Riwayat Aktivitas (20) | `member_type.create/update/archive` (snapshot penuh before+after) | Satu-satunya jejak perubahan rule |

## 3. Bahan mentah bila laporan ingin dibuat

`member-types/list` (`limit` ≤1000, `status`, `search`) + `audit_logs` (`member_type.*`, before+after
10 field). Tanpa itu, pertanyaan "rule Grosir berubah apa saja tahun ini" hanya bisa dijawab dari
audit mentah.

## 4. Yang eksplisit BUKAN laporan

- **Kolom Rule/Pembulatan**: render satu baris (`Harga beli + Rp 1.000`), bukan agregasi.
- **`pricing/quote`**: kalkulator pra-transaksi (tidak menyimpan, tidak berekor).
