# User Flows — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Alur penggunaan end-to-end per
skenario, dari sudut pandang apa yang dilihat dan dirasakan pengguna. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [business-rules.md](business-rules.md) ·
[test-cases.md](test-cases.md)

---

## Daftar Skenario

| # | Skenario | Pelaku | Hasil akhir |
|---|---|---|---|
| UF-01 | Membuka daftar cabang | Owner / Admin | Melihat seluruh cabang perusahaan |
| UF-02 | Membuka cabang baru sampai bisa berjualan | Owner | Cabang siap pakai (gudang + penomoran) |
| UF-03 | Memberi akses cabang baru ke pengguna | Owner | Pengguna bisa memilih cabang itu |
| UF-04 | Berpindah cabang aktif | Pengguna multi-cabang | Seluruh data modul dimuat ulang untuk cabang baru |
| UF-05 | Mengoreksi identitas cabang (nama/kota/alamat) | Owner | Data cabang berubah, penomoran tidak terganggu |
| UF-06 | Mengubah **kode** cabang | Owner | Prefix seluruh nomor dokumen berikutnya berubah |
| UF-07 | Mengganti nama gudang lewat form cabang | Owner | Nama lokasi stok default ikut berubah |
| UF-08 | Mencoba kode cabang yang sudah dipakai | Owner | Ditolak dengan toast umum |
| UF-09 | Admin membuka halaman cabang (tanpa hak kelola) | Admin | Hanya bisa melihat |
| UF-10 | Staff/Kasir mencoba membuka halaman cabang | Staff / Kasir | Dialihkan ke halaman 403 |
| UF-11 | Menutup cabang yang tidak dipakai lagi | Owner | **Tidak bisa** — tidak ada jalannya di aplikasi |

---

## UF-01 — Membuka daftar cabang

**Pelaku:** Owner atau Admin (punya `branch.view`).

1. Dari sidebar, buka grup **"Pengaturan"** → klik **"Cabang & Lokasi"**.
2. Halaman `/branches` terbuka. Judul **"Cabang & Lokasi"**, deskripsi
   *"Kelola daftar cabang dan lokasi operasional perusahaan Anda."*
3. Kartu **"Semua Cabang"** menampilkan tabel berisi seluruh cabang perusahaan, terurut nama A→Z.
4. Baris cabang yang sedang aktif diberi badge hijau **"Aktif Saat Ini"**.
5. Setiap baris memberi tahu apakah pengguna punya akses ke cabang itu
   (**"Anda Memiliki Akses"** / **"Tidak Ada Akses"**).

**Yang tidak terjadi:** tidak ada permintaan jaringan baru saat halaman dibuka. Daftar cabang
sudah dimuat sekali di awal (saat login atau menyegarkan halaman) dan dipakai ulang.

**Cabang non-aktif tetap muncul** dan tampak sama persis dengan cabang aktif.

---

## UF-02 — Membuka cabang baru sampai bisa berjualan

**Pelaku:** Owner (punya `branch.manage`). Ini alur inti modul ini.

1. Di `/branches`, klik **`+ Tambah Cabang`** (kanan atas). Halaman `/branches/create` terbuka.
2. Isi **Nama cabang**, mis. `Cabang Jakarta`.
3. Isi **Kode cabang**, mis. `JKT`. Kotak menampilkan huruf besar; maksimal 10 karakter.
4. Isi **Kota**, mis. `Jakarta`.
5. (Opsional) Isi **Alamat lengkap**.
6. Isi **Label lokasi stok awal**, mis. `Gudang Utama Jakarta`.
7. Klik **`Simpan Cabang`**.
8. Halaman langsung kembali ke `/branches`. **Tidak ada pemberitahuan sukses.** Cabang baru sudah
   ada di tabel.

**Yang terjadi di balik layar dan langsung terasa akibatnya** (semuanya sekaligus, gagal-bersama):

| Yang dibuat | Nilainya |
|---|---|
| Lokasi stok default cabang | Kode `GDG-JKT`, nama `Gudang Utama Jakarta` |
| Penghitung nomor order | Prefix `ORD-JKT` |
| Penghitung nomor pembayaran | Prefix `PAY-JKT` |
| Penghitung nomor surat jalan | Prefix `SJ-JKT` |
| Penghitung nomor retur penjualan | Prefix `RTR-JKT` |
| Penghitung nomor retur pembelian | Prefix `RTB-JKT` |

Setelah langkah ini, cabang **sudah bisa langsung** menerima produk, stok, order, surat jalan, dan
pembayaran — tanpa setup tambahan. Yang **belum** terjadi: pengguna belum punya akses ke cabang
itu (lanjut ke UF-03).

---

## UF-03 — Memberi akses cabang baru ke pengguna

