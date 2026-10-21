# PLAN — SJ Langsung (surat jalan dulu, order dibuat dari surat jalan)

**Proyek:** mini-erp revamp · **Dibuat:** 2026-10-21 · **Disesuaikan ke `main` (koreksi ADR-0023):** 2026-10-21 sore
**Keputusan:** [ADR-0022](../../../knowledge/decisions/ADR-0022-sj-langsung.md) ·
**Analisis, riset, dan flow pengguna:** [analysis/001-sj-langsung](../../../analysis/001-sj-langsung/analysis.md)
**Tujuan:** menambah jalur SJ Langsung tanpa mengubah perilaku SO → SJ, POS, dan SJ pengganti retur.
Setiap tugas punya kriteria terima, uji, dan bukti.

---

## 0. Protokol pengerjaan

1. **Baca dulu:** `CLAUDE.md`, `knowledge/INDEX.md`, `.claude/rules/*`; ADR-0022 dan analisis 001;
   ADR-0016 (idempoten + CAS), ADR-0018 D2/D12, ADR-0020, ADR-0021, **ADR-0023** (koreksi beralasan).
2. **Satu tugas = satu unit yang bisa di-commit dan di-revert.** Urutan fase: 1 → 2 → 3 → 4 → 5. Fase 6 dan 7
   berjalan bersama tiap tugas.
3. **Format tugas:** `- [ ] **ID** — judul` dengan `Aksi / Terima / Uji / Bukti`. Centang `[x]` hanya bila
   `Bukti:` terisi. Bila terblokir, ubah ke `[~]` dan isi `Blokir:`.
4. **Batasan:** tanpa join/FK lintas modul; satu migrasi satu modul; lingkup sesi dari `auth`; uang integer
   rupiah; tanggal ≤ hari ini (ADR-0021); perubahan skema diikuti `node scripts/db-schema-doc.mjs` + baris §8
   `DB_SCHEMA.md`; sqlc lewat `npm run sqlc:generate` (atau `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`).
5. **Regresi wajib nol:** semua test SJ, POS, retur, koreksi, dan jurnal yang ada tetap hijau tanpa mengubah
   niat ujinya.
6. **Penyimpangan** dicatat di §9, **temuan baru** di §8.

## 0a. Penyesuaian terhadap kode `main` (setelah penggabungan koreksi ADR-0023)

Rencana awal ditulis sebelum koreksi salah input masuk. Yang berubah dan cara rencana ini mengikutinya:

| Perubahan di `main` | Dampak ke rencana |
|---|---|
| Nomor ADR koreksi menjadi **0023**; 0022 tetap SJ Langsung | Tidak ada tabrakan nomor |
| Rantai migrasi sudah sampai `000041_stock_corrections` (`000038` kosong) | Migrasi rencana ini: **`000042_delivery_direct`** dan **`000043_sales_origin`** |
| `delivery_notes` punya `cancel_reason/cancelled_at/by`; batal SJ lewat web **wajib beralasan** (`validator.Reason`, CAS `CancelNote` hanya draf) | Batal SJ Langsung memakai alasan yang sama. Perlu CAS baru **`CancelDirectNote`** yang menerima draf **dan terkonfirmasi** selama `origin='direct' AND sales_order_id=0` (stok dibalik dulu); SJ yang sudah tertaut mengikuti aturan lama (retur) |
| `sales_orders` punya jejak batal & tutup sisa; `Correction()` memberi `editMode/canCloseShort` | Order dari SJ lahir `completed` → `editMode=none`, `canCloseShort=false`, batal ditolak ("Order sudah final"); tidak perlu penjaga tambahan. Selama orkestrasi belum selesai (order `confirmed`, `origin='delivery'`) `assertNoDependents` sudah mengunci harga begitu SJ tertaut |
| `UpdateOrder` terkonfirmasi: penuh bila tanpa dokumen, ringan bila ada | Tidak berubah |
| `CancelReplacement` di kontrak delivery; `Cancel` menolak SJ pengganti | Cabang SJ Langsung ditempatkan sebelum pemeriksaan jenis, tanpa menyentuh jalur pengganti |
| `enrichQueue` finance mengisi pihak dari order | SJ Langsung yang belum tertaut membawa `party_id` sendiri ke antrean (`partyID` dari SJ, ringkasan "SJ Langsung · menunggu order") |
| Antrian kirim (`queueWaiting`) membuang SJ draf yang ordernya tidak ada | SJ Langsung draf tetap tampil dengan nama pelanggan dari SJ (`party` contract); order `origin='delivery'` tidak masuk tab *buat SJ* |

