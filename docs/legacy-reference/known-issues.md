# Known Issues — Perilaku Aneh yang Butuh Konfirmasi

Berkas ini mengumpulkan **perilaku yang tampak seperti bug tapi mungkin sudah jadi kebiasaan
user**, dari seluruh modul. Semuanya **butuh konfirmasi pemilik sistem** sebelum diputuskan
apakah diperbaiki atau direplikasi apa adanya di sistem baru.

Aturan pengisian: satu temuan = satu baris ber-ID. Jangan hapus baris yang sudah dikonfirmasi —
ubah statusnya, supaya keputusannya tetap terlacak.

| Status | Arti |
|---|---|
| ⬜ | Belum dikonfirmasi |
| ✅ PERBAIKI | Dikonfirmasi sebagai bug — perbaiki di sistem baru |
| 🔁 REPLIKASI | Dikonfirmasi sebagai kebiasaan — pertahankan apa adanya |
| 🟡 SEBAGIAN | Perbaiki tapi dengan syarat (dicatat di kolom keputusan) |

---

## Modul 01 — Auth & Session

Sumber: [01-auth-session/](01-auth-session/). Analisis lengkap ada di berkas modul tersebut.

### KI-01 — User tanpa cabang aktif masuk ke jalan buntu ✅

**Tingkat: tinggi — user tidak bisa keluar tanpa bantuan teknis.**

Bila seorang user tidak punya akses cabang sama sekali (atau semua cabangnya berstatus non-aktif),
tiga perilaku bergabung menjadi jalan buntu:

1. Login **berhasil** — tapi `requires_branch_selection` bernilai `false`, padahal tidak ada
   cabang yang terpilih. Cabang "0" tidak diperlakukan sebagai kondisi yang perlu ditanyakan;
   hanya kondisi ">1 tanpa default" yang memicu penandaan itu.
2. Penjaga rute melihat tidak ada cabang aktif → mengalihkan user ke halaman pemilihan cabang.
3. Halaman pemilihan cabang menampilkan daftar **kosong**, dan **tidak punya tombol logout**.
   Menu avatar (satu-satunya tempat lain yang punya tombol logout di desktop) **tidak dirender
   sama sekali** tanpa cabang aktif.

Hasilnya: user terkunci di halaman kosong. Satu-satunya jalan keluar adalah menghapus data situs
di browser secara manual.

Deskripsi di halaman itu juga menyesatkan — teksnya berbunyi
`{Nama Perusahaan} has one branch available for your account.` bahkan ketika jumlah cabangnya
**nol**, karena kondisinya hanya memeriksa keberadaan perusahaan, bukan panjang daftar.

**Yang perlu dikonfirmasi:** apakah kondisi ini pernah benar-benar terjadi di production? Bila
user selalu dibuatkan akses cabang bersamaan dengan akunnya, situasi ini mungkin belum pernah
muncul — tapi tetap perlu diputuskan penanganannya (tolak login dengan pesan jelas, atau halaman
yang menjelaskan situasi + tombol logout).

---

### KI-02 — Ganti ke cabang non-aktif menghasilkan error 500, bukan pesan bisnis ✅

**Tingkat: sedang — pesan menyesatkan, tapi jalur keluarnya masih ada.**

Bila user punya akses ke sebuah cabang tapi cabang itu sudah berstatus non-aktif, permintaan ganti
cabang gagal sebagai **kesalahan internal server (500)** tanpa pesan penjelasan, alih-alih
penolakan bisnis (403) berpesan jelas.

Penyebabnya: pemeriksaan cabang memakai pencarian bergaya "wajib ada" yang melempar galat teknis,
bukan galat bisnis, sehingga tidak dikenali sebagai penolakan yang bisa dijelaskan.

