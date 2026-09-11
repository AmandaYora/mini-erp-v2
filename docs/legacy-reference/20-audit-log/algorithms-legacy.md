# Algorithms Legacy (Konsep) — Modul 20 Audit Log

**Kelompok B — ide algoritma**, bukan kode. Tiga logika + dua pola kipas penting.

## 1. Peta Algoritma → Lokasi

| Algoritma | Lokasi legacy | Invariansi rebuild |
|---|---|---|
| Tulis jejak + stempel | `audit-log.service.ts#log` (`:26-41`: `create` + null-kan opsional `?? null` + `happenedAt = new Date()` + `save`) | Null-kan opsional; stempel server; tanpa endpoint tulis |
| Daftar + filter + paging | `audit-log.service.ts#list` (`:43-59`: default page 1/limit 20, `Math.min(limit,100)`, `where` perusahaan + `orderBy happenedAt DESC` + `skip/take`, 3 `andWhere` opsional) | Lingkup perusahaan; aksi = substring, entitas/cabang = persis; cap-100; terbaru dulu |
| Resolusi nama + label + fallback | `toAuditLog` (`org.adapter.ts:36-57`) + peta label + `formatAction/Entity/Description` (`audit-log-pages.tsx:137-164`) | Tak pernah blank: `System`/`User #`/`Branch #`/fallback-kunci; waktu `id-ID` tanpa detik |
| Kipas tutup-buku (1 aksi → N+2 baris) | `closeDay` (`finance-posting.service.ts:897-955`: `:901` sync→1 baris `sync`, `:927-933` posting per source→N baris `journal.post`, `:944-952` 1 baris `close_day`) | Bila rebuild menggabung/memisah, jumlah baris per tutup-buku berubah — E2E hanya asersi `>0`, aman |
| Sekuens multi-audit (1 aksi → 2-3 baris) | `deliver-goods` (`delivery.service.ts:570-625`: `payment.create` bila COD + `order.approve_credit` bila alih-net + `order.goods_delivered`); `receive-goods` analog (`goods-receipt.service.ts:373-424`) | Urutan baris = urutan `await`; kegagalan di tengah menyisakan baris awal (cek! lihat business-rules temuan baru) |

## 2. Keputusan Desain (untuk rebuild)

- **Tanpa-FK sengaja**: jejak outlives master — jangan "rapikan" dengan FK +
  cascade (menghapus histori saat master dihapus justru bug!).
- **Substring untuk aksi, persis untuk entitas**: pola pencarian "semua tentang
  order" (`action_key: 'order'`) adalah fitur yang dipakai E2E — pertahankan
  semantiknya bila mengganti mekanisme filter.
- **Tanpa BranchGuard sengaja**: audit adalah alat review lintas-cabang pemilik —
  jangan kunci per-cabang tanpa keputusan.
- **Gagal-diam di UI**: halaman tak pernah menampilkan error muat — pertahankan
  atau ubah diputuskan bersama KI-129/KI-131 (jangan ubah diam-diam!).
- **Audit bukan bagian transaksi pemanggil**: `log()` memakai repositori modul
  sendiri tanpa parameter manager — rebuild dengan outbox/transaksi-bersama akan
  mengubah jaminan kegagalan (BR-19); putuskan eksplisit, jangan warisi diam-diam.