Keputusan desain yang dikunci saat penyesuaian (melengkapi ADR-0022):

- **Penyelesaian order milik `sales`.** `LinkDirect` hanya menautkan (CAS) dan memeriksa Σ qty; `sales`
  yang mengonfirmasi, menautkan, melunasi (bila tunai), lalu `SetStatus(completed)`. Bila tautan ditolak
  (SJ sudah diorder admin lain), order baru dibatalkan (belum punya dependen). Pembayaran tidak pernah dicatat
  sebelum tautan berhasil.
- **Kunci:** `sales` memegang `sjl:<clientRef>`; `delivery.LinkDirect` memegang `so:<id>` lalu `sj:<id>`
  urut naik; konfirmasi/batal SJ Langsung memegang `sj:<id>`.
- **SJ Langsung hanya untuk produk berstok** (barang, `tracked`). Baris jasa/non-stok ditambahkan di order
  (K6), sehingga `postNote` tanpa SO memposting semua baris.
- **Kontrak delivery baru:** `GetByIDs`, `ListOpenDirect(branch, party, limit)`, `OpenDirectSummary(branch)`,
  `LinkDirect(actor, branch, soID, noteIDs)`; field `Origin`, `PartyID`, `PartyName`, `ShipTo*`, `LinkedAt/By`.
- **Dashboard tidak bergantung pada `delivery`:** penghitung "SJ belum diorder" dibaca frontend dari
  `GET /deliveries/open-direct` (izin `delivery.view`), tanpa menambah tepi modul.
- **Pratinjau order dari SJ** (`POST /sales-orders/from-deliveries/preview`) memakai `priceLines` yang sama
  dengan eksekusi, sehingga harga katalog/member di form = harga yang disimpan.

## 1. Keputusan pemilik/akuntan (default dipakai bila tidak dijawab)

Rincian ada di ADR-0022: K1 akuntansi masa tunggu, K2 gabung beberapa SJ, K3 termin, K4 batas umur,
K5 pemberian izin, K6 baris jasa, K7 waktu rilis.

- [~] **G-01** — Kumpulkan jawaban K1–K7 dan ubah status ADR-0022 menjadi Accepted
  - Aksi: catat jawaban pemilik/akuntan, atau "default dipakai", di ADR-0022. Bila K1 = akun antara,
    aktifkan F-04.
  - Terima: setiap K punya status; baris ADR di `knowledge/INDEX.md` tidak lagi bertanda *(Proposed)*.
  - Uji: tidak ada.
  - Bukti:
  - Blokir: jawaban pemilik/akuntan belum ada; ADR-0022 diterima dengan default K1–K7 (dicatat di STATUS §3). Yang dibutuhkan: konfirmasi K1 (jurnal ditunda vs akun antara) dan K3 (termin) dari pemilik/akuntan, lalu catat di ADR
## 2. Fase 1 — Data & modul `delivery`

- [x] **D-01** — Migrasi `000042_delivery_direct`
  - Aksi: `origin varchar(20) NOT NULL DEFAULT 'order'`, `party_id bigint unsigned NULL`, `ship_to_label`
    (120), `ship_to_recipient` (150), `ship_to_phone` (30), `ship_to_address` text, `linked_at datetime(6)`,
    `linked_by bigint unsigned`; indeks `ix_delivery_notes_direct_open (branch_id, origin, sales_order_id,
    status, delivery_date)`; skrip down.
  - Terima: sqlc diregenerasi; `db-schema-doc --check` hijau; baris §8; data lama/ETL tetap `origin='order'`.
  - Uji: harness integrasi membangun ulang skema (hash migrasi berubah).
  - Bukti: `apps/api/migrations/000042_delivery_direct.{up,down}.sql`; sqlc diregenerasi (`delivery/infrastructure/sqlc`); `node scripts/db-schema-doc.mjs --check` up to date; §8 DB_SCHEMA; harness integrasi membangun ulang skema (hash berubah) dan `TestDirectDeliveryLifecycleWithoutOrder` lulus
- [x] **D-02** — Kontrak `delivery`
  - Aksi: konstanta `OriginOrder/OriginDirect`; field baru di `DeliveryNote`; `OpenDirectSummary` struct;
    metode `GetByIDs`, `ListOpenDirect`, `OpenDirectSummary`, `LinkDirect`.
  - Terima: `arch:check` hijau; komentar metode menjelaskan invariannya.
  - Uji: kompilasi + fake di modul pemakai.
  - Bukti: `delivery/contracts/client.go`: `OriginOrder/OriginDirect`, field `Origin/PartyID/PartyName/ShipTo*/LinkedAt/LinkedBy`, `OpenDirectSummary`, metode `GetByIDs/ListOpenDirect/OpenDirectSummary/LinkDirect`; `node scripts/arch-check.mjs` OK
