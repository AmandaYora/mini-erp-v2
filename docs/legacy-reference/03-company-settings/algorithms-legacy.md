# Algorithms Legacy — Modul 03 Company & Settings

**Kelompok B — fokus pada HASIL yang diharapkan, bukan cara implementasinya.**

Untuk aturan yang **presisi dan wajib dipertahankan**, rujuk
[business-rules.md](business-rules.md) — dokumen ini sengaja tidak mengulanginya secara detail.

---

## 1. Peta Logika Kunci

| # | Logika | Hasil yang harus dicapai |
|---|---|---|
| A-01 | Penyimpanan konfigurasi bertingkat | Mengubah satu setelan tidak menghapus setelan lain |
| A-02 | Pembersihan profil dokumen cetak | Yang tersimpan hanya yang benar-benar akan dicetak |
| A-03 | Kalibrasi kertas dengan nilai default | Kotak kosong berarti "pakai bawaan", bukan "pakai nol" |
| A-04 | Normalisasi nilai yang tak bertipe | Nilai rusak tidak pernah membuat sistem gagal |
| A-05 | Penyebaran perubahan kebijakan | Perubahan berlaku seketika tanpa siapa pun login ulang |
| A-06 | Konfigurasi tersedia sebelum halaman dirender | Tidak ada kedipan atau nilai kosong sesaat |
| A-07 | Mode baca-saja | Pemegang izin baca melihat konfigurasi tanpa bisa mengubahnya |
| A-08 | Pencatatan perubahan konfigurasi | Perubahan kebijakan dapat ditelusuri sepenuhnya |

---

## A-01 — Penyimpanan Konfigurasi Bertingkat

**Hasil yang diharapkan:** mengubah satu setelan **tidak boleh** menghapus setelan lain, meski
keduanya berada dalam kelompok yang sama.

Ini logika paling rawan di modul ini, dan cara sistem lama mencapainya perlu dipahami dengan tepat.

**Di server, penyimpanan bersifat ganti-total per kelompok.** Kelompok yang dikirim menggantikan
isinya sepenuhnya; kelompok yang tidak dikirim tidak disentuh. Tidak ada penggabungan per-setelan.

**Yang menyelamatkan dalam praktik adalah klien**, yang selalu membaca isi kelompok lama,
menyebarkannya, lalu menambahkan setelan yang berubah — sehingga hasil akhirnya tampak seperti
penggabungan.

Jadi properti "tidak menghapus setelan lain" **tidak dijamin oleh sistem** — ia dijamin oleh
disiplin setiap pemanggil. Konsekuensinya:

| Pemanggil | Perilaku |
|---|---|
| Halaman Pengaturan | Menggabung dengan benar |
| Test E2E modul Stock | Menggabung dengan benar (membaca → menyebarkan → memulihkan di akhir) |
| Integrasi lain yang mengirim satu setelan | **Akan menghapus sisanya** |

Hasil yang seharusnya dicapai di sistem baru: **penggabungan terjadi di tempat yang menyimpan,
bukan di tempat yang memanggil.** Dengan begitu properti itu berlaku untuk semua pemanggil, bukan
hanya yang ingat melakukannya.

Alternatif yang sama baiknya: pisahkan setelan skalar menjadi field tersendiri sehingga konsep
"kelompok yang ditimpa" hilang dengan sendirinya. Yang tetap perlu berbentuk gumpalan hanyalah
profil dokumen cetak, karena strukturnya bersarang dan berdaftar.

---

## A-02 — Pembersihan Profil Dokumen Cetak

**Hasil yang diharapkan:** yang tersimpan hanya data yang benar-benar akan muncul di dokumen —
tanpa baris kosong, tanpa spasi menggantung, tanpa field yang tak berisi apa-apa.

Ini contoh logika yang dirancang baik dan layak dipertahankan utuh. Prinsipnya:

| Prinsip | Wujudnya |
|---|---|
| Ruang kosong bukan isi | Setiap teks dipangkas; yang tersisa kosong dihilangkan sepenuhnya |
| Setiap daftar punya field penentu | Baris telepon tanpa **nomor** dibuang; baris rekening tanpa **nomor** dibuang |
| Daftar kosong bukan daftar | Bila seluruh barisnya terbuang, field daftarnya sendiri dihilangkan |
| Teks multi-baris adalah daftar | Daftar barang dipecah per baris, baris kosong dibuang |
| Kelompok kosong bukan kelompok | Blok kalibrasi dihilangkan bila seluruh isinya tidak diisi |

