# Test Cases — Modul 20 Audit Log

**Kelompok A — 17 kasus.** T-01…T-11 dicerminkan dari `audit-log.service.spec.ts`
(11 test: 3 `log` + 8 `list`); T-12 dari hook (5 test); T-13…T-15 dari halaman/slice/E2E;
T-16…T-17 dari `adapters.test.ts` (2 test `toAuditLog`).

## Tulis (T-01–T-03)

- T-01 `log` lengkap → `create` dipanggil dengan semua field + `save` dipanggil.
- T-02 `log` tanpa opsional → `idBranch/idActor/before/after/metadata = null`,
  `happenedAt` = Date kini (>= waktu panggil).
- T-03 Aktor `system` tanpa id → tersimpan; UI menampilkan `System`.

## Baca (T-04–T-11)

- T-04 `page: 2, limit: 10` → `skip(10)`, `take(10)`, `meta = { page: 2, limit: 10, total: 25 }`.
- T-05 Filter kosong → default `skip(0)`, `take(20)`, `meta.page = 1`, `meta.limit = 20`.
- T-06 `limit: 500` → `take(100)` (cap!).
- T-07 `id_branch: 5` → klausa `al.id_branch = :idBranch`.
- T-08 `entity_type: 'order'` → klausa `al.entity_type = :et` (persis!).
- T-09 `action_key: 'order.create'` → klausa `LIKE '%order.create%'` (substring!).
- T-10 Tanpa filter → nol `andWhere`.
- T-11 Tiga filter bersamaan → tepat 3 `andWhere`.

## Halaman, Slice & E2E (T-12–T-17)

- T-12 Hook (`use-audit-log-module.test.ts`, 5 test): `idle`+login → reload 1×;
  belum-login/`loading`/`ready` → 0×; return hanya `activeWorkspaceData` (identitas referensi!).
- T-13 Halaman: Functional — [PERLU KONFIRMASI] tak ada test halaman; verifikasi manual
  (teks §4 + paginasi 25 + empty-state).
- T-14 E2E 10 (`10-observability-permission-hardening.spec.ts:102-119`): setelah
  order + deliver-goods pay_now, `order.create`/`order`, `stock.adjust`/
  `inventory_balance`, `payment.create`/`payment` masing-masing `> 0` (limit 20).
  Plus E2E 08 (`08-inventory-return-transfer-adjustment.spec.ts:365-370`): setelah 3× adjust, `stock.adjust`/`inventory_balance`
  `> 0` (limit 10). Plus smoke menu 01 (`/audit-logs` cocok `/Riwayat Aktivitas/i`).
- T-15 Kunci tak berlabel (mis. `member_type.create`) → tampil fallback
  `Member - Type - Create` (bukan blank!).
- T-16 Adapter (`adapters.test.ts:597-624`): `toAuditLog` dengan `id_actor: 88` +
  `id_branch: 99` tanpa referensi → `User #88` / `Branch #99`, `entityLabel`
  `stock_movement #12`, `description` = actionKey mentah.
- T-17 Adapter (`adapters.test.ts:626-644`): tanpa aktor/cabang/stempel → `System`,
  `branchId/branchName undefined`, `happenedAt` = waktu-sistem-palsu
  (ditopang `vi.setSystemTime`, `:137-138` — cek! cakupannya).

## Celah Test

- G-01 Tanpa test halaman (render/kolom/paginasi/empty/error-diam).
- G-02 Tanpa test 403/401 endpoint list.
- G-03 Tanpa test batas stores (100 user → degradasi nama).
- G-04 Tanpa test E2E filter UI (tak ada UI filter — konsisten, tapi API filter hanya
  diuji unit + dipakai E2E mentah).
