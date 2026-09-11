# Algorithms Legacy — Modul 15 POS / Kasir

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — Jepit-stok-6-pintu (hasil: tak pernah minus dari kasir)

**Hasil yang diharapkan:** kartu/stepper/input/scan/tambah/reservasi dijepit-tersedia
(UOM-jual!); 0 = tanpa-tambah/hapus; non-tracked bebas. **Bebas diubah:** titik jepit.

## A-02 — Meterai-idempoten (hasil: klik-ganda = satu order)

**Hasil yang diharapkan:** sidik-isi + tertunda-sama → pakai-ulang; beda → error-ke-Order;
serah-ganda → abaikan-sukses; sukses-rotasi. **Bebas diubah:** format sidik.

## A-03 — Hantu-jangan-dibuat (hasil: reservasi tak menahan sia-sia)

**Hasil yang diharapkan:** hold ambient-350/TTL-600 (bukan proses/sukses!); rilis-saat-kosong;
konsumsi-saat-serah; kunci-per-coba. **Bebas diubah:** timer/TTL.

## A-04 — Angka-final-sekali (hasil: layar = kertas = fiskal)

**Hasil yang diharapkan:** web-hitung-semua (termasuk bruto + kembalian-bruto!); Kotlin-cetak-
apa-dikirim; omit-bila-tak-relevan; gagal-alert-manual. **Bebas diubah:** transport, kolom.

## A-05 — Quote-menghormati-tunai? (hasil: member-otomatis, manual-dihormati)

**Hasil yang diharapkan:** pilih = quote-otomatis menimpa semua baris (tanpa konsep
`priceEdited` — beda dengan form order yang menghormati! → KI baru); lepas = reset-standard;
gagal = banner + blokir-semua (termasuk-tunai!); diskon dijepit-ke-harga-baru.
**Bebas diubah:** pemicu, kecuali diputuskan.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Jepit-6-pintu (A-01) | Titik, UOM |
| Meterai + abaikan-ganda (A-02) | Sidik, tertunda |
| Hantu-guard + TTL (A-03) | Timer |
| Final-sekali (A-04) | Transport |
| Quote-otomatis + blokir-gagal (A-05) | Pemicu; hormat-manual? |
