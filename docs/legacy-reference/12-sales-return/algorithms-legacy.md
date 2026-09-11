# Algorithms Legacy — Modul 12 Sales Return (Retur Penjualan)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — Selisih bertingkat (hasil: yang berutang dulu yang dipotong)

**Hasil yang diharapkan:** selisih = pengganti − retur (rupiah); positif → tambah-bayar; negatif
→ potong-piutang-efektif (termasuk retur-lama) dulu, sisa → kredit (berpihak) / refund
(walk-in, izin); nol → tanpa-penyelesaian; paksa-salah → ditolak berpesan. **Bebas diubah:**
urutan, ambang 0.009.

## A-02 — Tunda Order vs langsung POS (hasil: konter vs supir)

**Hasil yang diharapkan:** tukar-POS-fisik keluar-seketika (+ guard modal); tukar-Order-fisik
ditunda ke SJ (tanpa gerak/guard/lokasi; stok-tua-tetap); jasa tak-perlu-kirim. Keputusan satu
fungsi teruji 4 kasus. **Bebas diubah:** pemicu (source), selama watak bertahan.

## A-03 — Proporsi retur (hasil: sebagian wajar)

**Hasil yang diharapkan:** nilai = rasio-qty × harga/pajak baris-asal (penuh, bukan harga-kini);
sisa = order − retur-selesai; lokasi RUSAK-otomatis vs pilih vs tanpa-gerak per kondisi.
**Bebas diubah:** pembulatan rasio.

## A-04 — SJ pengganti (hasil: pengganti sampai dengan jejak)

**Hasil yang diharapkan:** dispatch (sopir/gudang + lokasi-per-item + guard-modal-semua +
stok-keluar + nomor-SJ) → confirm (TTD-nama/alasan, tanpa-arsip-wajib) → `delivered`; baris
ditaut movement + lokasi. **Bebas diubah:** syarat arsip (beda SJ-order disengaja).

## A-05 — Preview-mengikat (hasil: yang dicek = yang disimpan)

**Hasil yang diharapkan:** modal = rencana-server-penuh (4 kartu + notice + 2 tabel); tiap ubah
menggugurkan; simpan memakai payload-preview; tanpa-preview ditolak. **Bebas diubah:** widget,
selama invalidasi-total bertahan.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Potong-dulu + kredit/refund + tanpa-baris (A-01) | Ambang, baris |
| Tunda-vs-langsung + guard-di-keluar (A-02) | Pemicu source |
| Rasio + sisa + RUSAK-otomatis (A-03) | Bulat, lokasi |
| Dispatch-confirm + nama-ketat (A-04) | Arsip-syarat |
| Preview-mengikat + invalidasi (A-05) | Modal, state |
