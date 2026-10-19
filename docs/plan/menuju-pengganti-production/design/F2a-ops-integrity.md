# Rancangan F2a — Integritas OPERASIONAL uang & stok

Cakupan: modul `delivery`, `goodsreceipt`, `sales`, `purchasing`, `salesreturn`, `purchasereturn`, `stock`, `payment` (sisi operasional; jurnal = agen finance), plus `branch` (penomoran) dan sisi web POS.
Basis: pembacaan kode sendiri di `apps/api` (revamp) dan `H:/dimasprasetio/SAAS/mini-erp/apps/api/src/modules` (legacy). Semua nomor baris di bawah hasil bacaan saya, bukan salinan audit.
Aturan yang dipatuhi: modul hanya impor `contracts/` modul lain; tanpa join/FK lintas modul; transaksi hanya di dalam satu modul; tanpa UnitOfWork lintas modul dan tanpa outbox (keputusan 3); uang integer rupiah; envelope `{success,message,data}`; sesi dari token; `/api/v1`; satu container; tanpa Redis. Kode baru bergaya `database/sql` (sqlc fase terakhir, di luar paket ini).

Konvensi label bukti: TERBUKTI = saya baca kodenya dan klaim benar; SEBAGIAN = inti benar, ada detail yang keliru/kurang; TERBANTAH = klaim salah.

---

## 0. Ringkasan eksekutif

1. Akar masalah integritas di paket ini hanya tiga pola, jadi perbaikannya juga tiga pola yang dipakai ulang di semua modul (bagian 1):
   - stok bergerak lewat `MoveIn/MoveOut` satu-satu, tiap panggilan tx sendiri, tanpa kunci idempotensi -> tidak aman diulang;
   - status dokumen dibaca di luar lock dan ditulis tanpa `WHERE status=` -> dua request bisa sama-sama lolos;
   - sisa kuantitas (kirim/terima/retur) dihitung hanya dari dokumen "final" dan tidak diakumulasi dalam satu request -> kuota bisa ditembus.
2. Satu perubahan kontrak besar di `stock/contracts`: `Post` (batch, atomik per dokumen, idempoten, terurut anti-deadlock) + `Reverse` menggantikan `MoveIn/MoveOut`. Semua kompensasi `_ = s.stock.MoveIn(...rollback)` dihapus; pemulihan = ulang request (forward recovery), bukan kompensasi.
3. Ada temuan tambahan yang TIDAK ada di audit (bagian 0.2): GR yang gagal MoveIn tetap dibaca finance sebagai "siap diposting" (`ListRecent` tanpa status), kunci `party:0` menyerialkan semua pembayaran walk-in, `delivery.Create` tidak pakai doclock sama sekali, rollback `Release/Hold` punya urutan kunci terbalik, kursor biaya finance berbasis `MAX(id)` bisa melewatkan baris yang commit terlambat.
4. Migrasi: semua additive, dikelompokkan jadi blok `000040`-`000043` (nomor final diberikan saat merge; agen squash tinggal melipat). Tidak ada perubahan skema yang menghapus data.
5. Urutan implementasi yang disarankan: F2a-11 -> F2a-0 -> F2a-4(harness) -> F2a-2 -> F2a-3 -> F2a-1 -> F2a-5 -> F2a-9a -> F2a-7 -> F2a-6 -> F2a-10 -> F2a-8 -> F2a-9b -> F2a-12 (detail bagian 4).

### 0.1 Verifikasi klaim audit (ringkas)

| # | Klaim audit | Status | Bukti / koreksi |
|---|---|---|---|
| 1a | `ReturnedQty` hanya hitung `confirmed` | TERBUKTI | `salesreturn/infrastructure/repository.go:85-99` (komentar "drafts reserve nothing"); `purchasereturn/infrastructure/repository.go` fungsi `ReturnedQty` (~77-92) identik |
| 1b | Confirm membaca status di luar lock; `confirmLocked` tidak cek ulang sisa | TERBUKTI | `salesreturn/application/service.go:384-389` (baca) lalu lock `392`; `confirmLocked` `407-428` tidak memanggil `ReturnedQty`. Purchase: `service.go:335-340`, lock `343`, `358-388` |
| 1c | Dua baris produk sama dalam satu request lolos | TERBUKTI | `computeReturn` `252-323`: tiap baris dibanding `delivered-returned` dari DB (`270-281`), tidak ada akumulator antar baris. Purchase `217-287` (`235-246`) |
| 1d | Dua draf penuh -> dua-duanya bisa dikonfirmasi -> stok masuk 2x, piutang dikredit 2x | TERBUKTI (turunan 1a+1b) | Confirm tidak cek ulang; `SetStatus` tanpa `WHERE status` (`repository.go:155-160`) |
| 1e | "Legacy menolak duplikat per order-item dan mengunci baris order (sales-return.service.ts:648-657; purchase-return.service.ts:213,458)" | SEBAGIAN | Tolak duplikat: benar (`sales-return.service.ts:648`, `purchase-return.service.ts:458`). Kunci baris order: HANYA retur beli (`purchase-return.service.ts:213`, komentar eksplisit: "sales-return.service.ts belum mengunci"). Retur jual legacy TIDAK mengunci -> jangan jadikan legacy rujukan perilaku untuk sisi jual; rujukan = rancangan ini |
| 2a | `delivery.Create` hanya bandingkan dengan SJ terkonfirmasi, tak jumlahkan baris lain dalam request | TERBUKTI | `delivery/application/service.go:198-205` (`DeliveredQty` = hanya `status='confirmed'`, `repository.go:109-122`), `orderedBase` tak dikurangi baris sebelumnya |
| 2b | `confirmLocked` tak cek ulang sisa; status nota dibaca di luar lock | TERBUKTI | `service.go:260-266` baca di luar lock, `271` lock, `286-377` tak ada pembacaan ulang nota; hanya `shouldComplete` (`300-335`) |
| 2c | (tambahan, tak ada di audit) `Create` tak memakai doclock sama sekali | TERBUKTI | `service.go:134-250` tak ada `doclock.Lock`; bahkan pasangan `Create`+`Create` pada SO yang sama balapan. Retur & GR memakainya, SJ tidak |
| 2d | GR: baris duplikat dalam satu request lolos | TERBUKTI | `goodsreceipt/application/service.go:168-189` (`ReceivedQty` per baris dari DB, `182`) |
| 2e | `ReceivedQty` menghitung baris yang MoveIn-nya gagal & di-rollback | TERBUKTI | `goodsreceipt/infrastructure/repository.go:75-86` tanpa kolom status; `CreateReceipt` (239) commit SEBELUM MoveIn (251-261); kompensasi `MoveOut ... _ =` (255-256) tidak menghapus baris GR |
| 2f | (tambahan) GR gagal tetap "postable" bagi finance | TERBUKTI | `goodsreceipt/contracts/client.go` komentar `ListRecent`: "every row is postable"; `finance/application/queue.go:78` memakainya. Jadi GR yang stoknya di-rollback tetap bisa dijurnal (Dr Persediaan/Cr Hutang untuk barang yang tak pernah masuk) |
| 3 | Saga tanpa kompensasi / kompensasi mengabaikan error | TERBUKTI | `delivery/service.go:347-376`; `ConfirmReplacement 587-608`; `salesreturn 413-427, 464-470` (Cancel: `return err` di tengah loop tanpa balik); `purchasereturn 372-386, 417-423`; `goodsreceipt 251-266` |
| 4a | Transfer: status dicek di luar tx, `SetTransferStatusTx` tanpa `WHERE status` | TERBUKTI | `stock/application/transactions.go:93-100,102,126`; `143-152`; `182-196`; `repository.go:710-722`. Draft-cancel memakai `SetTransferStatus` non-tx (`transactions.go:192`) |
| 4b | Modul stock tak pakai doclock | TERBUKTI | grep: doclock hanya di delivery, goodsreceipt, payment, purchasereturn, salesreturn |
| 4c | `move-location` tidak atomik | TERBUKTI | `stock/application/movelocation.go:19-29` tiga panggilan terpisah; jika `Receive` gagal -> transfer tertahan `dispatched` (in-transit di cabang yang sama) |
| 5a | Cancel SO/PO tidak cek pembayaran/SJ/GR | TERBUKTI | `sales/application/service.go:439-452`, `purchasing/application/service.go:413-426`; `SetStatus` kontrak (`sales 79-100`) juga tanpa CAS |
| 5b | `payment.Cancel` tak periksa efek | TERBUKTI | `payment/application/service.go:516-529`; legacy `payment.service.ts:408-411` menolak arsip jika order terminal non-net jadi belum lunas |
| 5c | settle-credit dua kaki bisa dibatalkan sendiri-sendiri | TERBUKTI | `service.go:603-609` dua `insertPayment` terpisah; kolom penaut tak ada (migrasi `000013`). Pemulihan kaki-1 saat kaki-2 gagal ada (`610-616`) tapi tidak menutup kegagalan `SetStatus` itu sendiri |
| 6 | Barang non-stok/jasa tak ditangani | TERBUKTI | grep `Tracked` di luar `product/`: kosong. `stock.MoveIn` (`service.go:410-430`) hanya cek produk `active`; tak ada snapshot `stock_tracked` di `sales_order_items`/`purchase_order_items` (`000009`, `000010`) |
| 7a | Lokasi RUSAK `active` + `kind` dibuang | TERBUKTI | `stock/infrastructure/repository.go:127-130` (status 'active' literal), `161-181` (`_ = kind` di baris 179). `resolveLocation` (`service.go:91-110`) tak membedakan; `SuggestLocations` (414-437) & `ProductAvailability` (259-277) menghitungnya |
| 7b | Legacy mengecualikan status damaged | TERBUKTI | `delivery.service.ts:246-282` memfilter `location.status !== 'active'`; lokasi RUSAK legacy berstatus `damaged` (`sales-return.service.ts:1081-1091`) |
| 7c | Retur tukar: total hanya nilai barang kembali; pengganti tanpa pajak & tanpa cek harga min; selisih tak dihitung | TERBUKTI | `salesreturn/service.go:354`; `priceReplacements 847-887` (`879` `p.SellingPrice`, `883` `LineTotal` tanpa pajak). Legacy: `sales-return.service.ts:672-680` (differenceAmount), `794-824` (`resolveSettlement`), `calculateReplacementLine 747-` cek `minSellingPrice` + konteks pajak order |
| 7d | Refund butuh izin khusus | TERBUKTI | legacy `sales-return.service.ts:904` `sales_return.refund`, `purchase-return.service.ts:569` `purchase_return.refund`; revamp: rute payment hanya `payment.create` (`payment/presentation/handler.go` RegisterRoutes) |
| 7e | Retur otomatis kurangi piutang/hutang | TERBUKTI | legacy `resolveSettlement` + `resolveOutstandingBeforeReturn` (`sales-return.service.ts:1143-1210`), `purchase-return.service.ts:270-300`; revamp memo-only (`salesreturn 649-695`) |
| 7f | Alasan retur wajib; tanggal retur beli tak boleh sebelum penerimaan/masa depan | TERBUKTI | legacy `sales-return.service.ts:637`, `purchase-return.service.ts:430-446`; revamp tak ada kolom/validasi alasan (`000014/000015`) |
| 8a | Create/UpdateLocation tak cek saldo induk | TERBUKTI | `stock/application/service.go:478-515, 518-560`. Legacy `stock-location.service.ts:85-182` (`PARENT_HAS_STOCK` + `force_transfer`) |
| 8b | Adjust legacy SELALU wajib approver + "harga modal" | SEBAGIAN | Approver SELALU: benar (`stock.service.ts:1168` -> `verifyStockAdjustmentApproval` tak bersyarat; `150-205`). "Harga modal" BUKAN input: itu pemeriksaan keberadaan basis biaya (`assertProductCostBasis`, `stock.service.ts:1137-1153`): in = average cost ATAU harga beli > 0, out = average cost > 0. Revamp: ambang `>100` unit (`transactions.go:322`), tak ada cek basis biaya |
| 8c | bcrypt di dalam tx saat memegang `FOR UPDATE` | TERBUKTI | `transactions.go:308` (`GetBalanceTx` FOR UPDATE) lalu `323` `checkApprover` -> `VerifyCredentials` |
| 8d | Alasan wajib di move-in/restore/write-off rusak | TERBUKTI (revamp: hanya write-off) | `transactions.go:432-435` (opsional), `470-472` (wajib hanya write-off); legacy `stock-damaged.service.ts:119,198,273` |
| 8e | Scan hanya barcode | TERBUKTI | `product/infrastructure/repository.go:428-443` (`barcode = ?`); legacy `stock.service.ts:985-996` (`variant_code OR barcode`, lalu fallback `product_code`) |
| 8f | Opening: tanpa guard finance, preview tak deteksi duplikat | TERBUKTI | `stock/application/opening.go:205-230` (preview), `239-302` (commit; `posKey` hanya untuk riwayat movement). Legacy `stock-opening.service.ts:296-298` (tolak jika FinanceOpeningBalance 'posted'), `531-533` (harga beli wajib >0; harga modal dari master, bukan Excel) |
| 8g | `stock_movements` tanpa balance_before/after & waktu efektif | TERBUKTI | `000008_stock.up.sql:36-50` |
| 8h | Transfer: tujuan per item (legacy) vs satu pasang per dokumen | TERBUKTI | `stock/infrastructure/repository.go:639-667`, tabel `stock_transfer_items` tanpa lokasi; legacy `stock-transfer.service.ts:112-130` per item `id_from_location`/`id_to_location` |
| 8i | `GET /stock/balances` tanpa productId mengembalikan `[]` | TERBUKTI | `stock/presentation/handler.go:143-170` |
| 8j | Deadlock: kunci urut baris dokumen, tak ada retry 1213/1205 | TERBUKTI | `transactions.go:107-125`; `shared/dberr/dberr.go` hanya memetakan 1062/1451/1452 |
| 9a | Revamp tanpa harga manual & harga minimum | TERBUKTI | `sales/presentation/handler.go` `linePayload` (66-73) tanpa `unitPrice`; `priceLines` (`sales/service.go:216-218`) hanya katalog/member; `p.MinSellingPrice` ada di kontrak product tapi tak dipakai modul order. Legacy `order.service.ts:430-436` (cek hanya jika sumber harga bukan `member_rule`) |
| 9b | Termin prepaid & COD wajib lunas | TERBUKTI | `sales/service.go:267` & `purchasing validateHeader` hanya `net`/`cod`; tak ada pemeriksaan pelunasan di delivery/GR. Legacy `delivery.service.ts:444-499` (deliverGoods), `708-715` (SJ prabayar), `880-916` (confirm SJ final) ; `goods-receipt.service.ts:153-203`; `order.service.ts:769-771, 1267-1280` |
| 9c | Net wajib customer aktif + HP | TERBUKTI | revamp `sales/service.go:258-272` (party 0 diizinkan untuk net); legacy `order.service.ts:1289-1313` |
| 9d | `order_date` hanya tanggal | TERBUKTI | `resolveDate` `sales/service.go:298-303`; kolom DATETIME(6) menyimpan 00:00 |
| 9e | Snapshot baris kurang; nama varian dibuang | TERBUKTI | `000010_sales.up.sql:32-49`; `purchasing/service.go:166` `_ = variantName`; legacy `order.service.ts:459-485` (variant name/code/attributes, base uom, pricing snapshot, discountReason, taxable, tax base/amount, notes, stockTrackedSnapshot) |
| 9f | notes VARCHAR(255) vs TEXT | TERBUKTI | `000010:18`, `000009:18`; legacy `order.entity.ts:107` `text` |
| 9g | Update setelah konfirmasi hanya draft | TERBUKTI | `sales/service.go:354`; legacy `order.service.ts:549` hanya tolak jika terminal; item hanya boleh diganti bila tak ada movement/payment (`assertNoStockMovementOrPayment`, `1236-1265`) |
| 9h | Selisih rounding diskon per-unit vs per-baris | TERBUKTI, maksimum Rp1/baris, hanya saat pecahan tepat .5 | Lihat F2a-10 (verifikasi aritmetika) |
| 10 | Kunci counter bulanan UTC, label WIB | TERBUKTI | `branch/infrastructure/repository.go:226-238` (`now.Format("2006-01")` dari `timeutil.NowUTC()`), `277-280`; label `branch/application/service.go:285-286` (Jakarta). Tumbukan: pukul 00:00-07:00 WIB tanggal 1 memakai counter bulan lalu tetapi label bulan baru; setelah 07:00 counter bulan baru mulai dari 1 dan akhirnya menyentuh nomor yang sama -> `uq_*_number` -> 409 |
| 11a | POS: langkah tidak idempoten, stok tak ditahan | TERBUKTI | `apps/web/src/modules/pos/pages/PosPage.tsx:397-560`, `pos.service.ts:92-111`; grep `reservations|/hold` di `apps/web/src`: kosong |
| 11b | `use-pos-cart.ts:150-169` | TERBANTAH (lokasi) | File tak ada; keranjang di `pos/lib/cart.ts` (hanya `qty*unitPrice`) dan `PosPage.tsx`; substansinya benar: tidak ada diskon per baris / klem harga minimum di UI POS |
| 11c | SO POS yatim tak muncul di antrean kirim | TERBUKTI | `sales/infrastructure/aggregates.go` `OpenHeaders` (`channel <> 'pos'`, ~baris 83-89) |
| 11d | uang tunai diterima tak disimpan | TERBUKTI | `payments` tak punya kolom `amount_tendered` (`000013`), handler Create tak menerimanya; legacy `delivery.service.ts:403-411` `resolveTenderedAmount` |
| 12a | FIFO urut id vs legacy dueDate lalu orderDate | TERBUKTI | `payment/service.go:399, 428` (iterasi terbalik dari `ListByParty ... ORDER BY id DESC`, `sales/infrastructure/repository.go:283`); legacy `payment.service.ts:518` `order:{dueDate,orderDate,id}` |
| 12b | Metode `check` | TERBUKTI | `payment/service.go:278-280` hanya cash/transfer; legacy enum `cash|bank_transfer|check` (`packages/shared-types/src/enums.ts:2`); finance `builders.go` memetakan cash/transfer/offset saja |

