# Algorithms Legacy — Modul 01 Auth & Session

**Kelompok B — fokus pada HASIL yang diharapkan, bukan cara implementasinya.** Dokumen ini
menjelaskan *apa yang harus dicapai* tiap logika kunci, supaya sistem baru bisa mencapai hasil
yang sama dengan cara yang berbeda.

Untuk aturan yang **presisi dan wajib dipertahankan**, rujuk
[business-rules.md](business-rules.md) — dokumen ini sengaja tidak mengulanginya secara detail.

---

## 1. Peta Logika Kunci

| # | Logika | Hasil yang harus dicapai |
|---|---|---|
| A-01 | Verifikasi kredensial | Menerima pemilik akun yang sah, menolak sisanya **tanpa memberi tahu penyebabnya** |
| A-02 | Penentuan konteks kerja awal | User mendarat langsung di tempat kerjanya bila tidak ambigu; ditanya bila ambigu |
| A-03 | Penerbitan & rotasi token | Sesi panjang tetap nyaman, tapi kebocoran token punya masa manfaat sangat singkat |
| A-04 | Verifikasi sesi per request | Perubahan hak & status berlaku seketika, tanpa menunggu user login ulang |
| A-05 | Degradasi akses cabang | Kehilangan akses cabang tidak mematikan sesi, hanya menyempitkan yang bisa dikerjakan |
| A-06 | Resolusi tampilan setelah ganti role | User tidak pernah tertinggal di halaman yang tak boleh ia lihat |
| A-07 | Pemulihan sesi di klien | Reload halaman terasa seperti tidak pernah keluar |
| A-08 | Antrean refresh | Banyak permintaan yang kedaluwarsa bersamaan tidak menimbulkan badai refresh |
| A-09 | Sanitasi tujuan redirect | Alur login tidak bisa dipakai mengarahkan user ke luar aplikasi |

---

## A-01 — Verifikasi Kredensial

**Hasil yang diharapkan:** hanya pemilik akun yang sah bisa masuk, dan pihak yang gagal **tidak
bisa menyimpulkan apa pun** tentang keberadaan akun.

Ini prinsip desainnya, dan yang paling penting untuk tidak hilang saat rebuild: **user tidak ada,
password salah, akun non-aktif, dan akun terkunci semuanya menghasilkan respons yang identik** —
status HTTP sama, pesan sama. Sistem sengaja tidak membedakannya.

Satu pengecualian yang ada sekarang: akun sah tanpa role sama sekali menghasilkan pesan berbeda
("User tidak memiliki role", status berbeda pula). Ini membocorkan bahwa akun itu **ada** dan
passwordnya **benar**. Apakah pengecualian ini disengaja (membantu admin mendiagnosis) atau
sebaiknya diseragamkan — **[PERLU KONFIRMASI]**, tercatat juga di
[business-rules.md](business-rules.md) BR-05.

**Yang menerima input identifier tunggal.** User cukup mengetik satu hal (username atau email);
sistem yang mencari di kedua kemungkinan. Hasil yang penting: user tidak perlu ingat "saya
mendaftar dengan apa". Bila di sistem baru identifier bisa lebih dari dua jenis (mis. nomor
telepon), prinsipnya sama — satu field, sistem yang mencocokkan.

**Yang tidak ada dan perlu diputuskan sadar:** tidak ada pembatasan laju (rate limiting), tidak
ada penguncian setelah N kegagalan, dan tidak ada pencatatan percobaan gagal. Artinya percobaan
menebak password bisa dilakukan tanpa batas dan tanpa jejak. Satu-satunya penghambat adalah biaya
komputasi verifikasi hash password. **[PERLU KONFIRMASI]** apakah ini perlu ditambahkan di sistem
baru — ini fitur baru, bukan replikasi.

---

## A-02 — Penentuan Konteks Kerja Awal

**Hasil yang diharapkan:** user langsung berada di konteks kerjanya ketika jawabannya jelas, dan
hanya ditanya ketika sistem benar-benar tidak bisa memutuskan.

Prinsip keputusannya:

- Bila hanya ada **satu** kemungkinan cabang → pilih untuk user. Jangan tanya sesuatu yang
  jawabannya cuma satu.
- Bila ada beberapa kemungkinan tapi user punya **preferensi tersimpan** → hormati preferensi itu.
- Bila ada beberapa kemungkinan **tanpa** preferensi → baru tanya.

Efek yang terlihat: kelima akun bawaan (semuanya punya satu cabang) **tidak pernah** melihat
halaman pemilihan cabang. Halaman itu hanya muncul untuk user multi-cabang tanpa default.

Detail penting yang harus dipertahankan: **cabang non-aktif tidak dihitung** sebagai kemungkinan.
Cabang yang ditutup tidak boleh muncul sebagai pilihan maupun terpilih otomatis.

