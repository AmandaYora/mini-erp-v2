# Performance Guide — mini-erp

Aturan biaya baca/tulis yang sudah ditegakkan di kode. Baca ini sebelum menambah endpoint daftar,
laporan, atau apa pun yang mengulang sesuatu per baris.

Prinsipnya satu kalimat: **biaya sebuah endpoint tidak boleh tumbuh mengikuti jumlah baris yang
dikembalikannya.** Kalau sebuah endpoint mengembalikan N baris, ia tidak boleh mengeluarkan N query.

## 1. Dekorasi baris: selalu berkelompok, tidak pernah per baris

Laporan mengagregasi angka, lalu menempelkan nama yang dibaca manusia. Agregasinya biasanya sudah
satu query berkelompok — bagian yang diam-diam menghidupkan N+1 adalah **dekorasinya**.

Yang tersedia di `contracts/`, pakai ini:

| Kebutuhan | Kontrak |
|---|---|
| kode+nama produk untuk banyak id | `ProductClient.LabelsByIDs` |
| kode+nama party untuk banyak id | `PartyClient.LabelsByIDs` |
| nama party saja | `PartyClient.NamesByIDs` |
| baris jurnal dari banyak entry | `finance` repo `LinesByEntryIDs` |

Di modul `finance`, `application/decorate.go` adalah **satu-satunya** tempat dekorasi laporan.
Dekorasi bersifat best-effort dan sengaja menelan error: laporan uang tidak boleh gagal hanya karena
sebuah produk diganti namanya. Datanya sendiri (mis. baris jurnal) tidak begitu — kegagalannya
dipropagasi.

Jangan panggil `GetByID` di dalam loop. Kalau kontraknya belum punya metode batch, **tambahkan** ke
`contracts/` — itu justru gunanya `contracts/`, dan arch-check tetap lolos (nol join lintas modul).

## 2. Jangan membayar seumur-hidup buku berkali-kali

`AccountFootings` mengagregasi SELURUH riwayat posting satu cabang. `FinanceClient.AccountBalance`
memanggilnya untuk **satu** akun, jadi meminta empat angka lewat empat panggilan berarti
mengagregasi seluruh buku empat kali.

- Butuh beberapa saldo kunci → `AccountBalances(ctx, branchID, codes)` (satu footing).
- Butuh laba per hari **dan** total rentang → `DailyTrend` sekali, lalu jumlahkan di Go. Jangan
  memanggil `NetProfit` untuk dua rentang yang saling tumpang tindih.
- Butuh nilai persediaan saja → `InventoryTotal`, bukan `InventoryValue` (yang ikut menamai tiap
  posisi).

`dashboard/application/service.go` adalah contoh acuannya: tiga pembacaan berkelompok. Yang konstan
adalah JUMLAH query (tidak tumbuh dengan jumlah akun, produk, atau hari); biaya tiap query tetap
tumbuh dengan jumlah baris jurnal dalam rentang (`GROUP BY` atas `finance_journal_lines`), jadi
indeks pada `(branch_id, entry_date)` dan akun tetap penting.

## 3. Yang sudah di-cache (dan cara mem-invalidasinya)

Dua cache in-process. Keduanya **bukan** TTL-first: TTL hanya jaring pengaman, yang menjaga
kebenaran adalah invalidasi eksplisit.

**Chart of accounts** — `finance/infrastructure/chartcache.go`. Setiap baris jurnal meresolusi akun
lewat kode sebelum bisa ditulis, jadi ini dulu beberapa query per dokumen di jalur checkout POS.
Cache dimuat sekali, di-drop oleh setiap tulis akun/mapping di repository yang sama. Kalau kamu
menambah jalur tulis ke `finance_accounts` atau `finance_account_mappings`, **panggil
`r.chart.invalidate()`**.

**Sesi terautentikasi** — `auth/application/sessioncache.go`. Dulu setiap request terautentikasi
membayar enam query (sesi, user, role, permission, role detail, cabang). Sekarang sekali, lalu
dipakai ulang.

