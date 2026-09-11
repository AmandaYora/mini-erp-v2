# Numbering Sequence — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format penomoran dokumen.

---

## 1. Satu-satunya penomoran: kode otomatis pihak

Bila Kode dikosongkan di form mana pun (menu, shortcut order, quick-add POS), server menerbitkan:

```
CUS-001, CUS-002, …   (pelanggan)
SUP-001, SUP-002, …   (pemasok)
```

| Aspek | Aturan (dari `generateNextPartyCode`) |
|---|---|
| Prefix | `CUS-` / `SUP-` (per tipe; manual tak berpola tidak memengaruhi) |
| Suffix | Angka tertinggi + 1 di antara kode berpola murni `PREFIX+digit` (regex `^PREFIX(\d+)$`), **termasuk baris terarsip**; `padStart(3, '0')`, tumbuh alami melewati 999 |
| Awal | `CUS-001` / `SUP-001` bila tak ada kandidat |
| Anti-pakai-ulang | Arsip ikut dihitung → kode auto tidak pernah dipakai ulang → restore auto-code tidak pernah bentrok |
| Bukan sequence terkunci | Tanpa transaksi/lock: konkuren bersamaan bisa kembar → satu gagal unique-key (→ KI-64) |
| Manual vs auto | Manual berpola (`CUS-007`) menggeser auto berikutnya (`CUS-008`); manual tak berpola (`CUST-9999`, `TOKO-A`) diabaikan generator tetapi tetap memblokir pemakaian ulang kodenya sendiri (BR-02) |

Contoh terkunci (unit test): `{CUS-001, CUS-003, CUST-9999}` → berikutnya `CUS-004`; `{SUP-005}` →
`SUP-006`; kosong/whitespace → `CUS-001`.

## 2. Yang bukan penomoran

- **Kode manual**: teks bebas ≤50 char, case-insensitive (collation), di-trim, unik lintas arsip.
  Bukan urutan, tidak ada pola wajib.
- **ID teknis** (`id_business_party`, `id_address`): auto-increment, FK oleh order/alamat/snapshot;
  tidak tampil ke user.
- **Label alamat** (`Utama` backfill dkk.): teks bebas ≤120, boleh sama antar alamat.
- Nomor order/payment/SJ milik modul Order/Branch — modul ini hanya dirujuk (`id_related_party`,
  `id_ship_to_address`).