Yang dilihat user: toast **"Gagal mengganti cabang"** / "Terjadi kesalahan saat memilih cabang." —
terdengar seperti gangguan sistem, padahal penyebabnya jelas dan bisa disampaikan ("Cabang X sudah
tidak aktif").

Catatan: kondisi ini hanya bisa terjadi bila cabang dinonaktifkan **sementara** akses user ke
cabang itu masih ada. Halaman pemilihan cabang sendiri sudah menyaring cabang non-aktif dari
daftar, jadi jalur pemicunya adalah data yang tidak sinkron atau permintaan langsung ke API.

**Yang perlu dikonfirmasi:** boleh diperbaiki menjadi 403 berpesan jelas? Ini mengubah kode status
yang dikembalikan API, jadi bila ada integrasi lain yang bergantung padanya perlu diketahui lebih
dulu.

---

### KI-03 — Aplikasi bisa gagal total (layar putih) bila browser memblokir penyimpanan lokal ✅

**Tingkat: sedang — hanya menimpa sebagian pengguna, tapi gejalanya membingungkan.**

Pembacaan token dari penyimpanan browser saat aplikasi dimuat **tidak dilindungi penanganan
galat**. Pada browser yang memblokir data situs (mode privat tertentu, pengaturan privasi ketat,
kebijakan perangkat), pembacaan itu melempar galat pada tahap paling awal — sebelum halaman apa
pun tampil.

Diperparah oleh: **tidak ada satu pun batas penangkap galat (error boundary) di seluruh
frontend**, sehingga hasilnya **layar putih tanpa pesan**, bukan halaman error yang menjelaskan.

Yang membuat ini tampak seperti kelalaian, bukan keputusan: dua pembaca penyimpanan lain di
aplikasi yang sama (ringkasan sesi dan preferensi sidebar) **sudah** dilindungi dengan benar,
salah satunya bahkan disertai komentar eksplisit tentang pengaturan privasi browser. Jadi polanya
sudah diketahui penulisnya — hanya jalur token yang terlewat.

**Yang perlu dikonfirmasi:** apakah pernah ada laporan "aplikasi tidak mau terbuka / layar putih"
dari pengguna tertentu? Bila ya, ini kandidat penyebabnya. Perbaikannya kecil (bungkus akses
penyimpanan + tambahkan error boundary) dan tidak mengubah perilaku yang terlihat.

---

### KI-04 — Pesan error login menampilkan kode mesin, bukan pesan Indonesia ✅

**Tingkat: sedang — sangat sering terlihat user, tapi tidak menghalangi pekerjaan.**

Saat login gagal, server sudah menyiapkan pesan berbahasa Indonesia yang ramah
("Kredensial tidak valid", "User tidak memiliki role"), tetapi frontend menampilkan **kode
mesinnya**:

| Penyebab | Yang tampil di kotak error | Yang seharusnya |
|---|---|---|
| Password salah / user tidak ada / akun non-aktif | `unauthorized` | "Kredensial tidak valid" |
| Akun tanpa role | `forbidden` | "User tidak memiliki role" |

Penyebabnya: pemilihan pesan mengambil kode mesin lebih dulu, dan karena kode itu **selalu
terisi**, pesan ramah di belakangnya tidak pernah terpakai. Pesan cadangan yang sudah disiapkan
("Login gagal. Periksa username dan password.") juga praktis tidak pernah muncul.

Kenapa ini tidak pernah ketahuan: test E2E hanya memeriksa bahwa judul kotak error
("Authentication Error") **terlihat**, tanpa memeriksa isinya.

**Yang perlu dikonfirmasi:** apakah pengguna sudah terbiasa melihat `unauthorized` dan
mengenalinya sebagai "password salah"? Bila iya, mengubahnya tetap disarankan (tidak ada yang
kehilangan informasi), tapi Anda mungkin ingin memberitahu pengguna lebih dulu.

---

### KI-05 — Role aktif user multi-role tidak dapat diprediksi, dan laporan role bisa tidak sinkron ✅

**Tingkat: rendah untuk kondisi sekarang, tinggi bila user multi-role dipakai.**

Dua perilaku terkait:

**(a) Role awal saat login tidak deterministik.** Sistem mengambil role **pertama** dari data user
tanpa aturan urutan apa pun, dan tidak ada penanda "role utama" tersimpan. Untuk user dengan satu
role hasilnya selalu benar. Untuk user dengan beberapa role, role mana yang aktif setelah login
**tidak dapat dipastikan dari kode** — bergantung urutan yang dikembalikan basis data.

**(b) Laporan role bisa berbeda dari hak yang berlaku.** Bila sebuah role dicabut dari user
sementara sesinya masih hidup, permintaan identitas melaporkan role **lain** (jatuh ke role
pertama), sementara hak akses yang benar-benar berlaku tetap dihitung dari role yang tercatat di
sesi. Nama role yang ditampilkan di antarmuka bisa tidak cocok dengan apa yang sebenarnya boleh
dilakukan user.

**Kondisi sekarang:** kelima akun bawaan masing-masing hanya punya **satu** role, sehingga kedua
perilaku ini tidak pernah muncul.

**Yang perlu dikonfirmasi:** apakah ada akun nyata di production yang memegang **lebih dari satu**
role? Bila tidak ada, seluruh fitur "Ganti Role" praktis tidak terpakai dan perlu diputuskan
apakah tetap dibawa ke sistem baru.

---

### KI-06 — Masa berlaku token dan masa berlaku sesi bisa berbeda ✅

**Tingkat: rendah — hanya muncul bila konfigurasi diubah.**

Penghitung masa berlaku milik sistem hanya mengenali format satuan tunggal sederhana (mis. `15m`,
`7d`, `24h`). Bila nilai konfigurasi diisi format lain yang **valid bagi pustaka token** tapi
tidak dikenali penghitung ini — contoh paling nyata `1w` (satu minggu) — hasilnya:

- Masa berlaku **token** mengikuti nilai konfigurasi (1 minggu)
- Masa berlaku **baris sesi** jatuh ke nilai cadangan 7 hari

Untuk `1w` keduanya kebetulan sama. Untuk nilai seperti `2w` atau `30d12h`, sesi akan mati lebih
dulu daripada tokennya — user terputus lebih cepat dari yang dikonfigurasi, tanpa peringatan
apa pun di log.

**Yang perlu dikonfirmasi:** apakah nilai konfigurasi masa berlaku di production memakai format
sederhana (`15m` / `7d`)? Bila ya, isu ini tidak pernah aktif dan cukup dicegah di sistem baru
dengan validasi saat boot.

---

### KI-07 — Menu "Profil Saya" tidak melakukan apa pun ✅

**Tingkat: rendah — mengganggu, tidak merusak.**

Item **"Profil Saya"** di menu avatar hanya menutup dropdown. Tidak ada navigasi, dan **tidak ada
halaman profil user** di seluruh aplikasi (rute `/profile` tidak pernah didefinisikan). Endpoint
`company/profile/*` yang ada adalah profil **perusahaan**, bukan profil user.

**Yang perlu dikonfirmasi:** hapus dari menu, atau memang direncanakan ada halaman profil user
(ganti password sendiri, ubah nama, dll.)? Bila direncanakan, itu **fitur baru** dan perlu masuk
lingkup terpisah — bukan bagian replikasi.

---

### KI-08 — Label "Cabang Aktif" di topbar menampilkan judul halaman ✅

**Tingkat: rendah — salah label, sudah dilihat setiap hari.**

Blok kiri topbar menampilkan label kecil **"Cabang Aktif"** dengan nilai tebal di bawahnya. Nilai
itu adalah **judul halaman yang sedang dibuka** (mis. "Daftar Order", "Dashboard"), bukan nama
cabang.

Nama cabang yang sebenarnya tampil di tempat lain: chip **"Ganti Cabang"** / **"Cabang"** di
kanan topbar, dan sebagai badge kode cabang di dalam menu avatar.

Karena label dan nilainya berdampingan setiap hari, kemungkinan besar pengguna sudah
mengabaikannya atau membacanya sebagai judul halaman.

**Yang perlu dikonfirmasi:** ubah labelnya menjadi "Halaman" / hapus labelnya, atau ganti nilainya
menjadi nama cabang? Keduanya mengubah tampilan yang sudah dikenal, jadi perlu keputusan Anda.

---

### KI-09 — Layar "Memuat sesi..." menampilkan teks putih di latar terang ✅

**Tingkat: rendah — hanya tampak sekejap.**

Layar tunggu saat memulihkan sesi memakai warna latar dari variabel gaya dengan nilai cadangan
gelap. Karena variabel itu **memang terdefinisi** dan bernilai **terang**, yang dipakai adalah
nilai terang — sementara teksnya berwarna putih dengan transparansi 70%.

Hasilnya teks **"Memuat sesi..."** hampir tidak terbaca. Nilai cadangan gelap yang sepertinya jadi
niat awal desain tidak pernah terpakai.

Layar ini hanya tampil sekejap saat memuat aplikasi, jadi dampaknya kecil — tapi pada koneksi
lambat ia bisa bertahan beberapa detik dan terlihat seperti halaman kosong.

**Yang perlu dikonfirmasi:** boleh diperbaiki (samakan dengan latar aplikasi + teks gelap)? Tidak
ada risiko fungsional.

---

### KI-10 — Daftar cabang tidak konsisten antara login dan pemulihan sesi ✅

**Tingkat: rendah — belum terbukti berdampak.**

Dua jalur mengembalikan daftar cabang user dengan aturan berbeda:

| Jalur | Yang dikembalikan |
|---|---|
| Saat login | **Hanya cabang berstatus aktif** |
| Saat memulihkan sesi (reload halaman) | **Semua cabang**, termasuk yang non-aktif |

Konsekuensi yang mungkin: setelah menyegarkan halaman, halaman pemilihan cabang dapat menampilkan
cabang yang sudah ditutup. Bila user memilihnya, hasilnya adalah error 500 dari KI-02.

Belum ada bukti ini pernah terjadi — ia menuntut adanya cabang non-aktif yang aksesnya masih
melekat pada user.

**Yang perlu dikonfirmasi:** apakah ada cabang berstatus non-aktif di data production? Bila belum
pernah ada cabang yang ditutup, KI-02 dan KI-10 keduanya belum pernah aktif.

---

### KI-11 — Tidak ada jejak percobaan login yang gagal ✅

**Tingkat: sedang — bukan bug, tapi keterbatasan yang perlu diketahui.**

Percobaan login yang gagal **tidak menghasilkan catatan apa pun**: tidak ada baris data, tidak ada
entri audit (modul auth memang dikecualikan dari audit log), dan tidak ada log terstruktur.

Ditambah tidak adanya pembatasan laju dan penguncian akun, pertanyaan berikut tidak bisa dijawab
oleh sistem:

- Berapa kali akun tertentu gagal login, dan kapan?
- Apakah ada percobaan menebak password dari satu alamat IP?
- Apakah akun yang dilaporkan "tidak bisa masuk" benar-benar mencoba, atau tidak pernah sampai ke
  server?

Ini tercatat di sini karena **konsekuensinya operasional**, bukan karena ada kode yang salah.

**Yang perlu dikonfirmasi:** apakah kemampuan ini dibutuhkan di sistem baru? Menambahkannya adalah
**fitur baru** (bukan replikasi), jadi perlu masuk lingkup secara sadar.

---

### KI-12 — Sesi tidak punya batas umur absolut 🟡

**Tingkat: perlu keputusan — bukan bug.**
**Keputusan analis:** Pertahankan sesi panjang untuk operasional kasir, tetapi batasi dengan umur absolut (mis. wajib login ulang tiap 30 hari) plus revoke oleh admin.

Setiap kali token diperpanjang, masa berlaku sesi **direset penuh**. Karena token akses hanya
berumur 15 menit, perpanjangan terjadi rutin sepanjang pemakaian. Hasilnya: **sesi yang dipakai
minimal sekali setiap 7 hari dapat hidup selamanya** — tidak ada titik di mana user dipaksa login
ulang.

Untuk operasional harian ini menguntungkan: kasir tidak pernah terputus di tengah transaksi.
Risikonya: perangkat yang hilang atau dipinjam tetap punya sesi hidup tanpa batas waktu, dan
satu-satunya cara memutusnya adalah menonaktifkan akun atau menjalankan seed ulang.

**Yang perlu dikonfirmasi:** apakah perlu batas umur absolut di sistem baru (mis. wajib login
ulang setiap 30 hari apa pun aktivitasnya)? Ini mengubah perilaku yang dirasakan pengguna, jadi
keputusannya milik Anda.

---

## Modul 02 — Users, Roles & Permissions

Sumber: [02-users-roles-permissions/](02-users-roles-permissions/).

### KI-13 — Pengguna bisa kehilangan seluruh role dan terkunci dari sistem ✅

**Tingkat: tinggi — satu penyimpanan tak sengaja dapat mengunci orang dari sistem.**

Penetapan role selalu "ganti total": semua role pengguna dihapus, lalu diisi ulang dari daftar
yang dikirim. Ada **dua jalur** yang berujung pengguna tanpa role, dan keduanya tidak dijaga:

1. **Mengosongkan semua centang role** di form pengguna → daftar kosong dikirim → semua role
   dihapus, tidak ada yang disisipkan.
2. **Kode role yang tidak dikenal** dikirim → semua role dihapus, lalu penyisipan **dilewati
   tanpa error dan tanpa peringatan**.

Akibatnya baru terasa nanti: pengguna itu masih bisa memakai sesi yang sedang berjalan, tetapi
begitu ia logout atau sesinya berakhir, **login menolaknya** dengan "User tidak memiliki role"
(403). Pemulihan hanya bisa dilakukan admin lain lewat UI, atau langsung di basis data.

Ironisnya sistem **sudah** menjaga kondisi yang sama dari arah lain: menghapus role yang masih
dipakai pengguna ditolak dengan pesan jelas. Jadi pintu belakang dijaga, pintu depan terbuka.

Perbandingan yang menunjukkan ini kelalaian, bukan keputusan: untuk **permission**, kode yang tidak
dikenal justru **ditolak dengan pesan spesifik sebelum apa pun dihapus**. Masalah yang sama
ditangani benar di satu tempat dan salah di tempat lain yang berdekatan.

Hal yang sama berlaku untuk cabang: mengosongkan semua centang cabang membuat pengguna bisa login
tetapi terjebak tanpa ruang kerja (bergandengan dengan KI-01).

**Yang perlu dikonfirmasi:** apakah kondisi ini pernah terjadi di production — ada pengguna yang
melapor tidak bisa login padahal akunnya aktif?

---

### KI-14 — Pengguna di luar 100 data pertama tidak bisa diedit ✅

**Tingkat: tinggi bila jumlah pengguna melebihi 100.**

Form edit pengguna **tidak** mengambil data dari server. Ia mencari pengguna di daftar yang sudah
dimuat ke store, dan daftar itu dimuat dengan batas **100 baris**.

Bila pengguna yang diedit tidak ada di 100 baris pertama (urut nama A→Z), form terbuka dalam
**mode tambah** — judul "Tambah Pengguna", seluruh field kosong, field password wajib — **tanpa
peringatan apa pun**. Admin yang tidak memperhatikan judul dapat mengisi ulang dan justru
**membuat pengguna baru**.

Halaman daftar sendiri berpaginasi 20 per halaman dan bisa menampilkan pengguna ke-150, sehingga
tombol "Detail" pada baris itu mengantar ke form yang salah mode.

**Yang perlu dikonfirmasi:** berapa jumlah pengguna di production? Bila di bawah 100, isu ini
belum pernah aktif — tetapi akan aktif tanpa peringatan begitu ambang itu terlewati.

---

### KI-15 — Pemegang hak kelola role dapat menaikkan hak dirinya sendiri ✅

**Tingkat: tinggi — melewati pembagian tugas yang disengaja.**

Izin role **sistem** dapat diubah oleh siapa pun yang memegang `role.manage`, dan role **admin**
bawaan **memegang izin itu**. Sementara `admin` sengaja **tidak** diberi izin mengelola akun
(`user.create`/`update`/`archive`).

Akibatnya seorang admin dapat: membuka Role & Akses → menekan "Atur" pada role **Admin** →
mencentang "Pengguna & Tim: Tambah" → dan seketika memperoleh izin yang sengaja tidak diberikan
kepadanya. Perubahan izin berlaku langsung tanpa perlu login ulang.

Pembagian tugas "admin mengatur struktur akses, owner mengelola akun orang" sudah Anda konfirmasi
**disengaja**. Temuan ini bukan menggugat keputusan itu — melainkan mencatat bahwa pembagian
tersebut saat ini **hanya ditegakkan oleh konvensi, bukan oleh sistem**.

Satu-satunya role yang benar-benar terlindungi izinnya adalah `superadmin` (lewat mekanisme
penyembunyian, bukan mekanisme perlindungan role sistem).

Cara menjaga maksud pembagian tugas secara teknis: **larang memberikan izin yang tidak dimiliki
aktor**, atau kunci sekelompok izin tertentu (`user.*`, `role.manage`, `finance.manage`) hanya
untuk `owner`.

**Yang perlu dikonfirmasi:** apakah ini perlu ditutup di sistem baru, atau kepercayaan pada
pemegang role admin memang dianggap cukup?

---

### KI-16 — Menyimpan profil dan password bukan satu operasi ✅

**Tingkat: sedang — bisa menyimpan sebagian tanpa penjelasan.**

Saat mengedit pengguna sambil mengisi password baru, antarmuka mengirim **dua permintaan
berurutan**: menyimpan profil, lalu mengganti password.

Bila yang pertama berhasil dan yang kedua gagal, hasilnya: **profil tersimpan, password tidak** —
tanpa pembatalan, dan pesan yang muncul hanya "Gagal menyimpan pengguna" tanpa menyebut bagian
mana yang gagal. Admin cenderung menyimpulkan seluruh penyimpanan gagal, lalu mengulang — dan
mendapati perubahan profilnya sudah tersimpan.

Kegagalan permintaan kedua bukan hipotetis: password superadmin yang diubah dari sesi
non-superadmin ditolak (404), dan pengguna yang dinonaktifkan bersamaan juga bisa memicu penolakan.

**Yang perlu dikonfirmasi:** apakah pernah ada laporan "password tidak berubah padahal katanya
gagal"? Perbaikannya bisa sederhana — satu endpoint yang menerima keduanya, atau pesan yang
menyebut bagian mana yang berhasil.

---

### KI-17 — Pesan error spesifik tidak pernah sampai ke pengguna ✅

**Tingkat: sedang — sangat sering terlihat, menghambat penyelesaian mandiri.**

Server sudah menyiapkan pesan berbahasa Indonesia yang spesifik, tetapi antarmuka membuangnya
lewat **dua pola berbeda**:

**Pola A — aksi pengguna:** pesan server diabaikan sepenuhnya; toast hanya menampilkan judul.

| Penyebab nyata | Yang dilihat admin |
|---|---|
| "Username sudah digunakan" | **"Gagal menyimpan pengguna"** (tanpa alasan) |
| "Email sudah digunakan" | **"Gagal menyimpan pengguna"** (tanpa alasan) |
| "Password awal minimal 8 karakter" | **"Gagal menyimpan pengguna"** (tanpa alasan) |

Ketiga penyebab yang sangat berbeda tampil **identik**. Admin tidak punya petunjuk apa yang harus
diperbaiki.

**Pola B — aksi role:** toast menampilkan **kode mesin** sebagai penjelasan.

| Penyebab nyata | Deskripsi toast |
|---|---|
| "Kode role 'kepala_gudang' sudah digunakan" | `error` |
| "Kode role tidak valid" | `error` |
| "Role masih dipakai oleh user dan tidak dapat dihapus" | `error` |
| "Permission tidak dikenal: xxx" | `validation_failed` |

Ada penyebab tambahan di lapisan tengah untuk pola B: galat konflik data memakai status HTTP yang
**tidak dipetakan** pembungkus respons, sehingga jatuh ke kategori generik. Jadi perbaikannya
menyentuh dua tempat.

Ini pola yang **sama** dengan KI-04 di modul 01 — artinya sistemik, bukan kelalaian satu tempat.
Memperbaikinya di lapisan bersama menyelesaikannya untuk semua modul.

**Yang perlu dikonfirmasi:** apakah admin sudah terbiasa menebak penyebabnya? Perbaikan ini tidak
menghilangkan informasi apa pun, jadi risikonya rendah.

---

### KI-18 — Seed memunculkan kembali izin yang sudah dihapus dari role sistem ✅

**Tingkat: sedang — dapat membatalkan konfigurasi hak yang sudah disesuaikan.**

Penetapan izin oleh seed bersifat **menambah saja** ("sisipkan bila belum ada") dan tidak pernah
menghapus. Sehingga izin yang **sengaja dihapus** dari role sistem lewat halaman Role & Akses akan
**muncul kembali** setiap kali seed dijalankan.

Contoh yang mungkin terjadi: perusahaan mencabut "Keuangan - Jurnal: Balikkan Jurnal" dari role
Admin sebagai kebijakan internal. Setelah seed dijalankan (mis. saat deploy atau pemulihan), izin
itu kembali — tanpa peringatan dan tanpa entri riwayat aktivitas, karena seed tidak menulis audit.

Seed juga menimpa password kelima akun bawaan ke nilai bawaan dan mencabut semua sesinya.

**Yang perlu dikonfirmasi:** apakah seed pernah dijalankan di production, atau hanya di lingkungan
dev/test? Bila hanya dev, isu ini tidak berdampak — tetapi tetap perlu penjagaan agar tidak
terjadi kelak.

---

### KI-19 — Admin dapat menonaktifkan akunnya sendiri ✅

**Tingkat: sedang — dapat mengunci pemegang hak terakhir.**

Tidak ada penjagaan yang mencegah seseorang menonaktifkan akunnya sendiri. Aksi itu berhasil, dan
karena penonaktifan **mencabut semua sesi** pengguna itu, admin langsung terputus dari semua
perangkat.

Bila ia satu-satunya pemegang izin `user.archive`/`user.update`, **tidak ada yang bisa
memulihkannya** selain akun superadmin developer atau intervensi langsung di basis data.

Tombolnya pun tidak punya dialog konfirmasi — satu klik pada baris sendiri langsung menjalankannya.

**Yang perlu dikonfirmasi:** boleh ditambahkan penjagaan "tidak dapat menonaktifkan akun sendiri"?
Ini fitur baru yang kecil, tapi mencegah kondisi yang sulit dipulihkan.

---

### KI-20 — Cabang default ditentukan urutan tak terlihat ✅

**Tingkat: sedang — memengaruhi pengalaman login pengguna tanpa terlihat siapa pun.**

Cabang yang dicentang pertama dalam urutan daftar menjadi **cabang default** pengguna — cabang
yang dipilih otomatis saat ia login. Tidak ada penanda di UI, tidak ada penjelasan, dan admin
tidak punya cara memilihnya secara sadar.

Urutannya mengikuti urutan cabang di daftar sistem, **bukan** urutan admin mencentang. Sehingga
mengubah kombinasi centang dapat memindahkan cabang default pengguna tanpa tanda apa pun.

Hasil akhirnya sendiri benar: tepat satu cabang selalu bertanda default, sehingga masalah
"beberapa cabang default" tidak dapat terjadi. Yang bermasalah adalah **cara mencapainya tidak
terlihat dan tidak terkendali**.

**Yang perlu dikonfirmasi:** apakah pernah ada keluhan "kenapa saya selalu masuk ke cabang X"?
Perbaikannya jelas — tambahkan pilihan eksplisit (mis. radio button) di samping daftar cabang.

---

### KI-21 — Hapus role tanpa dialog konfirmasi 🟡

**Tingkat: rendah — tapi operasinya permanen.**
**Keputusan analis:** Pertahankan hapus/nonaktifkan cepat, tetapi wajibkan dialog konfirmasi untuk hapus permanen dan nonaktifkan akun.

Menekan "Hapus" pada role kustom langsung menghapusnya, **tanpa dialog konfirmasi**. Penghapusan
ini adalah **hard delete** — baris role benar-benar hilang dari basis data, satu-satunya
penghapusan permanen di seluruh sistem.

Yang meredam dampaknya: role yang masih dipakai pengguna **tidak bisa** dihapus. Jadi yang
terhapus tanpa sengaja hanyalah role yang belum/tidak lagi dipakai. Jejaknya tetap ada di riwayat
aktivitas (kode dan nama role tercatat), tetapi izin-izinnya tidak — sehingga memulihkan role
berarti mengonfigurasi ulang dari nol.

Tombol "Nonaktifkan" pada daftar pengguna juga tanpa konfirmasi, meski dampaknya lebih besar
(memutus semua sesi pengguna).

**Yang perlu dikonfirmasi:** perlu dialog konfirmasi untuk keduanya, atau kecepatan tanpa dialog
memang disukai?

---

### KI-22 — Email kosong pada pengguna kedua menghasilkan error 500 ✅

**Tingkat: rendah — hanya bila email dibiarkan kosong.**

Jalur **tambah** pengguna menyimpan email kosong sebagai **string kosong**, sementara jalur
**edit** menormalkannya menjadi **null**. Karena kolom email ber-indeks unik dan basis data
memperlakukan string kosong sebagai nilai biasa (berbeda dari null yang boleh berulang), pengguna
**kedua** dengan email kosong gagal dengan **error 500** tanpa pesan.

Pemeriksaan keunikan email pun melewatkannya, karena pemeriksaan hanya berjalan bila nilai email
tidak kosong. Jadi tidak ada yang menangkap kondisi ini sebelum menyentuh basis data.

Yang meredam: form web menandai email sebagai **wajib**, sehingga kondisi ini tidak dapat dicapai
lewat UI — hanya lewat permintaan API langsung atau bila kelak field itu dijadikan opsional.

**Yang perlu dikonfirmasi:** apakah email pengguna memang selalu wajib? Bila iya, isu ini laten
saja. Bila ada rencana menjadikannya opsional, ini harus diperbaiki lebih dulu.

---

### KI-23 — Nama role berbeda ditolak sebagai duplikat tanpa penjelasan ✅

**Tingkat: rendah — membingungkan, tidak merusak.**

Kode role dibuat otomatis dari nama dengan menormalkan huruf besar/kecil dan mengganti karakter
non-alfanumerik. Sehingga nama yang **tampak berbeda** dapat menghasilkan kode yang sama:

| Nama pertama | Nama kedua | Kode sama |
|---|---|---|
| `Kepala Gudang` | `kepala-gudang` | `kepala_gudang` |
| `Sales & Marketing` | `Sales Marketing` | `sales_marketing` |

Yang kedua ditolak dengan pesan `Kode role '{kode}' sudah digunakan` — menyebut kode yang
**pengguna tidak pernah lihat**, karena form hanya punya field "Nama role". Dari sisi pengguna,
dua nama yang jelas berbeda ditolak sebagai sama tanpa alasan yang bisa dipahami.

Selain itu, huruf beraksen dan non-Latin **dibuang** alih-alih ditransliterasi: "Área Manager"
menjadi `rea_manager`, dan nama yang seluruhnya non-Latin menghasilkan kode kosong lalu ditolak
sebagai "Kode role tidak valid".

**Yang perlu dikonfirmasi:** apakah nama role di production selalu memakai huruf Latin biasa? Dan
apakah pesan penolakan boleh diperjelas dengan menyebut role mana yang sudah memakai identitas itu?

---

### KI-24 — Pencarian pengguna mengirim permintaan tiap ketikan ✅

**Tingkat: rendah — pemborosan, tidak merusak.**

Kotak "Cari pengguna" memicu permintaan API **setiap karakter** diketik, tanpa penundaan
(debounce). Mengetik "budi santoso" menghasilkan 12 permintaan berurutan.

Karena setiap permintaan juga menyalakan indikator memuat global, tampilan berkedip selama
mengetik. Respons yang datang tidak berurutan juga bisa membuat hasil sesaat tidak cocok dengan
kotak pencarian.

**Yang perlu dikonfirmasi:** boleh ditambahkan penundaan ~300ms? Tidak ada perubahan perilaku yang
terlihat selain berkurangnya kedipan.

---

### KI-25 — Pencarian pengguna tidak menemukan hasil bila kata dibalik ✅

**Tingkat: rendah — tapi berbeda dari kebiasaan di modul lain.**

Pencarian pengguna memakai **frasa utuh**, bukan per-kata. Sehingga:

| Data | Dicari | Hasil |
|---|---|---|
| `Budi Ahmad Santoso` | `budi santoso` | **tidak ditemukan** |
| `Budi Santoso` | `santoso budi` | **tidak ditemukan** |
| `Budi Santoso` | `budi` | ditemukan |

Modul lain (produk, mitra bisnis) memakai pencarian **per-kata** yang tahan urutan dan spasi
ganda, lewat helper bersama. Modul ini menulis pencariannya sendiri dan tidak memakai helper itu.

Konsekuensinya pengguna yang terbiasa mencari "santoso budi" di halaman produk akan bingung ketika
cara yang sama tidak bekerja di halaman pengguna.

**Yang perlu dikonfirmasi:** boleh diseragamkan memakai pencarian per-kata seperti modul lain?

---

### KI-26 — Batas baris tidak dijaga terhadap nilai tidak wajar ✅

**Tingkat: rendah — hanya lewat API langsung.**

Perhitungan batas baris pada daftar pengguna hanya memangkas nilai yang **terlalu besar**, tidak
menjaga yang terlalu kecil atau bukan angka:

| `limit` dikirim | Hasil |
|---|---|
| 500 | 100 (dipangkas — benar) |
| **0** | **daftar selalu kosong** |
| **-5** | query gagal |
| **`"abc"`** | query gagal → **500** |

Helper bersama yang dipakai modul lain sudah menangani ketiga kasus itu; modul ini menulis
perhitungannya sendiri. Perilaku identik ada di daftar riwayat aktivitas (modul 20).

Yang meredam: antarmuka selalu mengirim 20 atau 100, jadi kondisi ini hanya tercapai lewat
permintaan API langsung.

**Yang perlu dikonfirmasi:** cukup diseragamkan ke helper bersama di sistem baru? Tidak ada
perubahan perilaku yang terlihat pengguna.

---

### KI-27 — Operasi role dapat meninggalkan role tanpa izin ✅

**Tingkat: rendah kemungkinannya, sedang dampaknya.**

Dua operasi role dijalankan sebagai **beberapa langkah tanpa perlindungan transaksi**:

| Operasi | Langkah | Bila langkah kedua gagal |
|---|---|---|
| Hapus role | hapus izin → hapus role | Role tetap ada **tanpa izin apa pun** |
| Ubah izin role | hapus semua izin → sisipkan yang baru | Role kehilangan **seluruh** izin |

Seluruh operasi **pengguna** (tambah, edit, ubah status, ganti password) sudah dilindungi
transaksi — jadi ketiadaan perlindungan pada operasi role tampak sebagai kelalaian, bukan
keputusan.

Yang membuatnya sulit terdeteksi: kondisi akhir "role tanpa izin" **tidak dapat dibedakan** dari
role yang sengaja dikosongkan. Sehingga kegagalan seperti ini berlangsung tanpa disadari sampai
seseorang melaporkan menu yang hilang.

Diperparah oleh pola simpan-otomatis pada matriks izin: mengubah sepuluh izin berarti sepuluh
operasi berurutan, masing-masing menulis ulang seluruh daftar izin role.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat
di sini agar tidak terlewat saat rebuild.

---

## Modul 03 — Company & Settings

Sumber: [03-company-settings/](03-company-settings/).

### KI-28 — Zona waktu, mata uang, dan bahasa sistem tersimpan tapi tidak berpengaruh ✅

**Tingkat: sedang — pengguna wajar mengira setelan ini bekerja.**

Halaman Profil Perusahaan menampilkan tiga field yang bisa diubah — **Zona waktu**, **Mata uang**,
dan **Bahasa sistem** — lengkap dengan tanda wajib diisi. Ketiganya tersimpan ke basis data.

Tetapi **tidak ada satu pun bagian aplikasi yang membacanya.** Seluruh format tanggal dan angka
memakai `id-ID` yang ditulis tetap di kode, format uang memakai `IDR` tetap, dan zona waktu
laporan diambil dari variabel lingkungan. Mengubah zona waktu ke `Asia/Makassar` **tidak
menggeser satu pun tampilan waktu**.

Diperparah oleh ketiadaan validasi: `Mars/Olympus` diterima sebagai zona waktu, `xx-YY` diterima
sebagai bahasa sistem, dan `ABCDE` diterima sebagai mata uang. Karena nilainya tidak dibaca, nilai
salah pun tidak menimbulkan gejala apa pun — kesalahan tersimpan diam-diam.

Keputusan untuk **membuang** ketiga field ini di sistem baru sudah Anda ambil sebelumnya (locale
dan zona waktu menjadi konstanta aplikasi). Temuan ini mencatat wujud nyatanya di modul ini.

**Yang perlu dikonfirmasi:** apakah pernah ada yang mencoba mengubah salah satunya dan bingung
karena tidak terjadi apa-apa? Dan apakah **mata uang** juga ikut dibuang, atau disimpan untuk
kemungkinan penjualan lintas mata uang?

---

### KI-29 — Simpan sebagian berhasil tapi pengguna melihat konfirmasi sukses ✅

**Tingkat: sedang — menyesatkan, dan bagian yang gagal justru yang terbesar.**

Tombol **"Simpan Pengaturan"** menjalankan **dua** permintaan berurutan: menyimpan profil
perusahaan, lalu menyimpan pengaturan. Yang kedua dikirim **tanpa ditunggu**.

| Skenario | Yang terjadi |
|---|---|
| Keduanya berhasil | Toast sukses **"Profil perusahaan disimpan"** |
| Profil gagal | Pengaturan **tidak ikut dikirim**; hanya toast gagal |
| **Profil berhasil, pengaturan gagal** | **Toast sukses DAN toast gagal muncul bersamaan** |

Skenario ketiga adalah yang bermasalah: pengguna melihat konfirmasi keberhasilan, sementara bagian
yang gagal justru mencakup **hampir seluruh isi halaman** — mode persetujuan koreksi stok, kuota
AI, identitas dokumen cetak, dan kalibrasi kertas.

Diperparah oleh ketiadaan toast sukses untuk pengaturan itu sendiri. Bahkan ketika semuanya
berhasil, satu-satunya konfirmasi berbunyi "Profil perusahaan disimpan" — tidak menyebut
pengaturan sama sekali.

**Yang perlu dikonfirmasi:** apakah pernah ada laporan "identitas nota saya tidak berubah padahal
sudah disimpan"? Perbaikannya bisa sederhana: satu operasi untuk keduanya, atau satu pesan yang
menyebut bagian mana yang berhasil.

---

### KI-30 — Mengirim satu setelan dapat menghapus setelan lain di kelompok yang sama ✅

**Tingkat: sedang — belum pernah terjadi, tetapi tidak dijaga sistem.**

Penyimpanan pengaturan bersifat **ganti-total per kelompok**: kelompok yang dikirim menggantikan
isinya sepenuhnya. Tidak ada penggabungan per-setelan di sisi server.

Yang menyelamatkan dalam praktik adalah **disiplin setiap pemanggil** — halaman Pengaturan dan
test E2E modul Stock keduanya membaca isi lama, menyebarkannya, lalu menambahkan yang berubah.

Tetapi properti itu **tidak dijamin sistem**. Integrasi atau skrip apa pun yang mengirim satu
setelan saja akan menghapus sisanya. Contoh konkret: mengirim hanya
`{ stockAdjustmentApprovalMode: "simple" }` akan menghapus setelan lain di kelompok aturan
operasional, tanpa peringatan.

Belum ada bukti ini pernah terjadi — kedua pemanggil yang ada sudah benar.

**Yang perlu dikonfirmasi:** apakah ada skrip, integrasi, atau kebiasaan memanggil API pengaturan
di luar antarmuka? Bila tidak ada, isu ini laten saja — tetapi perbaikannya (memindahkan
penggabungan ke lapisan penyimpanan) murah dan menghilangkan seluruh kelas masalah.

---

### KI-31 — Menambah aturan perpindahan status tanpa memilih status menghasilkan error teknis ✅

**Tingkat: rendah — tapi pesannya tidak bisa dipahami.**

Pada halaman Status Order, form **"Tambah Aturan Perpindahan"** tidak memvalidasi apakah status
asal dan tujuan sudah dipilih. Tombolnya **tetap aktif** meski keduanya masih kosong.

Bila ditekan, nilai kosong dikonversi menjadi angka **`0`** lalu dikirim sebagai id status — dan
gagal di server sebagai kesalahan teknis. Yang muncul hanyalah toast
**"Gagal menyimpan transisi status"** tanpa penjelasan.

Bandingkan dengan form **"Tambah Status"** di halaman yang sama, yang justru dijaga dengan baik:
tombolnya dinonaktifkan bila field kosong, dan duplikat kode diperingatkan langsung saat mengetik.
Dua form berdampingan dengan tingkat penjagaan yang jauh berbeda.

Juga tidak dijaga: memilih status yang sama untuk "dari" dan "ke", serta menambahkan aturan yang
sudah ada.

**Yang perlu dikonfirmasi:** apakah pengguna pernah menemui ini? Perbaikannya sepele — nonaktifkan
tombol sampai kedua status dipilih.

---

### KI-32 — Riwayat perubahan feature flag mencatat identitas yang salah ✅

**Tingkat: rendah selama sistem flag mati, sedang bila dihidupkan.**

Pencatatan riwayat untuk perubahan feature flag menyimpan **identitas perusahaan** sebagai entitas
yang diubah, bukan identitas baris flag-nya. Padahal baris flag punya identitas sendiri.

Akibatnya seluruh entri riwayat flag menunjuk entitas yang sama, dan tidak mungkin menyaring
riwayat satu flag tertentu — kunci flag hanya tersimpan di dalam isi catatan, bukan di kolom yang
bisa disaring.

Untuk pengaturan perusahaan, menyimpan identitas perusahaan justru **benar**, karena kunci utama
tabel pengaturan memang identitas perusahaan. Kesalahannya khusus pada flag.

Tidak berdampak sekarang karena sistem flag tidak dipakai sama sekali (lihat KI-33).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat
agar tidak terlewat bila sistem flag dihidupkan.

---

### KI-33 — Seluruh sistem feature flag mati di tiga lapisan ✅

**Tingkat: perlu keputusan — bukan kerusakan, melainkan fitur yang tidak pernah selesai.**

Sistem feature flag ada secara lengkap di sisi data dan API, tetapi tidak berfungsi di satu lapisan
pun:

| Lapisan | Kondisi |
|---|---|
| Basis data | Tiga baris di-seed, semuanya **mati** |
| API | Dua endpoint ada dan berfungsi |
| Pemanggil API | **Nol** — tidak ada UI maupun kode yang memanggilnya |
| Pembaca nilai | **Nol** — tidak ada kode yang memeriksa apakah sebuah flag aktif |
| Jalur ke antarmuka | **Diblokir** — adapter selalu memaksa daftar flag menjadi kosong |

Tiga kunci yang di-seed — `whatsapp_assistant`, `knowledge_rag`, `multi_location_stock` — menamai
fitur yang **sudah berjalan** tanpa memeriksa flag ini sama sekali.

Dua kelemahan struktural bila sistem ini dihidupkan: **tidak ada daftar kunci yang sah** (salah
ketik menghasilkan flag baru alih-alih error), dan **tidak ada cara menghapus** flag yang
terlanjur dibuat.

Ini pola yang sama dengan prefix penyimpanan `vioni` dan field `guid`/`code`/`info` pada envelope
permintaan — fosil dari rancangan awal yang arahnya berubah.

**Yang perlu dikonfirmasi:** apakah feature flag pernah direncanakan dipakai, atau ini sisa
rancangan multi-tenant yang sama? Bila tidak dipakai, tabel dan endpointnya bisa dibuang di sistem
baru.

---

### KI-34 — Halaman pengaturan merender kosong sebelum data siap ✅

**Tingkat: rendah — tapi tampak seperti halaman rusak.**

Kedua halaman modul ini merender **kosong sama sekali** selama data konfigurasi belum tersedia —
bukan indikator memuat, bukan pesan, bukan kerangka tampilan.

Pengguna melihat area kosong di bawah topbar dan tidak punya cara membedakan "sedang memuat" dari
"halaman rusak". Pada koneksi lambat, keadaan itu bisa bertahan beberapa detik.

Ini pola yang sama dengan layar pemulihan sesi di modul 01 (KI-09): keadaan "sedang memuat"
dipakai berulang tanpa tampilan antara yang jelas.

**Yang perlu dikonfirmasi:** boleh ditambahkan indikator memuat? Tidak ada risiko fungsional.

---

### KI-35 — Status order baru selalu lahir dengan sifat tetap yang tidak bisa diubah ✅

**Tingkat: sedang — membatasi kegunaan fitur.**

Halaman Status Order hanya bisa **menambah** status. Setiap status baru dibuat dengan sifat tetap
yang tidak dapat dipilih pengguna:

| Sifat | Nilai tetap |
|---|---|
| Kelompok status | selalu **"Menunggu"** |
| Titik awal / titik akhir | selalu **bukan keduanya** |
| Warna | selalu **`#12343B`** |
| Urutan tampil | jumlah status saat ini **+ 1** |

Dan **tidak ada fungsi ubah maupun hapus** di halaman ini — padahal API modul Order menyediakan
endpoint untuk mengubah. Sehingga status yang terlanjur dibuat tidak bisa diperbaiki lewat
antarmuka mana pun.

Dua akibat yang terlihat pengguna:

1. Warna `#12343B` **tidak ada** di palet lima status bawaan, sehingga status buatan pengguna
   langsung terlihat berbeda (biru-hijau gelap) dari yang lain.
2. Status baru tidak bisa dijadikan titik akhir alur, sehingga alur order tidak bisa benar-benar
   diperluas — hanya ditambahi titik tengah bergaya "Menunggu".

Rumus urutan tampil juga rapuh: memakai *jumlah* status, bukan nilai urutan tertinggi. Bila ada
status yang pernah dihapus langsung di basis data, urutan baru bisa bentrok dengan yang sudah ada.

**Yang perlu dikonfirmasi:** apakah status order pernah ditambahkan lewat halaman ini di
production? Dan apakah kebutuhan sebenarnya adalah alur status yang bisa dikelola penuh (ubah,
hapus, urutkan, tandai titik akhir), atau lima status bawaan sudah cukup?

---

## Modul 04 — Branch (Multi-Cabang)

Sumber: [04-branch/](04-branch/). Analisis lengkap ada di berkas modul tersebut.

### KI-36 — Cabang tidak bisa ditutup maupun dihapus dari aplikasi ✅

**Tingkat: tinggi — kebutuhan operasional nyata yang tidak punya jalan sama sekali.**

Cabang yang sudah dibuat **permanen**. Tidak ada tombol hapus, tidak ada tombol arsip, dan tidak
ada pilihan status di form. Form cabang bahkan **selalu mengirim status "aktif"** setiap kali
disimpan.

Padahal status non-aktif punya arti nyata di lima tempat di sistem:

| Tempat | Yang terjadi bila cabang non-aktif |
|---|---|
| Halaman pemilihan cabang saat login | Cabang tidak ditawarkan |
| Data cabang aktif | Tidak dikembalikan |
| Ganti cabang aktif | Ditolak (sebagai galat teknis — KI-02) |
| Tujuan mutasi stok antar-cabang | Ditolak, `Cabang tujuan tidak ditemukan` |
| Metrik operasional harian (job 01:00) | Berhenti dihitung |

Jadi mekanismenya lengkap, hanya **tombolnya yang tidak pernah dibuat**.

Akibat yang lebih halus: bila status pernah diubah langsung lewat basis data, membuka lalu
menyimpan cabang itu dari form — bahkan hanya untuk memperbaiki alamat — akan **diam-diam
mengaktifkannya kembali**, beserta lokasi stok defaultnya.

**Yang perlu dikonfirmasi:** apakah pernah ada cabang yang perlu ditutup? Bila belum pernah,
kebutuhan ini mungkin memang belum muncul. Bila ya, bagaimana selama ini ditangani — dibiarkan
muncul terus di daftar, atau diubah lewat basis data?

---

### KI-37 — Nomor telepon cabang tidak pernah bisa diisi ✅

**Tingkat: rendah — fitur yang tidak selesai, bukan kerusakan.**

Kolom nomor telepon cabang ada di basis data, diterima oleh API, dan bahkan dikirim oleh skrip
pengujian otomatis. Tapi **form cabang tidak punya field-nya**, dan penyimpanan dari web tidak
pernah menyertakannya.

Tidak ada satu pun layar yang menampilkan nomor telepon cabang — termasuk kop nota dan surat
jalan, yang memakai nomor telepon dari profil dokumen di Pengaturan.

Jadi kolom ini kosong selamanya untuk semua cabang yang dibuat lewat aplikasi.

**Yang perlu dikonfirmasi:** apakah nomor telepon per-cabang memang dibutuhkan (mis. dicetak di
nota cabang yang bersangkutan), atau cukup satu nomor perusahaan di profil dokumen? Bila tidak
dibutuhkan, kolomnya sebaiknya dibuang di sistem baru daripada dibiarkan menggantung.

---

### KI-38 — Badge "Pusat" tidak pernah muncul untuk cabang buatan pengguna ✅

**Tingkat: rendah — penanda yang secara praktis mati.**

Daftar cabang menampilkan badge biru **"Pusat"** untuk cabang yang ditandai sebagai kantor pusat.
Tapi penanda itu **hanya pernah diisi oleh data bawaan sistem** (cabang Blora). Alur tambah maupun
ubah cabang tidak pernah menyentuhnya, dan tidak ada pilihan apa pun di antarmuka.

Akibatnya: berapa pun cabang yang ditambahkan pengguna, badge "Pusat" akan selamanya menempel di
cabang bawaan — bahkan bila kantor pusat sebenarnya sudah pindah.

**Yang perlu dikonfirmasi:** apakah penanda "Pusat" punya arti operasional bagi Anda (mis.
menentukan cabang mana yang dipakai sebagai rujukan), atau sekadar label tampilan? Bila punya arti,
ia butuh cara mengubahnya; bila tidak, sebaiknya dibuang.

---

### KI-39 — Kolom "Status / Akses" tidak pernah menampilkan status cabang ✅

**Tingkat: sedang — janji kolom tidak ditepati, dan cabang mati tampak hidup.**

Kolom kelima di daftar cabang berjudul **"Status / Akses"**, tapi isinya hanya:

- badge **"Pusat"** (bila cabang bertanda pusat), dan
- badge **"Anda Memiliki Akses"** / **"Tidak Ada Akses"**.

**Status cabang sendiri tidak pernah ditampilkan.** Daftar juga tidak menyaring status, sehingga
cabang yang sudah dinonaktifkan tampil **identik** dengan cabang yang aktif.

Ini bertemu dengan KI-36: bila suatu saat fitur menutup cabang dibuat, cabang yang ditutup akan
tetap tampak normal di daftar ini sampai kolomnya diperbaiki.

**Yang perlu dikonfirmasi:** apakah judul kolomnya yang keliru (seharusnya "Akses"), atau memang
status cabang yang seharusnya ikut tampil? Keduanya perbaikan kecil, tapi arahnya berbeda.

---

### KI-40 — Mengubah kode cabang memutus keseragaman nomor dokumen ✅

**Tingkat: tinggi bila kode pernah diubah — menyentuh dokumen resmi dan rekap pajak.**

Kode cabang adalah sumber prefix seluruh nomor dokumen. Mengubahnya — kapan pun, tanpa peringatan
apa pun — langsung menulis ulang kelima prefix:

| Prefix | Sebelum | Sesudah |
|---|---|---|
| Order | `ORD-JKT` | `ORD-SBY` |
| Pembayaran | `PAY-JKT` | `PAY-SBY` |
| Surat jalan | `SJ-JKT` | `SJ-SBY` |
| Retur penjualan | `RTR-JKT` | `RTR-SBY` |
| Retur pembelian | `RTB-JKT` | `RTB-SBY` |

Dokumen yang sudah terbit **tidak** ikut berubah, dan nomor urutnya **tidak** direset. Hasilnya,
dalam satu buku penjualan cabang yang sama:

```
ORD-JKT/PJ/2026/08/00007      (sebelum kode diubah)
ORD-SBY/PJ/2026/08/00008      (sesudah kode diubah)
```

Untuk rekap dan laporan siap-SPT, dokumen dari satu cabang yang sama jadi punya dua penanda
berbeda. Tidak ada peringatan, tidak ada konfirmasi, dan tidak ada penjagaan "kode tidak boleh
diubah setelah ada dokumen terbit".

Ada juga risiko yang lebih jauh: bila kode `JKT` kemudian diberikan ke cabang lain, cabang baru itu
bisa menerbitkan nomor yang **persis sama** dengan dokumen lama cabang pertama.

**Yang perlu dikonfirmasi:** apakah kode cabang pernah diubah di production? Dan apakah di sistem
baru kode sebaiknya dikunci setelah cabang punya dokumen, atau tetap bebas diubah dengan
peringatan?

---

### KI-41 — Mengubah kode cabang bisa gagal sebagai galat teknis ✅

**Tingkat: sedang — jalur galat yang seharusnya jadi pesan bisnis.**

Selain mengubah prefix nomor, mengubah kode cabang juga menulis ulang **kode gudang default**
menjadi `GDG-<KODE BARU>`. Ada dua cara langkah itu gagal, dan keduanya berakhir sebagai
**kesalahan internal server tanpa penjelasan**:

1. **Bentrok kode gudang.** Bila pengguna pernah membuat lokasi manual berkode `GDG-SBY` dari menu
   Stok, mengubah kode cabang menjadi `SBY` akan bertabrakan dengan lokasi itu.
2. **Kode terlalu panjang.** Batas 10 karakter hanya dijaga browser; server menerima sampai 50.
   Kode lebih dari 46 karakter membuat kode gudang turunannya melebihi kapasitas kolom.

Di layar keduanya tampil sama: toast **"Gagal menyimpan cabang"** tanpa deskripsi. Pengguna tidak
punya petunjuk apa pun tentang penyebabnya.

Kasus kedua tidak bisa dicapai lewat antarmuka biasa. Kasus pertama bisa, tapi menuntut kebetulan
penamaan yang cukup spesifik.

**Yang perlu dikonfirmasi:** apakah pernah muncul kegagalan menyimpan cabang yang tidak jelas
sebabnya?

---

### KI-42 — Nama gudang default ditimpa diam-diam setiap kali cabang disimpan ✅

**Tingkat: sedang — pekerjaan pengguna hilang tanpa peringatan.**

Setiap penyimpanan cabang — termasuk yang hanya mengubah kota — **selalu menulis ulang** kode dan
nama lokasi stok default cabang, mengambilnya dari data cabang.

Jadi bila pengguna mengganti nama lokasi default dari menu **Stok** (mis. dari
*Gudang Utama Jakarta* menjadi *Gudang Depan*), lalu suatu saat memperbaiki alamat cabang dari menu
**Cabang**, nama gudang itu **kembali** ke nilai lama. Tidak ada peringatan, dan pengguna kecil
kemungkinan menghubungkan kedua tindakan itu.

Akarnya: nama gudang default disimpan di **dua tempat** — sebagai label di data cabang dan sebagai
nama di data lokasi stok — dan data cabang selalu dianggap yang benar.

Ada varian yang lebih buruk: bila lokasi default sempat **diarsipkan** dari menu Stok, penyimpanan
cabang akan memperbarui baris arsip itu tanpa menghidupkannya kembali dan tanpa membuat lokasi
default baru — cabang berakhir tanpa lokasi default yang hidup, kebalikan dari tujuan aturannya.

**Yang perlu dikonfirmasi:** apakah nama gudang default pernah diganti dari menu Stok? Dan mana
yang seharusnya jadi pemilik nama itu — form cabang atau menu Stok?

---

### KI-43 — Menyimpan cabang tidak memberi kabar apa pun saat berhasil, dan kabar kosong saat gagal ✅

**Tingkat: sedang — pengguna harus menebak apa yang terjadi.**

Dua sisi dari masalah yang sama:

**Saat berhasil:** tidak ada pemberitahuan sama sekali. Halaman langsung berpindah ke daftar
cabang. Bandingkan dengan menyimpan profil perusahaan, yang menampilkan
*"Profil perusahaan disimpan"*.

**Saat gagal:** apa pun sebabnya, yang muncul adalah satu toast merah **"Gagal menyimpan cabang"**
**tanpa deskripsi**. Pesan asli server dibuang di lapisan antarmuka. Padahal server sudah mengirim
kalimat yang jelas dan siap pakai:

| Penyebab | Pesan server yang tidak pernah terlihat |
|---|---|
| Kode ganda | `Kode cabang 'BLR' sudah digunakan` |
| Cabang tidak ada | `Cabang tidak ditemukan` |

Ini akar yang sama dengan KI-04 (modul 01) dan KI-17 (modul 02).

**Yang perlu dikonfirmasi:** tidak perlu — ini jelas perlu diperbaiki. Yang perlu diputuskan hanya
bentuknya: pesan galat di dekat field yang bermasalah, atau tetap toast tapi dengan isi pesan
server.

---

### KI-44 — Alamat edit cabang yang tidak dikenal berubah diam-diam jadi form tambah ✅

**Tingkat: rendah — sulit dicapai, tapi hasilnya membingungkan.**

Halaman edit menentukan mode kerjanya dari apakah cabang dengan nomor itu ada di daftar yang sudah
dimuat. Bila **tidak ditemukan** — tautan lama, nomor yang salah ketik, atau cabang yang dihapus
langsung dari basis data — halaman tidak menampilkan galat. Ia berubah menjadi **form tambah**:
judulnya berganti jadi "Tambah Cabang", semua field kosong, dan menekan simpan akan **membuat
cabang baru**.

Pengguna yang mengira sedang memperbaiki cabang lama justru menambah cabang.

**Yang perlu dikonfirmasi:** apakah pernah muncul cabang duplikat yang tidak diketahui asalnya?

---

### KI-45 — Reset nomor tahunan yang dijanjikan tidak pernah terjadi 🔁

**Tingkat: sedang — memengaruhi rekap tahunan dan penomoran dokumen resmi.**
**Keputusan analis:** Replikasi tanpa-reset sesuai OQ-A17: penomoran berlanjut lintas tahun dan kebijakan yearly palsu dibuang.

Setiap deret nomor menyimpan kebijakan **"reset tiap tahun"** beserta pola formatnya. **Kedua
nilai itu tidak pernah dibaca oleh satu pun bagian sistem.** Semua nomor disusun langsung di kode,
dan tidak ada yang memeriksa pergantian tahun.

Yang benar-benar terjadi:

| Dokumen | Reset nyata |
|---|---|
| Order penjualan & pembelian | **Bulanan** — terjadi bukan karena kebijakan, tapi karena penghitungnya memang dibuat per bulan |
| Pembayaran, surat jalan, retur penjualan, retur pembelian | **Tidak pernah reset** |
| Mutasi stok | Tidak pernah reset (memang begitu kebijakannya) |

Yang akan dilihat pengguna saat pergantian tahun:

```
PAY-BLR/2026/01187      (pembayaran terakhir 2026)
PAY-BLR/2027/01188      (pembayaran pertama 2027 — bukan 00001)
```

**Yang perlu dikonfirmasi:** apakah nomor pembayaran, surat jalan, dan retur memang **seharusnya**
kembali ke `00001` setiap 1 Januari? Untuk kebutuhan rekap ini biasanya diharapkan — tapi karena
selama ini tidak terjadi, mengubahnya berarti sistem baru berperilaku berbeda dari yang sudah
biasa dilihat.

---

### KI-46 — Nomor mutasi stok memakai nomor internal cabang, bukan kode cabang ✅

**Tingkat: sedang — satu jenis dokumen menyimpang dari pola semua dokumen lain.**

Seluruh dokumen memakai kode cabang di nomornya — kecuali **surat mutasi stok antar-cabang**, yang
memakai **nomor internal cabang**:

```
ORD-BLR/PJ/2026/08/00001      (order — pakai kode cabang)
SJ-BLR/2026/00042             (surat jalan — pakai kode cabang)
TRF-2/2026/00001              (mutasi stok — pakai nomor internal)
```

Penyebabnya: penghitung mutasi stok dibuat sendiri oleh modul Stok saat mutasi pertama, bukan saat
cabang dibuat — sehingga ia tidak masuk daftar penghitung yang prefiksnya disegarkan mengikuti kode
cabang. Mengubah kode cabang juga **tidak** memperbaikinya.

Akibat bagi pengguna: nomor mutasi tidak bisa dibaca cabang asalnya, dan tampak seperti kesalahan
sistem di lembar cetak.

**Yang perlu dikonfirmasi:** apakah surat mutasi stok memang perlu memuat kode cabang seperti
dokumen lain? Kalau ya, apakah nomor mutasi lama boleh dibiarkan apa adanya?

---

### KI-47 — Peran tanpa izin lihat cabang kehilangan alamat cabang di kop nota ✅

**Tingkat: sedang — terlihat langsung di dokumen yang dipegang pelanggan.**

Daftar cabang dimuat saat aplikasi dijalankan, dan pemuatan itu butuh izin **"Lihat cabang"**.
Peran **staff** dan **kasir** tidak punya izin itu. Permintaannya ditolak, kegagalannya ditelan
diam-diam, dan aplikasi berjalan dengan daftar cabang **kosong**.

Untuk halaman `/branches` sendiri ini tidak jadi soal — peran itu memang tidak boleh membukanya.
Masalahnya di tempat lain: identitas cabang yang dipakai kop nota diambil dari daftar itu bila
tersedia, dan dari ringkasan sesi bila tidak. **Ringkasan sesi tidak memuat alamat cabang.**

Hasilnya, nota yang dicetak kasir kehilangan baris alamat cabang, sementara nota yang dicetak owner
untuk order yang sama menampilkannya. (Bila profil dokumen di Pengaturan sudah diisi alamatnya,
gejalanya tertutup — karena profil dokumen menang atas alamat cabang.)

Efek serupa muncul di dropdown cabang halaman **Saldo Awal Persediaan** untuk peran kustom yang
tidak diberi izin lihat cabang: dropdown-nya kosong tanpa penjelasan.

**Yang perlu dikonfirmasi:** apakah pernah ada laporan nota kasir tampak berbeda dari nota owner?
Dan apakah alamat cabang memang perlu tercetak — bila ya, membaca identitas cabang sendiri
seharusnya tidak butuh izin pengelolaan cabang.

---

### KI-48 — Kode cabang berspasi lolos pemeriksaan lalu berakhir jadi galat teknis ✅

**Tingkat: rendah — sulit dicapai lewat antarmuka.**

Saat menambah cabang, pemeriksaan "kode sudah dipakai?" memakai kode **mentah** yang dikirim,
sementara yang benar-benar disimpan adalah versi yang sudah dipangkas dan dibesarkan hurufnya.

Kode `" BLR"` (berspasi di depan) karena itu **lolos** pemeriksaan — tidak ada cabang berkode
`" BLR"` — lalu ditolak basis data saat disimpan sebagai `BLR`. Yang muncul: **kesalahan internal
server**, bukan pesan `Kode cabang 'BLR' sudah digunakan`.

Alur ubah tidak punya masalah ini; ia memeriksa memakai kode yang sudah dinormalkan.

Pesan penolakannya juga tidak seragam antara kedua alur: alur tambah menampilkan kode **apa adanya
yang diketik** (`Kode cabang 'blr' sudah digunakan`), alur ubah menampilkan versi huruf besar.

**Yang perlu dikonfirmasi:** tidak perlu — perbaikannya jelas (normalkan dulu, baru periksa).
Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-49 — Endpoint "cabang aktif" tidak pernah dipakai ✅

**Tingkat: rendah — beban perawatan, bukan gangguan.**

Endpoint `branches/active` mengembalikan data cabang aktif sesi. Pencarian seluruh kode frontend
dan pengujian otomatis menemukan **nol pemanggil** — data cabang aktif selalu diambil dari
ringkasan sesi di browser.

Sejenis dengan `company/profile/get` (KI di modul 03) yang juga tidak pernah dipanggil UI. Keduanya
sisa dari rancangan awal yang arahnya berubah: identitas dipindah ke sesi lokal, endpoint pembaca
tunggalnya tidak ikut dibersihkan.

**Yang perlu dikonfirmasi:** tidak perlu — buang saja di sistem baru, kecuali ada integrasi luar
yang memakainya.
---

## Modul 05 — Product / Catalog

Sumber: [05-product-catalog/](05-product-catalog/).

### KI-50 — Kode produk/kategori baru tidak dipangkas, kode lama dipangkas ✅

**Tingkat: sedang — duplikat berspasi lolos diam-diam, pesannya pun tidak seragam.**

Pemeriksaan duplikat saat **tambah** produk memakai kode **mentah** (`" PRD-1"` ≠ `"PRD-1"`),
sementara yang disimpan juga mentah — sehingga `" PRD-1"` tersimpan apa adanya di samping `"PRD-1"`.
Alur **ubah** justru memangkas dulu (`trim()`), lalu memeriksa. Kategori lebih parah: tambah **maupun**
ubah tidak pernah memangkas (update bahkan mengabaikan kode sama sekali — KI-51).

Dua akibat: (1) kode yang tampak sama di daftar bisa ganda karena spasi tak terlihat; (2) bila kode
berspasi lolos cek lalu menabrak unique key dengan versi terpangkasnya, yang muncul adalah **galat
internal tanpa penjelasan**, bukan pesan `Kode produk ... sudah digunakan`. Pesan penolakan tambah
pun menampilkan kode apa adanya yang diketik, bukan versi tersimpan.

Ini pola yang sama dengan KI-48 (kode cabang): normalkan dulu, baru periksa, baru simpan.

**Yang perlu dikonfirmasi:** tidak perlu — perbaikannya jelas (trim + samakan di semua alur).
Dicatat agar tidak ikut tersalin, dan agar data berspasi yang telanjur masuk ikut dibersihkan saat
migrasi.

---

### KI-51 — Kode kategori di form edit bisa diubah tetapi diabaikan server ✅

**Tingkat: sedang — pengguna yakin mengubah sesuatu yang tidak berubah.**

Form edit kategori menampilkan field **Kode kategori** yang terisi dan bisa diketik. Endpoint
`product-categories/update` **tidak menerima field kode** — nilai yang diketik dibuang diam-diam,
penyimpanan dilaporkan berhasil (form me-reset seperti biasa).

Bandingkan dengan KI-42 (nama gudang ditimpa diam-diam): di sana data hilang karena ditulis dari
tempat lain; di sini data tidak pernah ditulis sama sekali, tetapi pengguna diberi kesan sebaliknya.

**Yang perlu dikonfirmasi:** apakah kode kategori memang seharusnya tidak bisa diubah (maka field-nya
harus dikunci/disembunyikan saat edit), atau seharusnya bisa diubah (maka endpoint-nya yang kurang)?
Keduanya perbaikan kecil, tapi arahnya berlawanan.

---

### KI-52 — Harga beli wajib saat tambah, bebas dikosongkan saat edit ✅

**Tingkat: sedang — membuka kembali lubang yang sengaja ditutup.**

Produk berstok **wajib** punya harga beli > 0 saat dibuat — alasannya tercatat di kode: tanpa ini
produk tertahan `"Menunggu data"` di Keuangan (insiden HPP tercemar di masa lalu adalah alasan
guard `assertReasonableUnitCost` lahir). Tetapi endpoint **update tidak memeriksa sama sekali**:
produk berstok bisa diedit harga belinya menjadi 0 atau null dan tersimpan tanpa peringatan.

Import bulk justru memeriksa (setiap baris berstok tanpa harga beli = error). Jadi tiga alur punya
tiga sifat: tambah dijaga, import dijaga, edit bebas.

**Yang perlu dikonfirmasi:** apakah ada produk berstok di production yang harga belinya 0/null
akibat jalur edit? Bila ada, mereka adalah kandidat `"Menunggu data"` di Finance. Perbaikannya:
berlakukan guard yang sama di update (dengan pengecualian sadar bila memang dibutuhkan).

---

### KI-53 — Import bulk melewati penjagaan yang berlaku di form ✅

**Tingkat: sedang — jalur massal lebih longgar dari jalur satuan.**

Update produk via import menulis field langsung (`applyProductDraft`) tanpa tiga penjagaan yang
ditegakkan form/API satuan: **kunci UOM setelah ada movement** (BR-15), **larangan ubah tipe
berstok→non-fisik** (BR-11), dan **cek stok sebelum mengubah komposisi varian** (BR-24/25).
Satu-satunya yang tetap dijaga di import adalah harga-beli-wajib (KI-52 justru kebalikannya).

Dampaknya konkret: file Excel bisa mengubah faktor konversi produk yang sudah punya riwayat movement
— persis tindakan yang ditolak form dengan pesan "Buat produk baru jika konversi berbeda".

**Yang perlu dikonfirmasi:** apakah import pernah dipakai untuk mengubah (bukan menambah) produk
yang sudah bertransaksi? Bila ya, perubahan UOM/tipe lewat file perlu diaudit ulang sebelum migrasi.
Di sistem baru, satu jalur validasi harus dipakai kedua alur.

---

### KI-54 — Kategori lewat API: induk bebas, siklus bebas, arsip bebas ✅

**Tingkat: sedang — UI dan import dijaga, API-nya tidak.**

Tiga penjagaan hanya ada di lapisan atas: UI mengecualikan diri+keturunan dari pilihan induk,
import menolak induk tak dikenal dan siklus (`"membentuk putaran: A > B"`), dan tidak ada tombol
arsip di UI. API di bawahnya menerima semuanya: `id_parent_category` disimpan mentah (boleh tak ada,
boleh milik perusahaan lain), siklus A→B→A bisa ditulis langsung, dan kategori yang masih dipakai
puluhan produk bisa diarsipkan tanpa peringatan (produknya tetap menunjuk, namanya hilang dari daftar).

**Yang perlu dikonfirmasi:** tidak perlu — ini murni perbaikan teknis (validasi + cek pemakaian di
server). Dicatat karena pohon kategori yang rusak (siklus/orphan) akan membuat rekursi tree di
sistem baru berputar tanpa henti bila datanya ikut dimigrasi apa adanya.

---

### KI-55 — Batas halaman daftar produk tanpa batas bawah ✅

**Tingkat: rendah — hanya lewat API langsung.**

`products/list` hanya memangkas `limit` yang terlalu besar (maks 100). Nilai `0`, negatif, atau
bukan angka diteruskan ke query — hasilnya daftar selalu kosong atau galat 500. Akar yang sama
persis dengan KI-26 (modul 02); modul ini tidak memakai helper bersama yang sudah benar.

**Yang perlu dikonfirmasi:** cukup diseragamkan di sistem baru. Tidak ada perubahan perilaku
terlihat (FE selalu mengirim 20).

---

### KI-56 — Arsip produk satu arah + dialog yang menyebut "nonaktif" + pesan yang menunjuk jalan buntu ✅

**Tingkat: tinggi — tiga hal bergabung menjadi jebakan.**

1. **Arsip tidak bisa dibatalkan.** Tidak ada endpoint restore untuk produk, kategori, varian,
   maupun foto — satu-satunya master data tanpa restore (Business Party punya). Salah arsip =
   SQL manual.
2. **Dialog menyebut hal yang salah.** Teksnya: `"akan dipindahkan ke status nonaktif dan tidak
   muncul lagi di daftar aktif"` — padahal yang terjadi bukan pindah status (produk `inactive`
   masih bisa ditampilkan via filter Nonaktif), melainkan arsip permanen yang hilang dari **semua**
   filter termasuk "Semua".
3. **Pesan import menunjuk aksi yang tidak ada.** Kode produk/kategori/varian yang sudah diarsip
   ditolak import dengan perintah `"Pulihkan ... terlebih dahulu atau gunakan kode lain"` — tetapi
   tidak ada tombol/endpoint "Pulihkan" di mana pun.

**Yang perlu dikonfirmasi:** (a) apakah arsip produk memang harus satu arah, atau perlu tombol
Pulihkan (dan apakah kode arsip boleh dipakai ulang bila restore ada)? (b) Boleh teks dialog
diperbaiki menjadi "diarsipkan dan tidak bisa dikembalikan"? Ini keputusan produk sebelum rebuild.

---

### KI-57 — Arsip memakai stok fisik, bukan stok tersedia 🟡

**Tingkat: perlu keputusan — bukan bug yang terbukti.**
**Keputusan analis:** Arsip menolak bila stok tersedia > 0 atau ada reservasi/transfer berjalan; definisi "masih punya stok" adalah available, bukan on-hand.

Guard arsip menjumlah `on_hand_qty`, bukan `available_qty`. Artinya produk yang seluruh stoknya
ter-reservasi order (available 0, on_hand > 0) **tidak bisa** diarsipkan — padahal secara operasional
"tidak ada barang bebas" sudah terpenuhi. Sebaliknya tidak ada pengecekan reservasi/transfer
berjalan: produk yang sedang dalam mutasi antar-cabang bisa diarsipkan di tengah jalan.

**Yang perlu dikonfirmasi:** mana definisi "masih punya stok" yang benar untuk arsip — fisik
(`on_hand`) atau bebas (`available`)? Dan apakah reservasi/transfer berjalan harus menghalangi?

---

### KI-58 — Harga negatif dan harga minimum di atas harga normal lolos via API ✅

**Tingkat: rendah — belum terbukti terjadi, tapi tidak dijaga.**

Satu-satunya penjaga harga di FE adalah atribut browser `min="0"` — yang tidak berlaku untuk
panggilan API langsung. Server menerima harga beli/jual/minimum negatif, dan tidak memeriksa relasi
`min_selling_price ≤ selling_price`. Import pun hanya memeriksa "harus berisi angka", bukan tanda
atau relasi.

**Yang perlu dikonfirmasi:** apakah relasi harga perlu ditegakkan (tolak negatif? tolak min > normal
dengan pesan?), atau cukup peringatan? Bila data production mengandung harga negatif, itu perlu
dibersihkan sebelum migrasi.

---

### KI-59 — Urutan foto beku sejak upload; simpan kategori/arsip tanpa konfirmasi 🟡

**Tingkat: rendah — keterbatasan + keheningan, bukan kerusakan.**
**Keputusan analis:** Tambah toast sukses + pesan server untuk simpan kategori/arsip; urutan foto terkunci mengikuti urutan upload sampai seret-susun dikonfirmasi.

Dua hal digabung karena sama-sama "properti yang tampak bisa diatur tetapi tidak":

1. `sort_order` foto hanya ditulis sekali (maks+1 saat upload). Tidak ada endpoint maupun tombol
   untuk menyusun ulang — foto kedua selamanya kedua, kecuali yang pertama diarsipkan.
2. Simpan kategori **tidak punya toast sukses** (hanya toast gagal tanpa alasan — seakar KI-17);
   arsip produk **tidak punya toast sukses** (hanya kembali ke daftar). Pengguna harus menebak
   apakah simpan kategori berhasil.

**Yang perlu dikonfirmasi:** apakah urutan foto perlu bisa diatur (seret-susun) di sistem baru,
atau urutan upload memang cukup? Untuk toast: perbaiki mengikuti keputusan lapisan bersama
(KI-04/KI-17/KI-43).

---

### KI-60 — Kolom inline `Info tambahan:` di sheet produk kini jadi error total 🟡

**Tingkat: perlu keputusan — file lama yang dulu valid kini diblokir.**
**Keputusan analis:** Kolom inline lama diterima sebagai peringatan + migrasi otomatis ke atribut, bukan error total pemblokir.

Sheet Daftar Produk yang masih memakai kolom inline `Info tambahan: X` / `Atribut: X` (format lama
sebelum sheet Info Tambahan ada) mendapat isu **error per sel** — dan satu error saja memblokir
seluruh commit. Template baru memang tidak lagi mencantumkan kolom itu, tetapi file-file lama yang
tersimpan di komputer staff masih memakainya.

**Yang perlu dikonfirmasi:** apakah masih ada file lama beredar yang memakai kolom inline? Bila ya,
pilihannya: (a) perlakukan sebagai peringatan + migrasi otomatis ke atribut (ramah), atau (b) tetap
error (tegas, tetapi file lama harus dibuat ulang manual).

---

## Modul 06 — Business Party (Customer / Supplier)

Sumber: [06-business-party/](06-business-party/).

### KI-61 — Simpan pelanggan/pemasok yang gagal tidak memberi kabar apa pun ✅

**Tingkat: tinggi — pengguna mengira tersimpan, padahal tidak.**

`saveBusinessParty` di store **tidak memanggil notifikasi sama sekali** — sukses maupun gagal.
Sukses tertutup oleh navigasi ke daftar, tetapi gagal (kode duplikat, server mati, validasi member)
hanya membuat form diam di tempat: tidak ada toast, tidak ada notice, tidak ada pesan di dekat
field. Pengguna yang menekan Simpan lalu beralih pekerjaan akan mengira data masuk.

Bandingkan dalam modul yang sama: hapus/pulihkan memakai notice inline `Tindakan gagal` + pesan
server; modul produk memakai toast gagal + pesan. Jalur simpan pihak adalah satu-satunya jalur tulis
tanpa umpan balik gagal.

Diperparah oleh: tombol submit tanpa status sibuk/disabled — klik ganda mengirim dua request (risiko
duplikat nama; kode auto tetap unik, kode manual bisa 409 yang juga diam).

**Yang perlu dikonfirmasi:** tidak perlu — perbaikannya jelas (toast/notice + pesan server +
cegah submit ganda). Dicatat agar tidak ikut tersalin, dan karena E2E 20/21 selama ini hanya
menutup jalur sukses.

---

### KI-62 — Kode bisa diketik saat edit tetapi diabaikan, dan saran restore tak bisa dijalankan ✅

**Tingkat: sedang — dua sisi dari kode yang tak bisa diubah.**

1. Form edit menampilkan field kode terisi dan bisa diketik, tetapi endpoint update tidak menerima
   kode — perubahan hilang diam-diam tanpa pesan (sekelas KI-51 pada kategori produk).
2. Pesan restore saat bentrok kode memerintah: `"Ubah kode itu dulu sebelum memulihkan."` — tetapi
   tidak ada cara mengubah kode lewat API/UI mana pun (butir 1). Bila bentrok benar terjadi,
   pemulihan terkunci permanen kecuali via SQL.

Secara normal butir 2 mustahil (create me-reserve kode lintas arsip), sehingga jebakan ini laten.
Tetapi butir 1 aktif setiap hari bagi siapa pun yang mencoba merapikan kode.

**Yang perlu dikonfirmasi:** apakah kode pihak seharusnya bisa diubah (maka tambah field + jaga
riwayat/snapshot yang merujuk kode), atau tidak (maka kunci field saat edit + tulis ulang pesan
restore tanpa saran mustahil)? Arahnya berlawanan, perlu keputusan Anda.

---

### KI-63 — Buku alamat: hapus tanpa konfirmasi, gagal tanpa kabar, aturan tak merata 🟡

**Tingkat: sedang — operasi destruktif yang paling sunyi di modul ini.**
**Keputusan analis:** Tambah dialog konfirmasi hapus, notifikasi gagal, dan gate izin di UI; alamat milik pelanggan terarsip ikut terkunci.

1. Tombol **Hapus** alamat langsung mengarsip — tanpa dialog (padahal hapus pelanggan memakai
   dialog konfirmasi di halaman yang sama).
2. Gagal simpan/hapus/jadikan-utama alamat **diam total**: `use-customer-addresses` tanpa notifikasi,
   `CustomerAddressBook`/`AddressPicker` tanpa notice — `busy` hanya lepas.
3. `update`/`archive` alamat tidak memeriksa pelanggan pemilik masih aktif (beda dengan
   `list`/`create` yang menolak `404 'Pelanggan tidak ditemukan.'`) — alamat milik pelanggan di
   Sampah masih bisa diubah/dihapus.
4. Tombol Jadikan Utama/Edit/Hapus tampil tanpa gate izin di UI (server menolak bila tanpa
   `order.update`, tetapi pengguna melihat tombol yang pasti gagal).

**Yang perlu dikonfirmasi:** butir 1–2 dan 4 jelas diperbaiki; butir 3 perlu keputusan: bolehkah
buku alamat pelanggan terarsip tetap dikelola, atau harus ikut terkunci?

---

### KI-64 — Kode otomatis kembar bila dua kasir tambah bersamaan ✅

**Tingkat: rendah — menuntut detik yang sama, tapi ujungnya 500.**

`generateNextPartyCode` membaca max lalu menyimpan tanpa lock/transaksi. Dua create kosong
bersamaan membaca max yang sama → kode sama → satu gagal unique-key sebagai **galat internal**,
bukan pesan ramah (jalur ini tidak melewati cek `ConflictException` karena kodenya "baru").

**Yang perlu dikonfirmasi:** tidak perlu — perbaikannya teknis (retry dengan suffix atau sequence
terkunci). Relevan saat cabang/kasir bertambah dan tambah-cepat dari POS makin sering.

---

### KI-65 — Cari pihak tidak tahan kata dibalik, tidak seperti cari produk ✅

**Tingkat: rendah — inkonsistensi antar halaman yang bersebelahan.**

Pencarian pihak memakai **frasa utuh** (`LIKE %...%`): `"Santoso Budi"` tidak menemukan `"Budi
Santoso"`. Pencarian produk memakai **tokenized per-kata** yang tahan urutan dan spasi ganda
(A-01 modul 05). Pengguna yang terbiasa di halaman Produk akan bingung di halaman Pelanggan —
persis seperti KI-25 di modul Users.

**Yang perlu dikonfirmasi:** boleh diseragamkan ke tokenized seperti produk? Tidak ada perubahan
perilaku selain hasil lebih contributor.

---

### KI-66 — Nama/telepon/email tanpa validasi server ✅

**Tingkat: sedang — integrasi bisa menulis data sampah.**

Satu-satunya penjaga adalah browser/JS di tiga pintu UI (`required` HTML, cek trim, `type=email`).
API menerima: nama `""`, telepon bebas (kolom bernama `phone_e164` tanpa cek E.164 — M2-Q4 tetap
terbuka), email tanpa `@`, alamat apa pun. Update bahkan bisa **mengosongkan nama** yang tadinya
valid (`""` di-assign langsung).

**Yang perlu dikonfirmasi:** (a) apakah nama wajib di tingkat server (dan apakah `""` yang telanjur
masuk perlu dibersihkan)? (b) Apakah format telepon/email perlu divalidasi, atau kolom `phone_e164`
cukup diganti nama? (c) E2E helper selalu mengisi alamat+telepon — apakah ada pihak production yang
nomornya tidak bisa dihubungi?

---

### KI-67 — Tanpa izin member, keanggotaan tak terlihat tapi lestari 🟡

**Tingkat: perlu keputusan — bisa jadi fitur, bisa jadi bocor.**
**Keputusan analis:** Keanggotaan lestari dipertahankan (staff biasa tak boleh mencabut member) + tampilkan status member read-only di form.

Field Jenis Member hanya tampil bila `member_type.view`. Tanpa izin itu, form mengirim tanpa
`memberTypeId` — dan server memperlakukannya sebagai "tak tersentuh", sehingga pelanggan member
yang diedit non-member-staff **tetap member** (termasuk harga member di order berikutnya).

Bila maksudnya "staff biasa tidak boleh mencabut member", perilaku ini benar tetapi tak terlihat
(tidak ada badge/penjelasan di form). Bila maksudnya sebaliknya, ini kebocoran.

**Yang perlu dikonfirmasi:** perilaku mana yang diinginkan, dan apakah form perlu menampilkan status
member read-only bagi yang tak berizin mengubah?

---

### KI-68 — Tombol Kembali beda arah: pelanggan selalu ke daftar, pemasok ke riwayat ✅

**Tingkat: rendah — inkonsistensi kecil, membingungkan bila diperhatikan.**

Form pelanggan: `← Kembali` dan `Batal` selalu ke `/customers`. Form pemasok: `← Kembali` memakai
riwayat browser (`navigate(-1)` — bisa mendarat di luar modul), sedangkan `Batal` ke `/suppliers`.

**Yang perlu dikonfirmasi:** seragamkan ke mana? (Usulan: selalu ke daftar masing-masing, seperti
form pelanggan.)

---

### KI-69 — Primer alamat tak bisa diturunkan, hanya diganti 🟡

**Tingkat: rendah — kemungkinan disengaja, tapi tak tertulis.**
**Keputusan analis:** Satu-utama-wajib dipertahankan + dokumentasikan sebagai kontrak; keadaan "tanpa default" tidak didukung.

`is_primary: false` di update **diabaikan**; satu-satunya cara melepas status Utama adalah menunjuk
pengganti (atau mengarsipkan — itu pun mempromosikan pengganti otomatis). Pelanggan tidak bisa
berada dalam keadaan "punya alamat tapi tanpa default".

**Yang perlu dikonfirmasi:** apakah keadaan "tanpa default" memang tidak diinginkan (maka
pertahankan + dokumentasikan), atau perlu didukung (mis. untuk pelanggan yang alamatnya belum pasti)?

---

### KI-70 — Batas halaman daftar pihak tanpa batas bawah ✅

**Tingkat: rendah — hanya lewat API langsung.**

Sama seperti KI-26 (modul 02) dan KI-55 (modul 05): hanya batas atas 100 yang dijaga; `limit`
0/negatif/bukan-angka diteruskan ke query. FE selalu mengirim 20.

**Yang perlu dikonfirmasi:** cukup diseragamkan ke helper bersama di sistem baru.

---

## Modul 07 — Member Type & Member Pricing

Sumber: [07-member-pricing/](07-member-pricing/).

### KI-71 — Kode member milik arsip dipakai ulang → galat 500 🟡

**Tingkat: sedang — cek ramah tidak mencakup Sampah, tetapi unique key mencakupnya.**
**Keputusan analis:** Samakan ke pola pihak: reserve kode lintas arsip + pesan "(ada di Sampah)" + jaga restore, jangan 500.

`member-types/create` hanya memeriksa kode di antara rule **non-arsip**. Unique key
`uq_member_types_code (id_company, code)` mencakup baris terarsip — sehingga memakai ulang kode
milik arsip lolos cek lalu meledak sebagai galat internal MySQL 1062, bukan `409` berpesan.

Ini kebalikan dari modul 06 (pihak justru me-reserve lintas arsip dengan pesan `" (ada di
Sampah)"`) dan sekelas KI-50 (kategori). Tiga modul, tiga sifat berbeda untuk masalah yang sama.

**Yang perlu dikonfirmasi:** samakan ke arah mana — reserve lintas arsip seperti pihak (butuh pesan
+ jaga restore), atau izinkan pakai ulang seperti produk (butuh cek + pesan)? Jangan biarkan 500.

---

### KI-72 — Quote: id eksplisit galak, jalur pelanggan diam 🟡

**Tingkat: perlu keputusan — dua sifat berlawanan dalam satu endpoint.**
**Keputusan analis:** Degradasi ke harga standard dipertahankan agar kasir jalan terus, tetapi respons wajib membawa penanda member_inactive + badge di POS.

`pricing/quote` menyelesaikan member lewat dua jalur dengan filosofi beda:

| Jalur | Bermasalah | Respons |
|---|---|---|
| `id_member_type` eksplisit | tak dikenal → `404`; nonaktif → `400` | Galak & spesifik |
| `id_business_party` | tanpa member / member nonaktif/terarsip | **Sukses diam** (`member_type: null`, harga standard) |
| Keduanya dikirim | — | Eksplisit menang, pihak diabaikan diam-diam |

Jalur diam berarti kasir tidak tahu pelanggan "member" yang membernya baru dinonaktifkan kini
membayar harga normal — tidak ada penanda di quote maupun (yang terlihat) di UI.

**Yang perlu dikonfirmasi:** apakah degradasi diam disengaja (kasir jalan terus) atau perlu penanda
(mis. flag `member_inactive` di respons + badge di POS)? Dan bila keduanya dikirim dengan konflik,
apakah eksplisit-menang tetap benar?

---

### KI-73 — Mode pembulatan tanpa kelipatan = tanpa efek, diam-diam 🟡

**Tingkat: rendah — konfigurasi yang tampak jalan tetapi tidak.**
**Keputusan analis:** Wajibkan pasangan mode-kelipatan (peringatan/validasi di form + server); 0/null terkunci sebagai "tanpa pembulatan".

Memilih `Ke atas`/`Terdekat`/`Ke bawah` tanpa mengisi kelipatan menghasilkan **tanpa pembulatan
sama sekali** (kelipatan null menonaktifkan semua mode). Form tidak memperingatkan, daftar
menampilkan `"Ke atas "` (label + kosong) seolah ada aturan.

Pasangannya: mengisi kelipatan dengan mode `Tanpa pembulatan` juga diam (nilai tersimpan, tak
dibaca). Dan `0` berarti "tanpa pembulatan" (disimpan null), bukan "bulatkan ke satuan" — makna
yang hanya diketahui dari kode.

**Yang perlu dikonfirmasi:** cukup tambah peringatan/validasi pasangan di form (mode perlu
kelipatan, kelipatan perlu mode ≠ none), atau biarkan fleksibel? Serta: apakah `0` perlu diartikan
"bulatkan ke Rp 1"?

---

### KI-74 — Cari jenis member hilang saat refresh, tidak seperti daftar lain ✅

**Tingkat: rendah — inkonsistensi kecil antar halaman bersebelahan.**

Pencarian + halaman daftar member disimpan di **state lokal** (tanpa debounce, tanpa URL):
refresh/reset mengembalikannya ke awal. Daftar pelanggan/pemasok/produk menyimpan keduanya di URL
(+ debounce 400ms).

**Yang perlu dikonfirmasi:** seragamkan ke pola URL (dan apakah perlu filter status Aktif/Nonaktif
di UI — kini request selalu `all` tanpa pilihan)?

---

### KI-75 — Edit member ke-1001+ jatuh ke layar tak-ditemukan ✅

**Tingkat: rendah — plafon tinggi, tetapi tanpa jaring seperti modul tetangga.**

Form edit mengambil data dari store (limit 1000, semua status) **tanpa fetch by-id**. Melewati 1000
rule (atau membuka URL arsip), hasilnya layar `Jenis Member Tidak Ditemukan` — padahal datanya ada.
Modul 05/06 mengatasi ini dengan fetch `detail` by-id (perbaikan E2E 21).

**Yang perlu dikonfirmasi:** cukup tiru pola by-id + tambah endpoint `member-types/detail`
(kini tak ada), atau plafon 1000 dianggap cukup selamanya?

---

### KI-76 — Besaran/persen tanpa batas atas tersimpan bebas 🟡

**Tingkat: rendah — peluru ditolak di kasir, bukan di gudang.**
**Keputusan analis:** Persen dibatasi 0–100 (tolak dengan pesan); nominal di atas ambang wajar wajib konfirmasi, bukan tolak diam-diam.

Hanya `≥ 0` yang dijaga. Diskon 1000% atau tambah Rp 999 miliar tersimpan tanpa peringatan; yang
menolak hanyalah hitungan per baris (`Harga member ... menjadi negatif`) — itupun hanya bila
hasilnya negatif (tambah raksasa selalu lolos dan langsung dipakai kasir).

**Yang perlu dikonfirmasi:** perlu batas wajar (mis. persen ≤ 100, nominal ≤ basis?) atau peringatan
saja? Bila data production mengandung persen > 100, itu perlu ditinjau sebelum migrasi.

---

## Modul 08 — Order Purchasing

Sumber: [08-order-purchasing/](08-order-purchasing/).

### KI-77 — Badge "Diretur" di daftar order tidak pernah tampil ✅

**Tingkat: sedang — UI-nya ada, datanya dimatikan.**

Kolom Status daftar me-render badge `Diretur penuh`/`Diretur sebagian` dari
`order.returnSummary`. Tetapi daftar selalu meminta `summary_only: true`, dan server dalam mode
itu **melewati query retur** dan mengembalikan default `{hasReturn: false, fullyReturned:
false}` — sehingga badge tidak pernah tampil di daftar, untuk purchase maupun sales.

Detail order (non-summary) menghitungnya dengan benar. Jadi informasinya ada, hanya tidak sampai
ke satu-satunya tempat yang menampilkannya sebagai badge.

**Komplikasi (→ KI-102):** E2E 17 justru menuntut badge itu tampil di daftar. Salah satu dari
keduanya salah — jalankan E2E sebelum memutuskan.

**Yang perlu dikonfirmasi:** perbaikan (ikutkan peta retur batch di mode summary — query-nya sudah
batch-tunggal dan murah) atau hapus badge dari daftar (dan betulkan E2E)? Jangan biarkan kode mati
yang tampak hidup — apalagi yang dituntut test.

---

### KI-78 — `date_to` mentah kehilangan order di hari terakhir ✅

**Tingkat: rendah — hanya request API langsung.**

`orders/list` membandingkan `order_date <= date_to` mentah: `date_to: "2026-08-10"` berarti
`tengah malam` — order tanggal 10 Agustus jam berapa pun setelah 00:00:00 **tidak masuk**. FE
menutup lubang ini dengan mengirim `T23:59:59`, sehingga pengguna UI tidak terdampak; tetapi
integrasi/E2E/helper yang memanggil API langsung akan kehilangan sehari tanpa sadar.

**Yang perlu dikonfirmasi:** normalkan di server (akhir-hari bila tanpa waktu, seperti export yang
memakai `23:59:59`) agar semua pemanggil sama? Perbaikan kecil, tanpa perubahan terlihat di UI.

---

### KI-79 — Dialog Terima Barang bisa dibuka padahal tak ada sisa ✅

**Tingkat: rendah — jalan buntu yang sopan.**

Bila seluruh item sudah diterima tetapi status belum selesai (kondisi langka — mis. penerimaan
terakhir dicatat sebelum status selesai ada), tombol Terima Barang tetap muncul (syaratnya hanya
`!goodsReceivedAt` + ada transisi) dan dialog terbuka dengan teks `Semua item pada purchase order
ini sudah diterima.` Menekan konfirmasi memanggil API yang menolak `Tidak ada sisa item yang dapat
diterima`.

**Yang perlu dikonfirmasi:** sembunyikan/nonaktifkan tombol bila sisa nol (perlu info sisa di
detail — sudah ada `receiptSummary`), atau biarkan sebagai pesan? Bukan bug logika, hanya ujung
kasar.

---

### KI-80 — Penerimaan bisa bertanggal masa lalu (backdate) 🟡

**Tingkat: perlu keputusan — fleksibel operasional vs jujur kronologi.**
**Keputusan analis:** Backdate dipertahankan untuk operasional + dokumentasikan batasnya (tak boleh sebelum periode finance terkunci).

`Tanggal Terima` bebas diisi masa lalu: `receivedAt`, `goodsReceivedAt`, `movedAt` movement, dan
history penyelesaian semuanya memakai tanggal itu. Stok dan status "selesai" tercatat mundur —
mempengaruhi HPP/finance periode lalu dan urutan kronologi movement.

Untuk operasional (mencatat kiriman kemarin yang baru diinput hari ini) ini membantu. Untuk audit,
ini memungkinkan mengubah masa lalu tanpa jejak tanggal-input (yang tersimpan hanya `created_at`
baris penerimaan).

**Yang perlu dikonfirmasi:** apakah backdate memang dibutuhkan (maka pertahankan + dokumentasikan
batasnya, mis. tak boleh sebelum periode finance terkunci), atau tanggal harus selalu kini?

---

### KI-81 — Field `requires_payment_completion` selalu `false` ✅

**Tingkat: rendah — beban perawatan.**

Respons `receive-goods` selalu mengembalikan `requires_payment_completion: false` (literal di
kode), dan slice FE punya cabang toast untuknya yang tak terjangkau. Kemungkinan sisa rancangan
arus "terima dulu, bayar kemudian, selesaikan manual" yang tidak jadi dipakai — alur kini selalu
menyelesaikan (atau menolak) dalam satu panggilan.

**Yang perlu dikonfirmasi:** buang field + cabang toast di sistem baru, kecuali ada rencana arus
tunda-selesai.

---

### KI-82 — Nomor darurat + bulan memakai waktu server 🟡

**Tingkat: rendah-sedang — hanya aktif saat konfigurasi hilang/jam server geser.**
**Keputusan analis:** Tambah alarm bila nomor darurat terbit; bulan nomor tetap mengikuti waktu server + dokumentasikan.

Dua fallback yang menandai data tak normal: tanpa baris sequence `order` → nomor
`ORD-{idBranch}/PB/...` (tanpa kode cabang, sekelas nomor darurat KI di modul 04); tanpa sequence
`payment` → `PAY-{idBranch}-{Date.now()}` (bukan format resmi, tak berurut). Keduanya seharusnya
tak pernah terjadi (provisioning cabang membuatnya), tetapi tidak ada alarm bila terjadi.

Terpisah: bulan nomor memakai **waktu server**, bukan tanggal order — PO backdate/antedate tetap
memakan nomor bulan berjalan. Konsisten dengan temuan shared §7 (zona waktu belum konsisten).

**Yang perlu dikonfirmasi:** (a) apakah nomor darurat pernah terlihat di production (itu penanda
cabang lahir di luar alur normal, cf. M4-Q4)? (b) Bulan nomor seharusnya mengikuti tanggal order
atau tetap bulan berjalan?

---

### KI-83 — Invoice supplier bebas ganda; termin 30 hari terkirim walau tak disentuh 🟡

**Tingkat: rendah — kemungkinan disengaja, tetapi tak tertulis.**
**Keputusan analis:** Invoice ganda hanya memicu peringatan (faktur gabungan sah); default termin 30 hari dipertahankan eksplisit + dokumentasikan.

1. `supplier_invoice_number` tanpa cek unik: dua PO bisa memakai nomor faktur supplier yang sama
   (satu faktur gabungan untuk dua PO adalah praktik nyata — E2E 05 mencatatnya di notes — tetapi
   juga bisa salah ketik ganda).
2. Form selalu mengirim `paymentTermDays` (default 30 di state) untuk purchase net — server
   menyimpan 30 hari bahkan bila pengguna tak pernah menyentuh field itu. `0` vs `null` (tanpa
   termin bermakna) tidak dibedakan di UI.

**Yang perlu dikonfirmasi:** (a) peringatan duplikat invoice diinginkan atau justru dilarang
(mengingat faktur gabungan sah)? (b) Default 30 hari dipertahankan eksplisit atau hanya bila
diubah?

---

### KI-84 — Ganti jenis order tanpa ganti baris meninggalkan UOM sisi lama ✅

**Tingkat: sedang — validasi ada, komputasi ulang tidak.**

`orders/update` mengizinkan ganti `order_kind` (dengan validasi ulang pihak/termin) tetapi hanya
membangun ulang baris bila `items` ikut dikirim. Ganti jenis **tanpa** baris: baris lama bertahan
dengan snapshot UOM sisi lama (mis. PO berisi baris bersatuan jual, atau sebaliknya) — dokumen
campur yang tak bisa dibuat lewat form (form selalu kirim baris), hanya lewat API langsung.

**Yang perlu dikonfirmasi:** tolak ganti jenis tanpa baris, bangun ulang otomatis sisi-UOM-nya,
atau biarkan (dan kunci sebagai perilaku)? Form tidak terdampak apa pun pilihannya.

---

## Modul 09 — Order Sales

Sumber: [09-order-sales/](09-order-sales/).

### KI-85 — Pesan tipe-pihak salah menyebut "tempo" untuk semua penjualan ✅

**Tingkat: rendah — pesan menyesatkan, penolakan benar.**

`Pihak terkait untuk penjualan tempo harus bertipe customer, bukan {t}.` dipakai untuk
**seluruh** sales berpihak — termasuk tunai walk-in yang diisi supplier. Pengguna tunai yang salah
pilih pihak membaca pesan tentang "tempo" yang tidak ia pakai.

**Yang perlu dikonfirmasi:** perbaiki teks per konteks (tunai vs tempo) atau satu teks netral?
Perbaikan kecil; penolakannya sudah benar.

---

### KI-86 — Pengecualian POS di cek selesai tampak mati ✅

**Tingkat: perlu keputusan — redundansi atau jebakan.**

`update-status` punya dua lapis untuk selesai pre-serah: lapis-1 mengecualikan source `pos`
(`Order manual tidak boleh...`), lapis-2 menuntut serah untuk **semua** sales termasuk pos
(`Sales order harus diserahkan terlebih dahulu...`). Efeknya pengecualian lapis-1 tak pernah
berarti — POS pun selesai hanya lewat serah (yang memang pengecualian transisinya sendiri).

Bila maksudnya "POS boleh selesai tanpa serah", lapis-2 menggagalkannya. Bila maksudnya "POS
tetap wajib serah", lapis-1 menyesatkan pembaca kode (dan E2E 02 memang selalu serah via
`deliver-goods`).