Konsekuensinya untuk modul lain: `user` dan `branch` **memiliki** baris yang menentukan apa yang
boleh dilakukan sebuah sesi, jadi mereka wajib mengumumkan tulisannya lewat
`authcontracts.AuthzInvalidator`. Implementasinya `authcontracts.AuthzHub`: dibuat PERTAMA oleh
`bootstrap.Wire`, diberikan ke `user.NewModule` dan `branch.NewModule` sebagai argumen konstruktor, dan
auth mendaftarkan dirinya sebagai pelanggan (`Modules.Verify()` gagal bila auth tak tersambung):

- `InvalidateUser(id)` — status user, role user, akses cabang user.
- `InvalidateAll()` — permission sebuah role, hapus role, status cabang.

Menambah jalur tulis yang mengubah otorisasi tanpa memanggil salah satunya = perubahan itu baru
berlaku setelah TTL (30 detik), bukan di request berikutnya. Itu bug, bukan trade-off.

## 4. Indeks mengikuti bentuk query

Indeks daftar kini ada di baseline tiap modul (`apps/api/migrations/0000NN_<modul>.up.sql`, hasil squash
dari rantai lama termasuk migrasi `list_indexes`). Alasannya per tabel, ringkasnya:

- Daftar dokumen berbentuk `WHERE branch_id = ? … ORDER BY id DESC LIMIT ?`, jadi indeksnya
  `(branch_id, id)` — satu range scan melayani filter **dan** urutan, tanpa filesort, baik saat
  status difilter maupun tidak.
- Daftar yang diurutkan nama (`products`, `parties`) butuh indeks pada kolom urutnya, kalau tidak
  setiap halaman menyortir seluruh tabel dulu.
- Indeks posisi (`branch_id, product_id, variant_id, location_id`) **tidak** melayani filter waktu;
  kartu stok punya `(branch_id, created_at)` sendiri.

Tabel baru dengan endpoint daftar → tambahkan `(branch_id, id)` di migrasi yang sama.

## 5. Transport

`internal/server/compress.go` menggzip respons yang layak (≥1KB, tipe teks/JSON). Jangan menambah
lapisan kompresi lain di nginx — cukup satu.

Cache-Control diatur di `server.go`: aset build Vite (`/assets/*`, nama ber-hash) `immutable` setahun,
`/media/*` memakai `private, max-age` sebesar sisa umur URL bertanda tangannya (maks 1 jam, tidak lebih lama dari izin yang menerbitkannya), `index.html` `no-cache` karena dialah yang menunjuk hash terbaru.

## 6. Frontend

- Pencarian daftar memakai pola **apply-on-submit** (state input terpisah dari state filter), bukan
  fetch per ketikan. Typeahead memakai debounce.
- `useAsyncData(loader, deps)` memberi `loader` sebuah `AbortSignal`. Teruskan ke service →
  `apiGet`/`apiPage` supaya request usang benar-benar berhenti. Kalau loader punya `.catch` yang
  memunculkan toast, bungkus dengan `if (!isAbort(err))` — pembatalan sendiri bukan kegagalan.
- Turunan yang dihitung dari daftar (total keranjang, peta harga) di-memo. Halaman seperti POS punya
  belasan state lain yang berubah saat kasir mengetik; tanpa memo, setiap ketikan menghitung ulang
  seluruh matematika keranjang.
- Pencarian linier di dalam loop render (`array.find` per baris) diganti `Map` yang dibangun sekali.

## 7. Impor massal

Impor (produk, saldo awal stok) memvalidasi tiap baris terhadap data yang sudah ada. Ambil himpunan
pembandingnya **sekali** sebelum loop, lalu tambahkan apa yang baru saja ditulis ke himpunan itu
supaya baris berikutnya di file yang sama tetap melihatnya:

- `product` repo `ExistingCodes(ctx, codes)`.
- `stock` repo `MovedPositions(ctx, branchID)` + `PositionKey(...)`.