- [x] **D-03** — `CreateDirect` (draf SJ Langsung)
  - Aksi: pelanggan aktif bertipe customer (`party`); alamat kirim dari buku alamat (`GetAddress`) atau
    ketik; tanggal ≤ hari ini; baris hanya produk `barang` berstok, satuan dari `product.UOMFactor`, qty > 0,
    lokasi atau `stock.Allocate`; batas panjang sopir/plat/petugas; nomor `DocDeliveryNote`; idempoten
    `client_ref`; peringatan `draftWarnings` + "pelanggan punya N SJ belum diorder".
  - Terima: baris tersimpan `origin='direct'`, `sales_order_id=0`, `party_id` terisi; audit
    `deliveries.create_direct`.
  - Uji: integrasi: jalur valid, pelanggan arsip/supplier, produk jasa (ditolak), satuan salah, stok kurang,
    ulang `client_ref`.
  - Bukti: `delivery/application/direct.go` `CreateDirect/createDirect/directLines/resolveShipTo`; audit `deliveries.create_direct`; `TestDirectDeliveryLifecycleWithoutOrder` (jasa ditolak, pelanggan tak ada ditolak, replay clientRef, warnings)
- [x] **D-04** — Konfirmasi, batal, bukti untuk SJ Langsung belum tertaut
  - Aksi: `Confirm`: cabang `origin='direct' && sales_order_id=0` di kunci `sj:<id>` — tanpa SO, kuota,
    pelunasan (settlement → 400), dan tanpa `completeOrderIfDone`; `requireEvidence` menerima SO nil;
    `postNote(so=nil)`. `Cancel`: cabang yang sama → `stock.Reverse` bila ada mutasi, CAS
    `CancelDirectNote` (draf/terkonfirmasi), alasan tersimpan. Replay konfirmasi SJ tertaut `origin='direct'`
    tidak memanggil `completeOrderIfDone` (penyelesaian milik sales).
  - Terima: stok keluar `RefDelivery` tanpa pelunasan; batal membalik stok; jalur SJ order **tidak berubah**.
  - Uji: integrasi: konfirmasi (stok berkurang), konfirmasi ulang (idempoten), batal setelah konfirmasi (stok
    kembali, alasan tersimpan), settlement ditolak. Test SJ order lama hijau.
  - Bukti: `direct.go` `confirmDirect/cancelDirect` + cabang di `service.go` (`Confirm`, `confirmLocked`, `Cancel`, `requireEvidence` SO nil, `draftWarnings`); CAS `CancelDirectNote`; `TestDirectDeliveryLifecycleWithoutOrder` (stok 50→40→50, settlement 400, konfirmasi ulang idempoten, alasan tersimpan); test SJ order lama hijau (`go test ./...`)
- [x] **D-05** — `LinkDirect`
  - Aksi: kunci `so:<id>` + `sj:<id>` urut; SO di cabang, `confirmed`, `origin='delivery'`; tiap SJ
    `direct`, `confirmed`, `document_kind='order'`, pelanggan sama, `sales_order_id` 0 atau = SO (replay);
    Σ qty dasar SJ per produk/varian = qty dasar baris berstok SO; satu transaksi CAS untuk semua SJ
    (`sales_order_id 0 → id`, `linked_at/by`) + audit `deliveries.link_order` dalam tx; SJ yang ternyata
    milik SO lain → Conflict tanpa menautkan satu pun.
  - Terima: dua permintaan bersamaan atas SJ yang sama hanya satu yang menautkan.
  - Uji: integrasi: sukses, replay, pelanggan beda, SJ draf, SJ sudah tertaut ke SO lain, qty tidak cocok.
  - Bukti: `direct.go` `LinkDirect/linkDirectLocked/lockAll` + `infrastructure/direct.go` `LinkDirect` (satu tx, CAS `LinkDirectNote`, audit `deliveries.link_order`); `TestLinkDirectGuards` (draf, pelanggan lain, sudah tertaut ke SO lain, qty tak cocok, replay), `TestOrderFromDeliveriesRaceCompensates`