**Yang perlu dikonfirmasi:** hapus pengecualian mati (jelaskan: semua sales wajib serah), atau
hidupkan (POS boleh selesai tanpa serah — mengubah perilaku kasir)?

---

### KI-87 — Referensi bayar auto tanpa cek unik 🟡

**Tingkat: rendah — tabrakan acak kecil, edit bebas.**
**Keputusan analis:** Referensi wajib unik di sisi server (auto-retry untuk generate, tolak-dengan-pesan untuk input manual).

`REF-YYYYMMDD-XXXX` (36^4 kombinasi/hari) dibuat per buka form, bisa diubah bebas, tanpa cek unik
di FE maupun backend (disimpan apa adanya, diverifikasi di `payment.service.ts`). Dua kasir bisa
menerbitkan referensi sama di hari sama.

**Yang perlu dikonfirmasi:** perlu unik (server-side, seperti nomor bayar) atau referensi memang
bebas-ganda (bukti nyata = nomor bayar + bukti + audit)? Bila bebas, dokumentasikan agar tak
"diperbaiki" keliru.

---

## Modul 10 — Goods Receipt (Penerimaan Barang)

Sumber: [10-goods-receipt/](10-goods-receipt/). Mekanika PO milik 08; di sini sisi dokumen.

### KI-88 — Alias `goods-receipts/create` tanpa pemanggil; `list`/`detail` tanpa pemakai FE ✅