### 0.2 Temuan tambahan di luar audit (hasil bacaan saya)

- A1. `delivery.Create` tanpa doclock (2c) dan memanggil `s.branches.NextDocumentNumber` sebelum insert (`232-235`) -> gagal insert = nomor hilang.
- A2. GR gagal tetap postable (2f). Perbaikan di F2a-3 (status `pending|posted|void`), dan finance harus memfilter `posted`.
- A3. `doclock.Lock("party:0")` (`payment/service.go:228`) menyerialkan SEMUA pembayaran walk-in satu proses. Untuk POS (banyak kasir) ini kemacetan; kunci harus per order untuk walk-in (F2a-6).
- A4. Urutan kunci tak konsisten di `stock`: `Hold` = saldo lalu insert reservasi; `Release`/`Consume` = reservasi (FOR UPDATE) lalu saldo. Bila `Post` perlu konsumsi reservasi, aturan baku: **kunci baris reservasi dulu, baru saldo** (F2a-0).
- A5. Kursor biaya finance `MaxCostMovementID` + `ListCostMovements(afterID)` (`finance/application/costing.go:42-48`, `stock/infrastructure/repository.go:440-466`): baris dengan id lebih kecil yang commit belakangan tak pernah terambil. Pre-existing, tapi `Post` batch memperpanjang jendela commit. Mitigasi di F2a-0 (stock) + permintaan ke finance (bagian 5).
- A6. `ExpireReservationsTx` hanya menyapu posisi yang disentuh; reservasi kedaluwarsa di posisi yang tak disentuh tetap menggelembungkan `reserved` di daftar saldo -> perlu penyapu berkala (F2a-12).
- A7. `salesreturn.ListByBranch` memakai `repo.List(..., 10000, 0)` (`service.go:89`) untuk perhitungan saldo; tidak dalam paket ini, dicatat sebagai risiko performa (bagian 6).
- A8. Penyimpangan arsitektur kecil yang ikut dirapikan: `sales`/`purchasing` `SetStatus` kontrak tanpa CAS dan tanpa asal-status; `ConfirmOrder`/`CancelOrder` baca-lalu-tulis tanpa lock.

---

## 1. Prinsip desain lintas tugas (dipakai ulang)

### P1. Idempotensi gerakan stok per dokumen
Kunci unik `stock_movements(ref_type, ref_id, product_id, variant_id, location_id, direction)` (keputusan 3). `Post` menjumlahkan baris dokumen per posisi (produk, varian, lokasi) sebelum menulis, jadi dua baris produk+lokasi sama dalam satu SJ menjadi satu gerakan. Panggilan kedua dengan kunci sama = no-op sukses (`Replayed` dilaporkan). `ref_id` NULL (adjustment lama, opening) tidak terkena batasan (MySQL mengizinkan banyak NULL) dan tetap diizinkan untuk jalur non-dokumen.

### P2. CAS status (compare-and-set)
Setiap transisi: `UPDATE <tabel> SET status=?, ... WHERE id=? AND status IN (<asal sah>)` lalu cek `RowsAffected`. Aturan hasil:
- 1 baris -> sukses;
- 0 baris -> baca ulang: bila status sudah = target -> sukses replay (bukan error); bila status lain -> `apperror.Conflict`.
Setiap repositori mendapat satu fungsi `Transition(ctx, id, from []string, to string, actorID, extra ...)`; tidak ada lagi `SetStatus` telanjang. `sales.SetStatus`/`purchasing.SetStatus` (kontrak) diubah jadi CAS dengan asal sah eksplisit.

### P3. Doclock: baca ulang DI DALAM lock, urutan lock baku
Urutan kunci (selalu kiri ke kanan, tak pernah terbalik): `sr:<returnId>` -> `so:<id>` / `po:<id>` -> `party:<id>` (atau `wk:<tipe>/<orderId>` bila party 0). Modul `stock` tidak memakai doclock (memakai row lock DB). Pola kode:
```go
doclock.Lock("so:"+id, func() error {
    fresh := repo.GetNote(ctx, noteID)   // BARU dibaca di sini
    if fresh.Status == confirmed { return replay() }
    ...validasi sisa/cek ulang...
    ...Post stok (idempoten)...
    ok := repo.Transition(ctx, noteID, []string{draft}, confirmed, ...)  // CAS terakhir
})
```
`doclock` hanya menyerialkan satu proses (satu container) dan menyembunyikan balapan DB di tes; backstop sebenarnya = P1 + P2. Untuk tes DB-level, tambah `doclock.SetBypass(bool)` (hanya dipakai dari `_test.go`, dikomentari demikian).

### P4. Confirm aman diulang (replay-safe)
Setiap Confirm/Cancel disusun sebagai deret langkah idempoten dengan penanda final TERAKHIR:
1. validasi (baca ulang, cek sisa, cek termin) tanpa efek samping;
2. efek samping idempoten (`stock.Post` -> `payment.RecordForOrder(clientRef)` -> `sales.SwitchToNet`);
3. CAS status dokumen (penanda final);
4. langkah pasca-final idempoten (selesaikan SO/PO, offset retur).
Jika dokumen sudah final saat dipanggil lagi, jalankan langkah 4 lalu kembalikan 200 (bukan 409). Semua kegagalan di tengah = ulang request yang sama sampai selesai.

### P5. Forward recovery, bukan kompensasi
Hapus semua `_ = s.stock.MoveIn/MoveOut(... "_rollback" ...)`. Kompensasi hanya lewat `stock.Reverse` pada pembatalan TERMINAL yang sengaja (cancel/discard), ber-ref_type `<ref>_cancel` (konvensi yang sudah ada: `sales_return_cancel`, `purchase_return_cancel`, `transfer_cancel`), juga idempoten oleh kunci unik. Tidak ada `_ =` yang menelan error di jalur uang/stok; error dibungkus dengan id dokumen agar operator tahu apa yang harus diulang.

### P6. Rumus sisa kuantitas (semua modul)
`komitmen = Σ(draf + final)` per (dokumen induk, produk, varian) dalam basis qty dasar, ditambah akumulasi baris dalam request yang sedang diproses. Cek di dua titik: (a) Create (dalam lock), (b) Confirm (dalam lock, termasuk dirinya sendiri: `Σ komitmen <= batas`). Batas = dipesan (SJ), diterima-posted (retur beli), terkirim-confirmed (retur jual), dipesan (GR). Indeks komposit baru agar agregat murah (lihat tiap tugas).

### P7. Migrasi additive, satu blok
Blok `000040`-`000043` (stock, dokumen, payment, retur). Semua `ALTER ... ADD COLUMN ... NULL/DEFAULT`, indeks `ALGORITHM=INPLACE, LOCK=NONE` bila didukung. `down.sql` ditulis. Backfill ringan (UPDATE terindeks); tidak ada backfill berat wajib (balance_before/after untuk riwayat lama opsional, lihat F2a-9).

---

## 2. Kontrak baru / berubah (ringkasan tanda tangan)

> Hanya `contracts/` yang diekspos. Penerima kontrak yang berubah tercantum; fake di tes (`dashboard/application/service_test.go`, dll.) ikut diperbarui.

### 2.1 `stock/contracts` (StockClient)
```go
const ( // ref_type baku; reversal = ref_type + "_cancel"
    RefDelivery="delivery"; RefGoodsReceipt="goods_receipt"; RefSalesReturn="sales_return"
    RefPurchaseReturn="purchase_return"; RefTransfer="transfer"; RefAdjustment="adjustment"
    RefDamaged="damaged"; RefWriteOff="write_off"; RefOpening="opening"
)
type PostLine struct {
    ProductID, VariantID, LocationID int64
    Qty            float64 // qty dasar > 0; baris posisi sama dijumlahkan oleh Post
    ReservationKey string  // opsional, hanya out: konsumsi reservasi di tx yang sama
}
type PostRequest struct {
    BranchID  int64
    Direction string   // "in" | "out"
    RefType   string
    RefID     int64    // wajib > 0
    ActorID   int64
    MovedAt   time.Time // zero = sekarang; waktu efektif (backdate dokumen)
    Note      string
    AllowDamagedLocation bool // true hanya utk retur kondisi rusak / write-off
    Lines     []PostLine
}
type PostResult struct{ Posted, Replayed int }
Post(ctx, PostRequest) (*PostResult, error)                       // atomik, idempoten, terurut
Reverse(ctx, branchID int64, refType string, refID int64, actorID int64) (*PostResult, error) // balik semua gerakan ref itu, sekali
HasMovements(ctx, branchID int64, refType string, refID int64) (bool, error)
Suggest(ctx, branchID, productID, variantID int64, qty float64) ([]Suggestion, float64, error) // sudah ada di Service, tambahkan ke kontrak
CheckLocation(ctx, branchID, locationID int64) error               // KINI menolak lokasi kind=damaged
DamagedLocation(ctx, branchID int64) (*Location, error)            // ensureDefaults -> RUSAK
ListAdjustments(ctx, branchID, afterID int64, limit int) ([]*AdjustmentDoc, error) // untuk finance
GetAdjustment(ctx, id int64) (*AdjustmentDoc, error)
```
`MoveIn/MoveOut/Consume` dicabut dari kontrak setelah semua pemanggil dipindah (4 modul + fake); `Hold` menjadi upsert (F2a-12).

### 2.2 `payment/contracts` (PaymentClient)
```go
RecordForOrder(ctx, RecordInput) (p *Payment, replayed bool, err error) // 1 pembayaran penuh ke 1 order, idempoten via ClientRef
PaidForOrders(ctx, orderType string, ids []int64) (map[int64]int64, error)  // batch (repo sudah ada)
OffsetReturn(ctx, OffsetInput) (*Payment, bool, error) // auto-offset retur ke order (dua kaki atomik)
```
dan `Payment` mendapat `ClientRef, AmountTendered, Reference, PairID`; `PaymentMethod` mengenal `check`.

