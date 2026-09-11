# Numbering Sequence — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Satu-satunya penomoran:
surat transfer-dokumen (`stock/transfers/create` → `generateStockTransferNumber`,
`stock-transfer.service.ts:470-493`).

---

## 1. Format nomor transfer

```
TRF-{IDCABANG}/{TAHUN}/{5 digit}
Contoh: TRF-2/2026/00001 (contoh nyata KI-46 modul 04!)
```

| Aspek | Aturan (dari `generateStockTransferNumber`) |
|---|---|
| Baris sequence | `branch_document_sequences` kunci `stock_transfer` per-cabang; **buat-otomatis** bila tak ada (`prefix: TRF-{idBranch}`, `currentValue: 0`, `resetPolicy: 'none'`, `formatTemplate: null` — satu-satunya generator yang membuat-baris!); fallback-memori `TRF-{id}` bila tanpa-prefix |
| Kunci konkurensi | `pessimistic_write` saat baca-baris + increment + simpan dalam transaksi-buat-dokumen (tanpa-nomor-ganda!) |
| Periode | Tahun-server (`new Date().getFullYear()`); **tanpa reset** (`reset_policy='none'` — satu-satunya eksplisit!): urut-naik-lintas-tahun! |
| Format | `{prefix}/{tahun}/{currentValue padStart 5 '0'}` (contoh: nilai-1 → `00001`!) |
| Abadi | Tak ditulis-ulang (kirim/terima/batal pertahankan!); alasan-movement (`Transfer {nomor} ke...` / `Terima transfer {nomor} dari...`) + audit-create/dispatch/receive/cancel + cetak-surat! |
| Status-nomor | Draf pun sudah-bernomor (nomor ≠ status!); batal-tak-mendaur-ulang (cek!) |

**Catatan KI-46**: satu-satunya dokumen berkode-internal-cabang (bukan kode-cabang!) —
disengaja-teknis (baris dibuat-otomatis tanpa kode), perlu keputusan tampil (modul 04).

## 2. Yang bukan penomoran

ID saldo/mutasi/lokasi/reservasi/transfer (`id_*` auto-increment!) + `reservation_key`
(bebas-kasir, trim, per-cabang+user!) + kode-lokasi (manual-UPPER + unik-non-arsip-409!) +
`RUSAK` (tetap!) + tanggal/waktu (`transfer_date` default-sekarang, bisa-override! +
`dispatched/received/cancelled_at`!) + `force_transfer` (flag-sesaat!) + template-Excel
(tanpa-nomor!) + `idReference`-polimorfik (bukan-urutan!) + `paired_movement_id`
(pasangan, bukan-urutan!).