**Tingkat: sedang — permukaan API yang tampak resmi tetapi mati.**

Verifikasi grep seluruh repo (FE, E2E, helper, API-spec):

| Endpoint | Pemanggil produksi | Spec |
|---|---|---|
| `orders/receive-goods` | Slice + helper E2E | Ada (14 sebutan di order.service.spec) |
| `goods-receipts/list` | Hanya E2E 05 (verifikasi) | Tidak ada |
| `goods-receipts/detail` | Nol | Tidak ada |
| `goods-receipts/create` | Nol | Tidak ada |

Alias `create` (dengan 6 nama-alternatif: `order_id`, `stock_location_id`,
`quantity_received`, `notes`, `order_item_id`) tidak pernah dipanggil siapa pun dan tidak
diuji — perilaku kompatibilitasnya (normalisasi alias) hanya dibaca dari kode, belum pernah
dieksekusi di test mana pun. Sejenis KI-49 (endpoint cabang aktif tak terpakai).

**Yang perlu dikonfirmasi:** buang `create` (+ pertimbangkan `detail`) di sistem baru, atau
pertahankan + tulis test-nya? Bila dibuang, migrasi klien API langsung (bila ada di luar repo)
perlu pemberitahuan.

---

### KI-89 — Penerimaan tanpa nomor dokumen 🟡

