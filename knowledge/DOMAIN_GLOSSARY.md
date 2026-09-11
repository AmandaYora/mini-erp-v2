# Domain Glossary — mini-erp

Istilah bisnis yang dipakai konsisten di seluruh kode, dokumen, dan UI. Sumber: praktik yang sudah
berjalan di sistem lama (lihat `docs/legacy-reference/`). Tambahkan istilah baru di sini saat
sebuah modul mendefinisikan konsep baru — jangan biarkan istilah domain hanya hidup di kepala
satu orang.

| Istilah | Arti |
|---|---|
| **Company** | Satu perusahaan pemilik instalasi standalone (bukan multi-tenant). Baris tunggal di modul `company`; tidak ada tabel `companies`, tidak ada `company_id` di tabel mana pun (ADR-0009). |
| **Branch (Cabang)** | Unit operasional dengan gudang & penomoran dokumen sendiri. Modul `branch`. |
| **UOM (Unit of Measure)** | Satuan. Produk punya *satuan jual* (mis. dus) dan *satuan stok* (mis. pcs), dihubungkan lewat faktor konversi — kontrak milik modul `product`, dipakai `stock`/`purchasing`/`sales`. |
| **Varian Default** | Setiap produk selalu punya minimal satu baris varian, meski tidak dikonfigurasi bervarian secara eksplisit — jangkar saldo stok & harga. |
| **HPP (Harga Pokok Penjualan)** | Biaya perolehan barang terjual, dihitung modul `finance` dari harga beli `product` + mutasi `stock`. |
| **Party** | Istilah gabungan untuk customer & supplier di modul `party` (satu tabel, dibedakan `party_type`). |
| **Member Type / Member Pricing** | Kelompok customer dengan aturan harga khusus (persentase/nominal), dihitung modul `party` saat order dibuat. |
| **PO (Purchase Order)** | Order pembelian ke supplier — modul `purchasing`. |
| **SO (Sales Order)** | Order penjualan ke customer — modul `sales`. Termasuk kanal POS (`channel = 'pos'`). |
| **Goods Receipt (Penerimaan Barang)** | Konfirmasi barang PO sudah diterima gudang — modul `goodsreceipt`. |
| **Delivery Note / SJ (Surat Jalan)** | Dokumen pengiriman barang SO ke customer — modul `delivery`. |
| **Snapshot (Item Dokumen)** | Salinan nama/harga produk yang dibekukan saat dokumen dibuat — nota historis tidak ikut berubah saat master data produk berubah kemudian. Kontrak lintas modul, lihat SYSTEM_DESIGN §7. |
| **Stock Balance / Stock Movement** | Saldo stok saat ini vs riwayat mutasi yang membentuknya — selalu berpasangan (kontrak modul `stock`). |
| **Document Sequence (Penomoran Dokumen)** | Nomor berjalan per jenis dokumen per cabang, prefix dari kode cabang — kontrak milik modul `branch`. |
| **Journal Entry / Journal Line** | Baris jurnal akuntansi (double-entry) — satu-satunya sumber laporan keuangan, modul `finance`. |
| **Branch Scope** | `branchId` yang selalu diturunkan dari sesi terautentikasi (modul `auth`), tidak pernah dari body request. Tidak ada scope perusahaan (ADR-0009). |

## Peran Pengguna (Role Bawaan)

Diwarisi dari sistem lama (lihat
[docs/legacy-reference/02-users-roles-permissions/](../docs/legacy-reference/02-users-roles-permissions/)) —
detail permission per role ditetapkan saat modul `user` didesain:

| Role | Fokus operasional |
|---|---|
| Owner | Akses penuh, termasuk kelola akun pengguna lain |
| Admin | Kelola struktur akses (role & permission), operasional harian |
| Kasir | Transaksi POS, pembayaran |
| Staf Gudang | Stok, penerimaan barang, pengiriman |
| Staf Keuangan | Modul `finance` — jurnal, laporan, tutup periode |
