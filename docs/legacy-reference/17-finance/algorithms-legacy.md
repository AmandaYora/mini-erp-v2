# Algorithms (Legacy) — Modul 17 Finance / Accounting & Pajak

**Kelompok B — cukup dipahami maksudnya, BUKAN untuk ditiru implementasinya.** Fokus pada **hasil
yang diharapkan** dari tiap logika kunci.

Kontrak yang wajib presisi ada di [business-rules.md](business-rules.md) dan
[reports-list.md](reports-list.md).

---

## 1. Menerjemahkan Peristiwa Bisnis Jadi Jurnal

**Masalah yang dipecahkan:** pemilik UKM tidak menjurnal. Ia menjual, menerima barang, membayar
supplier. Sistem harus menerjemahkan itu jadi akuntansi yang benar tanpa ia perlu tahu istilah
debit-kredit.

**Hasil yang diharapkan:**

- Setiap peristiwa bisnis yang punya arti keuangan **pasti** berakhir jadi jurnal seimbang, atau
  tertahan dengan alasan yang bisa dibaca pemilik.
- Tidak ada peristiwa yang hilang diam-diam.
- Pemilik bisa **melihat dulu** jurnal yang akan terbentuk sebelum menyetujuinya.

**Konsep yang dipakai sekarang — tiga tahap yang sengaja dipisah:**

| Tahap | Yang terjadi | Bisa dibatalkan? |
|---|---|---|
| **Tangkap** | Memindai peristiwa bisnis → membuat baris antrian | Ya, sinkron ulang aman |
| **Nilai** | Menentukan apakah datanya cukup → `ready` atau `pending` + alasan | Ya |
| **Posting** | Membuat jurnal + menerapkan harga modal | Ya, lewat balik/batal |

**Untuk rebuild:** pemisahan tiga tahap ini adalah **inti karakter modul** dan wajib
dipertahankan. Yang bebas dipilih ulang adalah caranya — antrian tabel, event stream, atau
perhitungan saat dibaca.

---

## 2. Antrian yang Aman Disinkronkan Berulang

**Masalah:** sinkronisasi bisa dijalankan kapan saja, berkali-kali, sementara sebagian antrian
sudah diposting. Menyinkronkan ulang tidak boleh menggandakan apa pun atau menimpa hasil kerja.

**Hasil yang diharapkan:**

- Menjalankan sinkronisasi dua kali berturut-turut **tidak mengubah apa pun**.
- Baris yang sudah diposting **tidak tersentuh**.
- Dokumen yang **berubah** setelah disinkron (mis. nilai retur dikoreksi) ikut diperbarui selama
  belum diposting.
- Dokumen sumber yang **diarsipkan** otomatis ditandai, tidak tertinggal sebagai antrian hantu.

**Konsep sekarang:** identitas baris = (perusahaan, jenis sumber, id sumber, event key); perubahan
isi dideteksi lewat sidik jari yang dihitung dari serialisasi **berurutan kunci** — karena basis
data menormalkan urutan kunci JSON, perbandingan naif akan salah mendeteksi "berubah".

**Untuk rebuild:** sifat **idempoten** dan **deteksi perubahan** adalah yang mengikat. Detail
sidik jari bebas.

---

## 3. Batas Pindai Otomatis (Watermark)

**Masalah:** memindai seluruh sejarah transaksi setiap kali sinkronisasi dijalankan tidak realistis
pada VPS 2 GB dengan puluhan ribu dokumen.

**Hasil yang diharapkan:** sinkronisasi hanya memindai rentang yang mungkin berubah, tanpa pernah
melewatkan transaksi lama yang belum selesai.

**Konsep sekarang:** batas bawah pindai diambil dari yang paling awal di antara — tanggal cutover
saldo awal, tanggal sumber belum-final tertua, dan hari setelah periode tertutup terakhir. Bisa
dimatikan lewat variabel lingkungan untuk keperluan pemulihan.

**Untuk rebuild:** yang penting adalah jaminan **"tidak ada yang terlewat"**, bukan cara
menghitungnya. Sediakan juga jalan keluar untuk pemindaian penuh.

---

## 4. Urutan Posting Berbasis Dependensi

**Masalah:** jurnal HPP tidak boleh terbit sebelum jurnal penjualannya; penyelesaian retur tidak
boleh mendahului returnya. Bila urutannya salah, laporan sesaat menampilkan HPP tanpa pendapatan.

**Hasil yang diharapkan:** menekan "tutup hari" satu kali menghasilkan buku yang **konsisten pada
setiap titik**, bukan hanya di akhir.

**Konsep sekarang:** tiap jenis peristiwa punya nomor urut bisnis (penerimaan 10 → penjualan 20 →
retur 25/26 → HPP 30 → stok retur 35/36 → pembayaran 40 → penyelesaian 45/46 → koreksi stok 50).
Peristiwa yang menunggu induknya ditandai alasan bertipe *ordering* dan **tidak ditampilkan sebagai
tugas pemilik** — ia akan selesai sendiri pada gelombang berikutnya.

