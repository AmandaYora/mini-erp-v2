# Reports List — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, FE finance, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas (tetapi punya dua agregat operasional)

Tanpa export/rekap bayar. Yang ada — dan dipakai sebagai "laporan kerja" kolektor:

| Agregat | Endpoint | Logika |
|---|---|---|
| Saldo per pihak | `party-balances` (sisi wajib) | Terbuka per pihak: hitung + sisa + tertua; urut tertua; total global; paginasi opt-in; tanpa-pihak dilewati; batal dikecualikan; ambang 0.009 |
| Ledger pihak | `party-ledger` | Pihak (boleh arsip) + tagihan-terbuka (umur + bucket) + bayar (+ alokasi) + total; paginasi terpisah; total global |

Konsumen resminya kartu Piutang/Utang + dialog `Rincian {nama}` (modul 17). Rumus di BR-09…11.

## 2. Dokumen per bayar (kontrak angka)

Bukti bayar: 4 baris akumulatif (§F-06 feature); status = sisa-sesudah; stabil-cetak-ulang.

## 3. Angka operasional

Badge/seksi order (Dibayar/Total/Sisa/Status) · placeholder = sisa · `{n}` antrean-prepaid ·
umur-bucket ledger · `total_outstanding` kartu.

## 4. Bahan modul lain

Bayar + alokasi + retur-efek → gerbang order + badge + antrean + posting finance + rekap SPT;
bukti → audit; saldo → penagihan.

## 5. Yang eksplisit BUKAN laporan

- Daftar per-order (riwayat tabel) · tabel bayar ledger (riwayat) · toast/referensi ·
  `amount_tendered` (catatan struk).