- [x] **D-06** — Daftar, filter, antrean kerja
  - Aksi: filter `origin` dan `unordered=1`; `ListOpenDirect`/`OpenDirectSummary` memakai indeks D-01;
    `partyName` SJ Langsung diisi satu batch `NamesByIDs`; view JSON `origin`, `partyId`, `partyName`,
    `shipTo*`, `linkedAt`; `queueWaiting` menampilkan SJ Langsung draf dengan nama pelanggan dari SJ.
  - Terima: tanpa N+1; `contract:check` hijau.
  - Uji: integrasi daftar + antrean.
  - Bukti: `listfilter.go` (`Origin`, `Unordered`), `direct.go` `ListOpenDirect/OpenDirectSummary/GetByIDs/decorateParties` (batch `NamesByIDs`), `workqueue.go` SJ Langsung draf dengan nama pelanggan; `TestDirectDeliveryLifecycleWithoutOrder` (ListOpenDirect/Summary/ListFiltered), `TestDirectDeliveryWorkQueue`; `contract:check` OK
- [x] **D-07** — Rute HTTP `delivery`
  - Aksi: `POST /api/v1/deliveries/direct` (`delivery.create_direct`); `GET /api/v1/deliveries/open-direct?
    partyId&limit` (`delivery.view`) → `{items, count, oldestDate}`; konfirmasi/batal/bukti memakai rute lama.
  - Terima: `docs/API_CONTRACT.md` §6 diperbarui di commit yang sama.
  - Uji: test handler/integrasi (izin ditolak, validasi).
  - Bukti: `delivery/presentation/handler.go` `CreateDirect` (`POST /deliveries/direct`, `delivery.create_direct`), `OpenDirect` (`GET /deliveries/open-direct`), filter `origin/unordered`, noteView field baru; `docs/API_CONTRACT.md` §6
## 3. Fase 2 — Modul `sales`: order dari SJ

- [x] **S-01** — Migrasi `000043_sales_origin` + kontrak
  - Aksi: `sales_orders.origin varchar(20) NOT NULL DEFAULT 'standard'`; konstanta `OriginStandard/
    OriginDelivery`; field `Origin`; proyeksi `GetOrder*` + raw list; `InsertOrder` menerima origin; filter
    daftar `origin`; `ListOpenShipments` mengecualikan `origin='delivery'`.
  - Terima: sqlc, `db-schema-doc`, §8; ETL tidak terdampak.
  - Uji: harness membangun ulang skema; test daftar.
  - Bukti: `000043_sales_origin.{up,down}.sql`; `sales/contracts` `OriginStandard/OriginDelivery` + `SalesOrder.Origin`; proyeksi `GetOrder*`/raw list + `InsertOrder` origin; filter `origin` (`list_raw.go`, `prepareFilter`); `ListOpenHeaders*` mengecualikan `origin='delivery'`; sqlc diregenerasi; §8 DB_SCHEMA
- [x] **S-02** — `PreviewFromDeliveries` + `CreateFromDeliveries`
  - Aksi: masukan `deliveryIds` (1–20), header (termin `net|cod`, jatuh tempo, pajak, tanggal order ≥ tanggal
    SJ terakhir dan ≤ hari ini, faktur pajak, catatan, penanggung jawab), `lines[]` harga/diskon per kunci
    produk/varian/satuan (qty diabaikan, harus ada di SJ), `extraItems[]` non-stok, `settlement` untuk tunai,
    `clientRef` wajib. Langkah idempoten di kunci `sjl:<clientRef>`: (1) SJ lewat `GetByIDs` — terkonfirmasi,
    `direct`, belum tertaut, cabang & pelanggan sama; (2) baris dari SJ (qty terkunci) + harga lewat
    `priceLines`; (3) buat SO `origin='delivery'` + konfirmasi; (4) `LinkDirect`; gagal karena SJ sudah
    diorder → batalkan SO baru, Conflict; (5) tunai & sisa > 0: `pay_now` → `RecordForOrder` (`sjl:<soID>`)
    atau `switch_to_net` → `SwitchToNet`; (6) `SetStatus(completed)`; audit `sales_orders.create_from_delivery`.
    Ulang dengan `clientRef` sama melanjutkan dari status order. Pratinjau memakai builder yang sama tanpa
    menulis dan mengembalikan `warnings` (masa faktur pajak ≠ masa SJ, F-03).
  - Terima: SO `completed`, SJ tertaut, total = Σ baris; tidak ada SO yatim setelah kegagalan di langkah mana pun.
  - Uji: integrasi: 1 SJ, gabung 2 SJ, harga di bawah minimum (ditolak), prabayar (ditolak), tunai bayar
    sekarang, tunai jadi tempo, SJ sudah diorder (kompensasi), replay `clientRef`, balapan dua admin.
  - Bukti: `sales/application/from_deliveries.go` `PreviewFromDeliveries/planFromDeliveries/CreateFromDeliveries/settleFromDeliveries`; `TestOrderFromDeliveries` (gabung 2 SJ, jasa tambahan, prabayar/baris asing/barang stok ekstra/tunai tanpa settlement ditolak, replay clientRef, SJ sudah diorder, jurnal 2 SJ terbentuk), `TestOrderFromDeliveriesPayNow` (bayar sekali, replay tidak menggandakan), `TestOrderFromDeliveriesRaceCompensates` (1 selesai, 0 order gantung)