**Untuk rebuild:** pemisahan **"menunggu urutan"** dari **"menunggu data pemilik"** adalah keputusan
desain terbaik di modul ini. Tanpa itu, pemilik akan melihat puluhan "tugas" yang sebenarnya cuma
antrian internal.

---

## 5. Harga Modal Rata-Rata Bergerak

**Masalah:** menentukan HPP saat barang terjual, padahal barang yang sama dibeli berkali-kali
dengan harga berbeda.

**Hasil yang diharapkan:**

- Setiap pengeluaran stok punya harga modal yang bisa dipertanggungjawabkan.
- Nilai persediaan di neraca = jumlah × harga modal rata-rata.
- Riwayatnya bisa ditelusuri mundur: berapa harga modal **sebelum** dan **sesudah** tiap mutasi.
- Satu mutasi stok **tidak pernah** menghasilkan dua baris biaya.

**Konsep sekarang:**

| Arah | Perlakuan |
|---|---|
| Barang **masuk** | Nilai baru ditambahkan, lalu rata-rata dihitung ulang: `(nilai lama + nilai masuk) ÷ (jumlah lama + jumlah masuk)` |
| Barang **keluar** | Dinilai pada rata-rata **saat itu**; rata-rata tidak berubah |

Sumber harga untuk barang masuk, berurutan: harga retur → harga beli master → harga modal rata-rata
yang ada. Bila ketiganya kosong, peristiwa **tertahan** dengan alasan `missing_cost_basis`, bukan
diposting dengan nilai nol.

**Tiga penjagaan yang lahir dari insiden nyata** (241 produk, harga modal salah 10–1000×,
~Rp 3,2 M jurnal tercemar):

1. Harga modal saldo awal **tidak bisa diketik** — selalu dari harga beli master.
2. Rasio terhadap harga beli master wajib **0,1×–10×**.
3. Barang masuk yang akan menghasilkan rata-rata **negatif ditolak** dengan penjelasan bahwa basis
   biayanya sudah rusak dari transaksi sebelumnya.

**Untuk rebuild:** metode rata-rata bergerak, granularitas **(cabang, produk)**, riwayat
sebelum/sesudah, dan ketiga penjagaan itu semuanya wajib bertahan.

**Yang perlu diputuskan ulang:** granularitas mengabaikan **varian** — semua varian satu produk
berbagi satu harga modal, padahal stoknya dipisah per varian. Untuk produk yang harga belinya
berbeda antar varian, HPP-nya akan meleset.

---

## 6. Menutup Periode dengan Aman

**Masalah:** mengunci bulan terlalu cepat membekukan buku yang belum lengkap; terlalu lambat
membiarkan angka bisa berubah setelah dilaporkan.

**Hasil yang diharapkan:** pemilik melihat **daftar periksa yang bisa dibaca** sebelum mengunci,
tahu persis apa yang belum beres, dan tidak bisa mengunci tanpa sadar.

**Konsep sekarang:** enam pemeriksaan (lihat [reports-list.md](reports-list.md) §7), dibagi
"gagal" yang memblokir dan "peringatan" yang tidak. Kunci paksa tetap tersedia tetapi butuh
permission terpisah — bukan sekadar hak kelola finance.

Penulisan status terkunci dilakukan **dengan penguncian baris**, karena pemeriksaan kesiapan
dijalankan tanpa kunci dan dua permintaan bersamaan bisa saling menimpa.

**Untuk rebuild:** pola "cek dulu, kunci kemudian, dengan jalan paksa yang butuh izin lebih tinggi"
layak ditiru. Tambahkan yang belum ada: **jejak audit untuk unduh berkas** dan untuk aksi posting.

---

## 7. Layer 2 — Pembatasan Omzet pada Paket Berkas Pajak

**Ini bukan optimasi teknis. Ini keputusan bisnis yang tertanam di kode**, dan pembaca dokumen ini
perlu memahaminya secara utuh sebelum memutuskan membawanya ke sistem baru.

**Apa yang dilakukan:** menghasilkan versi kedua dari paket berkas pajak, di mana sebagian order
penjualan **dihilangkan seluruhnya** sehingga total omzet yang tampak berada di bawah
**Rp 4.800.000.000** — ambang peredaran bruto PP-23.

**Cara kerjanya:**

1. Hitung omzet per order dalam periode, dari baris jurnal berakun pendapatan.
2. Urutkan order dengan **urutan acak yang stabil** (fungsi hash atas id order — bukan acak nyata,
   supaya hasilnya sama setiap kali diunduh).
3. Akumulasi omzet mengikuti urutan itu. Order yang membuat akumulasi melewati plafon → **ditandai
   dibuang**, dan akumulasi tidak bertambah.
4. Kumpulkan **seluruh jurnal** milik order yang dibuang — penjualan, HPP, PPN, pembayaran — lewat
   kolom dimensi `id_order`.
5. Bangun kedelapan sheet dengan jurnal-jurnal itu dikecualikan.

