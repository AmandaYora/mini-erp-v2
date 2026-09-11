# Algorithms Legacy — Modul 16 Stock / Inventory & Gudang

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai** (+ rumus/parameter-eksak dari kode sebagai acuan).

---

## A-01 — Berpasangan-ke-daun (hasil: angka selalu bisa diaudit)

**Hasil yang diharapkan:** tiap saldo punya movement (`balanceBefore/After`, `referenceType` +
`idReference`-null-boleh, `reasonText`, `metadata`, `idMovedBy`, `movedAt`); tiap tulis ke
daun-aktif (`assertLeafLocation`: 404-hilang/arsip, 400-non-aktif/beranak!); agregat menjumlah
(`SUM(on/res/avail)`, `locationCount = SUM(on_hand > 0)`); `available = on_hand − reserved`
di 10-pintu-tulis; unik `(cabang, varian, lokasi)`. **Rumus:** `after_in = before + qty`;
`after_out = before − qty` (tolak bila `before < qty` — banding-`onHand`!);
`after_set = qty` (bebas!); `available = after − reserved` (bisa-negatif-bila-reservasi!).
**Bebas diubah:** tabel, kecuali invarian verbatim (aturan arsitektur!).

## A-02 — Petik-prioritas (hasil: keluar dari tempat terbaik dulu)

**Hasil yang diharapkan:** saran + reservasi (+ serah modul lain) memakai urut deterministik:
`isPickingArea` ↓ → `isPrimary` ↓ → `isDefault` ↓ → `available` ↓ (toleransi-0.0001!) →
`sortOrder` ↑ → `id` ↑ (`sortAllocationCandidates`, `stock.service.ts:298-319`); hanya
daun-aktif-tersedia (`onHand > 0` DAN `available > 0`!); serakah (`take = min(available,
sisa)`); manual = milik + cukup + total-pas; respons selalu `{enough, shortage(+base),
total_available(+base)}`. **Bukan-FIFO-tanggal** (tanpa-kolom-umur! — cek!). **Konversi:**
`factor = normalize(input ?? sisi-order ?? 1)` (null/''/≤0 → fallback!);
`minta_base = input ?? qty × faktor`; `tampil = base ÷ faktor`; label
`Y transaksi (X basis)`; step = faktor-bulat-≥1 ? 1 : 0.0001. **Bebas diubah:** urutan,
selama deterministik.

## A-03 — Kunci-sementara (hasil: kasir tak berebut)

**Hasil yang diharapkan:** hold (kunci-trim + ≥1-baris + **TTL-jepit-60–1800-default-600** +
kedaluwarsa-dulu + lepas-kunci-user-dulu + non-tracked-lewat + qty-base > 0 + serakah +
kurang-rollback-penuh + baris-`pos_cart`-`idUser`-`idOrder-null` + `reserved +=` +
`expiresAt = now + TTL`) → konsumsi (`consumed`, di luar modul!) / lepas-manual
(kunci-**user-sama** → `released` + `reserved −=` + `releasedAt` + `released_count`) /
kedaluwarsa-otomatis (aktif + habis → `expired` + `reserved −=` + `releasedAt`, **hanya saat
hold-berikutnya** — tanpa-penjadwal!). **Rumus:** `reserved' = reserved ± Σqty`;
`available' = onHand − reserved'`; `max(0, ...)` hanya di sisi-baca-rusak, bukan-reservasi!
**Bebas diubah:** TTL, pemicu kedaluwarsa.

## A-04 — Otorisasi-manusia (hasil: koreksi selalu ada penanggung jawab)

**Hasil yang diharapkan:** tiap `adjust` wajib kredensial (`identifier ?? username ?? email`
trim + `password`; kosong → 400); user aktif-perusahaan (username ATAU email) + bcrypt
(else 401 **satu-pesan-dua-sebab**); izin `stock.adjust.approve` (else 403; peran-global atau
secabang!); mode (`simple` = siapa-pun-**termasuk-diri**; `strict` = bukan-diri-else-403!);
default-`simple` (**salah-ketik = simple** — disengaja? cek!); metadata
`{requestedByUserId, approvedByUserId, approvalMode, approvedAt-ISO}` di movement +
audit-`stock.adjust`; modal-gagal berpesan-spesifik (modal vs keluar!). **Tanpa-threshold-
nominal** (tiap-adjust-approve, bukan batas-Rp!). **Bebas diubah:** mode, kecuali
default-`simple`-disengaja?

