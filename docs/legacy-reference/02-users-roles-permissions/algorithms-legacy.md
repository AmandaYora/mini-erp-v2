# Algorithms Legacy — Modul 02 Users, Roles & Permissions

**Kelompok B — fokus pada HASIL yang diharapkan, bukan cara implementasinya.** Dokumen ini
menjelaskan *apa yang harus dicapai* tiap logika kunci, supaya sistem baru bisa mencapai hasil
yang sama dengan cara yang berbeda.

Untuk aturan yang **presisi dan wajib dipertahankan**, rujuk
[business-rules.md](business-rules.md) — dokumen ini sengaja tidak mengulanginya secara detail.

---

## 1. Peta Logika Kunci

| # | Logika | Hasil yang harus dicapai |
|---|---|---|
| A-01 | Penetapan role & cabang | Kondisi akhir persis sesuai yang dipilih admin — bukan hasil tambah/kurang bertahap |
| A-02 | Pembuatan kode role | Nama bebas dari pengguna menjadi identitas teknis yang stabil dan aman |
| A-03 | Penjagaan role cadangan | Akun developer tidak bisa dikunci, dan role itu tidak bisa disebar |
| A-04 | Penyembunyian akun & role istimewa | Pihak tak berhak tidak bisa menyimpulkan keberadaannya |
| A-05 | Pemutusan akses saat kredensial berubah | Perubahan status/password langsung memutus akses, kecuali sesi pelakunya sendiri |
| A-06 | Perlindungan role sistem | Bentuk role bawaan tetap, isinya boleh disesuaikan |
| A-07 | Validasi izin sebelum menulis | Kesalahan input tidak boleh merusak konfigurasi yang sudah benar |
| A-08 | Penyimpanan matriks izin | Setiap klik langsung berlaku, tampilan tidak pernah menipu |
| A-09 | Perlindungan role yang sedang dipakai | Role tidak bisa hilang dari bawah kaki penggunanya |

---

## A-01 — Penetapan Role & Cabang

**Hasil yang diharapkan:** setelah admin menekan simpan, daftar role dan cabang pengguna **persis
sama** dengan yang tercentang di form — tidak lebih, tidak kurang.

Cara mencapainya secara konsep: perlakukan daftar sebagai **kondisi akhir yang dideklarasikan**,
bukan sebagai perintah tambah/kurang. Sistem lama melakukannya dengan menghapus seluruh penetapan
lalu menyusun ulang dari daftar baru.

Ini pilihan yang tepat dan layak dipertahankan, karena menghilangkan seluruh kelas masalah
"hasilnya bergantung pada kondisi sebelumnya". Yang penting dijaga adalah dua propertinya:

| Properti | Efek yang harus tetap ada |
|---|---|
| Daftar **tidak dikirim** = tidak ada perubahan | Update sebagian (mis. hanya mengubah nama) tidak boleh mengosongkan role |
| Daftar **dikirim** = kondisi akhir dideklarasikan | Termasuk daftar kosong, yang berarti "tidak ada" |

Sistem lama membedakan keduanya dengan benar. Yang **tidak** dijaga adalah akibat dari properti
kedua, dan di sinilah masalah seriusnya.

**Dua lubang yang harus ditutup di sistem baru:**

**Lubang 1 — kondisi akhir kosong diterima tanpa pertanyaan.** Mengirim daftar role kosong akan
mengosongkan role pengguna, dan pengguna tanpa role **tidak bisa login** (login menolaknya).
Artinya satu penyimpanan yang tidak disengaja dapat mengunci seseorang dari sistem, dan pemulihannya
hanya bisa dilakukan admin lain. Hal yang sama berlaku untuk cabang: pengguna tanpa cabang bisa
login tetapi terjebak di halaman pemilihan cabang yang kosong.

Hasil yang seharusnya dicapai: **kondisi akhir yang membuat pengguna tidak bisa bekerja harus
ditolak, bukan disimpan.** Minimal satu role dan minimal satu cabang.

**Lubang 2 — kode role yang tidak dikenal dilewati diam-diam.** Bila daftar memuat kode yang tidak
ada, sistem menghapus semua role lalu **gagal menyisipkan** yang tidak dikenal itu — tanpa error,
tanpa peringatan. Jalur berbeda, akibat sama: pengguna tanpa role.