Kenapa "dihilangkan" lebih baik daripada "disimpan kosong": halaman cetak memakai pola
"pakai nilai ini, atau kalau tidak ada pakai bawaan". Field yang tersimpan sebagai string kosong
**bukan** ketiadaan — ia akan mencetak kop kosong alih-alih memakai nama perusahaan. Jadi
membedakan "kosong" dari "tidak ada" bukan kerapian, melainkan syarat agar mekanisme fallback
bekerja.

Satu pilihan yang perlu diketahui: **nomor** adalah field penentu untuk telepon dan rekening.
Baris berisi label "TOKO" tanpa nomor akan **hilang tanpa peringatan**, sedangkan baris berisi
nomor tanpa label **dipertahankan** dengan label kosong. Ini masuk akal (nomor tanpa label masih
berguna dicetak; label tanpa nomor tidak), tetapi pengguna yang mengetik label lalu lupa nomornya
akan mendapati barisnya lenyap setelah menyimpan.

Hasil yang lebih baik di sistem baru: tetap buang barisnya, tetapi **beri tahu** bahwa ada baris
yang dibuang karena nomornya kosong.

---

## A-03 — Kalibrasi Kertas dengan Nilai Default

**Hasil yang diharapkan:** kotak yang dikosongkan berarti **"pakai nilai bawaan"**, bukan
"pakai nol" — dan pengguna dapat melihat nilai bawaan itu tanpa harus mengetiknya.

Cara sistem lama mencapainya sederhana dan efektif: **nilai bawaan ditampilkan sebagai
placeholder**. Kotak kosong menampilkan angka bawaan berwarna redup, sehingga pengguna tahu apa
yang akan dipakai tanpa nilai itu benar-benar tersimpan.

Yang membuat logika ini benar adalah pembedaan dua jenis angka:

| Jenis | Aturan | Alasan |
|---|---|---|
| Ukuran kertas (lebar, tinggi) | Harus **lebih dari nol** | Kertas selebar 0mm tidak masuk akal — 0 pasti salah ketik |
| Margin (empat sisi) | Boleh **nol** | Margin 0 sah: cetak mentok ke tepi |

Tanpa pembedaan ini, salah satu dari dua hal buruk terjadi: margin 0 ditolak (padahal sah), atau
lebar 0 diterima (padahal pasti salah). Sistem lama memisahkannya dengan sadar, dan komentar di
kode menjelaskan alasannya. **Wajib dipertahankan.**

**Nilai bawaan itu sendiri tidak boleh diubah.** Angka-angkanya berasal dari tes cetak fisik pada
printer dot-matrix nyata, dan salah satunya — margin kanan — bukan margin visual melainkan batas
jangkauan print head. Rinciannya ada di
[shared-data-model.md](../shared/shared-data-model.md) §7.3.

Satu pola yang perlu diketahui: kedua checkbox kalibrasi menyimpan nilainya **hanya ketika berbeda
dari bawaan**. Tidak dicentang berarti field itu dihilangkan, bukan disimpan sebagai "tidak". Ini
konsisten dengan prinsip A-02 (kelompok kosong bukan kelompok) dan memungkinkan seluruh blok
kalibrasi dianggap "tidak diisi" bila memang tidak ada yang disesuaikan.