- [x] **S-03** — Rute dan status lanjutan
  - Aksi: `POST /api/v1/sales-orders/from-deliveries/preview` dan `POST /api/v1/sales-orders/from-deliveries`
    (`sales.create`; `pay_now` butuh `payment.create`, `switch_to_net` butuh `sales.approve`); `nextStep`
    `StateByClientRef` untuk `origin='delivery'`: `confirmed` → `finish_from_deliveries`.
  - Terima: API_CONTRACT §6; `contract:check` hijau.
  - Uji: integrasi handler.
  - Bukti: `sales/presentation/handler.go` `FromDeliveriesPreview/FromDeliveries` (`POST /sales-orders/from-deliveries[/preview]`, izin settlement); `pos_state.go` `StepFinishFromDeliveries`; API_CONTRACT §6; `contract:check` OK
- [x] **S-04** — Penjaga order `origin='delivery'`
  - Aksi: `OpenShipments*` mengecualikan `origin='delivery'`; `origin` di view & filter daftar; detail memuat SJ
    tertaut lewat daftar SJ yang sudah ada.
  - Terima: SO dari SJ tidak pernah muncul sebagai "sisa kirim".
  - Uji: integrasi antrean kirim.
  - Bukti: `aggregates.sql` `ListOpenHeaders*` `origin <> 'delivery'`; `orderView.origin`; `TestDirectDeliveryWorkQueue` (order dari SJ tidak di tab buat SJ); `TestOrderFromDeliveries` (`Correction` = none, tanpa tutup sisa)
## 4. Fase 3 — Modul `finance`

- [x] **F-01** — Alasan antre "Menunggu order"
  - Aksi: `buildDelivery` → Conflict "Surat jalan langsung … menunggu order" untuk `origin='direct'` belum
    tertaut; `ReasonAwaitingOrder` di `classifyBuildFailure`/`reasonText`; `QueueItem.AwaitingOrder` dari
    `queueSources` (pihak dari SJ); `IgnoreSource` menolak dokumen `AwaitingOrder`.
  - Terima: setelah tertaut, posting menghasilkan jurnal identik dengan SJ order berharga sama, bertanggal SJ.
  - Uji: integrasi: SJ Langsung → antre alasan benar → abaikan ditolak → tautkan → posting → jurnal seimbang.
  - Bukti: `finance/application/builders.go` (Conflict "menunggu order"), `dailyclose.go` `ReasonAwaitingOrder` + `reasonText`, `queue.go` `QueueItem.AwaitingOrder` (pihak dari SJ), `ignore.go` menolak; `TestDirectDeliveryLifecycleWithoutOrder` (Post → conflict, Ignore → conflict, item antrean awaitingOrder berpihak)
- [x] **F-02** — Butir kesiapan "SJ Langsung belum diorder"
  - Aksi: `readiness.go` menambah `direct_unordered` (CheckFailed) dari item antrean `AwaitingOrder` bertanggal ≤
    akhir periode, detail jumlah + tanggal tertua.
  - Terima: `SafeClose` tertahan dengan pesan jelas; rekonsiliasi menjelaskan nilai SJ yang antre.
  - Uji: integrasi kesiapan sebelum/sesudah tautan.
  - Bukti: `readiness.go` butir `direct_unordered` (CheckFailed, jumlah + tertua); `TestDirectDeliveryLifecycleWithoutOrder` (failed, count 1, detail tanggal) dan `TestOrderFromDeliveries` (OK setelah tertaut)
- [x] **F-03** — Pajak: masa faktur pajak
  - Aksi: peringatan (tidak memblokir) di pratinjau/eksekusi S-02 bila bulan `taxInvoiceDate` ≠ bulan SJ.
  - Terima: peringatan muncul di respons.
  - Uji: unit.
  - Bukti: `from_deliveries.go` peringatan masa faktur pajak ≠ masa SJ (`preview.Warnings`, hanya bila taxType ≠ none); dikembalikan oleh pratinjau dan eksekusi