Hasil yang seharusnya dicapai: **rujukan yang tidak dapat diselesaikan adalah kegagalan, bukan
kondisi yang diabaikan.** Ini pola yang sama seperti yang ditemukan pada permission — di sana
sistem lama justru **sudah benar** (kode tak dikenal ditolak dengan pesan jelas sebelum apa pun
dihapus). Penanganan yang berbeda untuk masalah yang sama di dua tempat berdekatan adalah petunjuk
kuat bahwa yang di role adalah kelalaian, bukan keputusan.

**Satu detail yang perlu dipertahankan meski cara mencapainya boleh berubah:** tepat satu cabang
selalu menjadi cabang default pengguna. Sistem lama mencapainya sebagai efek samping — cabang
pertama dalam urutan array. Hasilnya benar (tidak pernah ada dua default), tetapi caranya membuat
pengguna tidak punya kendali dan tidak melihat tandanya. Di sistem baru: **hasil yang sama, tapi
dipilih secara sadar.**

---

## A-02 — Pembuatan Kode Role

**Hasil yang diharapkan:** nama role yang diketik bebas oleh pengguna berubah menjadi identitas
teknis yang aman dipakai di URL, API, dan perbandingan kebijakan — tanpa pengguna perlu memahami
konsep "kode".

Prinsip yang dicapai sistem lama:

| Tujuan | Bagaimana tercapai |
|---|---|
| Aman dipakai sebagai identitas teknis | Hanya huruf kecil, angka, dan garis bawah yang lolos |
| Stabil sepanjang umur role | Kode tidak pernah berubah meski nama diubah |
| Tidak menuntut pengetahuan teknis dari pengguna | Tidak ada field kode di form sama sekali |

Dua konsekuensi dari transformasi ini yang **wajib** tetap ditangani, apa pun cara barunya:

**Konsekuensi 1 — transformasi bersifat banyak-ke-satu, jadi bentrok mungkin terjadi.** "Kepala
Gudang", "kepala-gudang", dan "KEPALA GUDANG" semuanya menghasilkan identitas yang sama. Sistem
lama menolak yang kedua sebagai duplikat — hasil yang benar.

Yang belum baik adalah **cara menyampaikannya**: pesan penolakan menyebut kode (`kepala_gudang`)
yang pengguna tidak pernah lihat, sehingga ia melihat dua nama yang jelas berbeda ditolak sebagai
sama tanpa penjelasan. Hasil yang seharusnya dicapai: pengguna memahami **mengapa** namanya
ditolak, mis. dengan menyebut role mana yang sudah memakai identitas itu.

**Konsekuensi 2 — sebagian masukan menghasilkan identitas kosong.** Nama yang seluruhnya simbol,
spasi, atau huruf non-Latin habis tersaring. Sistem lama menolaknya, yang benar — tetapi
menggolongkannya sebagai "konflik" alih-alih "input tidak valid", sehingga pesan yang sampai ke
pengguna adalah kode error generik (lihat A-07 dan KI-17).

Satu hal yang perlu **diputuskan sadar**, bukan diwarisi: huruf beraksen dan non-Latin **dibuang**,
bukan ditransliterasi. "Área Manager" menjadi `rea_manager`. Untuk nama role berbahasa Indonesia
ini praktis tidak muncul, tetapi bila sistem baru perlu mendukungnya, aturannya harus dirancang —
bukan dibiarkan menghasilkan identitas yang terpotong aneh.

**Yang tidak perlu ditiru:** ruang nama kode digabung antara role global dan role perusahaan,
sehingga perusahaan tidak boleh membuat role berkode `admin`. Untuk sistem satu-perusahaan ini
sederhana dan aman. Bila multi-perusahaan pernah jadi target lagi, keputusannya perlu ditinjau.

---

## A-03 — Penjagaan Role Cadangan

**Hasil yang diharapkan:** dua jaminan yang harus berlaku bersamaan — akun developer **tidak
pernah** bisa terkunci dari sistem, dan role istimewanya **tidak pernah** bisa disebar ke akun
lain.

Cara sistem lama mencapainya: dua mekanisme yang bekerja dari arah berlawanan.

| Arah | Mekanisme | Hasil |
|---|---|---|
| Mencegah penyebaran | Kode role istimewa ditolak bila muncul di daftar yang dikirim | Tidak ada yang bisa memberikannya, termasuk ke diri sendiri |
| Mencegah terkunci | Role itu **selalu dipulihkan** untuk akun developer setelah penetapan | Apa pun yang dipilih di form, akun itu tetap memegangnya |