**Tingkat: perlu keputusan — merujuk batch lewat telepon itu sulit.**
**Keputusan analis:** Tambah nomor penerimaan (RCV-... per bulan seperti PO) + tampilkan di kartu riwayat.

Penerimaan tidak punya nomor: identitas operasional = tanggal + no SJ supplier (opsional, boleh
ganda) + lokasi. Dua batch sehari tanpa SJ hanya beda jam (`received_at` presisi 6 — tetapi jam
tidak tampil di kartu riwayat, hanya tanggal-waktu format lokal).

Untuk gudang yang menerima banyak mobil sehari, "batch yang mana?" dijawab dengan menunjuk baris
riwayat — tidak bisa disebut nomornya seperti PO (`.../PB/...`) atau bayar (`PAY-...`).

**Yang perlu dikonfirmasi:** apakah perlu nomor penerimaan (`RCV-...`/per bulan seperti PO) di
sistem baru, atau identitas tanggal + SJ + lokasi memang cukup? Menambah nomor adalah fitur baru
kecil dengan dampak ke kartu riwayat + (bila diinginkan) cetak.

---

### KI-90 — Presisi qty penerimaan: migrasi `(18,4)` vs entity `(24,12)` ✅

**Tingkat: rendah-teknis — perlu verifikasi live, bukan tebakan.**

Migrasi lahir 018 menulis `DECIMAL(18,4)`; entity meminta `(24,12)` tanpa migrasi lanjutan yang
mengubahnya (pola sama dengan faktor-UOM modul 05). Nilai live menentukan batas pecahan qty
nyata (4 vs 12 desimal) — relevan karena toleransi sisa memakai 0,0001.

**Yang perlu dikonfirmasi:** tidak perlu — cek `SHOW COLUMNS` di production, tetapkan satu
presisi di skema baru. Dicatat agar tak terlewat (bersama M5-Q8).

---

### KI-91 — Kolom arsip penerimaan tanpa endpoint 🟡

**Tingkat: rendah — skema setengah jadi.**
**Keputusan analis:** Tambah alur batal/koreksi penerimaan dengan reversal movement; kolom arsip dipertahankan sebagai penandanya.

`goods_receipts.archived_at` ada (dengan index + filter `IS NULL` di list/detail) tetapi tidak
ada endpoint arsip/batal/pulihkan — dan `goods_receipt_items` bahkan tanpa kolomnya. Salah catat
penerimaan hanya bisa dikoreksi via adjustment stok (modul 16), yang memperbaiki saldo tetapi
membiarkan dokumen penerimaan yang salah tetap tampil di riwayat selamanya.

**Yang perlu dikonfirmasi:** tambah alur koreksi/batal penerimaan (dengan reversal movement —
fitur baru), atau kunci "penerimaan final selamanya, koreksi via adjustment" sebagai kontrak
(dan bila dikunci: hapus kolom arsip mati dari skema baru)?

---

### KI-92 — Sisa penerimaan tanpa lock konkuren ✅

**Tingkat: rendah — menuntut dua kasir terima PO sama di detik sama.**

Tidak seperti retur-beli (`pessimistic_write` pada baris order), dua `receive-goods` bersamaan
membaca sisa yang sama dan keduanya bisa lolos cek — over-receipt + stok ganda. Praktik gudang
(satu PO dipegang satu admin) membuat ini jarang, tetapi satu-satunya tulis-kumulatif commerce
tanpa lock.

**Yang perlu dikonfirmasi:** tambah lock baris order saat terima (murah, pola sudah ada di
retur-beli), atau terima risiko? Tidak ada perubahan terlihat bila tanpa tabrakan.

---

## Modul 11 — Delivery / Pengiriman

Sumber: [11-delivery/](11-delivery/). Serah-langsung milik 09; SJ pengganti milik 12.

### KI-93 — API terbit SJ tak memeriksa tahap proses (UI + antrean yang menyaring) ✅

**Tingkat: sedang — pintu resmi lebih longgar dari etalasenya.**

`deliveries/create` hanya menolak: bukan-sales, terminal, tanpa-item, prabayar-berutang. Tidak
ada pemeriksaan "order sudah di tahap pengiriman" (transisi aktif kini→selesai) — tidak seperti
penerimaan (`Purchase order belum berada pada tahap penerimaan barang.`) maupun konfirmasi
sendiri (`Order belum berada pada tahap yang bisa diselesaikan.`). Syarat tahap hanya hidup di
tombol UI (`!nextProcessTransition`), shortcut `?action=create-sj`, dan antrean.

Akibat: request langsung bisa menerbitkan SJ untuk order Draft/pending (stok langsung berkurang!)
yang UI-nya sendiri tak pernah menawarkan.

**Yang perlu dikonfirmasi:** ketatkan API mengikuti UI (satu syarat di satu tempat), atau kunci
longgar sebagai perilaku (mis. untuk SJ darurat)? Jangan biarkan beda diam-diam.

---

### KI-94 — Nama sopir/petugas kosong lolos via API ✅

**Tingkat: rendah — kolom NOT NULL tanpa validasi isi.**

FE mewajibkan (`Nama sopir wajib diisi`, `Nama petugas gudang wajib diisi`), tetapi server
menerima string kosong (kolom hanya NOT NULL). SJ tercetak dengan kotak Sopir/Petugas kosong —
dokumen yang dibawa supir tanpa nama supir.

**Yang perlu dikonfirmasi:** tambah validasi non-kosong di server (sebaris dengan FE)? Tidak ada
perubahan UI.

---

### KI-95 — Cetak SJ tanpa izin `order.view` (cukup login) ✅

**Tingkat: perlu keputusan — kemudahan vs kebocoran.**

Rute cetak SJ `access: "authenticated"` — peran apa pun yang bisa login (termasuk tanpa
`order.view`) bisa membuka `/orders/:orderId/delivery/:sjId/print` bila tahu/p menebak id.
Rute cetak nota, struk, dan bukti-bayar semuanya `protected` + `order.view`.

Bila disengaja (cetak di perangkat gudang bersama tanpa akun berizin), ini fitur. Bila tidak,
ini satu-satunya dokumen yang bocor dari matriks izin (cf. KI-15 modul 02 tentang eskalasi).

**Yang perlu dikonfirmasi:** disengaja (maka dokumentasikan sebagai keputusan + pertimbangkan
token cetak khusus) atau kelalaian (maka samakan ke `order.view`)?

---

### KI-96 — `date_to` antrean akhir-hari, daftar order tidak ✅

**Tingkat: rendah — inkonsistensi antar dua daftar bersebelahan.**

Antrean menormalkan `date_to` panjang-10 menjadi `23:59:59`; `orders/list` membandingkan mentah
(KI-78). Filter tanggal yang sama di dua halaman memberi hasil beda di hari terakhir.

**Yang perlu dikonfirmasi:** seragamkan di server untuk keduanya (usulan: akhir-hari bila tanpa
waktu, seperti antrean)? Perbaikan kecil, menutup KI-78 sekalian bila diputuskan sama.

---

### KI-97 — Dispatch ganda bersamaan tanpa lock ✅

**Tingkat: rendah — sekelas KI-92.**

Sisa-kirim dihitung dari dispatch non-arsip tanpa lock: dua SJ bersamaan untuk sisa yang sama
bisa lolos berurutan-cek lalu keduanya mengurangi stok (over-dispatch). Berurutan aman.

**Yang perlu dikonfirmasi:** tambah lock seperti usulan KI-92 (satu pola untuk terima +
dispatch), atau terima risiko?

---

## Modul 12 — Sales Return (Retur Penjualan)

Sumber: [12-sales-return/](12-sales-return/).

### KI-98 — Kas collect/refund tanpa baris pembayaran 🟡

**Tingkat: tinggi — uang bergerak, jejak bayar tidak.**
**Keputusan analis:** Pertahankan kas-di-luar-sistem sebagai kontrak sementara + pastikan rekap kas/finance/SPT membaca settlement sampai diputuskan menulis `payments`.

`collect_payment` (tambah bayar) dan `refund` (kembali uang) hanya menulis baris
`sales_return_settlements` — **tanpa baris `payments`**. Piutang-efektif order langsung
terkoreksi via efek finansial, dan preview memerintah "tambah bayar sebelum disimpan" (kas
diambil di konter dulu, dicatat sesudah). Tetapi: tanpa nomor bayar, tanpa metode-tervalidasi
(sebaran `payment_method` bebas), tanpa bukti-lampiran, tanpa masuk antrean kas.

Untuk audit kas ("uang tunai kemarin ke mana?"), satu-satunya jejak adalah baris settlement +
referensi-teks-bebas. Bandingkan COD-serah/terima yang selalu menulis `payments` (+ nomor +
bukti opsional).

**Yang perlu dikonfirmasi:** apakah kasir memang menagih/mengembalikan tunai di luar sistem
lalu mencatat (maka kunci sebagai kontrak + pastikan rekap kas membaca settlement), atau
seharusnya menulis `payments` seperti COD (maka ini bug yang perlu diperbaiki + migrasi data
settlement lama)? Keputusan ini menyentuh rekap kas, finance, dan SPT.

---

### KI-99 — Kolom `cancelled_*` + status `cancelled` tanpa penulis ✅

**Tingkat: rendah — tipe dan filter siap, tombol tidak ada.**

Tipe `SalesReturnStatus` mengenal `cancelled`, filter daftar menawarkannya, entity punya
`cancelled_at/by/reason` — tetapi tidak ada endpoint, tombol, maupun kode yang menulisnya.
Dispatch menolak "Retur ini sudah dibatalkan" untuk status non-completed yang tak bisa dicapai.
Arsip pun disaring tanpa penulis (seperti penerimaan).

**Yang perlu dikonfirmasi:** butuh batal-retur (dengan reversal stok + settlement — fitur baru
besar), atau buang tipe/kolom/filter mati dari skema baru? Jangan biarkan filter yang tak bisa
berisi.

---

### KI-100 — Confirm pengganti: nama wajib, SJ order opsional + tanpa arsip-wajib ✅

**Tingkat: rendah — inkonsistensi antar dua konfirmasi bersaudara.**

Confirm SJ order: nama penerima opsional, arsip fisik wajib. Confirm pengganti: nama **wajib**
(`Nama penerima wajib diisi`), arsip **tidak disyaratkan sama sekali** (tanpa cek file).
Dua dialog yang tampak sama menuntut kebalikan.

**Yang perlu dikonfirmasi:** disengaja (pengganti dibawa supir yang sama sehingga arsip di SJ
asal cukup?) atau kelalaian silang? Seragamkan atau dokumentasikan bedanya.

---

## Modul 13 — Purchase Return (Retur Pembelian)

Sumber: [13-purchase-return/](13-purchase-return/).

### KI-101 — Izin `purchase_return.cancel` (+ `sales_return.cancel`) tanpa penegak ✅

**Tingkat: sedang — matriks menjanjikan tombol yang tidak ada.**

Seed 050 + matriks Role & Akses menawarkan `purchase_return.cancel` ("Batalkan Retur") ke
superadmin/owner/admin — tetapi tidak ada endpoint, guard, tombol, maupun kode yang
memeriksanya (`@RequirePermission('purchase_return.cancel')` nol hasil di seluruh repo).
Kembarannya `sales_return.cancel` sama persis (hanya di matriks).

Ini kebalikan KI-15 (izin yang bisa menaikkan hak): di sini izin yang **tidak bisa dipakai
untuk apa pun**. Admin yang diberi/menolak "Batalkan Retur" mengubah matriks tanpa efek —
kepercayaan palsu dua arah (mengira terlindungi / mengira berhak).

**Yang perlu dikonfirmasi:** buang kedua izin mati dari seed + matriks + union (usulan), atau
memang direncanakan fitur batal-retur (maka itu fitur baru besar: reversal stok + settlement —
masuk lingkup terpisah, bukan replikasi)?

---

### KI-102 — E2E menuntut badge yang menurut kode tak bisa tampil (kontrak vs kenyataan) ✅

**Tingkat: tinggi — salah satu dari keduanya salah, dan keduanya sudah didokumentasikan
sebagai kebenaran.**

Fakta yang bertentangan:

- E2E 17 (`17-revision-blackbox`, blackbox): setelah retur-sebagian, baris order di daftar
  **harus** memuat teks `Diretur sebagian` (dan `Selesai`); setelah retur-penuh: `Dibatalkan`
  + `Diretur penuh`. Ditambah: tombol `Retur ke Supplier` → URL pra-isi, stok 10→6→0, utang
  → 0, detail merender `Order pembelian dibatalkan`.
- Kode: daftar selalu meminta `summary_only: true`, dan server dalam mode itu melewati query
  retur dan mengembalikan default `{hasReturn: false, fullyReturned: false}` — badge
  `Diretur ...` **tidak pernah bisa tampil** (KI-77).

Kemungkinan: (a) E2E ini sedang gagal dan belum ditindaklanjuti; (b) ada jalur yang
terlewat dari analisis (mis. respons non-summary di tempat lain); (c) test ditulis aspiratif
(blackbox) mendahului perbaikan. Tidak dapat diputuskan tanpa menjalankan E2E.

**Yang perlu dikonfirmasi:** jalankan E2E 17 dan lihat. Bila gagal: putuskan — perbaiki kode
(ikutkan peta retur batch di mode summary, query-nya sudah murah) atau perbaiki E2E. Bila
lolos: tunjukkan jalur yang terlewat analisis ini dan koreksi KI-77. Jangan rebuild sebelum
ini terjawab — ini menentukan apakah badge daftar hidup atau mati.

Koreksi peta: cakupan E2E modul ini ada di **17**, bukan 08 seperti klaim 00-module-map
(E2E 08 nol menyebut retur-beli — verifikasi grep). FE unit test tetap nol (3 halaman tanpa
satu pun).

---

### KI-103 — Kas refund tanpa baris pembayaran (perluas KI-98) 🟡

**Tingkat: tinggi — sama dengan KI-98, sisi supplier.**
**Keputusan analis:** Ikut keputusan KI-98 secara simetris dengan perhatian arah kas berlawanan (masuk) beserta akun lawan dan rekap SPT.

Refund dari supplier hanya menulis baris settlement — tanpa baris `payments`, tanpa nomor
bayar, tanpa bukti-lampiran. Uang yang "diterima kembali" hanya ada sebagai angka settlement +
referensi-teks. Keputusan KI-98 (tulis `payments` vs kunci-kas-di-luar) harus berlaku simetris
di sini, termasuk akun lawan (kas vs hutang) dan rekap SPT.

**Yang perlu dikonfirmasi:** ikut keputusan KI-98 (satu keputusan dua sisi), dengan perhatian
khusus: arah kas berlawanan (masuk, bukan keluar).

---

## Modul 14 — Payment / Pembayaran

Sumber: [14-payment/](14-payment/). Form catat/dialog tempo milik 08/09; kartu finance milik 17.

### KI-104 — Alokasi manual tanpa UI 🟡

**Tingkat: sedang — setengah fitur.**
**Keputusan analis:** Kunci FIFO-saja + sederhanakan API dan hapus pesan manual mati kecuali diputuskan membangun UI alokasi-manual.

`allocations: [{id_order, amount}]` didukung API (4 pesan validasi) tetapi tidak ada UI yang
mengirimnya (form seksi maupun COD/terima/konfirmasi tak punya pemilih alokasi; helper E2E
`createPayment` pun tanpa alokasi — E2E 07 hanya FIFO). Satu-satunya jalan: request langsung.
Artinya kasir tak pernah bisa "bayar ini untuk order itu" dari aplikasi — selalu FIFO.

**Yang perlu dikonfirmasi:** butuh UI alokasi-manual (fitur baru kecil di seksi bayar), atau
kunci FIFO-saja (dan sederhanakan API + hapus pesan manual mati)? Jangan biarkan API tanpa
jalan UI tanpa keputusan.

---

### KI-105 — Ambang buka 0,009 vs bayar 0 + arsip longgar non-terminal ✅

**Tingkat: rendah — inkonsistensi ambang dan cakupan.**

Dua hal sekelas: (1) baris "terbuka" memakai ambang 0,009 (sisa Rp 0,005 dibulatkan = 0 tetap
dianggap terbuka? — tepatnya sisa ≤ 0,009 dilewati), sementara penolakan "sudah lunas" memakai
0 murni. Sisa Rp 0,004: tak bisa dibayar per-order (lunas!) tetapi masih dihitung terbuka di
saldo/FIFO. (2) Arsip menolak hanya terminal-non-net yang jadi-berutang; arsip yang membuat
non-terminal jadi-berutang (atau terminal-net) bebas — saldo "kembali" yang sebenarnya
mundur ke belum-lunas.

**Yang perlu dikonfirmasi:** satukan ambang (0,009 di mana-mana, sejalan anti-recehan) dan
tentukan: bolehkah arsip membuat order jadi-berutang di luar terminal-non-net? Keduanya
perbaikan kecil dengan efek saldo.

---

### KI-106 — Metode tanpa validasi + tanggal bebas + bukti-timpa yatim ✅

**Tingkat: rendah — tiga kelonggaran satu tempat.**

1. `payment_method` tanpa cek runtime (string sampah tersimpan; tipe TS saja).
2. `payment_date` bebas masa lalu/depan (backdate seperti terima/serah — konsisten, tetapi
   memengaruhi umur-bucket + overdue + `allocated_at`).