- [~] **F-04** — *(Hanya bila K1 = akun antara)* Akun "Barang Terkirim Belum Ditagih"
  - Aksi: pemetaan akun baru; jurnal saat konfirmasi SJ Langsung dan saat tertaut; rekonsiliasi disesuaikan.
  - Terima: buku persediaan = ledger setiap saat.
  - Uji: integrasi lintas periode.
  - Bukti:
  - Blokir: hanya bila K1 = akun antara; menunggu jawaban akuntan (G-01)
## 5. Fase 4 — Frontend (`apps/web`, Design System v2)

Ikuti `knowledge/FRONTEND_GUIDE.md` dan `docs/plan/redesain-ui/PANDUAN-MIGRASI-HALAMAN.md`.

- [x] **W-01** — Pengiriman: tombol bercabang dan filter
  - Aksi: `DeliveriesPage` "Buat surat jalan ▾": *Dari order jual* / *Tanpa order (SJ Langsung)* (hanya
    `delivery.create_direct`); filter "Asal" dan "Belum diorder"; lencana "SJ Langsung · Belum diorder".
  - Terima: tanpa izin, pilihan baru tidak tampil.
  - Uji: vitest.
  - Bukti: `DeliveriesPage.tsx` (DropdownMenu *Dari order jual* / *Tanpa order (SJ Langsung)* hanya dengan `delivery.create_direct`; filter Asal + Belum diorder lewat `delivery-list-filter.schema.ts`; lencana); `DeliveriesPage.test.tsx` (4 test); vitest delivery+print 129/129
- [x] **W-02** — Halaman buat SJ Langsung (`/deliveries/new-direct`)
  - Aksi: pemilih pelanggan, alamat kirim (buku alamat / ketik), tanggal, sopir/kendaraan/petugas, baris barang
    tanpa harga (satuan dari produk, lokasi opsional); peringatan dari respons; Zod; `clientRef` per draf form.
  - Terima: `message` server selalu ditampilkan.
  - Uji: vitest.
  - Bukti: `CreateDirectDeliveryPage.tsx` + `schemas/direct-delivery.schema.ts` + rute `/deliveries/new-direct` (`route-paths.ts`, `route-pages.ts`, `protected.routes.tsx`, izin `delivery.create_direct`); `deliveryService.createDirect/openDirect`; `CreateDirectDeliveryPage.test.tsx` (5 test)
- [x] **W-03** — Detail SJ dan Rel Dokumen
  - Aksi: langkah "Diorder" untuk `origin='direct'`; pelanggan/alamat dari SJ; tombol **Buat order dari SJ ini**
    (`sales.create`); dialog konfirmasi tanpa pelunasan; batal beralasan juga untuk SJ Langsung terkonfirmasi
    belum tertaut.
  - Terima: SJ order tampil seperti sekarang.
  - Uji: `delivery-rail.test.ts`, `DeliveryDetailPage.test.tsx`.
  - Bukti: `delivery-rail.ts` langkah "Diorder"; `DeliveryDetailPage.tsx` (pelanggan/alamat dari SJ, tombol *Buat order dari SJ ini*, konfirmasi tanpa pelunasan, batal beralasan untuk draf & terkonfirmasi belum diorder); `QueuePage.tsx`; `delivery-rail.test.ts` (+3), `DeliveryDetailPage.test.tsx` (+6), `QueuePage.test.tsx` (+1)
- [x] **W-04** — Cetak SJ
  - Aksi: `DeliveryNotePrintPage` memakai pelanggan/alamat dari SJ bila tanpa SO; tata letak tetap (ADR-0018 D12).
  - Terima: verifikasi cetak fisik oleh pemilik.
  - Uji: `delivery-document.test.ts`.
  - Bukti: `print/services/print-fetch.ts`, `print/lib/delivery-document.ts` (`productLinesForNote`, `deliveryPartyBlock`, status "Belum Diorder"), `DeliveryNotePrintPage.tsx` (pelanggan/alamat dari SJ, nomor order kosong, tata letak tetap); `delivery-document.test.ts` (+3), `print-pages.test.tsx` (+1). Uji cetak fisik: menunggu pemilik (Z-01)