### 2.3 `sales/contracts` & `purchasing/contracts`
`SalesOrder`/`Item` mendapat: `ClientRef, AssignedUserID`, item: `StockTracked, ItemType, VariantName, VariantCode, BaseUOM, Taxable, TaxBase, TaxAmount, DiscountReason, Note, PricingSource, BasePrice`. `PurchaseOrder`: `PaymentTermDays, AssignedUserID` + item yang sama. Kontrak klien baru: `SwitchToNet(ctx, branchID, id int64, dueDate string, actorID int64) error` (CAS cod|prepaid -> net; replay ok), `FindByClientRef(ctx, branchID, ref)`. `SetStatus` tetap, semantik CAS.

### 2.4 `delivery/contracts`, `goodsreceipt/contracts`, `salesreturn/contracts`, `purchasereturn/contracts`
- delivery: `ActiveNoteCount(ctx, soID int64) (int64, error)` (draf+terkonfirmasi, kind order), `CommittedQty(ctx, soID, productID, variantID)`; `DeliveryNote` + `ClientRef`, item `ReservationKey`.
- goodsreceipt: `ReceivedQty` = posted saja; `CommittedQty` = pending+posted; `ActiveReceiptCount(ctx, poID)`; `LastReceivedAt(ctx, poID)`; `GoodsReceipt` + `Number, Status, SupplierDeliveryNumber, ClientRef`; `ListRecent` hanya `posted`.
- salesreturn: `ActiveReturnCount(ctx, soID)`; `SalesReturn` + `Reason`, item `Condition`, header `ReplacementSubtotal, ReplacementTaxTotal, ReplacementTotal, DifferenceAmount, ReceivableReduction`; proyeksi `Return` + `Kind` (`refund|charge`), `Total` = jumlah yang boleh diselesaikan modul payment.
- purchasereturn: `ActiveReturnCount(ctx, poID)`; `PurchaseReturn` + `Reason, PayableReduction`.

### 2.5 `branch/contracts`
`DocGoodsReceipt` (format `GR-<kode>/<tahun>/<seq5>`); `NextDocumentNumber` tetap, perbaikan kunci bulan WIB (F2a-11); `ReleaseDocumentNumber(ctx, branchID, kind, number)` bila keputusan pertanyaan terbuka #3 = "pool nomor".

### 2.6 Permintaan ke modul lain (di luar paket saya, perlu agen pemilik)
- `product/contracts`: `ResolveScan(ctx, code) (*ScanMatch, error)` (variant_code ATAU barcode, fallback product_code), `SearchIDs(ctx, q string, limit int) ([]int64, error)`, `StockedByIDs(ctx, ids)`/`MinStockByIDs` (untuk daftar saldo).
- `finance/contracts`: lihat bagian 5.
- `auth`: permission baru `sales_return.refund`, `purchase_return.refund`, `stock.reserve` (seed di `PermissionCatalog` modul pemilik masing-masing).

---

## 3. Tugas

### F2a-0 — Fondasi stok: `Post`/`Reverse` idempoten, kunci unik, urutan kunci, retry deadlock  (ukuran L)