Mekanisme kedua adalah pengaman terakhir, dan ia bekerja **tanpa syarat**: bahkan bila seluruh
role akun developer dihapus, role istimewa itu kembali. Ini properti yang layak dipertahankan —
sistem selalu punya satu jalan masuk yang tidak bisa ditutup oleh kesalahan konfigurasi.

Efek samping yang perlu diketahui: form pengguna **menampilkan hasil yang berbeda dari yang
dipilih** untuk akun itu. Role istimewa bahkan tidak ada di daftar checkbox, tetapi akan tampil
melekat setelah disimpan. Di sistem baru, hasil yang sama sebaiknya dicapai dengan cara yang
**terlihat** — mis. menandai akun itu sebagai akun sistem yang rolenya tidak dapat diubah, alih-alih
menerima pilihan lalu menimpanya diam-diam.

Perbandingan yang perlu dipertahankan: pemeriksaan kode role dilakukan setelah **membuang spasi
dan menormalkan huruf besar/kecil**, sehingga variasi penulisan tidak bisa dipakai menembus
penjagaan.

**Satu titik lemah yang perlu diputuskan:** identitas akun developer adalah **angka yang ditanam
di kode**, bukan properti data. Bila basis data dibangun ulang dengan urutan berbeda, angka itu
bisa menunjuk akun lain. Di sistem baru, penanda "akun sistem" sebaiknya menjadi atribut data.

---

## A-04 — Penyembunyian Akun & Role Istimewa

**Hasil yang diharapkan:** pihak yang tidak berhak **tidak dapat menyimpulkan** keberadaan akun
atau role istimewa — bukan hanya dilarang mengaksesnya.

Ini prinsip yang dijalankan konsisten di enam titik berbeda, dan yang membedakannya dari
otorisasi biasa: **penolakan disamarkan sebagai "tidak ditemukan"**, bukan "tidak boleh". Perbedaan
itu penting — "tidak boleh" mengonfirmasi bahwa sesuatu ada di sana.

Hasil yang harus tetap tercapai:

| Dari sudut pandang pihak tak berhak | Yang ia lihat |
|---|---|
| Daftar pengguna | Akun istimewa **tidak ada** — termasuk tidak terhitung di total |
| Daftar role | Role istimewa **tidak ada** |
| Mencoba mengubahnya lewat API | "Tidak ditemukan" — identik dengan respons untuk id yang benar-benar tidak ada |

Detail yang membuatnya berhasil: penyaringan pada daftar pengguna terjadi **di dalam query**,
sehingga angka total ikut menyesuaikan. Bila penyaringan dilakukan setelah pengambilan data, total
akan membocorkan keberadaan baris yang disembunyikan — kebocoran halus yang mudah terlewat. Di
sistem baru, hasil yang sama harus dijaga: **angka apa pun yang ditampilkan tidak boleh
menghitung yang disembunyikan.**

Satu ketidakseragaman yang disengaja dan masuk akal: mencoba **memberikan** role istimewa ditolak
dengan pesan yang menyebutnya eksplisit. Alasannya jelas — pengirim sudah tahu nama role itu
karena ia sendiri yang mengirimnya, jadi tidak ada yang bocor.

---

## A-05 — Pemutusan Akses Saat Kredensial Berubah

**Hasil yang diharapkan:** ketika akses seseorang dicabut atau passwordnya diganti, ia
**benar-benar** kehilangan akses seketika — tidak menunggu sesinya berakhir sendiri.

Ini logika yang paling baik dirancang di modul ini, dan detailnya layak dipertahankan utuh.

| Peristiwa | Yang harus terjadi |
|---|---|
| Pengguna dinonaktifkan | Semua sesinya di semua perangkat langsung mati |
| Password pengguna diganti oleh admin | Semua sesinya mati — perangkat lama tidak bisa lanjut memakai password lama |
| Pengguna mengganti password **dirinya sendiri** | Semua sesi lain mati, **tetapi sesi yang sedang ia pakai tetap hidup** |
| Pengguna diaktifkan kembali | Sesi lama **tidak** dihidupkan — ia harus login ulang |