- [x] **W-05** — Order Jual: panel dan pintu masuk
  - Aksi: panel "SJ menunggu order (N) · tertua X hari" dari `GET /deliveries/open-direct`; "Tambah order ▾":
    *Order biasa* / *Dari surat jalan*; filter "Asal".
  - Terima: panel tersembunyi bila N = 0 atau tanpa izin.
  - Uji: vitest.
  - Bukti: `SalesOrdersPage.tsx` panel "SJ menunggu order (N) · tertua X hari" (`salesService.openDirectDeliveries`), tombol *Tambah order* bercabang + `FromDeliveriesPicker.tsx`, filter Asal + lencana "Dari SJ"; `SalesOrdersPage.test.tsx` (+4); vitest sales 145/145
- [x] **W-06** — Form order mode "dari SJ"
  - Aksi: `SalesOrderFormPage?fromDeliveries=1,2` atau pilih pelanggan → centang SJ; pratinjau dari server;
    pelanggan/alamat terkunci; qty terkunci; harga bisa diubah; baris jasa boleh; termin Tempo / Tunai (+ bayar
    sekarang / jadikan tempo); saran tanggal faktur pajak; **Simpan & selesaikan**; `clientRef` disimpan di
    `sessionStorage` dan dikirim ulang saat ulang.
  - Terima: sukses menawarkan *Cetak invoice* dan *Catat pembayaran*.
  - Uji: vitest.
  - Bukti: `components/FromDeliveriesForm.tsx` + `lib/from-deliveries.ts` (clientRef di sessionStorage, dipakai ulang sampai sukses) + `SalesOrderFormPage.tsx` cabang `?fromDeliveries=`; pratinjau server (debounce), qty & pelanggan terkunci, baris jasa, termin tunai/tempo + pelunasan bergating izin, saran tanggal faktur pajak, *Simpan & selesaikan*; `SalesOrderFormPage.test.tsx` (+8), `from-deliveries.test.ts` (6)
- [x] **W-07** — Banner di form order biasa
  - Aksi: pelanggan dengan SJ Langsung terbuka → "Pelanggan ini punya N SJ belum diorder — ambil dari SJ?".
  - Terima: tidak memblokir.
  - Uji: vitest.
  - Bukti: `SalesOrderFormPage.tsx` banner "Pelanggan ini punya N SJ belum diorder — ambil dari SJ?" (hanya dengan `delivery.view`, tidak memblokir); `SalesOrderFormPage.test.tsx`
- [x] **W-08** — Detail order, dashboard, dan tutup periode
  - Aksi: detail SO "Asal: SJ Langsung" + chip SJ tertaut; dashboard penghitung dari `open-direct`; halaman
    posting/periode menampilkan alasan "Menunggu order" dan butir `direct_unordered`.
  - Terima: angka dari backend, tanpa hitungan ganda di klien.
  - Uji: vitest.
  - Bukti: `SalesOrderDetailPage.tsx` + `sales-order-rail.ts` (lencana "Dari SJ", Asal: SJ Langsung, rel mulai dari Dikirim, aksi lanjutan cetak invoice/catat pembayaran); `DashboardPage.tsx` tile "SJ Langsung belum diorder" dari `GET /deliveries/open-direct?limit=0`; `PostingPage.tsx` lencana "Menunggu order" + tombol Abaikan disembunyikan; `PeriodsPage`/`DailyOverviewCard` merender label backend (`direct_unordered`, `awaiting_order`); test: `SalesOrderDetailPage.test.tsx` (+3), `DashboardPage.test.tsx` (+3), `PostingPage.test.tsx` (+3), `PeriodsPage.test.tsx` (+1)
## 6. Fase 5 — Izin, audit, seed

- [x] **P-01** — Izin `delivery.create_direct`
  - Aksi: `delivery/contracts.PermissionCatalog`; tidak diberikan ke role bawaan (K5); gating di frontend.
  - Terima: `TestCatalogCoversRoutes` dan `contract:check` hijau.
  - Uji: test katalog.
  - Bukti: `delivery/contracts.PermissionCatalog` `delivery.create_direct`; tidak ada di `DefaultGrants`/peta legacy (K5); `go test ./internal/bootstrap` (TestCatalogCoversRoutes) hijau
- [x] **P-02** — Label audit
  - Aksi: `audit-labels.ts`: `deliveries.create_direct`, `deliveries.link_order`,
    `sales_orders.create_from_delivery`.
  - Terima: `audit-labels.test.ts` hijau.
  - Uji: vitest.
  - Bukti: `apps/web/src/modules/audit/lib/audit-labels.ts`: `deliveries.create_direct`, `deliveries.link_order`, `sales_orders.create_from_delivery`; `audit-labels.test.ts` hijau (pemindai Go tidak menemukan kunci hilang/usang)
## 7. Fase 6 — Uji menyeluruh