**Kenapa yang dibuang adalah order utuh, bukan jurnal omzet saja:** karena satu penjualan tersebar
di beberapa jurnal terpisah. Membuang hanya jurnal pendapatannya akan menyisakan HPP-nya, sehingga
margin jadi janggal dan neraca tidak seimbang. Dengan membuang jurnal **utuh** (yang debit =
kreditnya), seluruh laporan tetap seimbang.

**Yang perlu dipahami untuk rebuild:**

- Kedua versi dihasilkan dari **data yang sama** — tidak ada data yang diubah atau dihapus. Buku
  besar sebenarnya tetap utuh; Layer 2 hanya berupa penyaringan saat laporan dibentuk.
- Kedua tombol muncul berdampingan dengan label yang jujur: *"Data riil (apa adanya)"* dan
  *"Versi dibatasi Rp4,8 M"*.
- **Tidak ada jejak audit sama sekali** saat berkas diunduh — tidak tercatat siapa mengunduh versi
  yang mana, kapan.
- Dijaga permission `finance.report.view` — permission yang sama dengan melihat laporan biasa.

**Keputusan yang harus diambil pemilik sebelum fase desain** — lihat
[open-questions.md](../open-questions.md) **OQ-A42**. Ini menyangkut kepatuhan pajak, jadi
sebaiknya dikonfirmasi bersama konsultan pajak, bukan diputuskan dari sisi teknis. Selama belum
ada keputusan, saya tidak akan merancang ulang bagian ini.

---

## 8. Membatalkan Tanpa Menghapus

**Masalah:** angka keuangan yang salah harus bisa diperbaiki, tetapi jejaknya tidak boleh hilang.

**Hasil yang diharapkan:**

- Tidak ada jurnal yang pernah dihapus atau diubah.
- Koreksi selalu berupa **jurnal lawan** yang bisa dilihat berdampingan dengan aslinya.
- Membatalkan posting mengembalikan antriannya ke keadaan sebelum diposting, sehingga bisa
  diposting ulang setelah datanya diperbaiki.
- Harga modal ikut dikembalikan — bukan hanya jurnalnya.

**Konsep sekarang:** tiga jalur pembalikan (balik jurnal, batal posting sumber, batal biaya usaha),
semuanya membuat jurnal bertipe `reversal` dan menandai yang asli `reversed`.

**Untuk rebuild:** prinsip "tidak ada penghapusan" wajib. Yang perlu **ditambahkan**: jejak audit —
sekarang balik jurnal dan batal posting **sama sekali tidak teraudit**.

---

## 9. Saldo Awal sebagai Titik Nol

**Masalah:** bisnis yang sudah berjalan tidak mulai dari nol. Sistem harus bisa menerima keadaan
awal tanpa memalsukan riwayat transaksi.

**Hasil yang diharapkan:**

- Satu jurnal pembuka yang seimbang, dengan ekuitas sebagai **penyeimbang otomatis**.
- Seluruh buku besar harga modal terisi sesuai stok fisik saat cutover.
- Setelah dikunci, saldo awal tidak bisa diedit — hanya **dilengkapi**.

**Konsep sekarang:** kas + bank + piutang + persediaan didebit, hutang dikredit, selisihnya
dilemparkan ke akun Modal Awal sebagai penyeimbang. Nilai persediaan datang dari buku besar harga
modal, bukan dari input.

**Jalur "Lengkapi Stok Awal"** adalah bagian penting dari desain ini: menambah produk yang terlewat
**setelah** saldo awal dikunci, secara aditif, dengan jurnal suplemennya sendiri. Ini pengganti
sadar untuk "buka kembali lalu posting ulang" — yang akan mereset jumlah dan menghapus pengurangan
stok dari transaksi yang sudah terjadi.

**Untuk rebuild:** pola aditif ini layak ditiru untuk setiap data yang "sudah dikunci tapi ternyata
kurang lengkap".

---

## 10. Tarif Pajak yang Tidak Pernah Jadi Pengaturan

**Yang mungkin diharapkan:** ada satu pengaturan "tarif PPN = 11%".

**Kenyataannya:** tarif diketik **per order** di modul Order, disimpan sebagai snapshot pada
dokumen, dan Finance hanya membacanya. Bawaannya **0** bila tidak diisi. Tidak ada angka 11% di
mana pun dalam kode.

**Konsekuensi yang harus dipahami:**

- Order kena pajak yang tarifnya lupa diisi akan menghasilkan PPN 0.
- Perubahan tarif pajak nasional tidak butuh perubahan sistem — tapi juga tidak ada satu tempat
  untuk mengubahnya.
- Kedua laporan PPN memperlakukan tarif 0 **secara berbeda** (lihat KI-145).

**Untuk rebuild:** snapshot per dokumen adalah pilihan yang benar (dokumen historis tidak boleh
bergeser saat tarif berubah). Yang perlu ditambahkan: **tarif bawaan yang bisa diatur**, supaya
operator tidak perlu mengetik ulang tiap order dan tidak bisa lupa.
