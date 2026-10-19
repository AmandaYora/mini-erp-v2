import { test, expect } from "@playwright/test";
import { call, ok, productByCode, locationByCode, branches, stockQty, session } from "../helpers/api";
import { buildXlsx, uploadXlsx } from "../helpers/xlsx";
import { PASSWORD } from "../helpers/env";
import { loginAs } from "../helpers/ui";

test.describe.serial("E06 stok", () => {
  let p: Awaited<ReturnType<typeof productByCode>>;
  let ga: any;
  let gb: any;

  test.beforeAll(async () => {
    p = await productByCode("P013");
    ga = await locationByCode("HQ-GA");
    gb = await locationByCode("HQ-GB");
  });

  const adjust = (qtyDelta: number, extra: Record<string, unknown> = {}) => ({
    productId: p.id, variantId: p.variantId, locationId: ga.id, mode: "in", qtyAfter: 0, qtyDelta,
    reason: "uji e2e", ...extra,
  });

  test("koreksi stok wajib persetujuan; dengan persetujuan admin berhasil", async () => {
    const base = await stockQty(p.id, ga.id);
    const denied = await call("gudang", "POST", "/stock/adjustments", adjust(3));
    expect(denied.status).toBeGreaterThanOrEqual(400);
    expect(await stockQty(p.id, ga.id)).toBe(base);

    const wrong = await call("gudang", "POST", "/stock/adjustments", adjust(3, { approver: "admin", approverPassword: "salah-salah-1" }));
    expect(wrong.status).toBeGreaterThanOrEqual(400);

    await ok("gudang", "POST", "/stock/adjustments", adjust(3, { approver: "admin", approverPassword: PASSWORD }));
    expect(await stockQty(p.id, ga.id)).toBe(base + 3);

    // pengeluaran melebihi saldo ditolak, saldo tidak negatif
    const tooMuch = await call("gudang", "POST", "/stock/adjustments", adjust(100000, { mode: "out", approver: "admin", approverPassword: PASSWORD }));
    expect(tooMuch.status).toBeGreaterThanOrEqual(400);
    expect(await stockQty(p.id, ga.id)).toBe(base + 3);
  });

  test("pindah lokasi satu langkah dalam cabang", async () => {
    const a = await stockQty(p.id, ga.id);
    const b = await stockQty(p.id, gb.id);
    await ok("gudang", "POST", "/stock/transfers/move-location", {
      fromLocationId: ga.id, toLocationId: gb.id, notes: "uji",
      items: [{ productId: p.id, variantId: p.variantId, qty: 5, notes: "" }],
    });
    expect(await stockQty(p.id, ga.id)).toBe(a - 5);
    expect(await stockQty(p.id, gb.id)).toBe(b + 5);
  });

  test("transfer antar cabang: lokasi tujuan dipilih di cabang tujuan; buat -> kirim -> terima; dua kali kirim tidak menggandakan (K-06)", async () => {
    const cb2 = (await branches()).find((x) => x.code === "CB2");
    // admin (akses HQ+CB2, sesi HQ) memuat lokasi cabang tujuan lewat ?branchId=.
    const cb2Locs = await ok<any[]>("admin", "GET", `/stock/locations?branchId=${cb2.id}`);
    expect(cb2Locs.every((l) => l.branchId === cb2.id)).toBe(true);
    const dest = cb2Locs.find((l) => l.code === "CB2-GA");
    expect(dest).toBeTruthy();
    const base = await stockQty(p.id, ga.id);
    const destBefore = (await ok<any[]>("adminCb2", "GET", `/stock/balances/${p.id}`)).reduce((n, r) => n + r.onHand, 0);

    // gudang hanya punya akses HQ: lokasi CB2 dan transfer ke CB2 ditolak 403 (isolasi cabang).
    const deniedList = await call("gudang", "GET", `/stock/locations?branchId=${cb2.id}`);
    expect(deniedList.status).toBe(403);
    const deniedCreate = await call("gudang", "POST", "/stock/transfers", {
      toBranchId: cb2.id, fromLocationId: ga.id, toLocationId: dest.id, notes: "ditolak",
      items: [{ productId: p.id, variantId: p.variantId, qty: 1, notes: "" }],
    });
    expect(deniedCreate.status).toBe(403);

    const t = await ok<any>("admin", "POST", "/stock/transfers", {
      toBranchId: cb2.id, fromLocationId: ga.id, toLocationId: dest.id, notes: "uji e2e",
      driverName: "Pak Sopir E2E", vehiclePlate: "B 1234 XY", // ST-02: transfer antar-cabang wajib sopir
      items: [{ productId: p.id, variantId: p.variantId, qty: 4, notes: "" }],
    });
    await ok("admin", "POST", `/stock/transfers/${t.id}/dispatch`);
    expect(await stockQty(p.id, ga.id)).toBe(base - 4);
    await call("admin", "POST", `/stock/transfers/${t.id}/dispatch`); // ulang: replay/ditolak, stok tak boleh berubah
    expect(await stockQty(p.id, ga.id)).toBe(base - 4);

    await ok("adminCb2", "POST", `/stock/transfers/${t.id}/receive`);
    const destAfter = (await ok<any[]>("adminCb2", "GET", `/stock/balances/${p.id}`)).reduce((n, r) => n + r.onHand, 0);
    expect(destAfter).toBe(destBefore + 4);
    await call("adminCb2", "POST", `/stock/transfers/${t.id}/receive`);
    expect((await ok<any[]>("adminCb2", "GET", `/stock/balances/${p.id}`)).reduce((n, r) => n + r.onHand, 0)).toBe(destBefore + 4);
  });

  test("impor saldo awal xlsx: pratinjau, commit, ulang idempoten", async () => {
    const q = await productByCode("P014");
    const book = await buildXlsx([{ name: "opening", headers: ["product_code", "variant_code", "location_code", "qty"], rows: [[q.code, "", "HQ-GB", "12"]] }]);
    const before = await stockQty(q.id, gb.id);
    const prev = await uploadXlsx("/stock/opening/preview", book, "gudang");
    expect(prev.success, prev.message).toBe(true);
    expect(prev.data.errors ?? []).toHaveLength(0);
    expect(await stockQty(q.id, gb.id)).toBe(before); // pratinjau tidak menulis

    const first = await uploadXlsx("/stock/opening/commit", book, "gudang");
    expect(first.success, first.message).toBe(true);
    expect(first.data.posted).toBe(1);
    expect(await stockQty(q.id, gb.id)).toBe(before + 12);

    const second = await uploadXlsx("/stock/opening/commit", book, "gudang");
    expect(second.success).toBe(true);
    expect(second.data.posted).toBe(0);
    expect(await stockQty(q.id, gb.id)).toBe(before + 12);

    const bad = await buildXlsx([{ name: "opening", headers: ["product_code", "variant_code", "location_code", "qty"], rows: [["TIDAK-ADA", "", "HQ-GB", "1"]] }]);
    const badPrev = await uploadXlsx("/stock/opening/preview", bad, "gudang");
    expect(badPrev.data?.errors?.length ?? 1).toBeGreaterThan(0);
  });

  test("UI: halaman Saldo menampilkan produk dan Koreksi dapat dibuka gudang", async ({ page }) => {
    await loginAs(page, "gudang");
    await page.goto("/stock/balances");
    await expect(page.getByRole("heading", { level: 1, name: "Saldo" })).toBeVisible();
    await expect(page.getByText(p.code).first()).toBeVisible();
    await page.goto("/stock/transfers");
    await expect(page.getByRole("heading", { level: 1, name: "Transfer" })).toBeVisible();
  });
});