Pengecualian ketiga adalah yang paling penting dan paling mudah terlewat: tanpanya, seorang admin
yang mengganti passwordnya sendiri akan **langsung ter-logout oleh aksinya sendiri** — pengalaman
yang membingungkan dan membuat orang ragu mengganti password. Sistem lama menanganinya dengan
benar, dan ada test khusus untuknya.

Pengecualian keempat juga disengaja: mengaktifkan kembali seseorang **tidak** memulihkan sesinya.
Ini benar — sesi yang sudah dicabut harus tetap mati, karena pencabutan mungkin terjadi justru
karena perangkatnya hilang.

**Kontras yang perlu dipahami:** perubahan **role** dan **akses cabang** justru **tidak** mencabut
sesi — dan itu juga benar, karena hak akses di-resolve ulang setiap permintaan (modul 01 A-04).
Jadi perubahan itu sudah berlaku seketika tanpa perlu memutus sesi. Aturannya konsisten: **cabut
sesi hanya ketika yang berubah adalah hal yang tidak diperiksa ulang setiap permintaan** —
yaitu kredensial dan status akun.

Satu hasil yang **belum** tercapai: pengguna yang terkena dampak **tidak diberi tahu apa pun**. Ia
sekadar mendapati dirinya di halaman login tanpa penjelasan, dan admin yang melakukannya juga
tidak melihat konfirmasi berapa perangkat yang terputus — padahal angka itu **sudah dihitung dan
dikembalikan**, hanya tidak ditampilkan. Menampilkannya adalah perbaikan kecil dengan dampak
kejelasan yang besar.

**Satu lubang yang perlu ditutup:** tidak ada penjagaan terhadap **menonaktifkan diri sendiri.**
Seorang admin dapat menonaktifkan akunnya sendiri dan langsung terputus — dan bila ia satu-satunya
pemegang hak kelola pengguna, tidak ada yang bisa memulihkannya.

---

## A-06 — Perlindungan Role Sistem

**Hasil yang diharapkan:** role bawaan tetap ada dengan nama dan identitas yang dapat diandalkan,
tetapi **isinya** dapat disesuaikan dengan kebutuhan perusahaan.

Sistem lama membedakan dua jenis perubahan dengan tegas:

| Jenis perubahan | Role sistem | Alasan |
|---|---|---|
| Nama & deskripsi | **Ditolak** | Kode dan nama role bawaan dirujuk kebijakan, dokumentasi, dan kebiasaan pengguna |
| Penghapusan | **Ditolak** | Menghilangkannya akan memutus penetapan yang ada |
| **Daftar izin** | **Diizinkan** | Kebutuhan hak tiap perusahaan berbeda; "Admin" di satu tempat tidak sama dengan di tempat lain |

Pembedaan ini masuk akal dan layak dipertahankan: bentuknya tetap, isinya lentur.

Cara mencapainya di sistem lama sederhana dan elegan — pencarian role untuk operasi "ubah nama"
dan "hapus" mensyaratkan role itu **milik sebuah perusahaan**, sehingga role global tidak akan
pernah ditemukan. Efek sampingnya bagus: pesan penolakannya pun sudah berbunyi "Role kustom tidak
ditemukan", yang secara tidak langsung menjelaskan aturannya.

**Konsekuensi keamanan yang perlu diputuskan sadar.** Karena izin role sistem dapat diubah oleh
siapa pun yang memegang hak kelola role, dan role **admin** bawaan **memegang hak itu**, maka
seorang admin dapat menambahkan izin apa pun ke role yang ia sendiri pegang — termasuk izin
mengelola akun yang sengaja tidak diberikan kepadanya.

Pembagian tugas "admin mengatur struktur akses, owner mengelola akun orang" sudah dikonfirmasi
disengaja. Tetapi pembagian itu saat ini **hanya ditegakkan oleh konvensi**, bukan oleh sistem.
Hasil yang seharusnya dicapai bila pembagian itu memang dimaksudkan berlaku: **seseorang tidak
dapat memberikan izin yang tidak ia miliki sendiri**, atau sekelompok izin tertentu dikunci hanya
untuk pemilik.

Satu-satunya role yang **benar-benar** terlindungi izinnya adalah role cadangan developer — dan
itu memakai mekanisme penyembunyian A-04, bukan mekanisme A-06.

