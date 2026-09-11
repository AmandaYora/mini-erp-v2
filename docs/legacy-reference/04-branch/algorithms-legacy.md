# Algorithms (Legacy) — Modul 04 Branch (Multi-Cabang)

**Kelompok B — cukup dipahami maksudnya, BUKAN untuk ditiru implementasinya.** Fokus dokumen ini
adalah **hasil yang diharapkan** dari tiap logika kunci, bukan cara kodenya mencapainya.

Kontrak yang wajib presisi ada di Kelompok A: [business-rules.md](business-rules.md) ·
[numbering-sequence.md](numbering-sequence.md).

---

## 1. Melahirkan Cabang Siap Pakai

**Masalah yang dipecahkan:** cabang yang baru dibuat harus bisa langsung dipakai berjualan.
Kalau ada satu langkah setup yang terlewat, cabang jadi "setengah jadi" dan kegagalannya baru
ketahuan saat transaksi pertama — di tangan pengguna, bukan di tangan admin.

**Hasil yang diharapkan:**

- Setelah tombol simpan ditekan, cabang punya **satu tempat menyimpan barang** dan **lima deret
  nomor dokumen** yang siap dipakai.
- Kalau apa pun gagal di tengah jalan, **tidak ada jejak apa pun** yang tertinggal — tidak ada
  cabang tanpa gudang, tidak ada gudang tanpa cabang, tidak ada penomoran separuh.

**Konsep yang dipakai sekarang:** semua penulisan dijalankan dalam satu unit gagal-bersama, dan
kode cabang dipakai sebagai bahan untuk menurunkan seluruh nama/prefix turunannya.

**Untuk rebuild:** yang mengikat adalah **jaminannya** (siap pakai + gagal-bersama), bukan caranya.
Boleh saja penyediaan dilakukan lewat mekanisme lain (event, langkah terpisah yang idempoten,
default yang dihitung saat dipakai) selama dua jaminan itu tetap terlihat oleh pengguna.

---

## 2. "Pastikan Ada", Bukan "Buat Ulang"

**Masalah yang dipecahkan:** sistem terus bertambah jenis dokumennya. Saat fitur retur penjualan
lahir, cabang-cabang yang sudah ada belum punya deret nomornya. Menuntut admin membuat ulang
cabang jelas tidak masuk akal.

**Hasil yang diharapkan:**

- Menyimpan cabang — apa pun yang diubah, bahkan tidak mengubah apa-apa — membuat cabang itu
  **lengkap kembali**: deret nomor yang hilang muncul, prefix yang usang diperbarui.
- **Nomor tidak pernah mundur.** Deret yang sudah berjalan tetap melanjutkan hitungannya.
- Menjalankan langkah ini berulang kali **tidak mengubah apa pun** setelah kali pertama.

**Konsep yang dipakai sekarang:** untuk tiap jenis dokumen wajib, cari dulu; kalau ada perbarui
sebagian saja, kalau tidak ada buat baru bernilai nol.

**Untuk rebuild:** sifat **idempoten** inilah intinya. Ini pola yang layak dipertahankan — ia
membuat penambahan jenis dokumen baru di masa depan hanya perlu menambah satu entri di daftar
"jenis wajib", tanpa migrasi data terpisah.

Catatan penting: daftar "jenis wajib" itu sekarang **tidak lengkap** — penomoran mutasi stok tidak
terdaftar dan karena itu tidak pernah ikut diperbaiki (KI-46). Sistem baru sebaiknya punya **satu
daftar** yang benar-benar mencakup semua jenis dokumen bernomor per-cabang.

---

## 3. Menurunkan Nama & Prefix dari Kode Cabang

**Masalah yang dipecahkan:** pengguna harus bisa membaca cabang asal sebuah dokumen hanya dari
nomornya, tanpa membuka sistem.

**Hasil yang diharapkan:**

- Semua turunan konsisten dan bisa ditebak: gudang default berawalan `GDG-`, nomor order `ORD-`,
  pembayaran `PAY-`, surat jalan `SJ-`, retur penjualan `RTR-`, retur pembelian `RTB-`, semuanya
  diikuti kode cabang huruf besar.