3. Ganti bukti menimpa path: file lama yatim di storage (tanpa hapus/catat), riwayat lampiran
   hilang.

**Yang perlu dikonfirmasi:** (1) validasi enum di server? (2) tanggal dibatasi (tak-boleh-depan
seperti retur-beli +60dk, atau bebas seperti sekarang)? (3) hapus objek lama / simpan riwayat
bukti? Ketiganya murah; pilih per butir.

---

## Modul 15 — POS / Kasir (+ Shell Android)

Sumber: [15-pos/](15-pos/). Mekanika order/bayar/member/stok milik 07/09/14/16.

### KI-107 — Kasir order.create tanpa payment.create = order yatim ✅

**Tingkat: tinggi — transaksi setengah jadi tanpa jalan kembali di POS.**

Guard halaman `/pos` hanya `order.create`. Tetapi checkout tunai memanggil `deliver-goods`
`pay_now` yang menuntut `payment.create` — tanpanya 403 **setelah** order POS jadi dibuat.
Hasil: order yatim (dibuat, tak terserah, tak terbayar) yang hanya bisa diselesaikan dari
halaman Order. Mekanisme tertunda mendeteksi cart-berubah, bukan izin-hilang.

**Yang perlu dikonfirmasi:** gabungkan syarat (tuntut keduanya untuk buka `/pos`, atau
sembunyikan tunai bila tanpa izin bayar), atau biarkan + latih kasir ke halaman Order?
Jangan biarkan 403-tengah-transaksi.

---

### KI-108 — Tiga kode mati: Termin-tersembunyi, Wide, `if (false)` ✅

**Tingkat: rendah — beban + jebakan baca.**

1. Blok Termin + Metode di ringkasan cart dibungkus `display:none` — satu-satunya jalan via
   dialog (duplikasi logika mode/metode/tempo yang harus dijaga dua tempat).
2. `PosCartItemRowWide` (tata-lebar + `+{n} lokasi`) tanpa pemanggil.
3. Blok konfirmasi-inline `if (false && ...)` (Batal + Proses + spinner) mati total.

**Yang perlu dikonfirmasi:** buang ketiganya (usulan), atau ada rencana menghidupkan (mis.
Wide untuk tablet landscape)? Khusus blok tersembunyi: bila dialog adalah jalan resmi,
hapus duplikasinya agar tak ada dua kebenaran.

---

### KI-109 — Quote POS menimpa harga manual (form order menghormati) ✅

**Tingkat: sedang — inkonsistensi antar dua kasir.**

Form order menandai `priceEdited` dan melewati baris itu saat requote. POS tanpa konsep itu:
setiap quote (ganti pelanggan!) menimpa `unitPrice` semua baris yang ter-quote (diskon hanya
dijepit, bukan dipertahankan-niatnya). Kasir yang negosiasi harga lalu ganti pelanggan
kehilangan negosiasinya diam-diam.

**Yang perlu dikonfirmasi:** tiru `priceEdited` (hormati-manual + klem seperti order), atau
kunci timpa-semua sebagai perilaku POS (cepat, tanpa pengecualian)? Beda watak boleh, tetapi
harus diputuskan sadar.

---

### KI-110 — Quote-gagal memblokir tunai walk-in? + cart-refresh-hilang 🟡

**Tingkat: perlu keputusan — dua sifat keras.**
**Keputusan analis:** Blokir quote-gagal hanya bila pelanggan terpilih (tunai walk-in bebas) + cart persisten-lokal atau kunci-jangan-refresh sebagai kontrak yang dilatihkan.

1. `pricingQuoteError` memblokir **semua** checkout termasuk tunai-tanpa-pelanggan (yang tak
   butuh quote sama sekali!). Gangguan quote (= master harga) melumpuhkan kasir tunai.
2. Cart + kunci-reservasi + meterai di memori: refresh = hilang + reservasi-yatim-600dk +
   order-tertunda-lupa (mekanisme anti-ganda tak selamat-refresh!).

**Yang perlu dikonfirmasi:** (1) blokir hanya bila pelanggan-terpilih (tunai bebas)? (2) cart
persisten-lokal (dengan risiko sidik-basi) atau kunci-refresh-sebagai-kontrak (latih kasir
jangan-refresh)? Keduanya menyentuh uang-di-meja.

---

## Modul 16 — Stock / Inventory & Gudang

Sumber: [16-stock-inventory/](16-stock-inventory/).

### KI-111 — Kode lokasi arsip dipakai ulang → 500 🟡

**Tingkat: sedang — sekelas KI-50/KI-71, modul ketiga dengan pola sama.**
**Keputusan analis:** Samakan ke pola pihak/KI-71: reserve kode lintas arsip + pesan "(ada di Sampah)" + jaga restore, jangan 500.

Cek aplikasi hanya non-arsip (`409` bila bentrok aktif), tetapi unique key
`uq_stock_locations_code (cabang, kode)` mencakup arsip. Pakai ulang kode lokasi terarsip
lolos cek → 500 DB. Tiga modul (produk? — produk bisa-pakai-ulang; kategori/member/lokasi
tidak-bisa) kini tiga sifat; lokasi = member = 500-diam.

**Yang perlu dikonfirmasi:** satu keputusan seragam semua master (reserve-lintas-arsip +
pesan-Sampah seperti pihak, atau izinkan-pakai-ulang seperti produk)? Jangan biarkan tiga
sifat.

---

### KI-112 — Update lokasi tak bisa pindah induk + nama gudang dua-pemilik ✅

**Tingkat: sedang — form tak bisa, API pun tak bisa.**

`locations/update` tidak menerima `id_parent_location` — struktur pohon hanya bisa dibangun
saat buat, tak bisa dirapikan sesudahnya (salah-letak = arsip + buat-ulang + stok dipindah
manual!). Bersamaan: nama gudang-default ditulis modul cabang setiap simpan (KI-42) —
pemilik ganda tanpa pemilik tunggal.

**Yang perlu dikonfirmasi:** butuh pindah-induk (dengan aturan stok: kosongkan/pindahkan
dulu seperti buat?), atau kunci-pohon-selamanya? Dan: siapa pemilik nama gudang — cabang
atau stok (tutup KI-42 sekalian)?

---

### KI-113 — Arsip lokasi tanpa-pakai bersaldo + susun-ulang tanpa-aman ✅

**Tingkat: rendah — dua operasi senyap.**

1. Arsip menolak anak-berisi dan stok-berisi, tetapi **tidak** menolak lokasi yang dipakai
   transaksi berjalan (reservasi aktif, alokasi, draft transfer!) — arsip di tengah jalan
   membuat referensi ke lokasi-arsip (daftar menyaring, transaksi menunjuk).
2. Susun-ulang drag-drop tanpa konfirmasi + tanpa-toast-gagal-spesifik (hanya sukses/gagal
   umum) — salah-geser diam-diam mengubah urutan petik (!) yang menentukan alokasi otomatis.

**Yang perlu dikonfirmasi:** (1) cek pemakaian sebelum arsip (reservasi/alokasi/draf)? (2)
konfirmasi + pesan-spesifik untuk susun-ulang, mengingat urutan = prioritas alokasi?

---

### KI-114 — Stok-awal tanpa-jaring-tanggal + template-tanpa-id + lewati-diam 🟡

**Tingkat: perlu keputusan — tiga sifat migrasi.**
**Keputusan analis:** Tambah tanggal-dokumen buka + template ber-id + laporan per-baris (masuk/lewati-sebab), atau kunci-ketiganya sebagai kontrak bila migrasi selesai.

1. `return_date`? — tidak ada tanggal-dokumen buka (movement = kini!): migrasi kemarin
   tercatat hari-ini (beda terima/serah/retur yang bisa-backdate; konsisten vs KI-80?).
2. Template tanpa id (hanya kode!): produk ganti-kode pasca-download = baris-gagal (nama
   hanya-peringatan!).
3. Terisi-dilewati-diam: upload-ulang file-sama melompati yang sudah-masuk tanpa rinci
   per-baris-masuk-vs-lewati (hanya hitungan!).

**Yang perlu dikonfirmasi:** (1) perlu tanggal-dokumen buka? (2) template pakai id + kode?
(3) laporan per-baris hasil (masuk/lepas/dilewati-sebab)? Atau kunci-ketiganya?

---

### KI-150 — Guard qty lolos-hilang/`NaN` → aritmetika-`NaN` ✅

**Tingkat: tinggi — tanpa-qty bisa mencemari stok.**

Guard `data.quantity <= 0` di adjust (`apps/api/src/modules/stock/stock.service.ts:1156`) dan transfer (`apps/api/src/modules/stock/stock-transfer.service.ts:368`) lolos untuk qty-hilang/`NaN` karena `undefined <= 0` bernilai `false`, sehingga permintaan tanpa-qty lanjut ke aritmetika-`NaN`.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-151 — Koreksi-nol diizinkan-FE ditolak-BE ✅

**Tingkat: sedang — koreksi-fisik-ke-nol via UI selalu-400.**

FE mengizinkan qty-0 untuk Koreksi-Fisik (`min="0"`, `stock-adjustment-page.tsx:189`) tetapi BE menolak qty ≤ 0, sehingga koreksi-ke-nol via UI pasti-400.

**Yang perlu dikonfirmasi:** mana yang benar — BE bolehkan nol untuk tipe koreksi-fisik, atau FE larang nol? Pertahankan tolak-nol (maka FE samakan batasnya), atau dukung koreksi-ke-nol (maka BE longgarkan guard untuk tipe itu)?

---

### KI-152 — Tipe-gerakan liar jatuh-ke-timpa-koreksi ✅

**Tingkat: tinggi — tipe tak dikenal jadi timpa-bebas.**