Satu lubang di logika ini: ketika jumlah kemungkinan adalah **nol**, sistem tidak memperlakukannya
sebagai kondisi yang perlu ditanyakan — ia meloloskan user masuk tanpa konteks kerja dan tanpa
menandai bahwa pemilihan dibutuhkan. Hasilnya jalan buntu di UI. Lihat
[known-issues.md](../known-issues.md) KI-01. Di sistem baru, "nol kemungkinan" harus jadi kondisi
tersendiri dengan penanganan eksplisit — entah menolak login dengan pesan jelas, atau memberi
halaman yang menjelaskan situasinya beserta jalan keluar.

**Role ditentukan dengan prinsip berbeda,** dan lebih lemah: sistem sekadar mengambil role
pertama yang muncul dari data, tanpa aturan urutan. Untuk user satu-role hasilnya selalu benar;
untuk user multi-role hasilnya tidak dapat diprediksi. Hasil yang seharusnya dicapai: **role awal
harus deterministik** — entah dari penanda default per user, atau dari aturan urutan yang tertulis
dan konsisten.

---

## A-03 — Penerbitan & Rotasi Token

**Hasil yang diharapkan:** dua tujuan yang saling bertolak belakang dipenuhi sekaligus — user
tidak perlu login ulang sepanjang hari kerja, tapi token yang bocor hanya berguna sesaat.

Cara mencapainya secara konsep: **pisahkan kredensial jangka pendek dari jangka panjang.**

| Peran | Sifat | Konsekuensi |
|---|---|---|
| Kredensial akses harian | Umur sangat pendek (default 15 menit), dibawa di setiap request | Bila bocor, jendela penyalahgunaannya sempit |
| Kredensial perpanjangan | Umur panjang (default 7 hari), dipakai jarang | Jarang berpindah jalur, lebih mudah dilindungi |

**Properti kunci yang wajib dipertahankan: kredensial perpanjangan bersifat sekali pakai.** Setiap
kali dipakai, ia digantikan yang baru dan yang lama langsung tidak berlaku. Hasil yang dicapai:
bila seseorang mencuri kredensial perpanjangan dan memakainya, pemakaian berikutnya oleh pemilik
asli akan gagal — dan sebaliknya. Kebocoran jadi terdeteksi lewat gangguan yang dirasakan, bukan
diam-diam berlangsung selamanya.

Properti kedua: setiap perpanjangan **juga memperpanjang masa sesi**. Hasilnya sesi yang dipakai
rutin tidak pernah kedaluwarsa. Ini keputusan kenyamanan yang jelas menguntungkan operasional
(kasir tidak terputus di tengah transaksi), tapi berarti **tidak ada batas umur absolut sesi** —
sesi bisa hidup bertahun-tahun. **[PERLU KONFIRMASI]** apakah sistem baru perlu batas absolut
(mis. wajib login ulang setiap 30 hari apa pun aktivitasnya).

Detail implementasi yang **tidak** perlu ditiru: pemilihan bcrypt cost 12 untuk menyimpan sidik
kredensial perpanjangan, dan urutan "buat baris sesi dulu → terbitkan token → timpa sidiknya".
Keduanya dibahas di [data-model-legacy.md](data-model-legacy.md) §3.1.

---

## A-04 — Verifikasi Sesi per Request

**Hasil yang diharapkan:** perubahan hak akses, status akun, dan akses cabang **berlaku seketika**
— tanpa menunggu user logout, tanpa menunggu token kedaluwarsa.

Ini keputusan arsitektur yang paling menentukan karakter modul ini, dan konsekuensinya perlu
dipahami sebelum diubah.

Pilihan yang diambil sistem lama: **jangan percayai apa pun yang dibawa token selain identitasnya.**
Token hanya menyatakan "saya user X di sesi Y". Semua hal yang bisa berubah — role aktif,
permission, status akun, kelayakan cabang — dibaca ulang dari sumber kebenaran pada **setiap**
request.

Hasil yang dicapai:

| Skenario | Efek |
|---|---|
| Admin mencabut permission dari sebuah role | Berlaku pada request berikutnya, di semua perangkat |
| Admin menonaktifkan akun | User langsung terputus, tidak perlu menunggu token habis |
| Admin mencabut akses cabang | Cabang langsung tidak bisa dipakai |
| Admin mengubah role aktif user dari sisi data | Langsung terasa |

Harganya: **4 pembacaan data per request terautentikasi**. Untuk skala UKM satu perusahaan ini
tidak terasa, dan ditukar dengan jaminan yang kuat. Bila sistem baru ingin memangkas biaya ini
(mis. dengan cache berumur pendek), yang harus dijaga adalah **hasilnya**: jeda antara perubahan
hak dan berlakunya harus tetap dalam hitungan detik, bukan menit atau sampai logout.