**Masalah (temuan 3, 8j, A4, A5).** `stock/application/service.go:410-466` (`MoveIn`/`MoveOut`) membuka tx sendiri per panggilan; `InsertMovementTx` (`repository.go:213-222`) tak punya kunci unik; pemanggil mengompensasi dengan `_ =` (lihat 0.1 #3). Retry setelah gagal di tengah mengeluarkan stok dua kali. Tak ada retry 1213/1205.

**Rancangan.**
- Migrasi `000040_f2a_stock_integrity`:
  - `stock_movements`: `ADD balance_before DECIMAL(24,12) NULL, ADD balance_after DECIMAL(24,12) NULL, ADD moved_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`; backfill `UPDATE stock_movements SET moved_at = created_at`.
  - `ADD UNIQUE KEY uq_stock_movements_ref (ref_type, ref_id, product_id, variant_id, location_id, direction)` (~272 byte, aman). **Pra-cek** sebelum ALTER: `SELECT ref_type,ref_id,product_id,variant_id,location_id,direction,COUNT(*) FROM stock_movements WHERE ref_id IS NOT NULL GROUP BY 1,2,3,4,5,6 HAVING COUNT(*)>1`; bila ada (mis. SJ dengan dua baris produk+lokasi sama di data dev) konsolidasikan dulu (jumlahkan qty ke baris terkecil, hapus sisanya; saldo tak berubah). Data produksi belum ada (ETL tahap K); ETL harus menjumlahkan per kunci.
  - ganti indeks `ix_stock_movements_branch_time (branch_id, created_at)` menjadi `(branch_id, moved_at)`; filter `dateFrom/dateTo` `ListMovements` memakai `moved_at`.
  - backfill `balance_before/after` riwayat lama opsional (window function `SUM(CASE ...) OVER (PARTITION BY branch,product,variant,location ORDER BY id)`), boleh dilewati: NULL = "tak diketahui".
- `stock/application/post.go` (baru): `Service.Post(ctx, PostRequest)`:
  1. validasi: `RefID>0`, `Direction`, qty>0, minimal 1 baris, produk aktif dan **tracked** (non-stok -> `Validation`, dipakai F2a-7); lokasi aktif, daun, cabang sama, `kind<>'damaged'` kecuali `AllowDamagedLocation`.
  2. jumlahkan baris per posisi (`productID,variantID,locationID`); `ReservationKey` berbeda pada posisi sama = validation error.
  3. urutkan posisi menaik (anti-deadlock; menggantikan urutan baris dokumen di `transactions.go:107-125`).
  4. `dberr.RetryTx` (baru, `shared/dberr/retry.go`: `IsRetryable(err)` untuk MySQL 1213/1205, maks 3x, backoff 20/60/150 ms + jitter) membungkus seluruh tx.
  5. per posisi dalam tx: (a) bila ada `ReservationKey`: kunci baris reservasi `FOR UPDATE` DULU; (b) `EnsureBalanceTx`, `ExpireReservationsTx`, `GetBalanceTx FOR UPDATE`; (c) cek duplikat `SELECT 1 FROM stock_movements WHERE <kunci unik>` — aman tanpa lock tambahan karena semua penulis posisi yang sama sudah terserialisasi oleh row-lock saldo; kunci unik = backstop (1062 diperlakukan replay); (d) ada -> `Replayed++`, lanjut; (e) out: tanpa reservasi cek `Available>=qty`; dengan reservasi cek aktif/tak kedaluwarsa/qty<=remaining dan `OnHand>=qty`, lalu `on_hand-=qty, reserved-=qty`, update reservasi (reservasi kedaluwarsa/hilang = jatuh ke jalur ketersediaan biasa, penjualan tak gagal hanya karena TTL); (f) `INSERT` movement dengan `balance_before/after`, `moved_at`; (g) `UPDATE` saldo.
  6. stok kurang menyebut produk+lokasi; seluruh tx rollback -> tak ada gerakan parsial.
- `Reverse(ref)`: satu tx; ambil semua gerakan `(ref_type, ref_id)` terurut posisi; tiap gerakan yang belum punya lawan `ref_type+"_cancel"`: kunci saldo, bila membalik `in` pastikan `Available>=qty` (jika tidak -> Conflict "stok sudah terpakai"), tulis gerakan lawan. Idempoten oleh kunci unik. Produk arsip tetap boleh dibalik; `Post` baru tetap menolaknya.
- `MoveIn/MoveOut` jadi pembungkus tipis sementara, lalu DICABUT dari kontrak di akhir F2a-1/2/3; fake di `dashboard/application/service_test.go` diperbarui.
- A5: `ListCostMovements` hanya mengembalikan baris `created_at <= UTC_TIMESTAMP(6) - INTERVAL 3 SECOND`; permintaan ke finance: kursor toleran celah (bagian 5).
- Penyapu reservasi kedaluwarsa di F2a-12.

**Reuse terverifikasi.** `postLocked` (`service.go:146-159`), `EnsureBalanceTx`, `GetBalanceTx` (FOR UPDATE, `repository.go:281`), `ExpireReservationsTx` (tanpa gap-lock, `305-360`), `resolveProduct/resolveLocation` (`service.go:73-110`), `nullRefID`. Hanya `InsertMovementTx` diperluas.

**Tes integrasi (`internal/integration/stock_post_test.go`, tanpa doclock).**
1. `Post` dua goroutine bersamaan kunci sama -> satu gerakan, satu pengurangan saldo, satu `Replayed`.
2. Replay berurutan -> no-op sukses.
3. Batch tiga posisi, posisi ke-3 stok kurang -> tak ada perubahan (rollback utuh).
4. Dua baris produk+lokasi sama -> satu gerakan, qty dijumlah.
5. Deadlock: dua dokumen posisi {A,B} urutan berlawanan, 50 iterasi paralel -> semua sukses; unit test `dberr.IsRetryable`/`RetryTx` dengan error 1213 tiruan.
6. `Reverse` dua kali -> satu set gerakan lawan; reverse `in` saat stok terpakai -> Conflict tanpa perubahan.
7. Konsumsi reservasi: Hold 10, Post out 10 dengan key -> `reserved` 0, `on_hand` -10; reservasi kedaluwarsa -> jalur ketersediaan.
8. Migrasi: pra-cek mendeteksi data uji yang sengaja digandakan.

**Volume/performa.** Per posisi: 1 SELECT duplikat (indeks unik), 1 INSERT, 1 UPDATE; indeks unik ~1 entri per gerakan. ALTER unik online inplace pada jutaan baris; pra-cek GROUP BY satu kali penuh-scan.
**Dependensi.** Independen (kecuali F2a-11 yang juga independen). Memblokir F2a-1,2,3,4,5,7,9,12.

---

### F2a-1 — Retur jual & beli: sisa, akumulasi, cek ulang, CAS  (ukuran M)

**Masalah (temuan 1).** Terbukti (0.1 #1a-1e). Dampak: dua draf penuh keduanya dikonfirmasi -> stok masuk 2x (`salesreturn confirmLocked 413-422`) dan nilai retur dikreditkan 2x.

**Rancangan.**
- Repositori: `ReturnedQty` diganti `CommittedQty` (draf+confirmed; Create/Confirm) dan `ConfirmedQty` (tampilan `ReturnContext`, ditambah kolom `drafted`). Indeks `ix_sales_returns_so_status (sales_order_id, status)` dan `purchase_returns (purchase_order_id, status)` (migrasi `000043`).
- `computeReturn` (`salesreturn/service.go:252-323`): akumulator `used[key]`; per baris `sisa = delivered - committed - used[key]`, tolak bila `qtyBase > sisa+1e-9`, lalu `used[key]+=qtyBase`. Preview memakai fungsi yang sama. Retur beli (`purchasereturn 217-287`): `received` = `goodsreceipt.ReceivedQty` (posted) dengan rumus sama.
- `Create` tetap di `doclock("so:"/"po:")`; tambah `reason` wajib (<=255; kolom `reason` di `sales_returns`/`purchase_returns`); tanggal retur beli: tidak di masa depan (toleransi 1 menit) dan, bila PO selesai, tidak sebelum `goodsreceipt.LastReceivedAt` (legacy `purchase-return.service.ts:437-444`).
- `Confirm` (jual & beli), P4, di dalam `doclock`: baca ulang retur; `confirmed` -> pasca-final lalu 200; selain `draft` -> Conflict; cek ulang `Σ komitmen (draf+confirmed termasuk dirinya) <= delivered/received` (satu `GROUP BY`); `stock.Post` (ref `sales_return`/`purchase_return`, `MovedAt=return_date`); CAS `draft->confirmed`; pasca-final: auto-offset (F2a-8).
- `Cancel`: baca ulang DI DALAM lock (sekarang status dibaca di `433` dan tak dibaca ulang di `459-479`: dua Cancel paralel bisa dua kali `MoveOut`). Draf: CAS `draft->cancelled`. Confirmed: guard refund (`HasActiveAllocations`, sudah ada) + guard memo settlement (tolak bila ada baris `*_settlements`), `stock.Reverse`, CAS `confirmed->cancelled`; stok kurang -> Conflict tanpa efek. Idem purchasereturn (`392-443`).
- Retur tukar: `SetReplacementStatus` jadi CAS (`WHERE replacement_delivery_status=?`).

**Reuse.** `lineEconomics`/`prorate`, `doclock`, `DeliveredQty` (sudah hanya `kind='order'`), `ReturnContext`, `GoodsReceiptClient.ReceivedQty`.

**Tes integrasi (`returns_concurrency_test.go`).**
1. Satu request dua baris produk sama masing-masing `qty=delivered` -> 409.
2. Dua draf penuh berurutan: draf kedua ditolak (draf menahan kuota); setelah draf pertama dibatalkan, draf kedua boleh.
3. Dua goroutine `Confirm` draf sama (`SetBypass(true)`) -> stok masuk sekali, tak ada nilai ganda.
4. Confirm lalu Confirm lagi -> 200 replay, stok sekali.
5. Dua Cancel paralel retur confirmed -> stok keluar sekali.
6. Injeksi gagal (F2a-4): `Post` sukses, CAS gagal -> ulang Confirm menyelesaikan.
7. Sisi beli sama; tanggal retur masa depan / sebelum penerimaan ditolak; alasan kosong ditolak.

**Volume.** Agregat per SO kecil; indeks komposit baru.
**Dependensi.** F2a-0, F2a-3 (retur beli memakai posted), F2a-4. Kondisi/lokasi/tukar/offset di F2a-8 (file sama, PR terpisah). **Dari finance:** tak ada tambahan.

---

### F2a-2 — Surat jalan: kuota, lock, Confirm aman-ulang, dokumen wajib  (ukuran L)

**Masalah (temuan 2, 2c).** 0.1 #2a-2c, #3.

**Rancangan.**
- Migrasi `000041` (delivery): `delivery_notes` `ADD client_ref VARCHAR(64) NULL`, `UNIQUE uq_delivery_notes_client_ref (branch_id, client_ref)`, indeks `ix_delivery_notes_so_status (sales_order_id, status, document_kind)`; `delivery_note_items` `ADD reservation_key VARCHAR(100) NULL` (`location_id=0` untuk barang non-stok; tanpa FK jadi tipe tak diubah).
- `Create` (`service.go:134-250`) dibungkus `doclock("so:ID")`:
  1. baca ulang SO dalam lock (status `confirmed`);
  2. `clientRef` sudah ada -> kembalikan nota itu (replay);
  3. header: sopir & petugas gudang wajib (KI-94), tanggal tak boleh sebelum periode finance terkunci (KI-80);
  4. akumulator `used[key]` + `komitmen = SJ draf+confirmed kind order` satu query `GROUP BY` (menggantikan `DeliveredQty` per baris di `198`); tolak bila `komitmen+used+qtyBase > ordered` (P6);
  5. termin prabayar belum lunas -> tolak (legacy `delivery.service.ts:708-715`), lihat F2a-10;
  6. `locationId` kosong pada produk tracked -> alokasi otomatis lintas lokasi lewat `stock.Suggest` (legacy `delivery.service.ts:246-282`), satu baris dipecah per lokasi; shortfall>0 -> ditolak (pesan produk+tersedia); non-stok -> `location_id=0`;
  7. `NextDocumentNumber` dipanggil TERAKHIR sebelum insert (F2a-11).
- `Confirm` (`259-377`), P4, di dalam `doclock("so:ID")`:
  1. baca ulang nota; `confirmed` -> pasca-final lalu 200;
  2. bukti (BR-09): `recipientSignatureStatus` WAJIB (`signed|missing`); `signed` -> nama penerima wajib; `missing` -> alasan wajib (sekarang `validateRecipient 379-399` meloloskan kosong total); foto/scan SJ fisik wajib terunggah (legacy `delivery.service.ts:821-835`; `s.media.List(OwnerDeliveryProof, id)` sudah tersedia). Channel POS dikecualikan dari foto (pertanyaan #1);
  3. baca ulang SO dan cek ulang kuota (`Σ draf+confirmed termasuk nota ini <= ordered` per posisi);
  4. termin (F2a-10): `prepaid & outstanding>0` -> tolak; `cod & outstanding>0` -> wajib `settlement.mode`, divalidasi SEBELUM stok bergerak;
  5. `stock.Post(out, RefDelivery, note.id, MovedAt=delivery_date)`; baris ber-`reservation_key` mengonsumsi reservasi (F2a-12);
  6. efek uang idempoten: `payment.RecordForOrder(clientRef="sj:<id>")` atau `sales.SwitchToNet`;
  7. SATU `UPDATE delivery_notes SET status='confirmed', recipient_*, dispatched_*, confirmed_* WHERE id=? AND status='draft'` (menggabung `SetStatus`+`SetDispatch`, `repository.go:287-310`, yang kini dua write terpisah);
  8. pasca-final: semua posisi `confirmed >= ordered` dan tak ada draf lain -> `sales.SetStatus(completed)` (CAS `confirmed->completed`). Berbeda tipis dari legacy (`delivery.service.ts:897`) hanya bila ada draf lain: order tak ditutup selagi ada draf aktif.
- `Cancel` draf: di dalam lock; `stock.HasMovements(delivery,id)` true -> `Reverse` dulu; CAS `draft->cancelled`. SJ terkonfirmasi tetap tak bisa dibatalkan (koreksi lewat retur).
- `ConfirmReplacement` (`554-615`): CAS + Post, hapus saga `587-598`; tetap tanpa lock sendiri (dipanggil di bawah `sr:`).
- `workqueue.go`: tambah `Drafted` dan `Unplanned = ordered-confirmed-draft` per baris (`DraftQtyBulk` satu GROUP BY); `Remaining` tetap = ordered-confirmed. SO POS yatim (F2a-12) ikut daftar.
- Handler: `Create` menerima `clientRef`, `locationId` opsional, `reservationKey` per baris; `Confirm` menerima `settlement{mode,method,amountTendered,dueDate,paymentDate,reference}`; respons memuat `replayed`.

**Reuse.** `doclock`, `lineKey`, `DeliveredQty/Bulk` (diperluas), `checkStockLine`, `validateRecipient` (diperketat), `enrich`, `OrderIDsByNotes`, kontrak `media`.

**Tes integrasi (`delivery_concurrency_test.go`).**
1. Dua baris produk sama total > sisa -> 409.
2. Dua `Create` paralel masing-masing sisa penuh -> satu lolos (dengan/tanpa `SetBypass`).
3. Draf 6+4 dari 10 lolos, draf ke-3 ditolak; confirm keduanya -> SO `completed`, stok -10.
4. Dua goroutine `Confirm` nota sama -> stok keluar sekali; satu replay.
5. Confirm gagal stok kurang pada baris ke-3 -> tak ada gerakan, nota tetap draf; ulang setelah isi stok sukses.
6. Injeksi gagal setelah `Post` sebelum CAS (F2a-4): ulang menyelesaikan, stok keluar sekali (menutup "stok keluar dua kali").
7. Sopir/petugas kosong, tanda tangan kosong, `missing` tanpa alasan, tanpa foto -> ditolak.
8. Cancel draf yang stoknya sudah keluar -> `Reverse` mengembalikan saldo.
9. Alokasi otomatis: L1=3, L2=4, SJ qty 6 tanpa lokasi -> L1=3, L2=3; total kurang -> ditolak.

**Volume.** Query kuota 1x per Create/Confirm; indeks komposit; `Post` 3 statement per posisi.
**Dependensi.** F2a-0, F2a-4; settlement butuh F2a-6a (`RecordForOrder`), F2a-10; reservasi/alokasi butuh F2a-12. Bagian non-uang bisa rilis dulu. **Dari finance:** `PeriodLockedThrough` (KI-80); builder pengiriman tak berubah (ref `delivery` + `nota.id`).

---

### F2a-3 — Penerimaan barang: status, duplikat, nomor, termin  (ukuran M)

**Masalah (temuan 2d-2f, KI-80/89).** 0.1 #2d-2f.

**Rancangan.**
- Migrasi `000041` (GR): `goods_receipts` `ADD number VARCHAR(60) NULL, supplier_delivery_number VARCHAR(60) NULL, status VARCHAR(12) NOT NULL DEFAULT 'posted', client_ref VARCHAR(64) NULL`; `UNIQUE (branch_id, client_ref)`; backfill `number = CONCAT('GR/', LPAD(id,6,'0'))` lalu `UNIQUE uq_goods_receipts_number`; indeks `(purchase_order_id, status)`. Nomor baru `branch.DocGoodsReceipt`.
- Status `pending` -> `posted` | `void`. `ReceivedQty` (retur beli & finance) = posted; `CommittedQty` = pending+posted (batas penerimaan, P6).
- `createLocked` disusun ulang: (1) `clientRef` ada -> lanjutkan receipt itu (replay langkah 5-7); (2) validasi dengan akumulator per posisi, tolak baris duplikat produk+varian+UOM (legacy `goods-receipt.service.ts:124`) dan total per posisi `<= sisa`; (3) termin PO (F2a-10): prabayar belum lunas -> tolak, COD pada penerimaan terakhir -> wajib `settlement`; (4) `INSERT goods_receipts(status='pending') + items` satu tx, nomor dialokasikan sesaat sebelum insert; (5) `stock.Post(in, RefGoodsReceipt, id, MovedAt=received_at)` hanya baris tracked; (6) efek uang idempoten bila COD terakhir; (7) CAS `pending->posted`, pasca-final `purchasing.SetStatus(completed)` bila semua posisi terpenuhi.
- Endpoint baru `POST /api/v1/goods-receipts/{id}/complete` (ulang 5-7, idempoten) dan `POST .../discard` (`Reverse` + CAS `pending->void`, hanya `pending`). UI menandai receipt `pending`.
- Tanggal terima: tidak di masa depan, tidak sebelum periode finance terkunci (KI-80).
- `ListRecent` (finance) hanya `posted` (menutup A2).

**Reuse.** `doclock("po:")`, `CheckLocation`, pola validasi produk/varian (`service.go:190-210`).

**Tes integrasi.**
1. Dua baris produk sama masing-masing sisa penuh -> 409.
2. `Post` gagal (lokasi dinonaktifkan di tengah) -> receipt `pending`, `ReceivedQty`=0, `CommittedQty`=qty, finance `ListRecent` tak memuatnya; `complete` setelah dibenahi -> `posted`, stok sekali.
3. Dua `Create` paralel clientRef sama -> satu receipt.
4. Dua `Create` paralel clientRef beda, sisa tunggal -> satu lolos.
5. `discard` pending yang stoknya sudah masuk -> saldo kembali; discard posted -> 409.
6. Retur beli atas receipt pending ditolak (cap = posted).
7. Nomor GR unik berurutan; `supplierDeliveryNumber` tersimpan.

**Volume.** Indeks `(purchase_order_id,status)`; `CommittedQty` satu GROUP BY per PO.
**Dependensi.** F2a-0, F2a-11, F2a-4; settlement butuh F2a-6a & F2a-10. **Dari finance:** hanya `posted` dijurnal; `PeriodLockedThrough`.

---

### F2a-4 — Matriks pemulihan saga & harness uji injeksi-gagal  (ukuran M)

**Masalah (temuan 3).** Kompensasi mengabaikan error dan kegagalan setelah stok keluar tak dipulihkan. Dengan F2a-0 (idempoten) + CAS, pemulihannya = ulang request. Tugas ini mendefinisikan matriks per pemanggil dan membuktikannya dengan tes.

**Matriks (titik gagal -> keadaan -> pemulihan).**

| Alur | Gagal di | Keadaan tersisa | Pemulihan aman |
|---|---|---|---|
| SJ Confirm | `Post` (stok kurang) | tak ada efek | perbaiki stok, ulang |
| SJ Confirm | setelah `Post`, sebelum CAS | draf + stok sudah keluar | ulang Confirm (Post no-op) ATAU Cancel (Reverse) |
| SJ Confirm | `RecordForOrder` gagal | draf + stok keluar, belum bayar | ulang (clientRef `sj:<id>` mencegah bayar ganda) |
| SJ Confirm | setelah CAS, sebelum `completed` | SJ confirmed, SO `confirmed` | ulang Confirm -> replay menjalankan pasca-final |
| GR Create | `Post` gagal | receipt `pending` | `complete` atau `discard` |
| GR Create | setelah Post sebelum CAS | `pending` + stok masuk | `complete` (replay) / `discard` (Reverse) |
| Retur Confirm | setelah Post sebelum CAS | draf + stok | ulang / Cancel(draf) -> Reverse |
| Retur Confirm | offset gagal | retur confirmed tanpa offset | ulang Confirm (replay menjalankan offset, clientRef `offset:sr:<id>`) |
| Retur Cancel | setelah Reverse sebelum CAS | confirmed tapi stok sudah balik | ulang Cancel (Reverse no-op) |
| Transfer Dispatch/Receive | tx tunggal | tak ada parsial | ulang |
| settle-credit | dua kaki dalam SATU tx (F2a-6) | tak ada kaki yatim | ulang |

**Rancangan harness.**
- `internal/integration/faults_test.go`: dekorator `faultyStock` yang membungkus `stockcontracts.StockClient` dengan hook `FailAfterPost(refType)`/`FailBeforePost`; dekorator `faultyRepo` tidak diperlukan karena CAS gagal disimulasikan dengan mengubah status baris langsung via SQL tes.
- Modul `delivery/goodsreceipt/salesreturn/purchasereturn` sudah menerima `StockClient` lewat konstruktor, jadi tes membangun modul dengan dekorator itu (pola yang sama dengan `testMods` di `integration_test.go`).
- `doclock.SetBypass(bool)` (komentar: hanya untuk tes) agar tes konkurensi menguji CAS/kunci unik DB, bukan mutex proses.
- Helper `runParallel(t, n, fn)` dengan penghalang start (`sync.WaitGroup` + channel) — belum ada tes goroutine di `internal/integration` (grep `go func` kosong).
- **Laporan konsistensi** (S, read-only, `GET /api/v1/ops/integrity` perm `stock.manage`; atau fungsi tes): tiga query: (a) SJ `draft` yang punya gerakan `delivery`; (b) GR `pending`; (c) retur `confirmed` tanpa gerakan atau `cancelled` dengan gerakan tak terbalik. Operator tahu persis apa yang perlu "dilanjutkan". Tanpa job penyembuh otomatis (bukan outbox).

**Tes.** Satu tes per baris matriks (injeksi lalu ulang lalu cek gerakan tepat sekali, saldo, status, `PaidForOrders`).
**Dependensi.** F2a-0; dipakai tes F2a-1,2,3,5,8.

---

### F2a-5 — Transfer stok: CAS, atomik, tujuan per item, urutan kunci  (ukuran M)

**Masalah (temuan 4, 8h).** 0.1 #4a-4c, #8h.

**Rancangan.**
- Migrasi `000040`: `stock_transfer_items ADD from_location_id BIGINT UNSIGNED NULL, to_location_id BIGINT UNSIGNED NULL` (backfill dari header, lalu NOT NULL); header `from_location_id/to_location_id` dipertahankan nullable sebagai ringkasan (kosong bila beragam). `CreateTransferRequest.items[].fromLocationId/toLocationId` (fallback ke pasangan dokumen untuk klien lama).
- `DispatchTransfer/ReceiveTransfer/CancelTransfer` (`transactions.go:91-222`): buka tx; `SELECT status FROM stock_transfers WHERE id=? FOR UPDATE` (kunci dokumen DB — modul stock tak memakai doclock); cek status asal dalam tx; gerakan lewat helper internal `postTx` (versi tx dari `Post`: terurut, idempoten; ref `transfer`, RefID=transfer.id, `Direction` out di lokasi asal per item, in di lokasi tujuan per item); `SetTransferStatusTx` menjadi CAS `WHERE status=?` + cek `RowsAffected`. Cancel draf juga lewat CAS (kini `SetTransferStatus` non-tx, `transactions.go:192`).
- Cancel dispatched: `ref_type='transfer_cancel'` via `Reverse`-semantik pada lokasi asal; hanya cabang asal.
- Lokasi tujuan wajib bukan `damaged` untuk Receive (kecuali transfer rusak tidak ada).
- `MoveLocation` (`movelocation.go:19-29`) dijadikan satu tx: `moveLocationTx` membuat dokumen status `received` langsung (nomor `TRF`), dua gerakan per item di satu tx (out asal, in tujuan) — tanpa melewati draf/dispatched, jadi tak ada keadaan in-transit/draf sisa bila gagal. Nomor dokumen dialokasikan sebelum tx (gap nomor dapat diterima, F2a-11).
- Urutan kunci: `postTx` mengurutkan semua posisi (kedua cabang) menaik; `Adjust` & damaged-move memakai urutan sama; retry via `dberr.RetryTx`.
- `Receive` bisa gagal kapasitas? tidak — hanya lokasi tujuan harus daun-aktif-non-damaged saat receive (dicek ulang).

**Reuse.** `postLocked`/`EnsureBalanceTx`, `resolveLocation`, `GetTransfer`, nomor `DocStockTransfer`.

**Tes (`transfer_concurrency_test.go`, perluas `transfer_scope_test.go`, `movelocation_test.go`).**
1. Dua `Dispatch` paralel dokumen sama -> stok keluar sekali.
2. `Dispatch` vs `Cancel(draf)` paralel -> tepat satu menang; stok konsisten.
3. `Receive` ganda -> stok masuk sekali; `Cancel` setelah `Receive` -> 409.
4. `MoveLocation` dengan stok kurang di item ke-2 -> tak ada dokumen/gerakan/in-transit; dengan lokasi tujuan diarsipkan setelah validasi (injeksi) -> rollback utuh.
5. Transfer dengan dua item lokasi tujuan berbeda -> saldo benar per item.
6. Deadlock: dua `MoveLocation` bolak-balik A<->B paralel 50x -> semua sukses.

**Volume.** Kecil; satu SELECT FOR UPDATE tambahan per aksi.
**Dependensi.** F2a-0, F2a-9a (kind damaged). **Dari finance:** transfer antar-cabang netral biaya (tak berubah); gerakan transfer cabang beda memakai `ref_type=transfer` seperti sekarang.

---

### F2a-6 — Guard pembatalan, pembayaran idempoten, pasangan offset, FIFO, metode `check`  (ukuran L)

Dibagi: **6a** kontrak payment yang dibutuhkan alur lain (`RecordForOrder`, `PaidForOrders`, `clientRef`, `amount_tendered`), **6b** guard pembatalan + pasangan offset, **6c** FIFO + `check`.

**Masalah (temuan 5, 12a, 12b, 11d, A3).** 0.1 #5a-5c, #12a-12b, #11d.

**Rancangan 6a.** Migrasi `000042_f2a_payment`: `payments ADD client_ref VARCHAR(64) NULL, amount_tendered BIGINT NULL, reference_number VARCHAR(100) NULL, pair_id BIGINT UNSIGNED NULL`; `UNIQUE (branch_id, client_ref)`; indeks `(pair_id)`.
- `Create` menerima `clientRef`, `amountTendered`, `reference`; `clientRef` sudah ada -> kembalikan pembayaran itu (replay, bukan duplikat). Tunai: `amountTendered >= amount` wajib bila diisi (legacy `delivery.service.ts:403-411`); kembalian = tendered-amount dihitung saat tampil.
- `RecordForOrder(ctx, RecordInput{ActorID,BranchID,OrderType,OrderID,Amount(0=lunas),Method,PaidAt,Reference,AmountTendered,ClientRef,Notes})`: membaca order, menurunkan party & arah, lalu memakai `createLocked` dengan satu alokasi eksplisit; lock = `party:<id>`, atau `wk:<tipe>/<orderId>` bila party 0 (menutup A3: kunci `party:0` shared). Dipanggil `delivery`/`goodsreceipt` (settlement) dan POS. Lock order: pemanggil memegang `so:`/`po:` lebih dulu, `party:` menyusul (aturan P3) — saya telah memeriksa `payment` tak pernah mengambil `so:/po:` (hanya baca via kontrak).
- Kontrak: tambah `RecordForOrder`, `PaidForOrders`, `OffsetReturn`.

**Rancangan 6b.**
- *Guard cancel SO/PO* (`sales/service.go:439-452`, `purchasing:413-426`): tambah `SetDependents(delivery, payments, salesReturns)` (setter, memecah siklus konstruksi, pola `SetPayments` di `cmd/server/main.go:88`). `CancelOrder` di `doclock("so:ID")`, baca ulang, tolak bila: ada SJ `draft|confirmed` kind order (`delivery.ActiveNoteCount`), ada alokasi pembayaran aktif (`HasActiveAllocations("sales",id)`, sudah ada), ada retur `draft|confirmed` (`ActiveReturnCount`); PO: GR `pending|posted`, alokasi bayar, retur beli. Pesan menyebut dokumen penghalang (legacy `order.service.ts:743-745`, `assertNoStockMovementOrPayment` `1236-1265`; cek `finance_posting_sources` legacy tak perlu karena jurnal revamp turunan SJ/pembayaran yang sudah diblokir).
- `SetStatus` (sales/purchasing) jadi CAS dengan asal sah eksplisit (`completed` dari `confirmed`; `cancelled` dari `draft|confirmed`); `ConfirmOrder` CAS `draft->confirmed`, replay bila sudah `confirmed` (penting untuk POS); `ApproveCredit`/`SwitchToNet` CAS `payment_terms IN ('cod','prepaid')` -> `net`.
- *`payment.Cancel`* (`516-529`): di `doclock(party)`, baca ulang status (CAS `active->cancelled` pada pembayaran + pasangannya), lalu guard (legacy `payment.service.ts:408-411`): untuk tiap alokasi ke order `sales|purchase`: bila order `completed` dan `payment_terms <> 'net'` dan `grand_total - (paid - alokasi ini) > 0` -> tolak ("order <termin> yang sudah selesai akan jadi belum lunas"). Alokasi ke dokumen retur: bila pembayaran refund dibatalkan, retur kembali terbuka (boleh). Pembayaran offset (method `offset`): batal hanya lewat pasangannya.
- *Pasangan settle-credit* (`602-617`): kedua kaki ditulis dalam SATU tx via `repo.CreatePaymentPair(ctx, leg1, leg2)` (modul payment sendiri, tx sah) dengan `pair_id` saling menunjuk; tak ada lagi kaki yatim dan blok pemulihan manual (`610-616`, `withSuffix`) dihapus. `Cancel` salah satu kaki membatalkan keduanya atomik (CAS dua baris dalam satu tx) setelah guard order di atas.
- `OffsetReturn` (dipakai F2a-8): `settleCreditLocked` yang dipanggil programatik dengan `amount=min(total retur sisa, outstanding order)`; clientRef `offset:sr:<id>` (idempoten).

**Rancangan 6c.**
- FIFO (`payment/service.go:385-466`): urut `due_date` (net: efektif; non-net: `order_date`) menaik, NULL terakhir, lalu `order_date`, lalu `id` — sorting di Go atas `OrderSummary.DueDate/OrderDate` (sudah ada di kontrak `sales.OrderSummary`, tak perlu kontrak baru; `ListByParty` tetap `ORDER BY id DESC`; hapus pembalikan loop `399/428`). Untuk purchase sama. Satu fungsi `sortFIFO([]cand)` + unit test pure.
- Metode `check`: `method IN ('cash','transfer','check')` (+`offset` internal); `reference_number` wajib untuk `check` dan `transfer`? (legacy: opsional) -> opsional. `finance builders.go`: pemetaan akun `check` -> bank (permintaan finance bagian 5). ETL: `bank_transfer -> transfer`.
- Izin refund: handler `Create` memeriksa `authcontracts.SessionFromContext(ctx).HasPermission("sales_return.refund"|"purchase_return.refund")` bila ada alokasi bertipe retur (service menerima flag `CanRefund`), jangan hanya `payment.create` (legacy `sales-return.service.ts:904`, `purchase-return.service.ts:569`).

**Reuse.** `doclock`, `resolveOrder`, `PaidForOrders` (repo), `insertPayment`, `HasActiveAllocations`, setter pattern `SetPayments`.

**Tes integrasi (`payment_guards_test.go`, `payment_flows_test.go` diperluas).**
1. Cancel SO dengan SJ draf/confirmed, pembayaran aktif, retur draf -> masing-masing ditolak; setelah dibereskan -> sukses. PO sama dengan GR/pembayaran/retur.
2. Cancel SO paralel dengan `delivery.Create` (`SetBypass`) -> hasil konsisten (tak ada SJ pada SO cancelled).
3. `payment.Cancel` order COD completed lunas -> 409; order net completed -> boleh; refund retur dibatalkan -> retur kembali outstanding.
4. settle-credit: satu kaki gagal insert (injeksi) -> tak ada kaki; cancel satu kaki membatalkan pasangan; dua Cancel paralel -> sekali.
5. `Create` clientRef sama dua kali -> satu pembayaran; 20 goroutine pembayaran walk-in ke order berbeda berjalan tanpa saling blok (ukur waktu < N).
6. FIFO: tiga order tempo jatuh tempo lebih awal belakangan dibuat -> dialokasi urut jatuh tempo; unit test `sortFIFO` tie-break.
7. `check` diterima; metode lain ditolak; refund tanpa izin ditolak.
8. `amountTendered < amount` ditolak; tersimpan bila cukup.

**Volume.** `ActiveNoteCount`/`ActiveReturnCount` COUNT berindeks; FIFO sort O(n log n) pada order terbuka satu party (+1 `PaidForOrders` batch); guard cancel pembayaran: satu `PaidForOrders` per tipe.
**Dependensi.** 6a: F2a-0 tak perlu; 6b memerlukan kontrak baru di delivery/goodsreceipt/salesreturn/purchasereturn (diberikan di F2a-1/2/3). Dipakai F2a-2,3,8,10,12. **Dari finance:** pemetaan `check`; semantik `offset` tak berubah; auto-offset memakai jurnal pembayaran offset yang sudah ada (tak perlu builder baru).

---

### F2a-7 — Barang non-stok / jasa  (ukuran S)

**Masalah (temuan 6).** 0.1 #6. Dampak: GR jasa membuat stok fiktif (dan finance menjurnal persediaan); SJ jasa memaksa lokasi dan menolak bila "stok" 0.

**Rancangan.**
- Migrasi `000041`: `sales_order_items` dan `purchase_order_items` `ADD stock_tracked TINYINT(1) NOT NULL DEFAULT 1, item_type VARCHAR(10) NOT NULL DEFAULT 'barang'` (snapshot dari `product.Tracked`/`Type` saat order dibuat; baris lama default tracked).
- `sales.priceLines` & `purchasing.priceLines` mengisi snapshot (`p.Tracked`, `p.Type`); `contracts.Item` membawa `StockTracked`, `ItemType`.
- `delivery.Create/Confirm`: baris `!StockTracked` -> `location_id=0`, dilewati `Post`, tetap dihitung kuota dan completion (legacy `delivery.service.ts:521` `if (!item.stockTrackedSnapshot) continue`). `goodsreceipt`: sama (`goods-receipt.service.ts:276`). `salesreturn`/`purchasereturn`: baris non-stok -> tak ada `Post`, nilai tetap diretur (legacy `sales-return.service.ts:...stockTrackedSnapshot`), lokasi tak wajib.
- `stock.Post` menolak produk non-tracked (F2a-0) sebagai backstop; `Hold` juga.
- `stock.CriticalStock`/`ListStocked` sudah memakai `stock_tracked=1`.
- Validasi tambahan produk: `Type='jasa'` -> `Tracked=false` dipaksa di `product.Service` (permintaan ke product, 1 baris).

**Tes.** SO berisi barang+jasa: SJ tanpa lokasi untuk jasa sukses, stok hanya barang bergerak, SO selesai; PO berisi jasa: GR tak membuat saldo jasa; retur jasa tak menyentuh stok; `Post` produk jasa ditolak.
**Volume.** Tidak ada. **Dependensi.** F2a-0, F2a-2, F2a-3. **Dari finance:** builder GR/SJ perlu memakai `Item.StockTracked`/`ItemType`: baris jasa -> beban (bukan 1300 Persediaan) dan tanpa COGS; kontrak saya menyediakan field-nya.

---

### F2a-8 — Retur: kondisi, lokasi RUSAK, tukar (selisih), refund, offset otomatis  (ukuran L)

**Masalah (temuan 7).** 0.1 #7a-7f.

**Rancangan.**
- Migrasi `000043_f2a_returns`:
  - `sales_returns ADD reason VARCHAR(255) NOT NULL DEFAULT '', replacement_subtotal BIGINT NOT NULL DEFAULT 0, replacement_tax_total BIGINT NOT NULL DEFAULT 0, replacement_total BIGINT NOT NULL DEFAULT 0, difference_amount BIGINT NOT NULL DEFAULT 0, receivable_reduction BIGINT NOT NULL DEFAULT 0`;
  - `sales_return_items ADD return_condition VARCHAR(12) NOT NULL DEFAULT 'normal'` (`normal|damaged|not_returned`);
  - `sales_return_replacement_items ADD tax_base BIGINT NOT NULL DEFAULT 0, tax_amount BIGINT NOT NULL DEFAULT 0` (kolom `unit_price`, `line_total` sudah ada; `line_total` KINI termasuk pajak);
  - `purchase_returns ADD reason VARCHAR(255) NOT NULL DEFAULT '', payable_reduction BIGINT NOT NULL DEFAULT 0`;
  - `*_settlements ADD auto TINYINT(1) NOT NULL DEFAULT 0`.
- *Kondisi barang (jual).* `LineInput.Condition`: `normal` -> lokasi dari request (`CheckLocation`, non-damaged, wajib bila produk tracked); `damaged` -> lokasi = `stock.DamagedLocation(branch)` otomatis (client tak boleh memilih), `Post` dengan `AllowDamagedLocation`; `not_returned` -> tak ada gerakan stok, nilai tetap diretur (legacy `sales-return.service.ts:658-663,930-935`). `Post` membagi baris per lokasi tujuan; kunci unik tetap per posisi. Cap kuota `delivered` berlaku semua kondisi. `stock.CriticalStock`/ketersediaan mengecualikan lokasi `damaged` (F2a-9a).
- *Tukar (selisih).* `priceReplacements` (`salesreturn/service.go:847-887`): harga = katalog (atau `unitPrice` manual opsional) + cek `>= product.MinSellingPrice` (kontrak product sudah memuatnya); pajak mengikuti konteks pajak SO (`so.TaxType/TaxRate`) lewat `lineEconomics` (sudah ada di modul): `tax_base`, `tax_amount`, `line_total` (inc pajak). Header: `replacement_subtotal/tax_total/total`; `difference_amount = replacement_total - total` bertanda (legacy `672-680`). Kontrak proyeksi `Return{Kind,Total}`: `difference<0` -> `Kind=refund, Total=-difference`; `>0` -> `Kind=charge, Total=difference`; `0` -> `Total=0`; return_only -> `refund, Total=total`. `payment.resolveOrder` untuk `sales_return` memakai `Kind` untuk menentukan arah (charge: arah masuk dari pelanggan, bukan refund) sehingga tambah-bayar bisa dicatat/di-RecordForOrder pada saat SJ pengganti dikonfirmasi (settlement `pay_now`, mekanisme F2a-10/6a).
- *Offset otomatis (KI-98 -> parity).* Langkah pasca-final `Confirm` retur jual: bila `party>0` dan `Kind=refund`: `payments.OffsetReturn(amount = min(Total, outstanding order = grand - paid))`, idempoten (`offset:sr:<id>`); baris memo `reduce_receivable` (`auto=1`) dan `receivable_reduction` disimpan; sisa `Total - reduction` tetap terbuka (refund/kredit pelanggan manual lewat payment dengan izin `sales_return.refund`). Retur beli simetris (`payable_reduction`, `purchase_return.refund`). Walk-in (party 0) tak ada offset.
- *Refund/izin.* Lihat F2a-6c; permission baru ditambahkan ke `PermissionCatalog` salesreturn/purchasereturn dan dipakai handler payment.
- *Alasan wajib* (`reason`) + tanggal beli: F2a-1.
- *Cancel retur confirmed* (F2a-1): guard memo/pembayaran sudah mencakup offset karena offset = alokasi pembayaran ke dokumen retur; pembatalan lewat `payment.Cancel(pair)`.
- *Cost basis pengganti*: `CreateReplacementDelivery` (`salesreturn:507-586`) sudah memeriksa `PurchasePrice>0`; legacy memakai average cost ATAU harga beli (`assertReplacementCostBasis`). Ganti dengan `finance.AverageUnitCost(...)>0 || PurchasePrice>0` (bagian 5) agar setara.

**Reuse.** `computeReturn`, `lineEconomics`, `prorate`, `SettleCredit`, `HasActiveAllocations`, `priceReplacements` (diperluas), `doclock("sr:")`.

**Tes integrasi (`returns_exchange_test.go`, memperluas `exchange_test.go`, `returns_test.go`).**
1. Kondisi `damaged` -> stok masuk ke RUSAK, `ProductAvailability` tak berubah; `not_returned` -> stok tak berubah, nilai retur tercatat.
2. Tukar lebih mahal: difference>0, `Kind=charge`; tukar lebih murah: difference<0; pengganti memakai pajak SO include/exclude (kasus uji tabel), harga di bawah minimum ditolak.
3. Offset otomatis: order outstanding 100, retur 60 -> reduction 60, outstanding order 40; retur 150 -> reduction 100, sisa 50 terbuka; replay Confirm tak menggandakan offset.
4. Refund tanpa izin ditolak; dengan izin diterima; Cancel retur dengan offset -> ditolak sampai pasangan dibatalkan.
5. Retur beli simetris (payable reduction).
6. Alasan kosong ditolak.

**Volume.** `OffsetReturn` = 2 INSERT payment + alokasi dalam 1 tx.
**Dependensi.** F2a-0, F2a-1, F2a-6a/b, F2a-9a (kind damaged), F2a-10 (pajak/termin). **Dari finance (kontrak yang SAYA sediakan):** `SalesReturn.{Reason, Items[].Condition, ReplacementSubtotal/TaxTotal/Total, DifferenceAmount, ReceivableReduction}`, `ReplacementItem.{TaxBase,TaxAmount,LineTotal}`, `Return.{Kind,Total}`, `PurchaseReturn.PayableReduction`. Builder retur harus: tidak membalik COGS untuk baris `not_returned`; menjurnal baris `damaged` sebagai persediaan-rusak/beban sesuai kebijakan; menjurnal SJ pengganti (`document_kind='replacement'`) dari `ReplacementItems` (kini builder pengiriman gagal "Baris surat jalan tak cocok SO" untuk produk di luar SO, `builders.go:~246`); dan membukukan selisih. Offset otomatis memakai pembayaran `method=offset` yang sudah dijurnal.

---

### F2a-9 — Modul stok: lokasi, penyesuaian, rusak, scan, opening, daftar saldo  (ukuran L, dipecah 9a/9b)

#### 9a — Lokasi `kind`, induk berstok, kolom saldo (sebelum 5, 7, 8)
**Masalah (7a/7b, 8a, 8g).**
- Migrasi `000040`: `stock_locations ADD kind VARCHAR(12) NOT NULL DEFAULT 'normal'`; `UPDATE stock_locations SET kind='damaged' WHERE is_system=1 AND code='RUSAK'`; `EnsureSystemLocation` (`repository.go:161-181`) menetapkan `kind` (hapus `_ = kind`), `CreateLocation` menerima kind. Indeks `(branch_id, kind)` tak perlu (satu RUSAK per cabang, dicari lewat `uq_stock_locations`).
- `resolveLocation` menolak `kind='damaged'` kecuali dipanggil jalur rusak; `CheckLocation` kontrak = non-damaged (menutup lolos CheckLocation penjualan).
- Query yang mengecualikan RUSAK: `SuggestLocations` (`repository.go:414-437`, tambah `l.kind <> 'damaged'`), `ProductAvailability` (`259-277`, JOIN `stock_locations` `kind<>'damaged'`), `CriticalStock`, daftar saldo (default `includeDamaged=false`). `GetBalance` per posisi tak berubah (posisi RUSAK tetap bisa dibaca).
- `CreateLocation/UpdateLocation` (`service.go:478-560`): bila induk punya saldo (`on_hand<>0 OR reserved<>0`) dan `forceTransfer=false` -> `Conflict` berkode `PARENT_HAS_STOCK`; `forceTransfer=true` -> satu tx: buat lokasi anak lalu pindahkan seluruh saldo induk ke anak lewat `postTx` (ref `transfer`, doc nol -> perlu ref: tulis baris dokumen `stock_transfers` otomatis status `received` agar punya `ref_id`; legacy `stock-location.service.ts:85-182`). Reservasi aktif di induk: blok (`Conflict`) sampai dilepas.
- `Update` memindahkan lokasi ke induk berstok: aturan sama.

#### 9b — Penyesuaian, rusak, scan, opening, daftar saldo
- **Adjust** (`transactions.go:265-343`): approver SELALU wajib (parity `stock.service.ts:1168`); ambang `stock_adjust_approval_threshold` dihapus dari alur (setting dibiarkan tak terpakai). Verifikasi kredensial (bcrypt) dipindah SEBELUM tx dan sebelum `FOR UPDATE` (kini `323` di dalam tx setelah `308`); mode `strict` (company setting `stock_adjust_approval_mode`) mewajibkan approver != pemohon. Basis biaya: tolak bila `in` tanpa (`finance.AverageUnitCost>0 || product.PurchasePrice>0`) atau `out` tanpa average cost (legacy `assertProductCostBasis`, pesan sama). Tiap penyesuaian = baris dokumen `stock_adjustments` (migrasi `000040`: `id, branch_id, kind(adjust|damaged_in|damaged_restore|write_off), product_id, variant_id, location_id, mode, qty_delta, qty_after, reason, requested_by, approved_by, approval_mode, created_at`) sehingga gerakan punya `ref_id` (ref_type `adjustment|damaged|write_off`) -> idempoten ditambah finance bisa membuat jurnal selisih stok/opname/rusak/hapus dari dokumen ini (kontrak `ListAdjustments`, bagian 5).
- **Rusak** (`411-504`): alasan WAJIB di move-in, restore, write-off (legacy `stock-damaged.service.ts:119,198,273`); lewat `Post` terurut (kedua kaki satu tx, `AllowDamagedLocation`).
- **Scan** (`service.go:646-676`): ganti `GetVariantByBarcode` dengan `products.ResolveScan(code)` (variant_code ATAU barcode, fallback product_code -> varian default). Permintaan ke product.
- **Opening** (`opening.go:205-302`): sebelum commit: `finance.OpeningPosted(branch)` true -> tolak ("Saldo awal finance sudah diposting; gunakan Penyesuaian Stok", legacy `stock-opening.service.ts:296-298`); baris tanpa harga beli > 0 ditolak di preview DAN commit (legacy `531-533`; harga modal dari master, bukan dari Excel); produk non-stok ditolak; preview mendeteksi (a) posisi duplikat dalam file (`PositionKey` set) dan (b) posisi yang sudah punya riwayat (`MovedPositions`) sebagai `skipped`, bukan hanya di commit; qty 0 dilewati (tak menulis gerakan kosong); `RefID` = id cabang + `ref_type=opening` agar idempoten (satu gerakan per posisi).
- **Daftar saldo** `GET /api/v1/stock/balances` (`handler.go:143-170` kini `[]` tanpa `productId`): parameter `search, criticalOnly, locationId, includeDamaged, productId, page, limit`; respons paginasi + `meta` (envelope standar) berisi per (produk,varian): `onHand, reserved, available, minStock, isCritical, locations[]` bila `locationId` diberikan. Implementasi: (i) tanpa filter teks/kritis: `GROUP BY product_id, variant_id` di `stock_balances` (indeks unik prefix `branch_id`) `ORDER BY product_id,variant_id LIMIT/OFFSET`, dekorasi satu `LabelsByIDs`; (ii) `search`: `products.SearchIDs(q, 200)` -> `IN`; (iii) `criticalOnly`: `products.ListStocked` (<=5000, sudah dipakai `CriticalStock`) dibandingkan `ProductAvailability` (satu GROUP BY) di Go lalu dipaginasi. Tanpa JOIN lintas modul.
- **Reservasi**: detail di F2a-12.

**Reuse.** `ensureDefaults`, `checkApprover` (dipindah), `ProductAvailability`, `LabelsByIDs`, `ListStocked`, `company.GetSettings`, `MovedPositions`.

**Tes integrasi (`stock_flows_test.go` diperluas, `stock_admin_test.go`).**
1. Lokasi RUSAK `kind=damaged`: `CheckLocation` menolak; `Suggest`/ketersediaan/kritis tak menghitungnya; `moveDamaged` tetap bekerja.
2. Buat anak di bawah induk berstok: tanpa `forceTransfer` -> `PARENT_HAS_STOCK`; dengan -> saldo pindah, induk 0, dua gerakan.
3. Adjust kecil (qty 1) tanpa approver ditolak; approver tanpa `stock.approve` ditolak; strict + approver=pemohon ditolak; produk tanpa basis biaya ditolak; bcrypt di luar tx: ukur bahwa `FOR UPDATE` tak ditahan selama verifikasi (tes: dua adjust paralel posisi sama, durasi bcrypt disimulasikan 300 ms -> total < 2x).
4. Rusak tanpa alasan di move-in/restore ditolak.
5. Scan kode varian, barcode, kode produk -> produk benar.
6. Opening: finance opening terposting -> ditolak; duplikat dalam file terdeteksi di preview; harga beli 0 ditolak; ulang commit -> `skipped`.
7. Daftar saldo tanpa productId mengembalikan baris; `criticalOnly`, `search`, `locationId`, `includeDamaged`, paginasi (`meta.total_pages`).

**Volume.** Daftar saldo: satu GROUP BY per halaman; kritis: GROUP BY penuh per cabang + <=5000 produk di memori (sudah pola `CriticalStock`). `stock_adjustments` tumbuh ~ jumlah penyesuaian (kecil).
**Dependensi.** 9a: F2a-0. 9b: F2a-0, 9a, product (ResolveScan/SearchIDs), finance (`OpeningPosted`, `AverageUnitCost`). **Dari finance:** konsumsi `ListAdjustments`.

---

### F2a-10 — Sales/purchasing: harga manual & minimum, termin, snapshot, ubah-setelah-konfirmasi  (ukuran L)

**Masalah (temuan 9).** 0.1 #9a-9h. Dibagi 10a harga, 10b termin & settlement, 10c snapshot & header, 10d tampilan bayar.

**10a — Harga manual & harga minimum (BR-08/BR-25).**
- `linePayload` (`sales/presentation/handler.go:66-73`) + `LineInput` menerima `unitPrice *int64`, `discountReason`, `note`, `taxable *bool`. `priceLines` (`sales/service.go:148-239`): sumber harga = (1) member rule bila `quote.Applied` (menang, seperti legacy `member_rule`), (2) `unitPrice` manual bila ada (>=0), (3) katalog. Cek minimum HANYA bila sumber bukan member: `net_baris >= round(MinSellingPrice * qty)` (setara legacy `order.service.ts:430-436` yang membandingkan harga satuan setelah diskon; dibandingkan di total baris agar bebas pembulatan). `MinSellingPrice>0` saja yang diperiksa. Pesan: produk, harga, minimum. Pembelian: harga manual sudah ada.
- Pembulatan (verifikasi aritmetika diminta): legacy menghitung `net = (u - u*p/100) * q` lalu satu kali `Math.round` (`order-pricing.service.ts:219-261`, `roundRupiah = Math.round(v+EPSILON)`), revamp `gross=round(q*u); disc=round(gross*p/100); net=gross-disc-nominal` (`domain/pricing.go`). Beda terjadi hanya bila `gross*p/100` berpecahan tepat .5: contoh `u=1250,q=1,p=1%`: legacy `1237.5 -> 1238`, revamp `disc=round(12.5)=13 -> 1237` (selisih Rp1/baris; pajak include: basis dihitung dari nilai tak terbulat vs terbulat dapat selisih Rp1). Rekomendasi (parity, satu pembulatan): ubah `CalcLine` (sales & purchasing, duplikat disengaja per modul) menjadi `netF = q*u*(1-p/100) - nominal` lalu `round(netF + 1e-9)` sekali, dan basis/pajak dari `netF`; `discount_nominal` tetap per baris (UI tak berubah; ETL mengalikan diskon per-unit legacy dengan qty). Test tabel 12 kasus (termasuk ketiga contoh di atas) memastikan pembulatan = legacy; total header = Σ baris.

**10b — Termin & settlement (BR prabayar/COD).**
- `payment_terms` ∈ `prepaid|cod|net` (sales & purchasing; tanpa migrasi, VARCHAR(20)). Default POS/regular: `cod`. Legacy `order.service.ts`/`delivery.service.ts` rujukan di 0.1 #9b.
- Net: wajib customer aktif ber-`phone` non-kosong (`order.service.ts:1289-1313`); `party=0` + net ditolak (kini lolos, `sales/service.go:258-272`). Tanggal jatuh tempo wajib. PO net: `payment_term_days` (0..3650) opsional; bila `dueDate` kosong dan `supplierInvoiceDate`+hari ada -> `due = invoice_date + hari` (legacy `order-pricing.service.ts:resolveOrderDueDate`).
- Prabayar: SJ/GR tak boleh dibuat/dikonfirmasi selama `grand_total - paid > 0` (F2a-2/3); status order untuk "proses" tak diubah (revamp tak punya status tengah).
- COD: pada SJ/GR yang MENYELESAIKAN order (posisi terakhir terpenuhi), `outstanding>0` -> `settlement.mode` wajib: `pay_now` (RecordForOrder dengan `method`, `amountTendered`, `reference`, `paymentDate`; butuh izin `payment.create` pemanggil — handler memeriksa `HasPermission`) atau `switch_to_net` (`sales.SwitchToNet(dueDate)`, izin `sales.approve`). Mode yang dikirim pada termin non-COD -> ditolak (legacy `delivery.service.ts:454-456`). Penerimaan PO COD hanya diproses pada penerimaan terakhir (legacy `goods-receipt.service.ts:153-166`).
- `outstanding` = `grand_total - paid` via `payment.PaidForOrders` (kontrak, batch). Efek retur ke outstanding sudah termuat sebagai pembayaran offset (F2a-8), jadi tak ada angka ganda.

**10c — Snapshot baris & header.**
- Migrasi `000041`: `sales_order_items`/`purchase_order_items` `ADD variant_name VARCHAR(255) NULL, variant_code VARCHAR(50) NULL, base_uom VARCHAR(30) NULL, pricing_source VARCHAR(12) NOT NULL DEFAULT 'catalog', base_price BIGINT NULL, discount_reason VARCHAR(255) NULL, line_note VARCHAR(255) NULL, taxable TINYINT(1) NOT NULL DEFAULT 1, tax_base BIGINT NOT NULL DEFAULT 0, tax_amount BIGINT NOT NULL DEFAULT 0` (+ `stock_tracked`,`item_type` dari F2a-7). Nama varian tak lagi dibuang (`purchasing/service.go:166`). `taxable=false` baris mengabaikan pajak order (legacy `line.taxable ?? orderTax.taxable`); `tax_base/tax_amount` tersimpan sehingga retur dan jurnal tak menghitung ulang (kini `lineEconomics` menghitung ulang di tiga tempat).
- Header: `sales_orders`/`purchase_orders` `notes` -> `TEXT`; `ADD assigned_user_id BIGINT UNSIGNED NULL`; PO `ADD payment_term_days SMALLINT UNSIGNED NULL`; `sales_orders ADD client_ref VARCHAR(64) NULL, UNIQUE (branch_id, client_ref)` (F2a-12). `order_date` tetap DATETIME(6): `OrderInput.OrderDate` menerima `YYYY-MM-DD` ATAU `YYYY-MM-DD HH:MM:SS`/RFC3339; kosong -> waktu sekarang UTC lengkap (kini `resolveDate` memotong ke tanggal, `sales/service.go:298-303`); tampilan WIB.
- `UpdateOrder` boleh status `draft` ATAU `confirmed` (legacy: non-terminal, `order.service.ts:549`). Header ringan (catatan, ship-to, due_date, nomor/tanggal faktur pajak, assigned user) selalu boleh sampai `completed`; perubahan harga/baris/pajak/party/termin hanya bila TIDAK ada SJ aktif, GR aktif, pembayaran aktif, atau retur aktif (`assertNoDownstream`, kontrak `ActiveNoteCount` dkk. dari F2a-6b) — padanan `assertNoStockMovementOrPayment` legacy. Di `doclock("so:ID")` dan CAS pada versi `updated_at` (kolom sudah ada) untuk mencegah penimpaan konkuren.

**10d — Tampilan dibayar/sisa/jatuh tempo efektif.**
- `GET /sales-orders` & `/sales-orders/{id}` (dan purchasing) menambah `paidAmount, outstandingAmount, paymentStatus (unpaid|partial|paid), effectiveDueDate, isOverdue` dihitung di handler/aplikasi lewat `payment.PaidForOrders` SATU panggilan per halaman (bukan N+1). `sales`→`payment` lewat setter `SetDependents` (F2a-6b). `effectiveDueDate` = `due_date` untuk net, kosong selain itu (legacy `order.service.ts:247-275, 1180-1219`).

**Reuse.** `priceLines`, `validateHeader`, `CalcLine`, `resolveShipTo`, `products.GetByID` (memuat `MinSellingPrice`, `Tracked`, `Type`), `pricing.Quote`.

**Tes integrasi (`orders_pricing_test.go`).**
1. Harga manual di bawah minimum ditolak; di atas diterima; member price menang dan melewati cek minimum.
2. Tabel pembulatan (12 kasus) = angka legacy; header = Σ baris.
3. Termin: prepaid SJ sebelum lunas ditolak, setelah bayar sukses; COD SJ final tanpa settlement ditolak SEBELUM stok bergerak; `pay_now` membuat 1 pembayaran (replay tak menggandakan) dan order lunas/selesai; `switch_to_net` mengubah termin + due; mode pada net ditolak; net tanpa HP/walk-in ditolak. PO COD hanya pada penerimaan terakhir.
4. Update order confirmed: catatan boleh; ganti harga setelah SJ/pembayaran ditolak; tanpa dependen diterima dan total dihitung ulang.
5. Snapshot varian/diskon alasan/catatan baris tersimpan; `taxable=false` mengecualikan pajak.
6. Daftar order memuat `paidAmount/outstanding/paymentStatus` dengan jumlah query konstan.

**Volume.** `PaidForOrders` satu IN-query per halaman (<= limit). Kolom tambahan kecil.
**Dependensi.** F2a-6a/b (PaidForOrders, SwitchToNet, ActiveNoteCount), F2a-7 (snapshot tracked), dipakai F2a-2,3,8,12. **Dari finance:** builder dapat memakai `tax_base/tax_amount` baris (opsional) — nilai identik dengan rumus sekarang kecuali pembulatan; baris `taxable=false`/jasa (F2a-7) harus diperlakukan.

---

### F2a-11 — Nomor dokumen: kunci bulan WIB, lubang nomor  (ukuran S)

**Masalah (temuan 10).** Terbukti, 0.1 #10. Skenario tumbukan konkret: 2026-09-30 18:00 UTC = 2026-10-01 01:00 WIB -> kunci `sales_order:2026-09`, label `.../10/00157`; setelah 07:00 WIB counter `2026-10` mulai 1..157 lalu menghasilkan `.../10/00157` kedua -> `uq_sales_orders_number` -> 409 "Data duplikat".

**Rancangan.**
- `branch/infrastructure/repository.go:226-238`: `sequenceKeys(kinds, now)` memakai `now.In(timeutil.Jakarta).Format("2006-01")`; satu sumber waktu: `NextDocumentNumber` menghitung `now := timeutil.NowUTC()` SEKALI dan menurunkan kunci + label dari instance yang sama (`service.go:279-286`). Unit test murni: `sequenceKeys` dengan `2026-09-30T18:00:00Z` -> `...:2026-10`; `2026-12-31T17:00:00Z` -> `...:2027-01` (batas tahun; format nomor kontinu lintas tahun tak berubah).
- Data lama: kunci bulan UTC lama tetap ada; tak perlu migrasi (nomor yang sudah terbit tak bentrok, karena bentrokan akan berhenti di `uq_*_number`). Tes regresi: nomor hasil `2026-09` counter pada 01:00 WIB vs counter `2026-10` setelah perbaikan tidak menghasilkan label+angka sama (kunci dan label kini selaras).
- Lubang nomor (`NextNumber` commit tx sendiri, `repository.go:253-272`): mitigasi murah: (a) semua modul memanggil `NextDocumentNumber` SETELAH semua validasi, tepat sebelum insert (delivery `232`, sales `317`, purchasing, retur, payment `355` diurutkan ulang); (b) opsi pool nomor terbebas (`ReleaseDocumentNumber`) — hanya bila pemilik meminta nomor tanpa lubang (pertanyaan #3). Legacy meng-increment di dalam tx dokumen (`delivery`/`sales-return` `generate*Number` dengan `pessimistic_write`), jadi legacy tanpa lubang; revamp tak bisa sama tanpa tx lintas modul.
- `DocGoodsReceipt` ditambahkan ke `AllDocKinds` dan `NextDocumentNumber`.

**Tes.** `TestSequenceKeyJakartaMonth` (unit); integrasi: 200 goroutine `NextDocumentNumber` -> semua unik; tes batas bulan memakai pembungkus waktu (`now` disuntik ke fungsi murni).
**Volume.** Tidak ada. **Dependensi.** Tak ada (kerjakan pertama). **Dari finance:** tak ada.

---

### F2a-12 — POS: client_ref, resume, reservasi, alokasi, tunai, antrean yatim  (ukuran L)

**Masalah (temuan 11, A6).** 0.1 #11a-11d. Alur `PosPage.tsx:397-560` = createOrder -> confirmOrder -> createDelivery -> confirmDelivery -> createPayment; tiap langkah tak idempoten, tak ada reservasi, satu lokasi untuk semua baris, tunai-diterima hanya di klien, tak ada diskon per baris/klem minimum, SO yatim tak tampak di antrean (`OpenHeaders` `channel <> 'pos'`).

**Rancangan (tanpa modul `pos` baru, ADR-0007).**
- **client_ref.** `sales_orders.client_ref` (unik per cabang, F2a-10c), `delivery_notes.client_ref` (F2a-2), `payments.client_ref` (F2a-6a). UUID per keranjang dibuat klien (`crypto.randomUUID()`, disimpan di `localStorage` agar bertahan reload) dan dipakai sebagai `clientRef` ketiganya (tabel berbeda -> tak bentrok). Replay mengembalikan dokumen yang ada (HTTP 200, `replayed:true`); bila `clientRef` sama tetapi isi beda (hash item) -> 409. `CreateOrder` menolak `clientRef` > 64.
- **Langkah idempoten.** `ConfirmOrder` CAS `draft->confirmed` dan replay-200 bila sudah `confirmed` (F2a-6b); `Create SJ` replay via clientRef; `Confirm SJ` replay-safe (P4) dan, untuk POS, membawa `settlement{mode:'pay_now', method, amountTendered, clientRef}` sehingga SATU panggilan menyelesaikan serah + pembayaran (memenuhi aturan COD "lunas saat serah", F2a-10b); `Create payment` replay via clientRef. Urutan baru klien: (1) order (2) confirm (3) SJ draf (4) confirm SJ+settlement. Bayar Nanti = termin net, langkah 4 tanpa `settlement`.
- **Resume server-side.** `GET /api/v1/sales-orders/by-client-ref/{ref}/state` (perm `sales.view`) -> `{order, deliveries[{id,status}], paidAmount, outstanding, nextStep: confirm|create_delivery|confirm_delivery|pay|done}`; sales memakai `delivery`/`payment` via `SetDependents` (F2a-6b). Klien membaca state saat memuat ulang/ketika langkah gagal dan melanjutkan dari `nextStep` — tak ada pesan "buat SJ susulan manual".
- **Reservasi stok saat keranjang.** `Hold` (`stock/application/service.go:163-229`) menjadi *upsert* menurut `key`: posisi sama -> ubah qty (delta diperiksa ke `Available`, `reserved` disesuaikan) dan perpanjang TTL; key milik posisi/cabang lain atau reservasi sudah `consumed/released` -> Conflict. Rute `POST /stock/reservations/hold|release` dipindah ke izin baru `stock.reserve` (kasir tak perlu `stock.manage`) dan di-scope cabang/pembuat (`Release` kini hanya by key, `service.go:234`: tambah cek `branch_id` sesi). Key: `pos:<cartRef>:<productId>:<variantId>:<locationId>` (<100 karakter). TTL default 20 menit, klien memperbarui tiap perubahan keranjang dan tiap 5 menit. Alur klien: ubah qty -> debounce 400 ms -> `POST /stock/allocations/suggest` (sudah ada) -> `hold` per lokasi hasil saran; hapus baris/kosongkan -> `release`; `beforeunload` -> `release` best-effort, sisanya mengandalkan TTL.
- **Konsumsi.** `delivery_note_items.reservation_key` (F2a-2) diisi klien dari key hold; `Post` mengonsumsi reservasi di tx yang sama dengan gerakan (F2a-0 5e). Reservasi kedaluwarsa -> jatuh ke ketersediaan biasa. Sisa hold (qty SJ < hold) dilepas oleh `Confirm` SJ via `Release(key)` pasca-final (idempoten).
- **Cek lokasi sebelum confirm.** Klien memanggil `suggest` sebelum langkah (2); bila `shortfall>0` checkout dihentikan sebelum order terkonfirmasi (kini gagal baru di langkah 3, `PosPage.tsx:476-495`, meninggalkan SO terkonfirmasi). Server: `CreateDelivery` dengan `locationId` kosong memakai alokasi otomatis lintas lokasi (F2a-2 langkah 6) — kasir tak lagi memilih satu lokasi.
- **Tunai diterima.** `amountTendered` ke `payments.amount_tendered` (F2a-6a); struk/`PosThermalReceiptPage` membaca kembalian dari data tersimpan.
- **Diskon per baris & klem minimum.** UI: `lib/cart.ts` + `PosPage.tsx` menambah diskon % / nominal per baris (opsional, izin `sales.update`), klem klien `unit >= minSellingPrice` (nilai dari `productDetail`); server tetap otoritas (F2a-10a menolak di bawah minimum; pesan ditampilkan). Harga manual POS tak diaktifkan (hanya member/katalog) kecuali izin.
- **Pembayaran POS selesai tak bisa dibatalkan -> order tak lunas.** Guard F2a-6b (`payment.Cancel`) menolak pada order COD `completed`; UI memberi jalur "Retur" bukan "Batalkan pembayaran".
- **SO POS yatim.** `OpenHeaders` (`aggregates.go`): `channel<>'pos' OR updated_at < UTC_TIMESTAMP(6) - INTERVAL 10 MINUTE` (POS confirmed yang tak selesai diserahkan dalam 10 menit muncul di antrean + penanda `orphanPos`); kondisi memakai indeks `ix_sales_orders_branch (branch_id,status)` (status 'confirmed' sempit).
- **Penyapu reservasi kedaluwarsa (A6).** Goroutine ticker 60 s di `cmd/server/main.go` memanggil `stock.SweepExpired(ctx, limit=500)` (UPDATE terbatas, `WHERE status='active' AND expires_at <= NOW` + kembalikan `reserved`), indeks baru `ix_stock_reservations_exp (status, expires_at)`; satu container, tanpa cron eksternal.
- **Android/POS shell**: tak disentuh.

**Reuse.** `suggest`/`Hold`/`Release` (ada, tak dipanggil web — grep kosong), `buildPosOrderPayload`, `posCheckoutSchema` (diperluas), `PosThermalReceiptPage`, F2a-2/6/10.

**Tes.**
- Integrasi (`pos_flow_test.go`): (1) `CreateOrder` clientRef sama dua kali (paralel) -> satu SO; (2) alur penuh dengan kegagalan disuntik di tiap langkah, lalu `state` -> `nextStep` benar, dilanjutkan sampai `done`, stok sekali, pembayaran sekali; (3) hold -> SJ confirm mengonsumsi reservasi (`reserved` 0, `on_hand` turun), hold kedaluwarsa tetap sukses bila stok ada; (4) dua kasir rebutan stok terakhir: hold ke-2 ditolak; (5) alokasi lintas lokasi; (6) SO POS confirmed 11 menit tanpa SJ muncul di antrean; (7) 20 pembayaran walk-in paralel tak saling blok; (8) `Hold` upsert menaikkan/menurunkan `reserved`; (9) penyapu mengembalikan `reserved`.
- Frontend (vitest): `lib/cart.test.ts` (diskon per baris, klem minimum), `checkout.test.ts` (urutan langkah + `nextStep`), perilaku `clientRef` bertahan setelah reload.

**Volume/performa.** `hold` per perubahan keranjang (debounce) = 1 tx kecil per lokasi; indeks `uq_stock_reservations_key` ada. Penyapu terbatas 500 baris/tick.
**Dependensi.** F2a-0, 2, 3(tak), 6a/b, 9a (kind damaged agar alokasi tak memilih RUSAK), 10; kerjakan terakhir.
**Dari finance:** tak ada selain pembayaran tunai biasa; `amount_tendered` tak mempengaruhi jurnal (kembalian bukan kas masuk bersih: jurnal tetap `amount`).

---

## 4. Urutan, dependensi, migrasi, paket tes

### 4.1 Urutan implementasi (tiap baris = satu PR yang bisa dirilis sendiri)
1. **F2a-11** (S) nomor/bulan WIB. Independen.
2. **F2a-0** (L) fondasi stok + migrasi `000040` bagian movement.
3. **F2a-4** (M) harness injeksi, `doclock.SetBypass`, `runParallel`; laporan konsistensi.
4. **F2a-2 bagian non-uang** (kuota, lock, CAS, bukti) lalu **F2a-3 bagian non-uang** (status pending/posted, duplikat, nomor GR). Pencabutan `MoveIn/MoveOut` untuk delivery/GR.
5. **F2a-1** retur (CAS, akumulasi, cek ulang); cabut `MoveIn/MoveOut` terakhir.
6. **F2a-5** transfer (+ kolom per-item di migrasi `000040`).
7. **F2a-9a** lokasi `kind`, induk berstok.
8. **F2a-7** non-stok/jasa (migrasi `000041` snapshot).
9. **F2a-6a** `RecordForOrder`/clientRef/tendered/`pair_id` (migrasi `000042`), lalu **F2a-6b/c** guard, pasangan offset, FIFO, `check`, izin refund.
10. **F2a-10** harga, termin, settlement, header, tampilan bayar; lengkapi sisi uang F2a-2/3.
11. **F2a-8** retur lanjutan (migrasi `000043`): kondisi, tukar, offset otomatis.
12. **F2a-9b** adjust/rusak/scan/opening/daftar saldo (butuh product & finance).
13. **F2a-12** POS + penyapu reservasi.

Graf ringkas: `11` bebas; `0 -> {4, 1, 2, 3, 5, 7, 9a, 12}`; `4 -> {1,2,3,5,8}`; `3 -> 1(beli)`; `6a -> {2uang, 3uang, 8, 10, 12}`; `6b -> {10, 12}`; `9a -> {5, 8, 12}`; `10 -> {2uang, 3uang, 8, 12}`; `8 -> 9b` tak bergantung; `9b` butuh product+finance.

### 4.2 Migrasi (additive; nomor final saat merge, agen squash melipat)
| Berkas | Isi |
|---|---|
| `000040_f2a_stock_integrity` | `stock_movements` (balance_before/after, moved_at, unique ref, indeks `(branch_id,moved_at)`); `stock_locations.kind`; `stock_transfer_items` lokasi per item; `stock_adjustments`; indeks `stock_reservations (status,expires_at)` |
| `000041_f2a_documents` | `delivery_notes` (client_ref, indeks komposit) + `delivery_note_items.reservation_key`; `goods_receipts` (number, status, supplier_delivery_number, client_ref, indeks); `sales_orders`/`purchase_orders` (notes TEXT, assigned_user_id, client_ref SO, payment_term_days PO); item SO/PO (snapshot, `stock_tracked`, `item_type`, `tax_base/tax_amount`) |
| `000042_f2a_payment` | `payments` (client_ref, amount_tendered, reference_number, pair_id) |
| `000043_f2a_returns` | `sales_returns`/`purchase_returns` (reason, replacement_*, difference_amount, *_reduction); item kondisi; replacement tax; `*_settlements.auto`; indeks `(order_id,status)` |
Semua `down.sql` tertulis. Tes integrasi membangun DB dari nol (`TESTDB_REBUILD=1`).

### 4.3 Paket tes (ringkas)
`stock_post_test`, `delivery_concurrency_test`, `goodsreceipt_pending_test`, `returns_concurrency_test`, `returns_exchange_test`, `transfer_concurrency_test`, `payment_guards_test`, `orders_pricing_test`, `stock_admin_test`, `pos_flow_test`, `faults_test` (matriks F2a-4), unit: `sequenceKeys`, `sortFIFO`, `CalcLine` tabel, `dberr.IsRetryable`. Semua pakai MySQL 8.4.3 `mini_erp_test`; skenario konkurensi memakai `runParallel` dengan `doclock.SetBypass(true)` untuk menguji CAS/unik DB, dan tanpa bypass untuk menguji jalur nyata. Regresi: tes yang ada (`money_paths_test`, `returns_test`, `exchange_test`, `movelocation_test`, `stock_flows_test`, `payment_flows_test`) harus tetap hijau; `TestAdjustSetMode` & sejenisnya diperbarui karena adjust kini selalu butuh approver (fixture `newShop` perlu user approver).

### 4.4 Butir "Juga" yang tidak di dalam paket ini
- Ledger pihak tanpa umur/bucket: ditangani agen paritas/laporan; dari sisi saya hanya menyediakan `effectiveDueDate` dan `PaidForOrders` (F2a-10d) sebagai bahan aging.
- FIFO urut due-date dan metode `check`: dikerjakan F2a-6c.

---

## 5. Antarmuka dengan finance

### 5.1 Yang SAYA butuhkan dari finance (`finance/contracts`)
1. `OpeningPosted(ctx, branchID) (bool, error)` — guard opening stok (F2a-9b). Implementasi: `OpeningStatus != nil` (`finance/application/opening.go:21-30`).
2. `AverageUnitCost(ctx, branchID, productID, variantID) (int64, error)` — basis biaya untuk adjust & barang pengganti (F2a-8/9b); 0 = belum ada.
3. `PeriodLockedThrough(ctx, branchID) (string, error)` (tanggal `YYYY-MM-DD`, kosong = tak ada periode terkunci) — batas backdate SJ/GR/retur (KI-80).
4. Pemetaan akun metode bayar `check` (-> bank) di `buildPayment` (F2a-6c).
5. Kursor biaya yang toleran celah (A5): `Sync` jangan bergantung pada `MaxCostMovementID` murni; ingest `id > cursor` DAN id yang belum ada di ledger dalam jendela 10 menit terakhir (atau `LEFT JOIN` anti-hilang). Dari sisi stok saya menambah jeda 3 detik pada `ListCostMovements`.

### 5.2 Yang SAYA sediakan untuk finance
- `StockClient.ListAdjustments/GetAdjustment` dan ref_type baku (`adjustment`, `damaged`, `write_off`, `*_cancel`) untuk jurnal selisih stok/opname/rusak/hapus; satu dokumen = satu `ref_id`, sehingga `MoveValues(ref_type, ref_id)` bekerja seperti SJ.
- Jaminan: setiap gerakan dokumen punya `ref_id>0` dan unik per posisi/arah; gerakan batal ber-`ref_type + "_cancel"` bersifat netral-biaya (cabang `default` di `costing.go:101-110` sudah menanganinya).
- `GoodsReceipt.Status` (hanya `posted` lewat `ListRecent`), `GoodsReceipt.Number`, `SupplierDeliveryNumber`.
- Retur: `Reason`, `Items[].Condition`, `Replacement*`, `DifferenceAmount`, `ReceivableReduction/PayableReduction`, `Return.Kind/Total` (F2a-8). Builder retur/SJ pengganti ada di finance (tidak saya sentuh).
- Item order: `StockTracked`, `ItemType`, `TaxBase/TaxAmount/Taxable` (F2a-7/10).
- Pembayaran: `Reference`, `AmountTendered` (informasional), method `check`, pasangan offset `PairID`.

---

## 6. Risiko

1. **Unik `stock_movements` pada data lama**: duplikat dev (dua baris produk+lokasi sama per SJ) memblokir ALTER; pra-cek + konsolidasi wajib (F2a-0). ETL (tahap K) harus menjumlah per kunci.
2. **Perubahan perilaku yang terlihat pengguna**: adjust selalu butuh approver; SJ/GR menolak prabayar/COD belum lunas; retur menahan kuota sejak draf; Confirm ulang = 200 bukan 409; SJ wajib sopir/petugas/tanda tangan/foto. Perlu catatan rilis + penyesuaian UI (web) sebelum dirilis; tes UI yang bergantung perilaku lama perlu diubah.
3. **doclock hanya satu proses**: aman untuk satu container (ADR deployment); jika kelak >1 instance, P1+P2 (DB) tetap menjaga stok dan status, tetapi cek kuota antar-dokumen (draf+draf) perlu `SELECT ... FOR UPDATE` pada baris order — saya menandai titik-titiknya (`Create`/`Confirm`) agar mudah diganti.
4. **Draf dengan stok sudah keluar** (kegagalan di antara `Post` dan CAS) tampak aneh bagi operator sampai diulang; mitigasi = laporan konsistensi F2a-4 + pesan error bernama dokumen. Tanpa job penyembuh otomatis (sesuai keputusan 3: tanpa outbox).
5. **Kursor biaya finance** (A5) — kehilangan baris biaya bila commit terlambat; mitigasi dua sisi (jeda 3 s + kursor toleran celah).
6. **Reservasi POS**: kasir yang menutup tab menahan stok sampai TTL (20 menit) -> penyapu + `release` beforeunload; kebocoran tetap terbatas TTL.
7. **Offset otomatis retur** mengubah angka piutang yang dilihat pelanggan secara otomatis (parity legacy). Retur lama (sebelum rilis) tidak di-offset ulang.
8. **Pembulatan single-round** mengubah total order baru hingga Rp1/baris terhadap perilaku revamp sebelumnya (mengikuti legacy). Order lama tak dihitung ulang.
9. **`salesreturn.ListByBranch` limit 10000** dan `GetPartyBalance` memuat seluruh retur cabang — risiko performa yang sudah ada, di luar paket (dicatat untuk agen laporan).
10. **Ukuran PR**: F2a-2/6/10/12 besar; bagi per sub-butir (non-uang dulu). Kontrak `StockClient` berubah, jadi semua fake tes harus diperbarui dalam PR yang sama.
11. **Zona waktu**: `moved_at` UTC; filter tanggal memakai `StartOfDayWIB/EndOfDayWIB` (OQ-A32) seperti sekarang.

---

## 7. Pertanyaan terbuka (hanya yang butuh keputusan pemilik)

1. **Bukti foto SJ untuk channel POS**: legacy mewajibkan scan SJ fisik pada konfirmasi SJ, tetapi serah POS (`deliverGoods`) tidak memakainya. Usulan saya: wajib untuk SJ regular, dikecualikan untuk SJ channel `pos` (diserahkan di tempat). Setuju?
2. **Nomor tanpa lubang**: legacy menaikkan counter di dalam tx dokumen (tanpa lubang); revamp memakai tx terpisah (lubang bila insert gagal / proses mati). Pilihan: (a) terima lubang langka + urutkan alokasi sedekat mungkin dengan insert [usulan], (b) pool nomor terbebas (pulih untuk kegagalan tertangani, lubang tetap bila proses mati), (c) wajib tanpa lubang -> butuh tx lintas modul (melanggar aturan arsitektur). Mana?
3. **Tambah-bayar retur tukar (difference > 0)**: ditagih saat SJ pengganti dikonfirmasi (`pay_now`, seperti COD) atau boleh jadi piutang tersendiri? Usulan: ditagih saat serah pengganti, kecuali pelanggan bertermin net (maka masuk `charge` terbuka di modul payment).
4. **PO yang seluruh isinya diretur** (legacy `purchase-return.service.ts` `cancelsOrder` + `cancelOrderIfFullyReturned`) otomatis dibatalkan. Dibawa atau tidak? Usulan: tidak dibawa (PO selesai tetap `completed`, retur memegang nilainya), karena membatalkan order yang sudah dijurnal bertentangan dengan guard F2a-6b.