**Catatan tentang penghapusan role kustom:** karena role tidak punya status non-aktif, satu-satunya
cara menghilangkannya adalah **hard delete** — satu-satunya di seluruh sistem. Hasil yang dicapai
benar (role hilang), tetapi jejaknya juga hilang: satu-satunya bukti keberadaannya setelah itu
adalah entri riwayat aktivitas. Di sistem baru, status non-aktif untuk role akan mencapai hasil
yang sama tanpa kehilangan jejak.

---

## A-07 — Validasi Izin Sebelum Menulis

**Hasil yang diharapkan:** kesalahan input **tidak pernah** merusak konfigurasi yang sudah benar.

Ini contoh terbaik penanganan galat di modul ini, dan pola yang seharusnya dipakai di tempat lain
juga (bandingkan dengan A-01 lubang 2 yang justru sebaliknya).

Urutan yang dicapai:

```
1. Kumpulkan seluruh izin yang diminta, buang duplikat
2. Pastikan SETIAP kode dikenali sistem
3. Bila ada yang tidak dikenali → GAGAL, sebutkan yang mana, JANGAN sentuh apa pun
4. Baru setelah semuanya valid → ganti konfigurasi
```

Properti yang wajib dipertahankan: **langkah 3 tidak meninggalkan kerusakan apa pun.** Ada test
khusus yang memverifikasi bahwa penghapusan izin lama tidak terjadi ketika validasi gagal. Tanpa
properti ini, satu salah ketik kode izin akan mengosongkan seluruh hak sebuah role.

Pesan galatnya pun baik: menyebut **kode mana** yang tidak dikenali, bukan sekadar "input tidak
valid".

Yang **tidak** tercapai adalah penyampaiannya ke pengguna. Pesan yang bagus itu tidak pernah
sampai — antarmuka menampilkan kode mesin sebagai gantinya (lihat A-08 dan KI-17). Jadi kualitas
validasi di lapisan dalam terbuang di lapisan luar.

Satu hal yang **tidak** divalidasi dan perlu diputuskan: tidak ada batas minimum jumlah izin. Role
tanpa izin apa pun diterima dan tersimpan. Itu mungkin sah (role yang sengaja dikosongkan
sementara), tetapi berarti tidak ada perbedaan antara "sengaja kosong" dan "gagal tersimpan
sebagian" — dan yang kedua **mungkin terjadi** karena operasi ini tidak dilindungi transaksi
(lihat §2).

---

## A-08 — Penyimpanan Matriks Izin

**Hasil yang diharapkan:** setiap perubahan langsung berlaku, dan **tampilan tidak pernah menipu**
— apa yang terlihat tercentang adalah apa yang sebenarnya tersimpan.

Dua tujuan yang saling menegangkan dipenuhi bersamaan:

| Tujuan | Cara mencapainya |
|---|---|
| Terasa responsif | Tampilan berubah lebih dulu, sebelum server menjawab |
| Tidak menipu | Bila server menolak, tampilan **dikembalikan** dan data dimuat ulang |

Pola ini benar dan layak dipertahankan. Detail penting yang mudah terlewat: setelah gagal,
sistem tidak hanya mengembalikan tampilan ke nilai sebelumnya — ia **juga memuat ulang dari
server**. Itu penting karena nilai "sebelumnya" yang disimpan di memori bisa saja sudah tidak
akurat.

Detail kedua yang perlu dipertahankan: setiap perubahan mengirim **seluruh daftar izin** role itu,
bukan hanya yang berubah. Dengan begitu tidak ada ambiguitas tentang kondisi akhir — konsisten
dengan prinsip A-01. Ada test E2E khusus yang memverifikasi bahwa mencentang izin kedua tidak
menghapus yang pertama.

**Yang perlu diputuskan sadar:** tidak ada tombol simpan. Setiap klik adalah satu permintaan.
Untuk mengubah sepuluh izin, terjadi sepuluh permintaan berurutan, masing-masing menulis ulang
seluruh daftar izin role itu.

Untuk skala UKM ini tidak terasa. Yang menjadi masalah adalah **tidak ada titik konsisten di
tengahnya**: bila jaringan terputus setelah klik keenam, role berada dalam kondisi setengah
dikonfigurasi — dan karena operasinya tidak dilindungi transaksi, satu klik yang gagal di tengah
bahkan bisa meninggalkan role **tanpa izin sama sekali**.

