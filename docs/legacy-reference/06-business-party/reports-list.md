# Reports List — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Laporan yang dihasilkan modul ini
+ logika perhitungannya. Diverifikasi dari controller, service, seluruh halaman FE, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan

Modul Business Party **tidak menghasilkan satu pun laporan** — tidak ada endpoint rekap, halaman
rekap, ekspor rekap, maupun agregasi terjadwal milik modul ini:

| Yang dicari | Hasil |
|---|---|
| Endpoint rekap pihak | Tidak ada (`customers/*`, `suppliers/*`, `customer-addresses/*` semuanya CRUD/list) |
| Halaman rekap di `modules/business-party/` | Tidak ada (2 halaman: daftar + form, keduanya operasional) |
| Ekspor CSV/PDF rekap pelanggan/pemasok | Tidak ada |
| Job agregasi | Tidak ada |
| Test yang mengasumsikan laporan pihak | Tidak ada |

Satu-satunya angka agregat — `"{n} pelanggan ditemukan."` / `"{n} pelanggan di Sampah."` — adalah
`meta.total` query list (hitung baris per filter), bukan laporan.

## 2. Data modul ini sebagai BAHAN laporan modul lain

| Laporan (modul pemilik) | Field pihak yang dipakai | Kontrak yang harus dijaga |
|---|---|---|
| Saldo & ledger pihak (14 Payment: `party-balances`, `party-ledger`) | id + nama + tipe pihak; **termasuk pihak terarsip** (kolektor tetap menagih — dikunci E2E 22-C) | Arsip tidak menghapus relasi; hapus `archived_at` dari filter saldo merusak penagihan |
| Daftar/export order + pencarian order (08–09) | kode/nama/telepon/email pihak (pencarian order mencakup identitas pihak); nama **live** by-id | Rename mengikuti tanpa duplikat (E2E 22-A) |
| Nota/SJ/cetak (08–11) | snapshot ship-to + snapshot item; fallback alamat primer/kontak | Snapshot stabil walau buku berubah (filosofi §BR-31) |
| Harga member (07) | `id_member_type` + badge nama | Lepas/arsip member konsisten (E2E 22-B) |
| Riwayat Aktivitas (20 Audit) | 11 `actionKey` modul ini | Satu-satunya jejak tertulis (update mencatat seluruh baris di `before`) |

## 3. Bahan mentah bila laporan pihak ingin dibuat

| Sumber | Isi | Batas |
|---|---|---|
| `customers/list` / `suppliers/list` (page/limit ≤100, `status`, `search`) | Semua field + relasi member + `meta.total` | Hanya satu tipe per panggilan; arsip hanya via `status=archived` |
| `customers/detail` / `suppliers/detail` | Satu pihak + member | Menolak arsip (404) |
| `customer-addresses/list` per pelanggan | Buku alamat primer-dulu | Satu request per pelanggan; tanpa paginasi |
| `audit_logs` (`customer.*`, `supplier.*`, `customer_address.*`) | Siapa + kapan (+ seluruh baris di `before` update pihak) | Alamat update hanya label/alamat/primer di before/after |

## 4. Yang eksplisit BUKAN laporan

- **Tab Sampah**: filter arsip operasional (dengan restore), bukan laporan penghapusan.
- **Kolom `Dihapus`**: satu tanggal per baris, tanpa agregasi.
- **Badge Member / opsi `(kode)`**: tampilan relasi, bukan rekap member.
