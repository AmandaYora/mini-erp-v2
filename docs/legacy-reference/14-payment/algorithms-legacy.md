# Algorithms Legacy — Modul 14 Payment / Pembayaran

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — FIFO-menutup-tua (hasil: kasir tak perlu pilih order)

**Hasil yang diharapkan:** tanpa alokasi: tutup tertua (tempo → tanggal → id) sampai habis;
berhenti ≤0.009; total > tunggakan ditolak; manual: milik + >0 + ≤baris + total-pas.
**Bebas diubah:** ambang, urutan.

## A-02 — Rupiah-bulat-anti-recehan (hasil: tak ada sisa 0,x)

**Hasil yang diharapkan:** desimal ditolak di semua pintu (API + FE-buang); arsip-terminal
menjaga selesai-tak-jadi-berutang; ambang buka 0.009 vs bayar 0 (bedakan sadar — KI baru).
**Bebas diubah:** pesan, ambang.

## A-03 — Arsip-selektif (hasil: selesai-tak-rusak, draf-bebas)

**Hasil yang diharapkan:** tolak hanya terminal-non-net yang jadi-berutang; else bebas
(termasuk jadi-berutang non-terminal!); serentak bayar + alokasi; saldo kembali.
**Bebas diubah:** cakupan, selama terminal-tuntas bertahan.

## A-04 — Akumulasi-cetak-stabil (hasil: cetak-ulang = kebenaran-saat-itu)

**Hasil yang diharapkan:** urut-tanggal sampai-bayar-ini; sisa-sesudah; status darinya;
tanpa-bayar pesan-khusus. **Bebas diubah:** layout.

## A-05 — Arsip-boleh-ditagih (hasil: hapus tak menghapus utang)

**Hasil yang diharapkan:** pihak-arsip tetap di saldo/ledger/bayar; walk-in tetap
tak-tertagih; alokasi campur-tampil. **Bebas diubah:** filter, selama arsip-tertagih bertahan.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| FIFO + manual-pas (A-01) | Ambang, UI-manual |
| Bulat + tolak-terminal (A-02) | Pesan, ambang-buka |
| Selektif + serentak (A-03) | Cakupan |
| Akumulasi + stabil (A-04) | Layout |
| Arsip-tertagih (A-05) | Filter |