Checkbox kedua memakai **logika terbalik** — labelnya berbunyi negatif ("kertas sudah preprinted,
jangan cetak ulang") sementara datanya menyimpan sisi positif ("tampilkan kop"). Pembalikan itu
benar dari sisi bahasa (pengguna berpikir "kertas saya sudah ada kopnya"), tetapi perlu disadari
saat mereplikasi.

---

## A-04 — Normalisasi Nilai yang Tak Bertipe

**Hasil yang diharapkan:** nilai konfigurasi yang rusak, salah ketik, atau di luar rentang **tidak
pernah** membuat sistem gagal — ia jatuh ke nilai yang aman.

Karena konfigurasi disimpan sebagai gumpalan tanpa tipe (lihat
[data-model-legacy.md](data-model-legacy.md) §3.2), tidak ada yang menjamin bentuknya saat dibaca.
Sistem lama mengatasinya dengan **menormalkan di sisi pembaca**, bukan memvalidasi di sisi penulis.

Pilihan itu punya konsekuensi yang perlu dipahami: **aturan normalisasi yang sama harus ditulis di
setiap tempat yang membaca.** Untuk mode persetujuan koreksi stok, aturan itu ada di dua tempat
(halaman pengaturan dan modul Stock) dan kebetulan identik. Tidak ada yang menjamin keduanya tetap
sinkron bila salah satu diubah.

**Arah kegagalan yang perlu diperhatikan.** Untuk mode persetujuan, nilai apa pun yang tidak persis
`strict` menjadi `simple` — yaitu mode yang **lebih longgar**. Sehingga salah ketik melemahkan
penjagaan, bukan memperketatnya. Untuk kebijakan keamanan, arah kegagalan yang aman seharusnya
sebaliknya.

Untuk kuota harian, normalisasinya lebih lengkap (bukan angka → bawaan; di luar rentang → dipangkas;
pecahan → dibulatkan) tetapi **hanya ada di klien**. Permintaan langsung ke API dapat menyimpan
nilai negatif, dan modul yang membacanya harus menangani itu sendiri.

Hasil yang seharusnya dicapai di sistem baru: **satu tempat yang mendefinisikan bentuk dan nilai
sah setiap setelan**, dipakai baik saat menulis maupun membaca. Dengan begitu normalisasi tidak
perlu diulang, dan arah kegagalannya dapat dipilih secara sadar per setelan.

---

## A-05 — Penyebaran Perubahan Kebijakan

**Hasil yang diharapkan:** mengubah kebijakan berlaku **seketika** untuk seluruh perusahaan, tanpa
siapa pun perlu login ulang atau memuat ulang aplikasi.

Sistem lama mencapainya dengan cara yang sama seperti resolusi hak akses di modul 01: **baca ulang
dari sumber setiap kali dibutuhkan**, jangan simpan salinan.

Modul Stock membaca mode persetujuan **setiap kali** koreksi stok divalidasi. Tidak ada cache,
tidak ada nilai yang dibawa di sesi. Sehingga administrator dapat mengubah kebijakan dan koreksi
stok berikutnya — bahkan yang sedang dikerjakan orang lain — langsung mengikuti aturan baru.

Ini properti yang layak dipertahankan. Biayanya satu pembacaan per operasi, yang untuk skala UKM
tidak terasa.

**Kontras dengan konfigurasi yang dipakai frontend.** Profil dokumen cetak dan identitas perusahaan
**tidak** dibaca ulang — keduanya dimuat sekali sebagai bagian konteks sesi. Sehingga perubahan
dari sesi lain baru terlihat setelah aplikasi dimuat ulang.

Perbedaan itu masuk akal secara teknis (yang satu dibaca server, yang lain dipakai browser) tetapi
menghasilkan perilaku yang tidak seragam: satu setelan berlaku seketika, setelan lain di halaman
yang sama tidak. Di sistem baru, keduanya sebaiknya punya jaminan yang dinyatakan jelas.

**Satu konsekuensi keamanan yang perlu diputuskan.** Karena kebijakan persetujuan koreksi stok
dapat diubah dalam satu klik oleh pemegang izin kelola konfigurasi, seseorang dapat melonggarkan
kebijakan, menyetujui koreksinya sendiri, lalu mengembalikan kebijakan seperti semula. Ketiga
aksinya teraudit, tetapi tidak satu pun dicegah — dan audit hanya berguna bila ada yang membacanya.

---

## A-06 — Konfigurasi Tersedia Sebelum Halaman Dirender

**Hasil yang diharapkan:** halaman yang membutuhkan konfigurasi (topbar, halaman cetak, halaman
pengaturan) tidak pernah menampilkan nilai kosong sesaat sebelum data tiba.

Sistem lama mencapainya dengan memuat konfigurasi **sebagai bagian konteks sesi**, bukan
mengambilnya saat dibutuhkan. Identitas perusahaan ikut disimpan bersama sesi di browser;
pengaturan dimuat saat bootstrap aplikasi.

Hasilnya tercapai — tidak ada kedipan. Tetapi cara mencapainya membawa dua konsekuensi:

**Konsekuensi 1 — dua endpoint baca menjadi tidak terpakai.** Halaman pengaturan mengisi formnya
dari data yang sudah ada di browser, sehingga endpoint untuk membaca profil dan pengaturan tidak
pernah dipanggil dari antarmuka.

**Konsekuensi 2 — nilai bisa basi.** Perubahan dari sesi lain tidak terlihat sampai aplikasi dimuat
ulang. Untuk sistem satu-perusahaan dengan sedikit administrator, risikonya kecil.

Ada satu perilaku yang **tidak** tercapai dengan baik: bila konfigurasi belum tersedia, halaman
merender **kosong sama sekali** — bukan indikator memuat, bukan pesan. Pengguna melihat layar
kosong di bawah topbar dan tidak tahu apakah halaman sedang memuat atau rusak.

Hasil yang seharusnya dicapai: keadaan "sedang memuat" harus **terlihat sebagai sedang memuat**.
Ini masalah yang sama seperti splash pemulihan sesi di modul 01 — pola "render kosong sampai data
siap" dipakai berulang tanpa keadaan antara.

Di sistem baru, satu permintaan bootstrap yang mengembalikan **seluruh** konteks sesi (identitas
pengguna, perusahaan, hak akses, dan konfigurasi) akan menyelesaikan keduanya sekaligus: tidak ada
penggabungan dari dua sumber, dan satu titik yang jelas untuk menyatakan "sedang memuat".

---

## A-07 — Mode Baca-Saja

**Hasil yang diharapkan:** pemegang izin baca dapat **melihat** seluruh konfigurasi tanpa bisa
mengubahnya, dan ia **tahu** bahwa ia sedang dalam mode baca.

Sistem lama mencapainya dengan tiga lapis sekaligus:

| Lapis | Wujudnya |
|---|---|
| Tampilan | Seluruh kendali dinonaktifkan; tombol tambah/hapus/simpan **tidak dirender** |
| Penjelasan | Teks berubah menjadi "Anda sedang melihat pengaturan dalam mode baca saja." |
| Perilaku | Penyimpanan diabaikan bahkan bila dipicu lewat cara lain |

Yang membuat ini baik: pengguna **tidak dibiarkan menebak**. Field yang tampak bisa diedit tetapi
gagal saat disimpan adalah pengalaman yang jauh lebih buruk daripada field yang jelas-jelas
dinonaktifkan disertai penjelasan.

Pola ini layak dipertahankan dan **layak ditiru di modul lain** — modul 02, misalnya, tidak punya
mode baca-saja yang setara.

**Satu ketidakseragaman yang perlu diputuskan.** Halaman Profil Perusahaan menuntut izin **baca**
untuk dibuka, sedangkan halaman Status Order menuntut izin **kelola** — meski separuh halaman itu
hanya menampilkan daftar. Akibatnya pemegang izin baca dapat melihat konfigurasi perusahaan tetapi
tidak dapat melihat daftar status order sama sekali.

Perbedaan itu mungkin disengaja (halaman Status Order memang tidak punya mode baca-saja, jadi
membukanya tanpa izin kelola akan menyesatkan), tetapi hasilnya tidak konsisten dari sudut pandang
pengguna.

---

## A-08 — Pencatatan Perubahan Konfigurasi

**Hasil yang diharapkan:** setiap perubahan kebijakan dapat ditelusuri sepenuhnya — siapa,
kapan, dari nilai apa, ke nilai apa.

**Ini logika audit terbaik di seluruh modul yang sudah dianalisis**, dan pola yang seharusnya
dijadikan acuan.

Yang dicatat untuk perubahan profil perusahaan: **keenam field identitas**, di sisi sebelum
**dan** sesudah. Untuk perubahan pengaturan: **kelima kelompok konfigurasi**, keduanya juga.

Bandingkan dengan modul 02, yang mencatat lima field di sisi "sebelum" tetapi hanya satu di sisi
"sesudah" — sehingga hasil perubahannya tidak dapat dibaca dari audit.

Hasil yang dicapai di sini: pertanyaan pengawasan yang nyata **dapat dijawab** — "siapa
melonggarkan kebijakan persetujuan koreksi stok, kapan, dan dari mode apa" terjawab sepenuhnya dari
satu entri audit.

Ini penting justru karena A-05 tidak mencegah pelonggaran kebijakan. Bila pencegahan tidak ada,
penelusuran menjadi satu-satunya pengaman — dan di sini penelusuran itu lengkap.

**Satu kesalahan yang perlu diperbaiki.** Pencatatan perubahan feature flag menyimpan identitas
**perusahaan** alih-alih identitas baris flag. Sehingga seluruh entri audit flag menunjuk entitas
yang sama, dan riwayat satu flag tertentu tidak dapat disaring. Untuk pengaturan, penyimpanan
identitas perusahaan justru **benar** karena kunci utama tabelnya memang itu — kesalahannya khusus
pada flag.

Kesalahan ini tidak berdampak sekarang (tidak ada yang memakai flag), tetapi akan berdampak bila
sistem flag dihidupkan.

**Satu penyimpangan konvensi.** Ketiga penanda aksi memakai tiga segmen
(`company.profile.update`), sementara konvensi proyek adalah dua. Karena penanda ini dipakai
sebagai penyaring di halaman Riwayat Aktivitas, penyimpangan itu memengaruhi cara pencarian audit
ditulis. Penyimpangan serupa juga ada di modul 02.

---

## 2. Ringkasan: Yang Harus Dicapai vs Yang Bebas Diubah

| Aspek | Status |
|---|---|
| Mengubah satu setelan tidak menghapus setelan lain | **Wajib dicapai** — tapi caranya harus dipindah ke lapisan penyimpanan |
| Membedakan "kosong" dari "tidak ada" pada profil dokumen | **Wajib dipertahankan** — syarat agar fallback bekerja |
| Baris telepon/rekening tanpa nomor dibuang | **Wajib dipertahankan** — tapi sebaiknya diberitahukan |
| Kotak kalibrasi kosong = pakai bawaan, ditampilkan sebagai placeholder | **Wajib dipertahankan** |
| Ukuran kertas wajib > 0, margin boleh 0 | **Wajib dipertahankan** — pembedaan yang disengaja |
| Nilai bawaan kalibrasi kertas | **Wajib dipertahankan persis** — hasil tes cetak fisik |
| Kebijakan persetujuan berlaku seketika tanpa login ulang | **Wajib dipertahankan** |
| Mode baca-saja tiga lapis (nonaktif + penjelasan + abaikan) | **Wajib dipertahankan**, dan layak ditiru modul lain |
| Audit mencatat seluruh isi sebelum & sesudah | **Wajib dipertahankan** — jadikan acuan |
| Mata uang selalu jadi huruf besar | Bebas — tapi hasilnya harus konsisten |
| Logika terbalik checkbox preprinted | Bebas — asal label tetap berbunyi dari sudut pandang pengguna |
| Konfigurasi dimuat sebagai konteks sesi | Bebas dirancang ulang — sebaiknya satu bootstrap, bukan gabungan dua sumber |
| Normalisasi ditulis di setiap pembaca | **Harus diperbaiki** — satu definisi bentuk & nilai sah |
| Arah kegagalan normalisasi mode persetujuan (salah ketik → longgar) | **Harus diperbaiki** — kegagalan sebaiknya ke arah aman |
| Halaman merender kosong sebelum data siap | **Harus diperbaiki** — keadaan memuat harus terlihat |
| Identitas salah pada audit feature flag | **Harus diperbaiki** — bila sistem flag dihidupkan |
| Empat field perusahaan tanpa pembaca | **Harus diputuskan** — tiga sudah diputuskan dibuang |
| Dua kelompok pengaturan tanpa pembaca | **Harus diputuskan** — buang atau hidupkan |
| Seluruh sistem feature flag tanpa pembaca | **Harus diputuskan** |
| Kebijakan approval dapat dilonggarkan tanpa penjagaan | **Perlu diputuskan** — sekarang hanya teraudit, tidak dicegah |
| Perbedaan izin antara dua halaman pengaturan | **Perlu diputuskan** — sekarang tidak seragam |
