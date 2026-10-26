# Analisis 004 — Satu sumber rekening perusahaan

**Tanggal:** 2026-10-26 · **Pemicu:** pengguna merasa input rekening ganda (Pengaturan Perusahaan ›
"Kontak dan rekening" dan Keuangan › Akun › "Rekening bank") · **Lanjutan dari:**
[analysis/002](../002-rekening-tujuan-dan-retur-sebagian/analysis.md), [ADR-0026](../../knowledge/decisions/ADR-0026-rekening-tujuan.md)

Folder ini tanpa `flow.mmd`/`sequence.mmd`: tidak ada alur data baru. Perubahannya memindahkan satu input
dan memperbaiki satu aturan cadangan cetak.

## 1. Keadaan sekarang (rilis `3c1c3ba`)

| | A. Profil dokumen (Pengaturan › Perusahaan) | B. Master rekening (Keuangan › Akun) |
|---|---|---|
| Penyimpanan | Teks `bankAccounts[]` di JSON `company_settings.document_profile` | Tabel `finance_bank_accounts`, 1:1 ke akun kas GL |
| Isi | bank, pemilik, nomor | bank, pemilik, nomor, akun GL, default, tampil di dokumen, urutan, status |
| Izin ubah | `company.manage` | `finance.manage` |
| Dipakai untuk | Cetak daftar rekening (kop/kaki nota, struk) **hanya bila B tidak punya rekening tercetak** | Cetak, "Transfer ke", pemilih kasir/POS, jurnal per rekening, wajib pilih saat transfer |
| Asal data | Sistem lama (ikut ETL), lalu diubah manual | Baru (ADR-0026), kosong sampai aktivasi |

Kode: `DocumentProfileForm.tsx` §"Rekening bank" (editor A), `usePrintProfile.ts` (`fromMaster.length > 0 ? master :
teks`), `document.ts` `documentBankAccounts` (hanya aktif + tampil di dokumen).

**Masalah yang ditemukan:**
1. **Dua editor untuk satu fakta.** Nomor rekening yang sama bisa diketik di dua tempat, oleh dua peran berbeda,
   dan isinya bisa saling menyimpang. Keterangan di form A ("hanya dipakai selama daftar keuangan kosong") tidak
   mencegah orang tetap mengisinya.
2. **Cadangan cetak bocor (bug laten).** Aturan cetak memakai A bila B tidak punya rekening yang *aktif dan tampil
   di dokumen*, bukan bila B *kosong*. Bila semua rekening master diarsipkan atau disetel tidak tampil, nota diam-diam
   kembali mencetak teks lama dari A, yang mungkin sudah usang. Padahal kasir tetap diwajibkan memilih rekening dari B.
3. Pengguna tidak tahu mana yang benar, karena keduanya tampil sebagai form yang bisa disunting.

## 2. Riset: bagaimana sistem profesional memodelkannya (diakses 2026-10-26)