**Pelaku:** Owner. Alur ini melintasi ke modul 02 (Users) — dicatat di sini karena tanpa langkah
ini cabang baru tidak bisa dipakai siapa pun, termasuk pembuatnya sendiri.

1. Buka **Pengaturan → Pengguna**, pilih pengguna, klik Edit.
2. Di daftar centang cabang, centang cabang baru.
3. Simpan.
4. Pengguna itu perlu **login ulang atau menyegarkan halaman** agar daftar cabang di sesinya
   diperbarui — daftar cabang yang bisa diakses disimpan di sesi lokal saat login.

**Efek samping yang perlu diketahui:** menyimpan akses cabang **menulis ulang seluruh** akses
cabang pengguna itu, dan **cabang pertama dalam urutan daftar** otomatis jadi cabang default
pengguna. Mencentang cabang baru bisa memindahkan cabang default pengguna tanpa penanda apa pun —
sudah tercatat sebagai KI-20.

---

## UF-04 — Berpindah cabang aktif

**Pelaku:** pengguna yang punya akses ke lebih dari satu cabang.

1. Di topbar, chip cabang berlabel **"Ganti Cabang"** dan berisi nama cabang sekarang. Klik chip.
2. Halaman **"Select Branch"** terbuka, menampilkan kartu per cabang: nama, kota, badge kode, dan
   badge **"Default"** pada cabang default pengguna.
3. Klik kartu cabang tujuan.
4. Sistem mengganti cabang aktif sesi, lalu **memuat ulang data dasar** (profil perusahaan, daftar
   cabang, pengaturan, definisi status order) dan **mereset status seluruh modul** sehingga tiap
   modul mengambil ulang datanya saat dikunjungi.
5. Toast hijau muncul: **"Cabang aktif diperbarui"** dengan deskripsi
   *"Data tiap modul akan dimuat ulang saat Anda mengunjunginya."*
6. Pengguna dikembalikan ke halaman asal (atau dashboard).

**Bila gagal:** toast merah **"Gagal mengganti cabang"** dengan deskripsi
*"Terjadi kesalahan saat memilih cabang."*

**Bila cabang tujuan ternyata sudah non-aktif:** permintaan gagal sebagai kesalahan internal
server — sudah tercatat sebagai KI-02.

**Pengguna satu cabang** tidak melihat alur ini sama sekali: chip topbar tidak bisa diklik.

---

## UF-05 — Mengoreksi identitas cabang (nama / kota / alamat)

**Pelaku:** Owner.

1. Di `/branches`, klik **`Edit`** pada baris cabang.
2. Halaman `/branches/:id/edit` terbuka, judul **"Edit Cabang"**, remah roti memuat nama cabang.
3. Ubah nama / kota / alamat sesuai kebutuhan. **Jangan** ubah kode.
4. Klik **`Simpan Perubahan`**.
5. Kembali ke `/branches`, data sudah berubah. Tanpa toast sukses.

**Yang aman:** nomor dokumen tidak terpengaruh sama sekali selama kode tidak berubah.

**Yang perlu diketahui:** menyimpan form ini **selalu** menandai cabang sebagai aktif, dan **selalu
menulis ulang** kode serta nama lokasi stok default cabang — meski pengguna hanya mengubah kota.
Bila nama gudang default sebelumnya diubah dari menu Stok, perubahan itu **tertimpa kembali**
(KI-42).

---

## UF-06 — Mengubah **kode** cabang

**Pelaku:** Owner. Alur ini punya konsekuensi permanen — perlu perhatian khusus saat rebuild.

1. Buka form edit cabang (langkah 1–2 UF-05).
2. Ubah **Kode cabang**, mis. dari `JKT` menjadi `SBY`.
3. Klik **`Simpan Perubahan`**.

**Yang berubah seketika:**

| Yang berubah | Dari | Menjadi |
|---|---|---|
| Prefix nomor order | `ORD-JKT` | `ORD-SBY` |
| Prefix nomor pembayaran | `PAY-JKT` | `PAY-SBY` |
| Prefix nomor surat jalan | `SJ-JKT` | `SJ-SBY` |
| Prefix nomor retur penjualan | `RTR-JKT` | `RTR-SBY` |
| Prefix nomor retur pembelian | `RTB-JKT` | `RTB-SBY` |
| Kode lokasi stok default | `GDG-JKT` | `GDG-SBY` |

**Yang TIDAK berubah:**

- **Nomor dokumen yang sudah terbit tetap apa adanya.** Nota lama tetap bernomor `ORD-JKT/...`.
- **Nilai penghitung tidak direset.** Bila nomor order terakhir `.../00007`, order berikutnya
  `.../00008` dengan prefix baru.