Hasil yang lebih baik di sistem baru: kumpulkan perubahan, simpan sekali, dengan satu titik
keberhasilan/kegagalan yang jelas. Atau pertahankan simpan-otomatis tetapi jadikan setiap
penyimpanan atomik.

---

## A-09 — Perlindungan Role yang Sedang Dipakai

**Hasil yang diharapkan:** sebuah role tidak bisa hilang dari bawah kaki penggunanya.

Sistem lama mencapainya dengan pemeriksaan sederhana sebelum penghapusan: bila masih ada satu saja
pengguna yang memegang role itu, penghapusan ditolak dengan pesan yang menjelaskan alasannya.

Hasil ini penting karena akibat sebaliknya serius: pengguna yang kehilangan satu-satunya rolenya
**tidak bisa login lagi** (lihat A-01 lubang 1). Jadi pemeriksaan ini adalah satu-satunya tempat
di modul ini yang **berhasil** mencegah kondisi "pengguna tanpa role".

Ironinya, kondisi yang sama dapat dicapai dengan mudah lewat jalur lain — mengosongkan centang
role di form pengguna, yang **tidak** dijaga sama sekali. Jadi sistem lama menjaga pintu belakang
tetapi meninggalkan pintu depan terbuka.

Di sistem baru, hasil yang seharusnya dicapai adalah **invarian, bukan pemeriksaan per-jalur**:
"setiap pengguna aktif memegang minimal satu role" harus benar apa pun operasi yang dilakukan —
menghapus role, mengubah penetapan, atau apa pun yang belum terpikirkan.