| Sumber | Temuan |
|---|---|
| [Odoo — Journals](https://www.odoo.com/documentation/19.0/applications/finance/accounting/get_started/journals.html) | Jurnal bank menunjuk akun GL Bank & Kas dan **nomor rekening**. Nomor rekening itu adalah rekening milik kontak perusahaan. Satu record, dipakai jurnal, pembayaran, dan faktur |
| [Modul IBAN on invoice (Odoo)](https://apps.odoo.com/apps/modules/18.0/auguria_iban_on_invoice), [OCA Recipient Bank Currency](https://odoo-community.org/shop/account-invoice-recipient-bank-currency-716003) | Rekening di faktur ("Recipient Bank") diambil bawaan dari rekening perusahaan, bisa diganti per faktur. Tidak ada daftar teks terpisah |
| [Forum Odoo — bank details on invoice](https://www.odoo.com/forum/help-1/bank-details-on-invoice-254277) | Menaruh rekening di teks "Syarat & ketentuan" hanyalah jalan pintas bila rekening terstruktur tidak tampil |
| [Frappe/ERPNext forum — Bank Account Sales Invoice](https://discuss.frappe.io/t/bank-account-sales-invoice/50454) | Pengguna menolak mengetik rekening di T&C: rekening di faktur seharusnya ditarik dari rekening default perusahaan (master) |
| [Xero — branding theme](https://helpdesk.getpinch.com.au/xero-how-do-i-configure-my-bank-account-and-branding-themes) | Pola lain: rekening hanya teks bebas di templat ("Terms & Payment Advice"). Murah, tetapi tidak terhubung ke buku besar |
| [Panorama — duplicate data entry](https://www.panorama-consulting.com/how-to-prevent-duplicate-data-entry-by-eliminating-erp-shadow-systems/), [CluedIn — system of record](https://www.cluedin.com/resources/articles/how-to-fix-inconsistent-master-data-between-erp-and-crm-systems) | Input ganda = celah konfigurasi. Obatnya **satu pemilik data per domain**. Sinkron dua arah memicu saling timpa |

Kesimpulan riset: daftar teks (pola Xero/T&C) dan master terstruktur (pola Odoo/ERPNext) adalah **dua generasi
model yang sama**. mini-erp punya keduanya karena ADR-0026 menambah generasi kedua tanpa mematikan yang pertama.
Ini wajar sebagai **masa transisi**, bukan desain akhir. Desain akhirnya: **satu record, satu tempat ubah, banyak
tempat baca.**

## 3. Opsi

| Opsi | Inti | Penilaian |
|---|---|---|
| 1. Biarkan dua-duanya (sekarang) | Keterangan saja | Redundan, bug #2 tetap ada. **Tolak** |
| 2. Sinkron dua arah A ↔ B | Simpan di satu tempat, salin ke tempat lain | Rumit, rawan saling timpa (CluedIn), dan A tidak punya akun GL. **Tolak** |
| 3. Pindahkan editor master ke Pengaturan Perusahaan | Satu editor di Perusahaan | Rekening = akun kas GL, jadi harus dikelola peran keuangan (`finance.manage`), bukan `company.manage`. Odoo juga mengelolanya di Akuntansi. **Tolak** |
| **4. Master = satu-satunya sumber; Perusahaan hanya menampilkan** | Hapus editor A; Perusahaan menampilkan rekening master (baca saja) + tautan "Kelola rekening"; teks A dibekukan sebagai cadangan cetak sampai master berisi, lalu dipindahkan sekali | **Dipilih** |

## 4. Rekomendasi implementasi (Opsi 4)

**Prinsip:** rekening perusahaan hanya dibuat/diubah di Keuangan › Akun › Rekening bank. Tempat lain hanya membaca.

1. **Pengaturan › Perusahaan › "Kontak dan rekening"** (`DocumentProfileForm.tsx`)
   - Hapus editor "Rekening bank" (input bank/pemilik/nomor, Tambah/Hapus). Judul bagian jadi "Kontak".
   - Ganti dengan panel **"Rekening di dokumen"** (baca saja):
     - master berisi → daftar rekening master yang aktif & tampil di dokumen, urut `sortOrder`, tanda *Default*;
     - master kosong dan profil masih punya teks lama → daftar teks itu, berlabel "Dari data sistem lama, dicetak
       sampai rekening dibuat di Keuangan";
     - tombol **Kelola rekening** → `/finance/accounts?tab=rekening` (hanya bila `finance.manage`; selain itu
       keterangan "Diatur oleh tim keuangan").
   - Menyimpan form tetap **mempertahankan** `bankAccounts` yang ada di JSON (tidak dihapus diam-diam), karena
     masih menjadi cadangan cetak sebelum aktivasi.
2. **Keuangan › Akun › Rekening bank** (`BankAccountsPanel.tsx`)
   - Bila master kosong dan profil punya teks lama: Notice "N rekening dari profil dokumen belum dipindahkan", dengan
     tombol **Pindahkan** per baris yang membuka form rekening terisi bank/pemilik/nomor. Pengguna tinggal memilih
     atau membuat akun GL (1011, 1012, …) dan default. Duplikat bank+nomor sudah ditolak server.
3. **Aturan cetak** (`usePrintProfile.ts`): pakai teks lama **hanya bila master tidak punya rekening sama sekali**
   (status apa pun). Begitu master berisi, dokumen hanya mencetak rekening master yang aktif & tampil di dokumen,
   walau daftarnya kosong. Ini menutup bug #2.
4. **Backend/skema:** tidak ada perubahan API, migrasi, atau izin. `document_profile.bankAccounts` tetap terbaca untuk
   data lama. Setelah aktivasi stabil, teks itu boleh dikosongkan (opsional, tidak wajib).
5. **Uji:**
   - vitest `DocumentProfileForm`: tidak ada input rekening; daftar baca saja (master / teks lama); tautan sesuai izin;
     simpan tidak menghapus `bankAccounts`;
   - vitest `BankAccountsPanel`: notice impor + isi awal form;
   - vitest aturan cetak: master hanya berisi rekening arsip/tersembunyi → tidak jatuh ke teks lama;
   - e2e E26 diperluas: Pengaturan menampilkan rekening master + tautan.
6. **Dokumen:** revisi ADR-0026 butir 6 (cadangan teks = hanya saat master kosong, editor teks dihapus), RUNBOOK §6f
   langkah 1 (pakai **Pindahkan** dari daftar lama), STATUS, glosarium bila perlu.

**Akibat yang perlu disadari:** sebelum aktivasi, daftar rekening di nota tidak bisa lagi diubah dari Pengaturan.
Untuk mengubahnya, buat rekening di master (itulah aktivasi: kasir wajib memilih rekening untuk transfer). Di v2 saat
ini (data percobaan yang akan dimuat ulang) ini tidak berisiko. Sesudah cutover, aktivasi memang langkah pertama RUNBOOK
§6f, jadi masa "hanya teks lama" berlangsung singkat.

## 5. Alur pengguna sesudahnya

- **Admin perusahaan** membuka Pengaturan › Perusahaan, lalu melihat rekening yang tercetak (baca saja) dan tombol
  "Kelola rekening".
- **Keuangan** membuka Rekening bank. Saat pertama, sistem menawarkan memindahkan rekening lama: Pindahkan, pilih
  akun, simpan. Sejak itu semua dokumen, POS, dan pembayaran memakai daftar ini.
- **Kasir** tidak berubah: memilih rekening tujuan saat transfer (setelah aktivasi).

## 6. Belum terverifikasi

- Isi `document_profile.bankAccounts` di production (pembacaan DB production ditolak pengaman sesi). Perkiraan: berisi
  rekening dari sistem lama hasil ETL.
- Apakah bot WhatsApp membaca rekening: pencarian kode tidak menemukan pemakaian `bankAccounts` di modul `whatsapp`.

## 7. Revisi keputusan pengguna (2026-10-26): pemindahan otomatis, tanpa tombol

Pengguna menolak tombol **Pindahkan** di §4 butir 2: pemindahan hanya terjadi sekali, jadi tombolnya akan menjadi sisa
yang tidak terpakai. Yang diimplementasikan sebagai gantinya:

- **`bootstrap.ImportLegacyBankAccounts`**, dipanggil `cmd/server` setiap start (setelah seed otorisasi). Ia membaca
  `document_profile.bankAccounts` lewat layanan `company`, lalu memanggil `finance.ImportLegacyBankAccounts`. Lapisan ini
  di composition root, sehingga `finance` tidak membaca tabel `company`.
- **Hanya bila master kosong sama sekali.** Sesudah ada satu rekening apa pun (termasuk yang diarsipkan), langkah ini tanpa
  efek. Setelah cutover final memuat ulang data legacy, master kembali kosong, jadi pemindahan berjalan lagi pada start
  berikutnya (RUNBOOK §6f: mulai ulang aplikasi sesudah muat ulang).
- **Akun buku besar:** rekening pertama menjadi default di atas akun pemetaan `bank` (1010), yang sudah menampung semua
  transfer. Dengan satu rekening, tidak perlu jurnal reklasifikasi sama sekali. Rekening berikutnya mendapat akun baru 1011,
  1012, … dan saldo awalnya direklasifikasi akuntan dari 1010. Ini menyimpang dari rencana awal ADR-0026 (semua rekening
  memakai akun baru, lalu 1010 dikosongkan). Penyimpangan dipilih karena pemindahan berjalan tanpa manusia: memakai 1010
  untuk rekening pertama membuat saldo langsung bermakna tanpa menunggu jurnal.
- Urutan dan tanda tampil di dokumen mengikuti teks lama. Baris kosong dan nomor ganda (tanpa spasi/titik/strip)
  dilewati. Atas nama kosong diisi nama perusahaan. Semua baris divalidasi sebelum ada yang ditulis. Hasil atau galat
  tercatat di log, dan galat tidak menghentikan start. Teks profil tidak diubah.
- UI: §4 butir 1 dan 3 tetap. Butir 2 (kotak + tombol Pindahkan) dibatalkan. Uji: integrasi Go
  `TestImportLegacyBankAccountsOnce` / `TestImportLegacyBankAccountsNothingToMove`, vitest form Pengaturan, dan e2e E26
  (Pengaturan hanya menampilkan master + tautan kelola).