- Prefix nomor **mutasi stok** — penomoran mutasi memakai pola berbeda dan **tidak ikut
  diperbarui** (KI-46).

Akibat yang terlihat pengguna: dalam satu buku penjualan, dokumen sebelum dan sesudah perubahan
punya prefix berbeda dengan urutan nomor yang menyambung. Untuk kebutuhan laporan pajak/SPT ini
perlu diputuskan sadar — lihat KI-40.

**[PERLU KONFIRMASI]** Apakah kode cabang pernah diubah di production? Bila ya, apakah dokumen
lama pernah menimbulkan kebingungan saat rekap?

---

## UF-07 — Mengganti nama gudang lewat form cabang

**Pelaku:** Owner.

1. Buka form edit cabang.
2. Ubah **Label lokasi stok awal**, mis. dari `Gudang Utama Jakarta` menjadi `Gudang Depan`.
3. Simpan.
4. Buka menu **Stok → Lokasi**: lokasi default cabang sekarang bernama `Gudang Depan`.

Ini satu-satunya jalur di mana modul Cabang menulis ke data gudang. Arah sebaliknya tidak
sinkron: mengganti nama lokasi default dari menu Stok **tidak** memperbarui label di form cabang,
dan akan tertimpa saat cabang disimpan berikutnya (KI-42).

---

## UF-08 — Mencoba kode cabang yang sudah dipakai

**Pelaku:** Owner.

1. Buka form tambah cabang.
2. Isi kode yang sudah dipakai cabang lain, mis. `BLR`.
3. Klik **`Simpan Cabang`**.
4. Muncul toast merah **"Gagal menyimpan cabang"** — **tanpa deskripsi**.
5. Pengguna tetap di form; isian tidak hilang.

**Yang hilang di jalan:** server sebenarnya mengirim pesan jelas
`Kode cabang 'BLR' sudah digunakan`, tapi antarmuka tidak menampilkannya. Pengguna harus menebak
sendiri bahwa masalahnya ada di kode, bukan di field lain. Lihat KI-43 dan akar bersama KI-04/KI-17.

Perbandingan huruf besar-kecil **tidak** membedakan: mengetik `blr` tetap ditolak sebagai duplikat
`BLR`.

---

## UF-09 — Admin membuka halaman cabang (tanpa hak kelola)

**Pelaku:** Admin (punya `branch.view`, tidak punya `branch.manage`).

1. Menu **"Cabang & Lokasi"** tetap terlihat di sidebar.
2. Halaman `/branches` terbuka normal, tabel terisi lengkap.
3. Tombol **`+ Tambah Cabang`** **tidak ada**.
4. Kolom **Aksi** beserta seluruh tombol Edit **tidak ada**.
5. Bila keadaan kosong muncul, kartu "Belum ada cabang" tampil **tanpa tombol aksi**.
6. Mengetik langsung `/branches/create` di alamat → dialihkan ke halaman **403**.

---

## UF-10 — Staff / Kasir mencoba membuka halaman cabang

**Pelaku:** Staff atau Kasir (tidak punya `branch.view`).

1. Menu **"Cabang & Lokasi"** **tidak muncul** di sidebar.
2. Mengetik `/branches` langsung → dialihkan ke halaman **403**.

**Efek samping yang tidak terlihat langsung:** peran ini juga gagal memuat daftar cabang saat
aplikasi dijalankan. Akibat yang bisa dilihat pengguna: **baris alamat cabang hilang dari kop nota
yang mereka cetak**, karena alamat cabang tidak tersedia di sesi mereka. Lihat KI-47.

---

## UF-11 — Menutup cabang yang tidak dipakai lagi

**Pelaku:** Owner.

1. Buka `/branches`, cari cabang yang ingin ditutup.
2. Klik **`Edit`**.
3. **Tidak ada** pilihan status, tidak ada tombol Hapus, tidak ada tombol Arsipkan.
4. Alur berhenti di sini.

**Kesimpulan:** cabang yang sudah dibuat **tidak bisa ditutup atau dihapus lewat aplikasi**.
Satu-satunya jalan adalah mengubah status langsung di basis data — dan bahkan itu akan
dibatalkan diam-diam bila cabang tersebut disimpan ulang dari form (form selalu mengirim status
aktif). Lihat KI-36.

Padahal status non-aktif **punya arti nyata** di sistem: cabang non-aktif hilang dari daftar
pemilihan cabang saat login, ditolak sebagai tujuan mutasi stok, dan berhenti dihitung dalam
metrik harian.

**[PERLU KONFIRMASI]** Apakah pernah ada cabang yang perlu ditutup? Bila belum pernah, kebutuhan
ini mungkin memang belum muncul — tapi tetap perlu diputuskan untuk sistem baru.