Properti kecil yang mudah terlewat tapi penting: verifikasi memeriksa **identitas sesi dan
identitas pemilik secara bersamaan**. Token yang menyebut sesi milik orang lain ditolak, bukan
diterima sebagian.

---

## A-05 — Degradasi Akses Cabang

**Hasil yang diharapkan:** kehilangan akses ke cabang **tidak** memutus sesi user; ia hanya
menyempitkan apa yang bisa dikerjakan, lalu diarahkan memilih cabang lain.

Prinsipnya: bedakan **"siapa kamu"** (autentikasi — masih sah) dari **"di mana kamu bekerja"**
(konteks — sudah tidak berlaku). Kehilangan yang kedua tidak boleh membatalkan yang pertama.

Rangkaian hasil yang terlihat user:

1. Akses cabangnya dicabut oleh admin sementara ia sedang bekerja
2. Sesinya tetap hidup — ia tidak dilempar ke halaman login
3. Operasi yang butuh konteks cabang ditolak dengan pesan yang menyebut cabang, bukan pesan
   otentikasi
4. Navigasi berikutnya mengantarnya ke halaman pemilihan cabang
5. Ia memilih cabang lain yang masih boleh diakses dan melanjutkan pekerjaan

Ini perilaku yang bagus dan layak dipertahankan utuh. Satu-satunya catatan: langkah 5 gagal bila
**semua** akses cabangnya dicabut — dan pada titik itu tidak ada jalan keluar dari UI (KI-01).
Prinsip yang sama seperti A-02: kondisi "nol cabang" perlu penanganan eksplisit.

---

## A-06 — Resolusi Tampilan Setelah Ganti Role

**Hasil yang diharapkan:** setelah berganti role, user tidak pernah tertinggal di halaman yang
role barunya tidak berhak melihat.

Logikanya sederhana dan hasilnya yang penting: **periksa apakah halaman saat ini masih boleh
diakses; bila tidak, pindahkan ke tempat yang pasti boleh** (dashboard). User tidak melihat
halaman kosong, tidak melihat pesan error, dan tidak perlu mencari sendiri jalan keluar.

Satu detail yang perlu dipahami maksudnya: pemeriksaan memakai daftar permission **role baru yang
dibaca dari data perusahaan**, bukan dari ringkasan sesi. Alasannya urutan — ringkasan sesi belum
tentu sudah ter-update saat pemeriksaan berjalan. Di sistem baru, bila respons ganti role langsung
dipakai sebagai sumber, masalah urutan ini hilang dengan sendirinya.

---

## A-07 — Pemulihan Sesi di Klien

**Hasil yang diharapkan:** menyegarkan halaman atau membuka aplikasi lagi terasa seperti tidak
pernah keluar — user kembali ke tempat yang sama dengan konteks yang sama.

Dua kebutuhan yang harus dipenuhi bersamaan, dan keduanya menjelaskan bentuk penyimpanan lokal
yang ada sekarang:

1. **Tampilan harus siap sebelum server menjawab.** Menu sidebar perlu tahu permission untuk
   menyembunyikan item; tanpa data lokal, setiap muat halaman akan berkedip antara "menu lengkap"
   dan "menu sesuai hak".
2. **Kebenaran tetap milik server.** Data lokal tidak boleh dipercaya sebagai izin — ia hanya
   pemercepat tampilan.

Yang dilakukan sistem lama: menampilkan layar tunggu, memvalidasi sesi ke server, lalu menyusun
ulang konteks dari gabungan jawaban server dan data lokal. Bila validasi gagal, semua dibersihkan
dan user berada di halaman login.

Detail yang **tidak** perlu ditiru, dan sebaiknya tidak: jawaban server pada tahap ini **tidak
memuat data perusahaan**, sehingga informasi itu bertahan **hanya** karena tersimpan di browser.
Ketergantungan ini tersembunyi dan rapuh. Hasil yang seharusnya dicapai: satu permintaan yang
mengembalikan **seluruh** konteks sesi, sehingga pemulihan tidak perlu menggabungkan dua sumber.

Satu perilaku yang perlu diputuskan: ketika sesi ternyata sudah tidak sah, user dikembalikan ke
halaman login **tanpa penjelasan apa pun**. Dari sisi user, aplikasi sekadar "melupakan" dirinya.
Menambahkan pesan seperti "Sesi Anda telah berakhir, silakan masuk kembali" adalah perubahan
kecil dengan dampak kejelasan yang besar — **[PERLU KONFIRMASI]** apakah boleh ditambahkan atau
harus tetap sunyi seperti sekarang.

---

