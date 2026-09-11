# Data Model Legacy — Modul 15 POS / Kasir

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tanpa tabel
sendiri (migrasi 015: kolom `source` di `orders`!); state cart + kunci di memori.

---

## 1. Tanpa tabel milik modul (1 kolom!)

`orders.source` (`manual`/`pos`, 015): penanda kasir. Efek: lewati-cek-transisi-serah (satu-
satunya!), tunda-Order-vs-langsung-POS (retur!), antrean-SJ-kecualikan, laporan-pisahkan.
Tanpa tabel cart/reservasi-persisten (reservasi TTL-600 di modul 16!).

## 2. Relasi (konsep)

```
POS --buat--> orders(source=pos) --serah--> payments(tunai/kembalian) + stock-out
POS --quote--> member (tanpa-simpan!) ; POS --snapshot--> cart-memori (kunci:varian)
POS --cetak--> payload-final --> shell-ESC/POS-LAN
```

Cabang = batas (guard-halaman + semua-API); tanpa-perusahaan-langsung.

## 3. Jejak baca-tulis

Baca: produk-36 + saldo-100 + scan + quote + pelanggan-10. Tulis: reservasi-hold/rileks +
order + serah (+ bayar) + saldo. Cetak: baca-order + payload (tanpa-tulis!).

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Satu kolom, empat efek** (`source` mengendalikan serah + retur + antrean + lapor).
2. **Cart-tanpa-persistensi** (refresh = hilang! tanpa-draf-tersimpan — [PERLU KONFIRMASI]
   disengaja kasir-cepat?).
3. **Kunci-acback-reservasi di memori** (refresh = yatim-600dk!).
4. **Tanpa-id-struk** (rujuk via order/bayar).
5. **Web-tanpa-bundle** (APK = URL-server! offline = mati-total).
6. **Termin-tersembunyi + Wide-mati + false-mati** (kode-mati-3!).