- [x] **T-01** — Skenario e2e (`apps/e2e`)
  - Aksi: gudang membuat SJ Langsung → konfirmasi; admin membuat order dari 2 SJ → Selesai; tutup harian →
    jurnal; bayar; batalkan SJ Langsung lain sebelum diorder.
  - Terima: lulus di CI lokal.
  - Uji: Playwright.
  - Bukti: `apps/e2e/specs/E17-sj-langsung.spec.ts` (gudang ditolak 403, admin membuat 3 SJ Langsung, konfirmasi stok −6, pelunasan ditolak, antrean `awaitingOrder`, abaikan ditolak, pratinjau qty 5, prabayar ditolak, order dari 2 SJ selesai + replay clientRef, SJ tertaut tidak bisa dipakai/dibatalkan, tutup harian menjurnal 2 SJ, batal SJ ketiga stok kembali, `direct_unordered` ok): `npm run test -w apps/e2e -- E17` → 4 passed (23 dtk) 2026-10-21
- [x] **T-02** — Regresi
  - Aksi: `npm run verify` (dan `npm run test:e2e` bila T-01 ada) pada pohon bersih.
  - Terima: semua hijau; tidak ada test lama yang diubah niatnya.
  - Uji: perintah di atas.
  - Bukti: `npm run verify` HIJAU 2026-10-21 (go build/vet, go test termasuk integrasi 370 dtk, tsc, eslint, vitest, ui-ratchet, arch-check + self-test, sqlc-sync, db-schema-doc, contract-check) pada pohon kerja dengan seluruh perubahan SJ Langsung; test lama yang disesuaikan hanya `sqlc_delivery_test.go` (nilai bawaan `Origin: "order"`) dan `role_matrix_test.go` (deklarasi `delivery.create_direct` = owner)
## 8. Fase 7 — Dokumen & penutup

- [x] **DOC-01** — Dokumen pemilik fakta
  - Aksi: `docs/API_CONTRACT.md` §6; `docs/DB_SCHEMA.md` (regenerasi + §8); `knowledge/DOMAIN_GLOSSARY.md`;
    `docs/SYSTEM_DESIGN.md` (kontrak baru); `knowledge/MODULE_MAP.md` (delivery → `party`); `docs/STATUS.md`;
    `docs/RUNBOOK.md` (operasi harian: SJ belum diorder sebelum tutup).
  - Terima: setiap fakta hanya di pemiliknya.
  - Uji: `db-schema-doc --check`, `contract:check`.
  - Bukti: API_CONTRACT §6 (deliveries/direct, open-direct, filter origin/unordered, from-deliveries[/preview], nextStep); DB_SCHEMA §3 regenerasi + §8 (000042, 000043), `db-schema-doc --check` up to date; DOMAIN_GLOSSARY (SJ Langsung, Order dari SJ); SYSTEM_DESIGN (kontrak delivery, L5 + party, kontrak lintas modul #13); MODULE_MAP (delivery → party); STATUS §3; RUNBOOK §6; README docs (rencana aktif); INDEX ADR-0022
- [~] **Z-01** — Audit penutup dan rilis
  - Aksi: tinjau ulang terhadap ADR-0022; catat penyimpangan di §9; rilis lewat CI → GHCR → `./deploy.sh`
    (ADR-0019) **setelah** cutover; uji cetak fisik SJ Langsung.
  - Terima: semua tugas `[x]` dengan bukti, atau `[~]` dengan blokir yang disetujui pemilik.
  - Uji: tidak ada.
  - Bukti:
  - Blokir: rilis dijadwalkan setelah cutover (K7); uji cetak fisik SJ Langsung dan konfirmasi K1/K3 menunggu pemilik/akuntan. Audit penutup terhadap ADR-0022: semua tugas lain `[x]` dengan bukti; penyimpangan tercatat di §0a/§9
Temuan baru selama pengerjaan: tambahkan di sini dengan ID `N-xx`.

## 9. Log keputusan & penyimpangan

| Tanggal | Tugas | Catatan |
|---|---|---|
| 2026-10-21 | — | Rencana dibuat dari analisis 001 dan ADR-0022 (Proposed) |
| 2026-10-21 | T-01 | Spec e2e dibuat API-only (E17): UI dipakai spec lain; alur SJ Langsung dibuktikan lewat endpoint, konsisten dengan E13 |
| 2026-10-21 | §0a | Disesuaikan ke `main` setelah koreksi ADR-0023: nomor migrasi 000042/000043, batal beralasan, CAS batal SJ Langsung terkonfirmasi, penyelesaian order milik `sales`, dashboard tanpa tepi modul baru |