## A-08 — Antrean Refresh

**Hasil yang diharapkan:** ketika banyak permintaan kedaluwarsa bersamaan, hanya **satu**
perpanjangan yang terjadi, dan seluruh permintaan tertunda itu berhasil setelahnya.

Kenapa ini penting: halaman yang memuat beberapa data sekaligus bisa mengirim lima permintaan
serentak. Bila kelimanya kedaluwarsa dan masing-masing memperpanjang sendiri, empat di antaranya
akan memakai kredensial yang sudah dirotasi oleh yang pertama — dan **gagal**. User akan melihat
sebagian halaman kosong tanpa alasan jelas.

Hasil yang harus dicapai: perpanjangan bersifat **tunggal dan dibagi**. Permintaan yang datang
saat perpanjangan sedang berjalan menunggu hasil yang sama, bukan memulai perpanjangan baru.

Dua properti pendukung yang juga perlu dipertahankan:

- **Percobaan ulang hanya sekali.** Bila permintaan tetap ditolak setelah diperpanjang, ia gagal —
  tidak berputar tanpa akhir.
- **Perpanjangan tidak terlihat sebagai aktivitas user.** Indikator memuat global tidak menyala
  untuknya, karena ini pekerjaan infrastruktur, bukan aksi yang user minta.

---

## A-09 — Sanitasi Tujuan Redirect

**Hasil yang diharapkan:** kemudahan "kembali ke halaman yang tadi saya buka" tidak bisa dipakai
untuk mengarahkan user ke luar aplikasi.

Alur login menyimpan halaman tujuan supaya user tidak selalu mendarat di dashboard. Nilai itu
berasal dari navigasi, jadi ia **tidak boleh dipercaya**.

Tiga hasil yang harus dijamin:

| Jaminan | Kenapa |
|---|---|
| Tujuan selalu berupa halaman **di dalam** aplikasi ini | Mencegah alur login dipakai mengarahkan ke situs luar |
| Tujuan bukan halaman auth itu sendiri | Mencegah lingkaran redirect tak berujung |
| Bila tujuan tidak layak, ada tempat pasti yang bisa dituju | User tidak pernah mendarat di kondisi tak terdefinisi |

Yang ditolak secara khusus: alamat lengkap ke domain lain, dan bentuk yang tampak relatif tapi
sebenarnya menunjuk host lain (diawali dua garis miring). Yang kedua adalah jebakan klasik dan
mudah terlewat bila validasinya sekadar "harus diawali garis miring".

Bagian ini adalah **satu-satunya logika di modul auth yang sudah punya cakupan test menyeluruh**
(4 kasus, termasuk kedua bentuk penolakan di atas) — pertanda ia pernah jadi masalah nyata.

---

## 2. Ringkasan: Yang Harus Dicapai vs Yang Bebas Diubah

| Aspek | Status |
|---|---|
| Pesan gagal login tidak membedakan penyebab | **Wajib dipertahankan** — properti keamanan |
| Kredensial perpanjangan sekali pakai | **Wajib dipertahankan** — deteksi kebocoran |
| Verifikasi ganda identitas sesi + pemilik | **Wajib dipertahankan** — mencegah token silang |
| Hak akses berlaku seketika tanpa login ulang | **Wajib dipertahankan** — karakter inti modul |
| Kehilangan akses cabang tidak memutus sesi | **Wajib dipertahankan** — perilaku UX yang baik |
| Auto-pilih cabang bila tidak ambigu | **Wajib dipertahankan** — kenyamanan harian |
| Perpanjangan tunggal & dibagi | **Wajib dipertahankan** — mencegah halaman gagal sebagian |
| Sanitasi tujuan redirect | **Wajib dipertahankan** — properti keamanan |
| Pindah ke dashboard bila halaman tak lagi boleh | **Wajib dipertahankan** — perilaku UX |
| Cara menyimpan sidik kredensial (bcrypt cost 12) | Bebas diganti — hash cepat cukup untuk token acak |
| Urutan buat-sesi-lalu-timpa-sidik | Bebas diganti — hasilkan identitas sesi lebih dulu |
| Jumlah pembacaan data per request | Bebas dioptimalkan, **selama hasil A-04 tetap tercapai** |
| Bentuk & isi penyimpanan lokal | Bebas dirancang ulang — sebaiknya satu sumber, bukan gabungan |
| Penentuan role awal | **Harus diperbaiki** — sekarang tidak deterministik |
| Penanganan kondisi "nol cabang" | **Harus diperbaiki** — sekarang jalan buntu |
| Batas umur absolut sesi | **Perlu diputuskan** — sekarang tidak ada |
| Pembatasan laju & jejak login gagal | **Perlu diputuskan** — sekarang tidak ada sama sekali |