- Spasi dan huruf kecil yang diketik pengguna **tidak pernah bocor** ke hasil turunan.

**Untuk rebuild:** aturan turunannya adalah kontrak (ada di [business-rules.md](business-rules.md)
§9). Yang bebas dipilih ulang: apakah prefix **disalin** ke tiap deret nomor (seperti sekarang,
demi kecepatan) atau **dihitung saat dipakai** dari kode cabang. Pilihan "dihitung saat dipakai"
menghilangkan seluruh kelas masalah "prefix usang" sekaligus, termasuk KI-46 — tapi mengubah
perilaku: dokumen lama tidak berubah, sedangkan sekarang prefix yang tersimpan ikut berubah.

---

## 4. Menerbitkan Nomor Dokumen Tanpa Bentrok

**Masalah yang dipecahkan:** dua kasir menekan "simpan" pada detik yang sama tidak boleh
menghasilkan dua dokumen bernomor sama.

**Hasil yang diharapkan:**

- Setiap dokumen mendapat nomor yang **unik dalam cabang dan jenisnya**.
- Bila dokumen gagal disimpan setelah nomornya diambil, nomor itu **kembali tersedia** — tidak ada
  lubang di deret.
- Nomor order **kembali ke `00001` setiap awal bulan** tanpa ada pekerjaan terjadwal yang harus
  berjalan tepat waktu.

**Konsep yang dipakai sekarang:** dua pendekatan hidup berdampingan —

1. Untuk **order**, kenaikan dilakukan sebagai satu operasi tulis yang aman meski barisnya belum
   ada. Ini penting justru pada dokumen pertama tiap bulan, saat deretnya memang baru lahir.
2. Untuk **dokumen lain**, baris deret dikunci selama transaksi berlangsung sehingga permintaan
   kedua menunggu.

**Untuk rebuild:** yang mengikat adalah keunikan, ketiadaan lubang, dan reset bulanan order.
Mekanismenya bebas. Yang **perlu diputuskan sadar** adalah reset tahunan yang selama ini
dijanjikan di data tapi tidak pernah dijalankan (KI-45) — apa pun keputusannya, keduanya sah,
tapi harus dipilih, bukan diwarisi.

Ada juga jalur cadangan "nomor darurat" berbasis cap waktu ketika deret nomor tidak ditemukan.
Maksudnya baik — jangan sampai transaksi pengguna gagal hanya karena kesalahan setup — tapi
bentuknya berbeda dari nomor normal dan bisa lolos ke dokumen resmi tanpa ada yang sadar. Sistem
baru sebaiknya tetap punya jalur cadangan, tapi disertai penanda yang terlihat.

---

## 5. Menjaga Satu Lokasi Stok Default

**Masalah yang dipecahkan:** cabang harus selalu punya minimal satu tempat menyimpan barang, dan
penamaannya harus mengikuti apa yang diketik pengguna di form cabang.

**Hasil yang diharapkan:**

- Setiap cabang punya tepat satu lokasi bertanda default.
- Nama dan kode lokasi itu selalu mencerminkan data cabang terkini.

**Yang sebenarnya terjadi dan perlu diputuskan ulang:** karena "selalu mencerminkan data cabang"
diterjemahkan sebagai **menulis ulang setiap kali cabang disimpan**, perubahan yang dilakukan
pengguna dari menu Stok akan hilang (KI-42). Dan karena pencarian lokasi default tidak
memperhitungkan lokasi yang sudah diarsipkan, ada jalur di mana cabang berakhir **tanpa** lokasi
default yang hidup — kebalikan dari tujuan aturan ini.

**Untuk rebuild:** tentukan **satu pemilik** nama gudang default. Dua pilihan yang sama-sama masuk
akal:

| Pilihan | Konsekuensi |
|---|---|
| Cabang pemiliknya | Form cabang tetap seperti sekarang; menu Stok tidak boleh mengubah nama lokasi default |
| Lokasi stok pemiliknya | Field di form cabang hanya dipakai **saat pembuatan**, lalu jadi baca-saja |

---

## 6. Menyusun Daftar Cabang untuk Layar

**Masalah yang dipecahkan:** dua sumber data cabang harus jadi satu daftar di layar — daftar
lengkap dari server (untuk pengguna yang berhak) dan ringkasan cabang yang boleh diakses (dari
sesi lokal).

**Hasil yang diharapkan:** halaman menampilkan seluruh cabang perusahaan, dan tiap baris tahu
apakah pengguna punya akses ke cabang itu.

**Konsep yang dipakai sekarang:** kedua sumber digabung berdasarkan nomor cabang, dengan data dari
server menang bila tersedia.

**Yang perlu dipahami:** penggabungan ini menyembunyikan sebuah kegagalan. Ketika pengguna tidak
berhak memuat daftar cabang, sumber pertama kosong dan yang tersisa hanya ringkasan sesi — yang
tidak memuat alamat cabang. Halaman `/branches` sendiri tidak terpengaruh (pengguna itu memang
tidak bisa membukanya), tapi **kop nota yang mereka cetak kehilangan baris alamat** (KI-47).

**Untuk rebuild:** kalau kop dokumen memang perlu alamat cabang, alamat itu harus tersedia untuk
**semua** peran yang bisa mencetak — entah lewat data sesi yang lebih lengkap, atau dengan tidak
menjadikan izin pengelolaan cabang sebagai syarat membaca identitas cabang sendiri.

---

## 7. Berganti Cabang Aktif

**Masalah yang dipecahkan:** seluruh data yang sedang tampil milik cabang lama dan harus tidak
boleh tercampur dengan cabang baru.

**Hasil yang diharapkan:**

- Setelah berganti, **tidak ada satu pun angka atau daftar dari cabang sebelumnya** yang masih
  terlihat.
- Pengguna kembali ke tempat ia berada, bukan dilempar ke dashboard.
- Pergantian hanya boleh ke cabang yang **benar-benar boleh diakses** dan **masih aktif**.

**Konsep yang dipakai sekarang:** cabang aktif diubah di sesi server, lalu sisi web menandai
seluruh modul sebagai "belum dimuat" sehingga masing-masing mengambil ulang datanya saat
dikunjungi — bukan memuat semuanya sekaligus.

**Untuk rebuild:** strategi "batalkan semua, muat saat dibutuhkan" ini bagus dan layak
dipertahankan — ia menjaga pergantian cabang tetap cepat. Yang perlu diperbaiki adalah **pesan
saat penolakan**: cabang yang sudah tidak aktif sekarang menghasilkan galat teknis, bukan
penjelasan (KI-02).

---

## 8. Menyaring Cabang Berdasarkan Status

**Masalah yang dipecahkan:** cabang yang sudah ditutup tidak boleh lagi dipakai bertransaksi, tapi
data historisnya harus tetap utuh.

**Hasil yang diharapkan:** cabang non-aktif hilang dari tempat-tempat yang menawarkan pilihan
(pemilihan cabang saat login, tujuan mutasi stok) dan berhenti diikutkan dalam agregasi harian,
sambil seluruh dokumen lamanya tetap ada.

**Yang sebenarnya terjadi:** penyaringan ini **tidak konsisten** — daftar cabang di halaman
`/branches` tidak menyaring apa pun dan tidak menandai apa pun (KI-39), sementara jalur pemulihan
sesi juga tidak menyaring (KI-10). Dan karena tidak ada UI untuk menonaktifkan cabang (KI-36),
seluruh mekanisme ini praktis belum pernah dipakai.

**Untuk rebuild:** ini bukan soal algoritma, ini soal **melengkapi produk**. Kalau "menutup cabang"
memang kebutuhan nyata, ia butuh: tombolnya, penanda statusnya di daftar, penyaringan yang
konsisten di semua jalur, dan pesan penolakan yang bisa dibaca pengguna.