`movement_type` apa pun selain `in`/`out` (mis. `set` mentah) jatuh ke cabang `else` = perilaku-timpa-koreksi (`apps/api/src/modules/stock/stock.service.ts:1181-1188`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-153 — `stock/detail` baca-lintas-perusahaan + saring-tanpa-arsip ✅

**Tingkat: tinggi — baca lintas-perusahaan mungkin.**

`stock/detail` mencari produk tanpa filter perusahaan (`findOne({id, archived}`) (`apps/api/src/modules/stock/stock.service.ts:1106`) sehingga baca-lintas-perusahaan mungkin; filternya juga hanya `status active` tanpa `archivedAt`, tidak seperti daftar (`apps/api/src/modules/stock/stock.service.ts:1115-1117`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-154 — Halaman/batas-tanpa-bawah di daftar-transfer + movements ✅

**Tingkat: rendah — hanya lewat API langsung.**

`stock/transfers/list` + `stock/movements`: `page` tanpa-batas-bawah dan `limit` tanpa-batas-bawah (limit-0 = take-0, page-negatif = skip-negatif) (`apps/api/src/modules/stock/stock-transfer.service.ts:67-68`, `apps/api/src/modules/stock/stock.service.ts:1264-1265`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-155 — `locations/update` abaikan-induk → pindah-diam ✅

**Tingkat: sedang — pohon tak bisa dirapikan, susun-ulang diam.**

`locations/update` mengabaikan `id_parent_location` (`apps/api/src/modules/stock/stock-location.service.ts:226-261`) sehingga pindah-induk dan susun-ulang lintas-level diam-tak-pindah, padahal FE `reorderLocations` mengirim `parentLocationId` (`apps/web/src/store/slices/stock.slice.ts:184-189`).

**Yang perlu dikonfirmasi:** butuh pindah-induk (dengan aturan kosongkan/pindahkan dulu seperti saat buat?), atau kunci-pohon? Dan susun-ulang lintas-level didukung atau ditolak-dengan-pesan?

---

### KI-156 — Lokasi-default tanpa-jalur-set ✅

**Tingkat: rendah — konsep default tanpa cara memilih.**

`locations/create` selalu `isDefault: false` (`apps/api/src/modules/stock/stock-location.service.ts:200-211`) — tak ada jalur API menjadikan lokasi-default.

**Yang perlu dikonfirmasi:** butuh cara menandai lokasi-default, atau buang konsep default?

---

### KI-157 — Cek-stok pakai-`onHand` + `reserved`-nyangkut-di-induk ✅

**Tingkat: sedang — arsip/pindah lolos saat reservasi > 0.**

Cek stok lokasi (`getStockQtyAtLocation`) menjumlahkan `onHand` saja (`apps/api/src/modules/stock/stock.service.ts:276-280`) sehingga arsip/pindah-induk lolos saat `reserved > 0`; `force_transfer` juga membiarkan `reserved` nyangkut di induk yang dikosongkan (`available = max(0, available − qty)`) (`apps/api/src/modules/stock/stock-location.service.ts:123-126`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-158 — Koreksi-`adjustment` bisa buat-`available`-negatif ✅

**Tingkat: sedang — tersedia bisa negatif saat ada reservasi.**

`adjust` mode-`adjustment` menghitung `available = after − reserved` tanpa-guard (`apps/api/src/modules/stock/stock.service.ts:1190-1192`) sehingga `available` bisa negatif saat ada reservasi.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-159 — Reservasi-`consumed` tanpa-pemanggil + `expired` catat-kolom-salah 🟡

**Tingkat: rendah — kontrak mati + kolom salah-nama.**
**Keputusan analis:** Pertahankan consumed/id_order sebagai kontrak order/kasir + dokumentasikan pemakainya; expired tetap memakai releasedAt yang didokumentasikan.

Status reservasi `consumed` + kolom `id_order`/`consumed_at` tak punya pemanggil di modul ini (konsumsi milik order/kasir di luar berkas) (`apps/api/src/modules/stock/stock.service.ts:401-437`); reservasi `expired` mencatat `releasedAt`, bukan kolom khusus (`apps/api/src/modules/stock/stock.service.ts:439-456`).

**Yang perlu dikonfirmasi:** pertahankan `consumed`/`id_order` sebagai kontrak untuk order/kasir (maka pemakainya didokumentasikan), atau buang? Dan `expired` perlu kolom sendiri atau tetap pakai `releasedAt`?

---

### KI-160 — `hold` lewati non-`tracked` diam-diam → 200-kosong 🟡

**Tingkat: rendah — tanpa pesan, tanpa baris.**
**Keputusan analis:** Pertahankan lewati non-tracked tetapi kembalikan penanda dilewati/jumlah + dokumentasikan sebagai kontrak.

`hold` melewati produk-non-tracked diam-diam (`apps/api/src/modules/stock/stock.service.ts:476`) sehingga permintaan semua-non-tracked = 200 tanpa-baris.

**Yang perlu dikonfirmasi:** tolak-dengan-pesan (produk tak dilacak tak perlu reservasi), atau pertahankan lewati-diam (maka dokumentasikan sebagai kontrak)?

---

### KI-161 — `location_path` saran selalu satu-elemen ✅

**Tingkat: rendah — bukan path-penuh.**

Saran-alokasi `location_path` selalu `[nama]` satu-elemen, bukan path-penuh (`apps/api/src/modules/stock/stock.service.ts:369`).

**Yang perlu dikonfirmasi:** perlu path-penuh berjenjang, atau nama-tunggal memang cukup (maka ganti nama field-nya)?

---

### KI-162 — Agregat-`detail` pakai identitas baris-pertama ✅

**Tingkat: rendah — identitas agregat menyesatkan.**

Agregat `detail` memakai identitas baris-pertama (id/lokasi = saldo-`id ASC`-pertama) (`apps/api/src/modules/stock/stock.service.ts:1120-1131`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat agar tidak ikut tersalin ke sistem baru.

---

### KI-163 — FE transfer-dokumen selalu satu-baris 🔁

**Tingkat: sedang — kemampuan multi-baris API tak terpakai.**
**Keputusan analis:** Pertahankan FE satu-baris sebagai kontrak operasional; kemampuan multi-baris API tetap tanpa UI baru.

FE dokumen-transfer selalu satu-baris (`transferStock` mengirim `items: [satu]`) walau API mendukung multi-baris (`apps/web/src/store/slices/stock.slice.ts:133-138`).

**Yang perlu dikonfirmasi:** butuh form multi-baris di UI, atau kunci-satu-baris sebagai kontrak?

---

### KI-164 — Entity lokasi tanpa-`Unique`-DB, andal-guard-saja ✅

**Tingkat: rendah — duplikat hanya dijaga aplikasi.**

Entity `stock_locations` tanpa `@Unique` DB — duplikat-kode hanya dijaga guard-409 aplikasi; pakai-ulang kode-arsip aman-di-aplikasi — cek migrasi (`apps/api/src/modules/stock/entities/stock-location.entity.ts:1-55`).

**Yang perlu dikonfirmasi:** tambah unique-DB + putuskan seragam nasib kode-arsip (cadangkan vs boleh-pakai-ulang), atau pertahankan guard-aplikasi?

---

## Modul 17 — Finance / Accounting & Pajak

Sumber: [17-finance/](17-finance/). Analisis lengkap ada di berkas modul tersebut.

### KI-115 — Ubah-nilai-pasca-posting hanya peringatan, jurnal tak berubah 🔁

**Tingkat: perlu keputusan — kebenaran laporan vs kelancaran operasional.**
**Keputusan analis:** Replikasi snapshot: jurnal adalah momen posting dan selisih dikoreksi manual periode berjalan, bukan bekukan posting.

`sourceHash` sengaja **tidak** memuat jumlah, sehingga koreksi nilai operasional
(diskon susulan, salah ketik nominal) setelah diposting hanya memicu peringatan
`Source operasional berubah setelah diposting...` — jurnal tidak ikut berubah.
Ubah tipe/tanggal/jumlah/pihak memicu pesan review terpisah.

**Yang perlu dikonfirmasi:** pertahankan (jurnal = momen posting, selisih
dikoreksi manual periode berjalan) atau bekukan posting bila nilai berubah?
Atau ambang selisih (mis. > Rp X atau > Y%) yang memicu tahan?

---

### KI-116 — Tutup-paksa tercatat di audit tapi tanpa penanda di UI ✅

**Tingkat: tinggi — aksi paling berbahaya modul ini tidak terlihat.**

Tombol `Tutup Paksa` (hak `finance.close.force`) mengunci periode yang belum
siap dan menulis audit, tetapi halaman tidak menampilkan banner/catatan
"ditutup paksa" — tercatat diam-diam. Konsultan pajak yang me-review kemudian
tidak bisa membedakan periode bersih vs periode paksa tanpa membuka audit log.

**Yang perlu dikonfirmasi:** tambah penanda "ditutup paksa + alasan + oleh"
di halaman Tutup Periode, atau pertahankan diam?

---

### KI-117 — Notice tanpa-hak menyebut kode izin mentah ✅

**Tingkat: rendah — bocoran istilah teknis ke user awam.**

Halaman Biaya Usaha menampilkan `Butuh izin {perm}` (mis. kode
`finance.expense.create`) saat user tak punya hak tulis, padahal bahasa UI
modul ini sengaja awam-usaha ("Pembukuan", "Siap direkap").

**Yang perlu dikonfirmasi:** ganti kalimat manusiawi ("Minta pemilik mengaktifkan
hak catat biaya") atau pertahankan kode mentah?

---

### KI-118 — Direktori `apps/e2e/tests/finance/` tidak ada ✅

**Tingkat: rendah — klaim dokumentasi basi.**

`knowledge/MODULE_FINANCE.md` §9 merujuk skenario E2E di
`apps/e2e/tests/finance/` (termasuk skenario dedicated tutup-harian), tetapi
direktori tersebut **tidak ada** — skenario finance tersebar sebagai berkas
bisnis (`03/09/13/14/15`). Klaim "skenario dedicated" tidak terbukti.

**Yang perlu dikonfirmasi:** perbaiki knowledge (ikuti struktur aktual) atau
kembalikan direktori dedicated? (Dampak: E2E 15-tutup-buku-daily-close tetap
berlaku apa pun strukturnya.)

---

### KI-119 — Tarif PPN & logika DPP bukan di finance ✅

**Tingkat: sedang — pemilik logika pajak tidak jelas.**

Laporan pajak finance hanya membaca `tax_amount` tersimpan + menandai faktur
tanpa nomor; tarif PPN dan fallback DPP tidak ditemukan di modul ini
(kemungkinan milik modul 11/12 penjualan/pembelian).

**Yang perlu dikonfirmasi:** modul mana pemilik tunggal tarif & DPP-fallback?
Rebuild harus tahu sebelum merekonstruksi SPT.

---

### KI-120 — Kunci audit aksi batal-biaya belum dikonfirmasi ✅

**Tingkat: rendah — kelengkapan jejak.**

Pembatalan biaya usaha menjanjikan jurnal pembalik + jejak
(`Dibatalkan: ... Karena: ...`), tetapi kunci `actionKey` audit untuk aksi
batal belum dibaca langsung (baru `business_expense.create` yang pasti).

**Yang perlu dikonfirmasi:** (verifikasi kode 5 menit) — bila tidak ada audit
batal, tambah atau sengaja?

**Hasil verifikasi (analisis modul 20):** kunci `business_expense.cancel` ADA
(`business-expense.service.ts:450`) — bukan issue, tertutup tanpa tindakan.

---

### KI-121 — Daftar tanpa `limit` = semua baris ✅

**Tingkat: rendah — risiko banjir data.**

Beberapa daftar (GL, tutup-hari, laporan) bersifat opt-in: tanpa `limit`
dikembalikan semua; pengaman hanya batch-5000 sync dan paging UI 200/500.
Perusahaan besar dengan ratusan ribu jurnal bisa membanjiri respons.

**Yang perlu dikonfirmasi:** tambah cap server (mis. 5000 + pesan) atau
pertahankan (kasus belum terjadi)?

---

### KI-122 — Worksheet final yang salah = jalan buntu ✅

**Tingkat: tinggi — tidak ada jalan koreksi sama sekali.**

Worksheet penyesuaian pajak berstatus `final` tidak bisa diubah, tidak bisa
diarsipkan ("buat catatan baru jika perlu koreksi") — tetapi buat baru dengan
kode periode sama **juga ditolak** (`Catatan untuk periode {kode} sudah ada`,
termasuk yang terarsip). Salah-final = kode periode terkunci permanen tanpa
jalan koreksi di dalam sistem.

**Yang perlu dikonfirmasi:** tambah aksi "buka-kembali final ber-alasan"
(seperti periode), atau izinkan arsip-final, atau pertahankan (koreksi di luar
sistem)?

---

## Modul 19 — Reporting

Sumber: [19-reporting/](19-reporting/). Analisis lengkap ada di berkas modul tersebut.

### KI-123 — `total_sales` mencakup pembelian & pembatalan ✅

**Tingkat: sedang — nama berbohong bila filter longgar.**

`total_sales` di `reporting/orders` = SUM atas seluruh baris terfilter tanpa
memilah jenis: bila pemanggil tak memfilter `order_kind: 'sales'`, nominal
order pembelian (dan pembatalan) ikut dijumlah sebagai "sales".

**Yang perlu dikonfirmasi:** batasi ke sales saja di server, atau pertahankan
(JUMLAHKAN-YANG-DIMINTA) dan dokumentasikan + ganti nama di sistem baru?

---

### KI-124 — Kritis-per-baris vs kritis-per-produk ✅

**Tingkat: sedang — dua definisi "stok kritis" berbeda.**

`reporting/stock` (`critical_only`) membandingkan tiap baris saldo lokasi
dengan ambang produk, sedangkan dashboard memakai total per produk dibanding
ambang (knowledge melarang versi per-lokasi untuk dashboard!). Produk dengan
total cukup tapi satu lokasi tipis muncul di reporting tapi tidak di dashboard.

**Yang perlu dikonfirmasi:** samakan ke definisi per-produk, atau pertahankan
beda (dengan label berbeda)?

---

### KI-125 — Cron kemarin-menggunakan-tanggal-UTC ✅

**Tingkat: rendah — pergeseran batas hari vs WIB.**

Komentar kode menyebut "server time", tetapi tanggal agregat diambil dari
`toISOString` (UTC) sementara operasional memakai WIB: order 00:00–07:00 WIB
masuk tanggal-kemarin-UTC yang salah, dan `DATE(order_date)` memakai zona DB.

**Yang perlu dikonfirmasi:** seragamkan ke WIB (batas hari ikut operasional),
atau pertahankan UTC (konsisten dengan fungsi DATE DB)?

---

### KI-126 — Tabel metrik harian tulisan-tanpa-pembaca ✅

**Tingkat: perlu keputusan — hapus, hidupkan, atau ganti.**

`daily_operational_metrics` ditulis cron tiap hari (komentar migrasi: "tidak
dipakai di MVP") tetapi nol SELECT di seluruh backend maupun frontend —
tidak ada endpoint baca, dashboard tidak memakainya.

**Yang perlu dikonfirmasi:** hapus tabel + cron di sistem baru, atau hidupkan
(bangun endpoint/konsumen), atau ganti mekanisme agregat?

---

### KI-186 — Agregat reporting tanpa pembulatan rupiah ✅

**Tingkat: sedang — pecahan sen tampil padahal tak bisa ditagih.**

`reporting.service.ts:51` (`Number(totalSales)`) dan cron `metrics-job.service.ts:83–84` (`Number(row.total)`) tanpa `money()` maupun `roundRupiah()` (`packages/shared-types/src/money.ts:17–18`); kolom cron `DECIMAL(18,2)` (`001_baseline.sql:416`) membulatkan diam-diam saat tulis.

**Yang perlu dikonfirmasi:** satukan ke satu kebijakan rupiah (utuh) di sistem baru, atau pertahankan mentah?

---

### KI-187 — Kritis cron tanpa filter lokasi & perusahaan ✅

**Tingkat: sedang — metrik bisa over-hitung vs dashboard.**

Hitungan kritis cron di `metrics-job.service.ts:92–103` hanya memakai `ib.id_branch + p.stock_tracked + p.min_stock_qty + available + p.archived`, TANPA `stock_locations.status/archived` yang disyaratkan dashboard (`dashboard.service.ts:168–169,210–211`) dan TANPA `p.id_company`. Lokasi arsip/nonaktif ikut terhitung kritis di metrik tetapi tidak di dashboard/laporan-stok.

**Yang perlu dikonfirmasi:** samakan ke filter dashboard (lokasi aktif + perusahaan), atau pertahankan longgar?

---

### KI-188 — Injeksi `metricRepo` tak dipakai ✅

**Tingkat: rendah — kode mati, tak berpengaruh perilaku.**

`metrics-job.service.ts:23–24` meng-inject `DailyOperationalMetric` tetapi semua tulis via `dataSource.query` mentah (`61,92,106`); satu-satunya pemakaian di spec adalah mock `find` yang tak pernah dipanggil.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis.

---

### KI-189 — Presisi `generated_at` 3 vs 6 ✅

**Tingkat: rendah — beda presisi stempel saja.**

Migrasi memakai `DATETIME(3)` (`001_baseline.sql:418`) sedangkan entity memakai `datetime precision 6` (`daily-operational-metric.entity.ts:35–36`).

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis.

---

### KI-190 — Batas-hari `DATE()` vs rentang mentah ✅

**Tingkat: sedang — order tengah-malam bisa masuk hari berbeda.**

Laporan order memakai perbandingan mentah inklusif `order_date >=/:` di `reporting.service.ts:35–36` (zona = pengirim), sedangkan cron memakai `DATE()` server di `metrics-job.service.ts:68`. Order `2026-05-12T00:30:00+07` bisa masuk hari berbeda di kedua laporan.

**Yang perlu dikonfirmasi:** seragamkan ke WIB seperti finance, atau pertahankan beda?

---

### KI-191 — E2E menutupi kaveat `total_sales` ✅

**Tingkat: rendah — test selalu lolos padahal kaveat ada.**

`total_sales` mencakup pembelian/batal tanpa filter (KI-123!) tetapi E2E selalu memfilter `order_kind:'sales'` (`10-observability…:26–32`) sehingga kaveat tak pernah terlihat di test. Tanpa mengubah KI-123!

**Yang perlu dikonfirmasi:** pertahankan kontrak + perbaiki cakupan test, atau ganti nama/kontrak di sistem baru?

---

## Modul 20 — Audit Log

Sumber: [20-audit-log/](20-audit-log/). Analisis lengkap ada di berkas modul tersebut.

### KI-127 — Peta label basi dua arah (mati + hilang) ✅

**Tingkat: sedang — review pemilik melihat teks salah/format-mentah.**

Inventarisasi 98 pemakaian vs `ACTION_LABELS` menemukan: (a) label untuk kunci
yang tak ada di backend — `company.update` (aslinya `company.profile.update`!),
`delivery_note.create/update` (aslinya `delivery.*`!), `category.*` (aslinya
`product_category.*`!), `payment.upload_proof` (aslinya `payment.proof_uploaded`!),
`finance.period.close` (aslinya `finance.period.safe_close`!), `status.*` +
`transition.*` (order-status tak menulis audit!), `user.archive`,
`stock.adjust.approve` ([PERLU KONFIRMASI] keduanya — tak ditemukan di backend!);
(b) kunci nyata tanpa label — `member_type.*`, `customer_address.*`,
`purchase_return.create`, `finance.opening.supplement`, `finance.posting.close_day`,
`product.bulk_import`, `sales_return.dispatch/confirm_replacement` (tampil fallback
`Member - Type - Create` dst + tipe tanpa label entitas).

**Yang perlu dikonfirmasi:** sinkronkan peta (hapus mati + tambah hilang), atau
kunci keduanya apa adanya?

---

### KI-128 — Kolom Keterangan selalu default; before/after tak tampil ✅

**Tingkat: sedang — data ditulis tapi tak pernah dibaca manusia.**

Adapter mengisi `description = actionKey` sehingga `formatDescription` selalu
mengembalikan `Aktivitas tercatat otomatis.` — kolom Keterangan tak pernah
menampilkan apa pun, dan JSON `before/after/metadata` (yang ditulis ~semua
pemanggil!) tidak bisa dilihat di mana pun kecuali query DB langsung.

**Yang perlu dikonfirmasi:** tampilkan ringkas before→after di UI, atau hapus
kolom Keterangan + hentikan tulis JSON yang tak dibaca?

---

### KI-129 — Halaman tanpa filter; indeks hanya untuk perusahaan+cabang+waktu ✅

**Tingkat: rendah — keterbacaan + performa masa depan.**

Halaman selalu memuat tanpa filter (25/halaman) padahal API mendukung filter
cabang/entitas/aksi; indeks DB hanya `(id_company, id_branch, happened_at)` —
filter aksi/entitas (pola E2E!) full-scan di volume besar.

**Yang perlu dikonfirmasi:** tambah filter UI + indeks aksi/entitas, atau kunci
minimalis (halaman = 25 terbaru saja)?

---

### KI-130 — Path E2E observability di knowledge basi ✅

**Tingkat: rendah — sekelas KI-118.**

Knowledge §9 merujuk `apps/e2e/tests/audit-log/*` dan `dashboard/*`, tetapi
`apps/e2e/tests/` hanya berisi `business/` — cakupan audit/dashboard/reporting
hidup di `10-observability-permission-hardening.spec.ts`.

**Yang perlu dikonfirmasi:** perbaiki knowledge (ikuti struktur aktual)?

---

### KI-131 — Prefetch 100 user + gagal-diam → nama terdegradasi ✅

**Tingkat: rendah — degradasi sunyi.**

Slice memuat `users/list` (limit 100) + `audit-logs/list` (limit 100) dengan
catch-per-request → `{ items: [] }`: perusahaan >100 user melihat `User #{id}`
untuk sisanya, dan gagal jaringan terlihat sebagai "data kosong", bukan error.

**Yang perlu dikonfirmasi:** naikkan/tambahkan paginasi prefetch + tampilkan
error muat, atau pertahankan?

---

### KI-192 — Audit restatement ditulis SQL mentah di luar service ✅

**Tingkat: rendah — hanya jalur skrip sekali-jalan.**

Kunci `finance.cost_ledger.restatement` ditulis skrip SQL mentah (`rebuild-cost-ledger.ts:416-430`, aktor `system`, `NOW(6)`) — melewati `AuditLogService`, tanpa label UI, dan tanpa validasi panjang (`action_key` VARCHAR(100) vs string ini 31 char — aman, tetapi pola tulis-langsung-SQL tak terdokumentasi di mana pun).

**Yang perlu dikonfirmasi:** wajibkan tulis lewat `AuditLogService` + tambah label UI, atau pertahankan pola skrip-SQL-langsung dan dokumentasikan sebagai pengecualian?

---

### KI-193 — Satu aksi mengipas jadi N+2 baris; gagal sebagian tak ikut rollback ✅

**Tingkat: sedang — riwayat mengembang dan bisa terpotong sebagian.**

Satu aksi operasional (`closeDay`) mengipas jadi N+2 baris audit: 1× `finance.posting_source.sync` (via `syncSources` di `finance-posting.service.ts:901`) + N× `finance.journal.post` (satu per source, `finance-posting.service.ts:927-933`) + 1× `finance.posting.close_day` (`finance-posting.service.ts:944-952`). Deliver-goods menulis 3 baris berurutan (`payment.create` → `order.approve_credit` → `order.goods_delivered`, `delivery.service.ts:570-625`); receive-goods analog (`goods-receipt.service.ts:373-424`). Bila audit ke-2/ke-3 gagal di pola DALAM-transaksi, audit ke-1 yang sudah terlanjur tersimpan terpisah tak ikut rollback (mengikuti BR-19 — perlu test kegagalan untuk membuktikan).

**Yang perlu dikonfirmasi:** pertahankan satu-baris-satu-kejadian apa adanya, atau kelompokkan tampilan + jadikan penulisan auditnya atomis?

---

### KI-194 — Dua kunci nyata tampil fallback karena labelnya mati/salah nama ✅

**Tingkat: sedang — teks review salah/menyesatkan.**

`company.profile.update` (kunci nyata, `company.service.ts:70`) vs label `company.update` (mati): profil perusahaan selalu tampil sebagai fallback `Company - Profile - Update` di UI. Sama untuk `delivery.proof_uploaded` (nyata, `delivery-proof.service.ts:111`) vs label `payment.upload_proof` (mati, nama mirip-menyesatkan!).

**Yang perlu dikonfirmasi:** sinkronkan peta (tambah label untuk kunci nyata + hapus/selaraskan label mati), atau kunci fallback apa adanya?

---

### KI-195 — Baris tanpa stempel terlihat "baru saja" karena fallback waktu browser ✅

**Tingkat: rendah — hanya bila respons tanpa stempel, tapi stempelnya menyesatkan.**

`toAuditLog` mengisi `happenedAt` dengan waktu browser (`new Date().toISOString()`) bila respons tak membawa kedua varian field (`org.adapter.ts:47`) — baris tanpa stempel terlihat "baru saja terjadi" di UI, bukan `-` seperti kolom Waktu halaman (yang memakai `-` hanya untuk falsy — `utils.ts:25-28`). Kapan respons tanpa stempel terjadi masih perlu dicek.

**Yang perlu dikonfirmasi:** tampilkan `-` untuk baris tanpa stempel, atau pertahankan fallback waktu browser?

---

### KI-196 — Test tanpa-aktor bergantung waktu-palsu global ✅

**Tingkat: rendah — hanya kerapuhan test, tak menyentuh produksi.**

Test `toAuditLog` tanpa-aktor mengandalkan waktu-sistem-palsu global (`adapters.test.ts:137-138`) — cakupan `describe`-nya perlu dicek sebelum mengandalkan bahwa fallback waktu selalu deterministik.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat di sini agar tidak terlewat saat rebuild.

---

### KI-197 — `list()` tak menjaga page/limit non-positif ✅

**Tingkat: rendah — hanya lewat API langsung.**

`list()` tak memvalidasi `page`/`limit` selain cap-atas (`audit-log.service.ts:44-45`): `page: 0` → `skip()` negatif, `limit: 0` → `take(0)` + `meta.limit: 0`, `limit` negatif lolos `Math.min` — respons MySQL untuk OFFSET negatif perlu dicek (kemungkinan 500). Frontend/slice aman (halaman memakai 25, slice 100 — keduanya non-positif tak pernah dikirim).

**Yang perlu dikonfirmasi:** cukup diseragamkan validasinya ke helper bersama di sistem baru, atau pertahankan perilaku cap-atas saja?

---

## Modul 18 — Dashboard

Sumber: [18-dashboard/](18-dashboard/). Analisis lengkap ada di berkas modul tersebut.

### KI-132 — Fallback workspace berlogika beda tanpa penanda ✅

**Tingkat: tinggi — angka diam-diam berbeda saat API gagal.**

Bila `dashboard/summary` gagal, halaman memakai hitungan store lokal yang
logikanya BERBEDA: kritis per-baris-lokasi (vs agregat-produk server!),
tren tanpa-retur (vs net server!), prioritas tanpa-skor (5 terbaru vs skor!),
tanpa penanda "data lokal/sementara" — pemilik bisa mengira angka resmi.

**Yang perlu dikonfirmasi:** tambah penanda + samakan logika, atau hilangkan
fallback (tampilkan error), atau pertahankan?

---

### KI-133 — Tiga batas-hari berbeda (server vs zona-perusahaan vs UTC) ✅

**Tingkat: sedang — "hari ini" bisa tiga arti.**

Default backend = hari kalender server; halaman mengirim rentang naive
`YYYY-MM-DD HH:mm:ss` zona perusahaan (diartikan zona DB!); E2E memakai ISO-UTC;
skor prioritas memakai `CURRENT_DATE` DB. Beda zona = order tengah-malam bisa
masuk hari berbeda antar angka.

**Yang perlu dikonfirmasi:** seragamkan semua ke zona perusahaan (WIB), atau
dokumentasikan bedanya?

---

### KI-134 — Field diambil tapi tak ditampilkan ✅

**Tingkat: rendah — payload mati.**

`order_summary.completed/cancelled` dan `total_purchase/sales_transactions`
(dihitung 2 query!) tidak dirender di mana pun di halaman.

**Yang perlu dikonfirmasi:** tampilkan (kartu baru?), atau hapus dari respons?

---

### KI-135 — Persen margin 0 untuk produk rugi ✅

**Tingkat: rendah — persen berbohong saat rugi.**

`estimated_margin_percent = 0` bila omzet ≤ 0 — produk merugi tampil
"0%", bukan persen negatif (nominal margin negatifnya tampil benar!).

**Yang perlu dikonfirmasi:** tampilkan persen negatif apa adanya, atau
pertahankan 0 (dengan arti "tak-terdefinisi")?

---

### KI-179 — Uang 2-desimal vs rupiah utuh ✅

**Tingkat: sedang — agregat bisa beda dari angka transaksi.**

Semua nominal dashboard memakai `money()` 2-desimal di `dashboard.service.ts:31–33`, sedangkan order/payment/retur memakai `roundRupiah()` utuh di `packages/shared-types/src/money.ts:17–18` (dipakai `order.service.ts:89–90`, `payment.service.ts:809–810`, `order-pricing.service.ts:94–95`, `sales-return.service.ts:142–143`). Selisih ≤ Rp 0,50 per angka; `total_purchase/sales_transactions.amount` di `dashboard.service.ts:511–539` bahkan tanpa `money()` sama sekali.

**Yang perlu dikonfirmasi:** samakan ke rupiah utuh di sistem baru, atau pertahankan 2-desimal?

---

### KI-180 — Order tanpa tempo kalah prioritas tanpa penjelasan 🟡

**Tingkat: sedang — urutan tindak-lanjut bisa menyesatkan.**
**Keputusan analis:** Beri aturan eksplisit untuk order tanpa tempo + tampilkan penjelasan di UI, jangan kalah diam-diam.

Order dengan `due_date NULL` jatuh ke cabang skor `pending→60 else 40` di `dashboard.service.ts:344–351` dan alasan jatuh ke status di `dashboard.service.ts:391–399` dengan `days_until_due` null. Artinya order tanpa tempo bisa kalah dari order bertempo jauh tanpa penjelasan di kode.

**Yang perlu dikonfirmasi:** bolehkah order tanpa tempo kalah prioritas dari order bertempo jauh, atau perlu aturan sendiri?

---

### KI-181 — `id_inventory_balance` kritis tak dipakai navigasi ✅

**Tingkat: rendah — field mati, tak merusak tampilan.**

Stok kritis mengisi `id_inventory_balance = MIN()` per produk di `dashboard.service.ts:194`, tetapi halaman menavigasi ke `/stock/{idProduk}` di `dashboard-page.tsx:605` sehingga id saldo itu tak dipakai navigasi.

**Yang perlu dikonfirmasi:** hapus dari respons di sistem baru, atau pertahankan sebagai kontrak?

---

### KI-182 — Tooltip tren Rp ≠ 0 dengan 0 order ✅

**Tingkat: rendah — tooltip menyesatkan, hitungan uang tetap benar.**

`order_count` tren selalu 0 untuk bulan yang hanya berisi retur (`dashboard.service.ts:306, 320–328`), sehingga tooltip bisa menampilkan nominal ≠ 0 dengan 0 order.

**Yang perlu dikonfirmasi:** pertahankan (retur bukan order), atau hitung retur sebagai order / pisahkan penanda?

---

### KI-183 — Lingkup pantau vs kritis beda cabang-perusahaan ✅

**Tingkat: sedang — dua angka seolah sebanding padahal beda lingkup.**

`tracked_product_count` lingkup perusahaan di `dashboard.service.ts:401–409` sedangkan `critical_stock_count` lingkup cabang+perusahaan; halaman melabeli keduanya seolah satu cabang (`dashboard-page.tsx:419–421` vs `397–402`). Di perusahaan multi-cabang angka pantau ≠ angka kritis.

**Yang perlu dikonfirmasi:** samakan ke lingkup cabang, atau pertahankan beda dengan label berbeda?

---

### KI-184 — Fallback knowledge menutupi "belum siap" ✅

**Tingkat: rendah — status kesiapan tertutup angka total.**

Halaman memakai `knowledge_ready_count ?? knowledge_document_count` di `dashboard-page.tsx:295–298`: bila siap = 0 tetapi total > 0, yang tampil total bukan 0, sehingga kondisi "ada dokumen tapi belum siap" tertutup.

**Yang perlu dikonfirmasi:** tampilkan angka siap apa adanya + penanda, atau pertahankan fallback?

---

### KI-185 — Nilai `ready` WA tanpa sumber backend ✅

**Tingkat: rendah — label status bisa salah.**

Halaman menganggap `ready` sebagai terhubung di `dashboard-page.tsx:302`, tetapi backend hanya meneruskan `sessionStatus` mentah di `dashboard.service.ts:428–435`; entity yang dibaca hanya mengenal `connected/reconnecting/disconnected` (`whatsapp-channel.entity.ts:18`), sehingga asal nilai `ready` tak jelas.

**Yang perlu dikonfirmasi:** dari mana `ready` berasal, dan nilai apa saja yang boleh dianggap terhubung di sistem baru?

---

## Modul 21 — Assistant AI + WhatsApp + Knowledge/RAG

Sumber: [21-assistant-whatsapp-knowledge/](21-assistant-whatsapp-knowledge/).

### KI-136 — Cek kanal simulate-vs-nyata terbalik? ✅

**Tingkat: sedang — simulate wajib connected, inbound nyata bebas.**

`if (status!=='connected' && !sendViaGateway) throw` membuat simulate
(`sendViaGateway:false`) wajib connected sementara pesan WA nyata diproses
meski `disconnected/reconnecting` — kemungkinan operand tertukar.

**Yang perlu dikonfirmasi:** tukar (nyata wajib, simulate bebas) atau
pertahankan (nyata = antrean-tahan-gangguan)?

---

### KI-137 — Pengirim asing ditolak senyap total 🟡

**Tingkat: sedang — keamanan vs UX.**
**Keputusan analis:** Balas penolakan sopan generik dengan rate-limit ketat tanpa membocorkan daftar nomor, jangan sunyi total.

Pesan dari nomor tak-terotorisasi/revoked ditelan `catch {}` tanpa balasan
apa pun (kecuali rate-limit yang justru dibalas!) — pengirim sah yang belum
terdaftar mengira BOT mati.

**Yang perlu dikonfirmasi:** balas penolakan sopan (`Nomor belum diotorisasi...`),
atau pertahankan sunyi (hindari oracle-enumerasi nomor)?

---

### KI-138 — Status dokumen menipu + pipeline diam-diam ✅

**Tingkat: tinggi — badge `Siap` berbohong + data hilang diam-diam.**

Create/update selalu tulis `ready` walau pipeline `pending`/async; PDF tanpa
teks → skip diam + `pending` selamanya; update tanpa file menimpa teks dengan
kosong; badge `Diproses/Gagal` praktis unreachable; audit arsip hardcode
`before:{status:'ready'}`.

**Yang perlu dikonfirmasi:** status jujur (processing→ready/failed + polling UI)
+ tolak PDF-tanpa-teks + jangan timpa-teks-tanpa-file, atau kunci semua?

---

### KI-139 — Angka hitung dibatasi 5 diam-diam ✅

**Tingkat: sedang — `Order pending: 5` padahal 50.**

`pendingOrderCount`/`criticalStockCount` di jawaban = panjang array ber-limit-5,
bukan hitungan sebenarnya — tanpa kata "5 teratas" atau total.

**Yang perlu dikonfirmasi:** hitung terpisah (angka benar + daftar 5), atau
tambah kata "teratas", atau pertahankan?

---

### KI-140 — Info produk praktis kode-saja ✅

**Tingkat: sedang — tanya nama hampir pasti gagal.**

Cabang LIKE tak-terjangkau (kode selalu dikirim!) + over-strip kata
(`Item Spesial` → `Spesial`!) — `Info produk "Kopi Susu"` menjawab tak-ketemu
walau produk ada; kapabilitas mengklaim bisa ditanya.

**Yang perlu dikonfirmasi:** hidupkan pencarian nama (LIKE/fuzzy) + perbaiki
strip, atau ubah kapabilitas ("tanya pakai kode")?

---

### KI-141 — Mode efektif & kuota tak terlihat; simulate/process tanpa UI ✅

**Tingkat: rendah — downgrade diam + fitur tanpa tombol.**

Downgrade AI (tanpa-key/kuota-50) tak tampil di UI; simulate, process-ulang,
detail-mentah, update-dokumen hanya via API.

**Yang perlu dikonfirmasi:** tampilkan mode-efektif + sisa-kuota + tombol
uji/antre-ulang, atau pertahankan minimal?

---

### KI-142 — Sesi Baileys at-rest tanpa enkripsi ✅

**Tingkat: sedang — kredensial di disk predictabel.**

`storage/whatsapp-sessions/company-{id}` menyimpan kredensial tanpa enkripsi;
path predictabel; scope paparan via static-serve/backup belum diverifikasi.

**Yang perlu dikonfirmasi:** enkripsi-at-rest + kunci path, atau terima risiko
(akses-server = akses-penuh)?

---

### KI-143 — Path E2E assistant di knowledge basi ✅

**Tingkat: rendah — sekelas KI-118/130.**

Knowledge §9 merujuk `apps/e2e/tests/assistant/*`; aktual di `business/`
(E2E 19 + bagian E2E 10).

**Yang perlu dikonfirmasi:** perbaiki knowledge?

---

### KI-165 — Normalisasi mode preview-vs-inbound berbeda ✅

**Tingkat: rendah — hanya jalur preview superadmin, jawaban tetap sah.**

Preview hanya menerima `ai_assisted` persis (`assistant.service.ts:577-579`), sementara config/inbound menerima alias `ai`/`hybrid` (`whatsapp.service.ts:486-489`). Akibatnya `mode:"ai"` via `assistant/preview` jatuh ke `rule_based` diam-diam meski config tersimpan `ai`.

**Yang perlu dikonfirmasi:** samakan preview agar menerima alias `ai`/`hybrid` seperti inbound, atau pertahankan preview ketat (hanya `ai_assisted` persis)?

---

### KI-166 — Preview melewati kuota AI harian 🟡

**Tingkat: rendah — hanya superadmin, tapi batas tidak diperiksa di jalur ini.**
**Keputusan analis:** Preview superadmin tetap bebas kuota harian tetapi dibatasi rate + dicatat terpisah dari kuota publik.

Preview tidak memeriksa kuota harian (`assistant.service.ts:581-584`), sementara inbound memeriksa `COUNT(bot_ai hari-ini) < ai_daily_limit` (`whatsapp.service.ts:519-549`). Preview superadmin bisa memakai AI melewati `ai_daily_limit`.

**Yang perlu dikonfirmasi:** preview ikut cek kuota harian, atau preview memang bebas kuota karena hanya superadmin?

---

### KI-167 — Kuota AI lolos-terus bila batas non-numerik ✅

**Tingkat: sedang — satu nilai salah menonaktifkan pembatas biaya AI diam-diam.**

`resolveEffectiveMode` memakai `Number(prefs.ai_daily_limit ?? 50)` (`whatsapp.service.ts:530`): bila tersimpan string non-numerik, hasilnya `NaN` dan `usedToday >= NaN` selalu false — kuota tidak pernah tercapai.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat di sini agar tidak terlewat saat rebuild.

---

### KI-168 — Urutan info produk memakai properti relasi ✅

**Tingkat: rendah — perlu verifikasi di dialek produksi.**

`getProductInfo` memakai `.orderBy('p.productName', 'ASC')` (`tools.service.ts:270`) — properti relasi camel, bukan kolom basis data. Tergantung dialek, pengurutan bisa diabaikan diam-diam atau melempar galat.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat di sini agar tidak terlewat saat rebuild.

---

### KI-169 — Stok kritis dimuat semua lalu dipotong di memori ✅

**Tingkat: sedang — aman untuk SKU sedikit, berat bila ribuan SKU kritis.**

`GetCriticalStock` tanpa `take`/limit SQL (`tools.service.ts:191-235`): seluruh baris kritis dimuat, lalu `slice(0,5)` di format dan `items.length` dipakai untuk angka komposit. Beban query/memori tumbuh sebesar data kritis.

**Yang perlu dikonfirmasi:** batasi di SQL + hitung total terpisah di rebuild, atau pertahankan muat-semua (cukup untuk skala sekarang)?

---

### KI-170 — Angka komposit performa/operasional ikut terpotong 5 ✅

**Tingkat: sedang — dashboard BOT menampilkan maks 5 walau data 50.**

`GetTodayPerformance`/`GetOperationalSummary` menghitung pending/kritis dari `items.length` yang sudah ber-limit-5 (`tools.service.ts:249-254,303-315`); hanya `getSalesSummary` yang berhitung benar di SQL.

**Yang perlu dikonfirmasi:** hitung terpisah (angka benar + daftar 5), tambah kata "teratas", atau pertahankan?

---

### KI-171 — Thread lama tidak mengikuti cabang simulate baru ✅

**Tingkat: rendah — hanya simulate multi-cabang ke nomor yang sama.**

`findOrCreateThread` hanya menulis `idBranch` saat thread dibuat (`whatsapp.service.ts:399-431`): pesan simulate cabang-B lalu cabang-C ke nomor sama tetap memakai thread/cabang lama untuk histori — jawaban bisa memakai konteks cabang yang salah.

**Yang perlu dikonfirmasi:** perbarui/pecah thread saat cabang simulate berubah, atau pertahankan satu thread per nomor?

---

### KI-172 — Tunggu QR 8 detik berakhir diam-diam ✅

**Tingkat: rendah — connect tampak sukses padahal QR belum tentu ada.**

`waitForQrOrConnection` timeout 8 detik tanpa galat (`whatsapp-gateway.service.ts:257-267`): connect mengembalikan sukses-tanpa-QR, dan UI hanya tahu lewat polling 3-detik. Pemanggil API tidak bisa membedakan "QR siap" dari "waktu habis".

**Yang perlu dikonfirmasi:** kembalikan status eksplisit (menunggu/timeout) di rebuild, atau pertahankan sukses + polling?

---

### KI-173 — Status reconnecting hilang saat QR terbit ✅

**Tingkat: rendah — UI tidak mengandalkan state ini.**

`handleConnectionUpdate` saat QR menulis `sessionStatus:'disconnected'` (`whatsapp-gateway.service.ts:189-192`): keadaan `reconnecting` hilang tepat saat QR terbit — UI selamat karena mengandalkan `showQr`, bukan state.

**Yang perlu dikonfirmasi:** tulis status jujur (`reconnecting`/menunggu-scan) di rebuild, atau pertahankan karena UI tidak memakainya?

---

### KI-174 — Non-teks (pdf/docx/xlsx) jatuh ke ringkasan diam-diam ✅

**Tingkat: sedang — isi dokumen tidak terindeks tapi badge berkata Siap.**

`extractRawText` hanya menangani txt/md/csv/json (`knowledge.service.ts:218-224`): pdf/docx/xlsx selalu jatuh ke fallback (judul/ringkasan UI) tanpa pesan ke pemanggil, dan UI tetap menampilkan badge `Siap`.

**Yang perlu dikonfirmasi:** tolak dengan pesan jelas (atau OCR) di rebuild, atau pertahankan fallback diam-diam?

---

### KI-175 — Pipeline tanpa-teks macet pending/ready selamanya ✅

**Tingkat: sedang — dokumen tampak Siap padahal tak terindeks.**

`runPipeline` return-dini tanpa mengubah status bila teks kosong (`knowledge.service.ts:334-339`): versi macet `pending`/`pending` sementara dokumen tertulis `ready` selamanya — tanpa pesan dan tanpa jalan keluar selain hapus/unggah ulang.

**Yang perlu dikonfirmasi:** tulis status jujur (gagal/ditolak + pesan ke pemanggil) di rebuild, atau pertahankan perilaku legacy?

---

### KI-176 — Unik telepon hanya di migrasi, entity tanpa @Unique ✅

**Tingkat: rendah — balapan tulis bisa berakhir 500 basis data.**

Keunikan `(id_company, phone_e164)` ada di migrasi 003 (`003_phase3_assistant_mvp.sql:14`) tapi entity tanpa `@Unique` — perlindungan hanya mengandalkan basis data + cek di kode, sehingga tulis bersamaan lolos cek lalu gagal sebagai galat teknis.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat di sini agar tidak terlewat saat rebuild.

---

### KI-177 — Audit arsip hardcode before ready ✅

**Tingkat: rendah — jejak audit berbohong untuk arsip non-ready.**

`audit.archive` menulis `before:{status:'ready'}` hardcode (`knowledge.service.ts:176`): mengarsipkan dokumen dari status apa pun tercatat seolah dari `ready` — riwayat tidak bisa dipercaya untuk kasus ini.

**Yang perlu dikonfirmasi:** tidak ada yang perlu diputuskan — ini murni perbaikan teknis. Dicatat di sini agar tidak terlewat saat rebuild.

---

### KI-178 — Kirim teks gugur-diam bila sesi hilang ✅

**Tingkat: sedang — balasan (termasuk pesan rate-limit) bisa hilang tanpa jejak.**

`sendText` return-diam bila tanpa sock (`whatsapp-gateway.service.ts:92-96`): bila sesi hilang di tengah jalan, balasan rate-limit maupun outbound nyata gugur tanpa galat, tanpa log, tanpa antrean.

**Yang perlu dikonfirmasi:** antrekan/ulangi kirim saat sesi kembali (minimal catat log) di rebuild, atau pertahankan gugur-diam?

---

## Modul 17 — Finance (tambahan dari pendalaman)

Enam temuan berikut muncul saat modul 17 diperdalam (setelah KI-115…KI-122 yang lebih dulu
tercatat). Sumber: [17-finance/](17-finance/).

### KI-144 — Ekspor berkas pajak tanpa validasi rentang tanggal sama sekali ✅

**Tingkat: tinggi — bisa menjatuhkan server produksi.**

Endpoint ekspor paket berkas pajak menerima **rentang tanggal apa pun** tanpa satu pun pemeriksaan:

| Yang dikirim | Yang terjadi |
|---|---|
| Tanggal kosong | Diterima |
| Tanggal terbalik (akhir < awal) | Diterima |
| Rentang 5 tahun | Diterima |

Setelah diterima, delapan sheet dibangun **seluruhnya di memori**: laba rugi, neraca, neraca saldo,
**buku besar penuh**, dua rincian PPN, dan worksheet penyesuaian. Buku besar penuh untuk rentang
bertahun-tahun bisa berisi ratusan ribu baris.

Server produksi hanya punya **2 GB RAM**. Modul Order punya batas 366 hari / 5.000 baris untuk
ekspornya; Finance **tidak punya batas apa pun**.

**Yang perlu dikonfirmasi:** apakah pernah ada ekspor yang menggantung atau membuat aplikasi
lambat/mati? Dan berapa rentang terpanjang yang benar-benar Anda butuhkan sekali unduh?

---

### KI-145 — Dua laporan PPN memakai rumus DPP berbeda, hasilnya tidak rekonsiliasi ✅

**Tingkat: tinggi — angka pajak yang tidak cocok, dan keduanya masuk berkas yang sama.**

Bila sebuah order kena pajak punya nominal PPN tetapi **tarif pajaknya 0 atau kosong**:

| Laporan | DPP yang ditampilkan |
|---|---|
| **Ringkasan PPN** (halaman Pajak) | **Rp 0** |
| **Rincian PPN per Faktur** | **Subtotal sebelum pajak** (nilai sebenarnya) |

Contoh: order dengan PPN Rp 110.000, subtotal Rp 1.000.000, tarif snapshot 0 → ringkasan
menampilkan DPP Rp 0, rincian menampilkan DPP Rp 1.000.000.

Keduanya masuk **paket berkas pajak yang sama** (sheet berbeda), sehingga konsultan pajak akan
menemukan dua angka DPP yang berbeda untuk periode yang sama.

Tarif pajak sendiri **tidak pernah jadi pengaturan sistem** — ia diketik per order di modul Order
dan bawaannya 0. Jadi kondisi ini muncul setiap kali operator lupa mengisi tarif pada order kena
pajak.

**Yang perlu dikonfirmasi:** pernahkah konsultan pajak Anda menanyakan selisih DPP antara ringkasan
dan rincian? Dan mana yang benar menurut Anda — DPP nol, atau subtotal?

---

### KI-146 — Pembatalan biaya dini hari menghasilkan jurnal bertanggal kemarin ✅

**Tingkat: rendah — hanya terjadi pukul 00:00–07:00 WIB.**

Seluruh modul Finance memakai kalender **Asia/Jakarta** untuk menentukan periode dan nomor
dokumen — kecuali satu tempat: perhitungan "hari ini" pada **pembatalan biaya usaha**, yang masih
memakai **UTC**.

Akibatnya, membatalkan biaya antara pukul 00:00 dan 07:00 WIB menghasilkan jurnal pembalik
bertanggal **hari sebelumnya**. Bila hari sebelumnya itu ada di bulan yang sudah dikunci, jurnalnya
tetap terbentuk (penjagaan periode memeriksa periode jurnal **asli**, bukan tanggal pembalik).

**Yang perlu dikonfirmasi:** tidak perlu — ini jelas perlu diseragamkan ke WIB. Dicatat agar tidak
ikut tersalin ke sistem baru.

---

### KI-147 — Tipe akun dan arah saldo bebas diubah meski akun sudah punya ratusan jurnal ✅

**Tingkat: tinggi — satu klik bisa membalik seluruh laporan historis.**

Alur ubah akun menerima perubahan **tipe akun** (harta/hutang/modal/pendapatan/biaya) dan **arah
saldo normal** (debit/kredit) kapan pun, **tanpa penjagaan apa pun** — termasuk untuk akun yang
sudah dipakai ratusan baris jurnal.

Mengubah akun "Penjualan" dari tipe pendapatan jadi tipe biaya akan membalik laporan Untung Rugi
untuk **seluruh riwayat**, seketika, tanpa peringatan.

Kontrasnya tajam dengan penjagaan lain di modul yang sama:

| Aksi | Penjagaan |
|---|---|
| Mengarsipkan akun yang pernah berjurnal | **Ditolak** |
| Melepas akun berjurnal dari mapping | **Ditolak** |
| **Mengubah tipe akun yang berjurnal** | **Diizinkan tanpa syarat** |

Dua penjagaan pertama ada justru untuk melindungi laporan historis — yang ketiga membiarkan pintu
yang jauh lebih lebar terbuka.

**Yang perlu dikonfirmasi:** apakah tipe akun pernah diubah setelah dipakai? Bila belum pernah,
penjagaannya bisa langsung ditambahkan di sistem baru tanpa mengganggu kebiasaan siapa pun.

---

### KI-148 — Harga modal dihitung per produk, bukan per varian ✅

**Tingkat: sedang — HPP meleset untuk produk bervarian yang harga belinya berbeda.**

Buku besar harga modal disimpan per **(cabang, produk)**. Kolom varian ada di tabel mutasinya,
tetapi **tidak ikut membentuk identitas keadaan** — sehingga seluruh varian satu produk berbagi
**satu** harga modal rata-rata.

Padahal **stoknya dipisah per varian** (modul Stok menghitung saldo per varian). Jadi untuk produk
yang harga belinya berbeda antar varian:

```
KAOS Merah  dibeli Rp 50.000
KAOS Biru   dibeli Rp 80.000
→ keduanya ber-HPP Rp 65.000 (rata-rata gabungan)
```

Margin per varian karena itu tidak akurat, dan nilai persediaan per varian tidak bisa dipisah.

**Yang perlu dikonfirmasi:** apakah ada produk bervarian yang harga belinya benar-benar berbeda
antar varian? Bila semua varian selalu seharga sama (warna/ukuran dengan harga seragam), masalah
ini tidak pernah muncul di praktik.

---

### KI-149 — Aksi paling sensitif di modul keuangan tidak meninggalkan jejak audit ✅

**Tingkat: tinggi — tidak ada cara menelusuri siapa melakukan apa.**

Modul Finance mencatat 17 `actionKey`, tetapi **justru aksi yang paling berkonsekuensi tidak
termasuk**:

| Aksi | Teraudit? |
|---|---|
| Ubah akun / mapping / kas | ✅ |
| Buat / kunci / buka periode | ✅ |
| Kunci & lengkapi saldo awal | ✅ |
| Catat & batalkan biaya usaha | ✅ |
| Tutup & buka periode pajak | ✅ |
| **Memposting sumber** (satu / massal / tutup hari) | ✅ (`finance.posting_source.sync`, `finance.journal.post`, `finance.posting.close_day` di `finance-posting.service.ts`) |
| **Membalik jurnal** | ✅ (`finance.journal.reverse`, baris 1152) |
| **Membatalkan posting** | ✅ (`finance.posting_source.cancel_posting`, baris 745) |
| **Mengabaikan / memulihkan sumber** | ✅ (`finance.posting_source.ignore`/`restore`, baris 644/682) |
| **Menarik transaksi (sinkronisasi)** | ✅ (`finance.posting_source.sync`, baris 164) |
| **Mengunduh paket berkas pajak** (Layer 1 maupun Layer 2) | ❌ (`finance-export.service.ts` tidak mengimpor `AuditLogService` sama sekali) |
| **Menyimpan / mengarsipkan penyesuaian pajak** | ✅ (`finance-tax-adjustment.service.ts` baris 126/176/198) |

Terverifikasi ulang dari kode: dari 7 baris yang dulu dilaporkan tanpa jejak, **6 sudah teraudit** —
hanya pengunduhan berkas pajak yang benar-benar tanpa jejak.

Untuk posting masih ada jejak parsial (kolom "diposting oleh" di barisnya), tetapi pembalikan,
pengabaian, dan **pengunduhan berkas pajak tidak meninggalkan apa pun**.

Kesenjangan yang paling perlu diputuskan adalah **unduh berkas pajak**: tidak ada catatan siapa
mengunduh versi mana, kapan — padahal ada dua versi berkas dengan isi berbeda (lihat OQ-A42).

Bandingkan modul 03 Company & Settings yang mencatat seluruh field sebelum **dan** sesudah untuk
perubahan pengaturan biasa.

**Yang perlu dikonfirmasi:** aksi mana yang menurut Anda wajib punya jejak? Saran saya minimal:
pembalikan jurnal, pembatalan posting, dan pengunduhan berkas pajak.

---

## Catatan Metodologis

Semua temuan di atas ditemukan dari **pembacaan kode**, bukan dari pengujian aplikasi berjalan.
Untuk temuan bertanda "belum terbukti terjadi" (KI-01, KI-02, KI-06, KI-10, KI-41, KI-44),
konfirmasi tercepat biasanya datang dari pengalaman Anda sendiri sebagai pemilik sistem — apakah
gejalanya pernah dilaporkan pengguna.

Temuan bersifat **keputusan produk**, bukan perbaikan teknis: KI-07 (halaman profil),
KI-08 (label mana yang benar), KI-11 (jejak login gagal), KI-12 (batas umur sesi),
KI-15 (menutup eskalasi hak atau tidak), KI-19 (penjagaan nonaktifkan diri sendiri),
KI-21 (dialog konfirmasi), KI-25 (menyeragamkan cara pencarian), KI-28 (nasib field mata uang),
KI-33 (nasib sistem feature flag), KI-35 (seberapa jauh alur status order perlu dikelola),
KI-36 (apakah cabang perlu bisa ditutup), KI-37 (nasib nomor telepon cabang),
KI-38 (arti penanda "Pusat"), KI-40 (boleh tidaknya kode cabang diubah),
KI-44 (normalisasi kode), KI-50 (normalisasi kode produk), KI-51 (kode kategori bisa diubah atau tidak),
KI-56 (arsip satu arah atau perlu pulihkan), KI-57 (definisi stok untuk arsip),
KI-58 (aturan relasi harga), KI-60 (kolom inline lama: peringatan atau error),
KI-62 (kode pihak bisa diubah atau tidak), KI-63 (buku alamat arsip ikut terkunci atau tidak),
KI-64 (retry kode auto), KI-65 (tokenized atau frasa), KI-66 (validasi nama/telepon/email),
KI-67 (member lestari: sengaja atau bocor), KI-68 (arah tombol Kembali), KI-69 (perlu keadaan
tanpa-default atau tidak),
KI-71 (kode member: reserve atau boleh-pakai-ulang), KI-72 (degradasi diam atau penanda +
eksplisit-menang), KI-73 (validasi pasangan mode-kelipatan + makna 0), KI-74 (pola URL +
filter status), KI-75 (endpoint detail atau plafon cukup), KI-76 (batas besaran/persen),
KI-77 (badge daftar atau hapus), KI-78 (normalisasi akhir-hari di server), KI-79 (sembunyikan
tombol tanpa sisa), KI-80 (backdate: batas atau kunci), KI-81 (buang field mati), KI-82 (nomor
darurat pernah terlihat? bulan ikut tanggal order?), KI-83 (peringatan invoice ganda +
default 30 hari), KI-84 (tolak/bangun-ulang/biarkan ganti jenis tanpa baris),
KI-85 (teks per konteks atau netral), KI-86 (hapus pengecualian atau hidupkan), KI-87 (unik
atau dokumentasikan-bebas),
KI-88 (buang atau uji + beritahu klien luar), KI-89 (butuh nomor atau tetapkan tanpa),
KI-90 (tetapkan satu presisi), KI-91 (tambah koreksi atau kunci-final + hapus kolom),
KI-92 (tambah lock atau terima risiko),
KI-93 (ketatkan API atau kunci-longgar), KI-94 (validasi server), KI-95 (disengaja +
token atau samakan), KI-96 (seragamkan akhir-hari, tutup KI-78), KI-97 (satu pola
dengan KI-92),
KI-98 (kas di luar sistem + rekap, atau tulis payments + migrasi), KI-99 (fitur batal atau
buang mati), KI-100 (seragamkan atau dokumentasikan beda),
KI-101 (buang izin mati atau bangun fitur batal), KI-102 (jalankan E2E 17! perbaiki kode atau
test), KI-103 (ikut KI-98, perhatian arah-kas),
KI-104 (UI alokasi atau kunci-FIFO), KI-105 (satukan ambang + batas arsip), KI-106 (enum,
batas tanggal, riwayat bukti — per butir),
KI-107 (gabung syarat atau latih), KI-108 (buang atau hidupkan + satu kebenaran),
KI-109 (tiru-hormat atau kunci-timpa), KI-110 (blokir-bersyarat + persisten-atau-kontrak),
KI-111 (satu keputusan semua master), KI-112 (pindah-induk + pemilik-nama), KI-113 (cek
pakai + konfirmasi-urut), KI-114 (tanggal-buka + id-template + rincian-baris),
KI-104 (UI alokasi atau kunci-FIFO), KI-105 (satukan ambang + batas arsip), KI-106 (enum,
batas tanggal, riwayat bukti — per butir),
KI-45 (apakah nomor perlu reset tahunan), KI-46 (keseragaman nomor mutasi stok).
KI-115 (peringatan-vs-beku untuk ubah-nilai-pasca-posting), KI-116 (penanda tutup-paksa),
KI-117 (kode izin mentah atau kalimat manusiawi), KI-118 (perbaiki knowledge atau
kembalikan direktori E2E), KI-119 (pemilik tunggal tarif & DPP-fallback), KI-120 (audit
batal-biaya), KI-121 (cap server atau pertahankan), KI-122 (buka-kembali final atau
arsip-final atau koreksi-luar).
KI-123 (total_sales khusus-sales atau jumlahkan-yang-diminta), KI-124 (samakan kritis
per-produk atau pertahankan beda), KI-125 (WIB atau UTC), KI-126 (hapus, hidupkan,
atau ganti tabel metrik).
KI-127 (sinkronkan peta label atau kunci), KI-128 (tampilkan before→after atau
hapus kolom + hentikan tulis JSON), KI-129 (filter UI + indeks atau kunci minimalis),
KI-130 (perbaiki path E2E observability di knowledge), KI-131 (prefetch + error muat
atau pertahankan degradasi sunyi).
KI-132 (penanda + samakan logika, atau hilangkan fallback, atau pertahankan),
KI-133 (seragamkan zona perusahaan atau dokumentasikan beda), KI-134 (tampilkan
atau hapus dari respons), KI-135 (persen negatif apa adanya atau pertahankan 0).
KI-136 (tukar cek kanal atau pertahankan), KI-137 (balas penolakan atau pertahankan
sunyi), KI-138 (status jujur + tolak PDF-tanpa-teks + jangan timpa, atau kunci),
KI-139 (hitung terpisah atau kata "teratas" atau pertahankan), KI-140 (pencarian
nama atau ubah kapabilitas), KI-141 (tampilkan mode/kuota/tombol atau minimal),
KI-142 (enkripsi-at-rest atau terima risiko), KI-143 (perbaiki path E2E assistant).

Temuan yang **berbagi akar yang sama** dan sebaiknya diperbaiki sekali di lapisan bersama:

| Akar | Temuan terkait |
|---|---|
| Pesan galat spesifik dibuang di lapisan antarmuka | KI-04 (modul 01), KI-17 (modul 02), seluruh notifikasi gagal modul 03, KI-43 (modul 04) |
| Perhitungan batas baris tanpa helper bersama | KI-26 (modul 02), KI-55 (modul 05), KI-70 (modul 06), dan modul 20 |
| Halaman merender kosong tanpa keadaan "sedang memuat" | KI-09 (modul 01), KI-34 (modul 03) |
| Fosil rancangan awal yang arahnya berubah | prefix `vioni`, field `guid`/`code`/`info` pada envelope, KI-33 (feature flag), KI-49 (endpoint cabang aktif) |
| Satu nilai disimpan di dua tempat tanpa pemilik tunggal | KI-42 (nama gudang default), KI-40 (prefix nomor disalin dari kode cabang) |
| Mekanisme lengkap di server tapi tombolnya tidak pernah dibuat | KI-33 (feature flag), KI-35 (ubah status order), KI-36 (tutup cabang), KI-37 (telepon cabang), KI-38 (tandai cabang pusat) |
| Data bergantung izin, lalu kegagalannya ditelan diam-diam | KI-47 (modul 04) — pola `catch` yang mengganti galat dengan daftar kosong dipakai di seluruh pemuatan awal |
| Aksi berbahaya tercatat di audit tapi tanpa penanda di UI | KI-116 (tutup-paksa finance) |
| Klaim dokumentasi yang kodenya sudah tidak ada | KI-118 (direktori E2E finance), prefix `vioni` (fosil, lihat baris di atas) |
