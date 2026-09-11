# Data Model Legacy — Modul 09 Order Sales

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Struktur tabel =
modul 08; di sini deltas sisi-sales.

---

## 1. Pemakaian kolom sisi-sales

- `orders`: `id_related_party` boleh null (walk-in non-net); ship-to 5 kolom aktif (snapshot
  + id-lunak); `goods_delivered_at` sekali-tulis; `paymentTerms` cod default; invoice-supplier
  selalu null (ditulis hanya purchase).
- `order_items`: `pricing_source` bermakna (standard/member_rule/manual) + 7 snapshot member;
  `lineDiscountAmount/Percent` aktif; `unit_price_before_discount` = penuh (anti-ganda).
- `payments`: + `amount_tendered` (tunai-serah; null else) — detail milik 14.

## 2. Relasi (konsep, delta dari 08)

```
business_parties(customer) 1──* orders(sales; null-boleh; live by-id)
orders ──snapshot──> ship-to (buku 06; fallback kontak)
orders 1──* payments receivable (seksi + tempo + COD-serah)
member_types ──snapshot──> order_items (7 kolom; future-only)
stock_reservations ──konsumsi──> deliver (kunci user; consumed + id order)
```

## 3. Jejak baca-tulis (delta)

| Fitur | Tulis |
|---|---|
| Buat/ubah SO | order + items (snapshot member + ship-to) + history(buat) + audit |
| Serah langsung | stok keluar + reservasi + tanggal + selesai + history + 1–3 audit |
| Approve tempo | termin + tempo + approve + audit |
| Bayar/bukti/hapus | milik 14 (UI di sini) |

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Pesan tipe-salah menyebut tempo** untuk semua sales (E-02) — teks vs jangkauan.
2. **Carve-out pos lapis-1 vs lapis-2** (KI baru) — pengecualian tampak ada tetapi tak berpengaruh.
3. **`amount_tendered` hanya tunai** — non-tunai tidak mengenal "diterima" (kembalian n/a).
4. **Referensi auto tanpa unik** (KI baru).
5. **`lineDiscount*` ganda makna** (input-total vs simpan-perunit) — FE vs DB beda satuan.
6. **Ship-to id-lunak** (boleh null; cetak pakai snapshot) — rujukan, bukan FK keras.