## A-05 — Buka-sekali + harga-master (hasil: migrasi tak mencemari HPP)

**Hasil yang diharapkan:** konteks-saran (`existing_stock`/`single_location`) + template-
hanya-belum + validasi-ketat-12-isu (wajib-3-kolom + non-negatif + master/varian/lokasi/daun/
aktif + **tanpa-harga-master = tolak** + duplikat + terisi-`already_filled`-lewati!) +
timpa-saldo (`saldo = qty`, bukan-tambah!) + movement-`opening_stock`
(`idReference` = id-buka-keuangan!) + finance-gabung-aditif + tolak-posted-dua-lapis +
tolak-error/nol/terisi. **Rumus:** `avg = roundMoney(beli ÷ faktorBeli)` (base!;
`resolveBaseUomCost`); `nilai = roundMoney(qty × avg)`; agregat-per-produk
(`qty-sum`, `nilai-sum`, `avg = nilai ÷ qty`); gabung (`qty +=`, `nilai +=`, `avg-ulang`).
**Parser:** sheet-alias-4; header-alias-7; angka-fleksibel (ID `1.234,5` vs US `1,234.5`,
pemisah-terakhir = desimal!); kode-normalisasi (lower-tanpa-`?`-rapat-UPPER); baris-2;
kosong-lewat. **Bebas diubah:** kolom, kecuali harga-master verbatim.

## A-06 — Rusak-terpisah + pulih/buang (hasil: rugi terlihat, layak kembali)

**Hasil yang diharapkan:** RUSAK-otomatis (`RUSAK`/`Barang Rusak`/9999/`damaged`; buat-bila-
hilang; tolak-beranak; auto-perbaiki-status!) + daftar-rugi (`qty × beli`, `roundQty`-
4-desimal, `total_estimated_loss`-**global**!) + masuk (yatim-dari-nol **by-design** vs
asal-kurang-`available`!) + pulih (cukup-`onHand` + daun-tujuan + `max(0,...)`-tersedia!) +
buang-final-satu-gerak (`damaged_write_off` + `estimatedLossAmount`!); varian-wajib-FE;
hidden-jadi-null. **Gerak:** masuk = asal-`out`-`damaged_stock` + rusak-`in`; pulih =
rusak-`out` + normal-`in` (`damaged_restore`); buang = rusak-`out` saja. **Bebas diubah:**
lokasi-istimewa, kecuali status-`damaged`.

## A-07 — Dokumen-tiga-status (hasil: jalan-tanpa-hilang)

**Hasil yang diharapkan:** draf-tanpa-gerak (+ nomor-kunci-tulis!) → kirim-kurang
(cabang-asal + `draft` + tersedia → `out`-`stock_transfer`-`in_transit` + `idOutMovement`) →
terima-masuk (cabang-tujuan + `in_transit` + wajib-`idOutMovement` → `in` +
`pairedOutMovementId` + `idInMovement`) / batal-draf-tanpa-gerak (cabang-asal!);
seketika-dua-gerak-`transfer` (+ `paired_movement_id`!); final-tanpa-kembali
(`received`/`cancelled`!); cetak-fisik-tanda-3; arah-badge
(`Dalam Cabang`/`Keluar`/`Masuk`). **Nomor:** `TRF-{cabang}/{tahun}/{5-digit}`
(kunci-tulis + buat-otomatis + tanpa-reset — lihat
[numbering-sequence.md](numbering-sequence.md)). **Bebas diubah:** status, kecuali
final-tanpa-kembali.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Berpasangan + daun + rumus `available = on_hand − reserved` (A-01) | Tabel (verbatim!) |
| Prioritas petik-primer-default + manual-pas + `enough/shortage` (A-02) | Urutan (deterministik!) |
| Hold/konsumsi/lepas/habis + TTL-jepit (A-03) | TTL, pemicu kedaluwarsa |
| Manusia + mode + metadata + tanpa-threshold (A-04) | Mode-default? (cek!) |
| Sekali + master + tolak-posted + timpa (A-05) | Kolom (kecuali harga!) |
| Terpisah + rugi-global + final-buang (A-06) | Istimewa (kecuali `damaged`!) |
| Draf-kirim-terima/batal + seketika-2-gerak (A-07) | Status (kecuali final!) |