Yang perlu diperbaiki dari penyampaiannya: pesan penolakan sudah jelas ("Role masih dipakai oleh
user dan tidak dapat dihapus"), tetapi tidak menyebut **siapa** yang memakainya — sehingga admin
harus mencari sendiri. Menyebut jumlah atau daftar penggunanya akan membuat pesan itu langsung
dapat ditindaklanjuti.

Satu hal lagi: penghapusan dijalankan **tanpa dialog konfirmasi**. Untuk operasi yang bersifat
permanen (hard delete) dan tidak dapat dibatalkan, ini menyimpang dari kebiasaan yang wajar.

---

## 2. Operasi Multi-Langkah Tanpa Perlindungan Transaksi

Perlu bagian sendiri karena memengaruhi tiga logika di atas (A-07, A-08, A-09).

Operasi pengguna (`create`, `update`, ubah status, ganti password) **semuanya** dijalankan dalam
transaksi — benar dan sesuai aturan proyek.

Operasi role **tidak**:

| Operasi | Langkah | Bila langkah kedua gagal |
|---|---|---|
| Hapus role | hapus izin → hapus role | Role tetap ada **tanpa izin apa pun** |
| Ubah izin role | hapus semua izin → sisipkan yang baru | Role kehilangan **seluruh** izin, tidak mendapat yang baru |

Kedua kondisi akhir itu **tidak dapat dibedakan** dari hasil yang sah (role yang sengaja
dikosongkan), sehingga kegagalan seperti ini bisa berlangsung tanpa disadari — sampai seseorang
melaporkan menu yang hilang.

Hasil yang harus dicapai di sistem baru: **operasi ini bersifat satu-atau-tidak-sama-sekali.**
Caranya boleh apa pun (transaksi basis data, penulisan yang idempoten, atau operasi tunggal yang
mengganti seluruh himpunan sekaligus) — yang penting tidak ada kondisi setengah jadi yang
tersimpan.

---

## 3. Penyampaian Galat: Kualitas yang Terbuang di Lapisan Luar

Pola yang berulang di modul ini dan perlu diangkat sebagai satu temuan.

Lapisan dalam menghasilkan pesan galat yang **baik dan spesifik**:

- "Username sudah digunakan"
- "Email sudah digunakan"
- "Role masih dipakai oleh user dan tidak dapat dihapus"
- "Kode role 'kepala_gudang' sudah digunakan"
- "Permission tidak dikenal: whatsapp.simulate"
- "Status user hanya dapat diubah lewat aksi aktifkan/nonaktifkan."

Semuanya berbahasa Indonesia, menyebut penyebab konkret, dan dapat langsung ditindaklanjuti.

Tetapi **tidak satu pun sampai ke pengguna.** Antarmuka memakai dua pola yang keduanya membuangnya:
aksi pengguna menampilkan judul saja tanpa alasan, dan aksi role menampilkan **kode mesin**
(`error`, `validation_failed`, `not_found`) sebagai penjelasan.

Untuk aksi role ada penyebab tambahan di lapisan tengah: konflik data memakai kelas galat yang
**tidak dipetakan** oleh pembungkus respons, sehingga jatuh ke kategori galat generik. Jadi
memperbaiki penyampaian pesan menuntut perbaikan di dua tempat, bukan satu.

Hasil yang harus dicapai: **pesan yang sudah ditulis dengan baik di lapisan dalam adalah yang
dilihat pengguna.** Ini bukan fitur baru — hanya menghentikan pembuangan yang sedang terjadi.

Pola ini identik dengan temuan modul 01 (KI-04), yang berarti ia **sistemik**, bukan kelalaian di
satu tempat. Memperbaikinya di lapisan bersama akan menyelesaikannya untuk semua modul sekaligus.

---

## 4. Ringkasan: Yang Harus Dicapai vs Yang Bebas Diubah

| Aspek | Status |
|---|---|
| Penetapan role/cabang mendeklarasikan kondisi akhir | **Wajib dipertahankan** — menghilangkan kelas masalah urutan |
| Daftar tidak dikirim = tidak berubah | **Wajib dipertahankan** — memungkinkan update sebagian |
| Kode role stabil, tidak berubah saat nama diubah | **Wajib dipertahankan** — menjaga rujukan kebijakan |
| Pengguna tidak perlu tahu konsep "kode role" | **Wajib dipertahankan** |
| Penolakan akun/role istimewa disamarkan sebagai "tidak ditemukan" | **Wajib dipertahankan** — properti keamanan |
| Angka total tidak menghitung yang disembunyikan | **Wajib dipertahankan** — mencegah kebocoran halus |
| Akun developer tidak bisa terkunci | **Wajib dipertahankan** — jalan masuk terakhir |
| Menonaktifkan & ganti password memutus semua sesi | **Wajib dipertahankan** |
| Ganti password sendiri mempertahankan sesi sendiri | **Wajib dipertahankan** — mudah terlewat, dampak UX besar |
| Aktivasi ulang tidak menghidupkan sesi lama | **Wajib dipertahankan** |
| Nama role sistem tetap, izinnya lentur | **Wajib dipertahankan** — pembedaan yang tepat |
| Validasi izin mendahului penghapusan | **Wajib dipertahankan** — mencegah kerusakan akibat salah ketik |
| Setiap perubahan izin mengirim daftar lengkap | **Wajib dipertahankan** — kondisi akhir tak ambigu |
| Perubahan izin optimistis + dikembalikan bila gagal | **Wajib dipertahankan** |
| Role yang sedang dipakai tidak bisa dihapus | **Wajib dipertahankan** |
| Cara membuang karakter non-Latin di kode role | Bebas diubah — **perlu diputuskan** bila nama non-Latin didukung |
| Ruang nama kode role digabung global + perusahaan | Bebas ditinjau bila multi-perusahaan jadi target |
| Simpan-otomatis per klik pada matriks izin | Bebas diganti — **selama tidak ada kondisi setengah jadi** |
| Cara menetapkan cabang default (urutan array) | **Harus diperbaiki** — hasil benar, tapi tak terlihat & tak terkendali |
| Kondisi akhir kosong (nol role / nol cabang) diterima | **Harus diperbaiki** — mengunci pengguna dari sistem |
| Kode role tak dikenal dilewati diam-diam | **Harus diperbaiki** — jalur lain ke masalah yang sama |
| Operasi role tanpa transaksi | **Harus diperbaiki** — bisa meninggalkan role tanpa izin |
| Pesan galat spesifik dibuang di lapisan luar | **Harus diperbaiki** — sistemik, perbaiki di lapisan bersama |
| Menonaktifkan diri sendiri tanpa penjagaan | **Harus diperbaiki** |
| Penghapusan role tanpa konfirmasi | **Harus diperbaiki** — operasi permanen |
| Izin role sistem dapat dinaikkan sendiri oleh pemegang hak kelola | **Perlu diputuskan** — pembagian tugas disengaja, tapi tak ditegakkan |
| Batas minimum izin per role | **Perlu diputuskan** — sekarang tidak ada |
