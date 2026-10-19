import { describe, expect, it } from "vitest";
import {
  emptySettlementDraft,
  resolveSettlement,
  settlementFieldErrors,
} from "@/modules/goodsreceipt/lib/settlement-draft";
import { receiveSchema } from "@/modules/goodsreceipt/schemas/goodsreceipt.schema";

describe("resolveSettlement", () => {
  const base = emptySettlementDraft("2026-10-09");

  it("mode kosong = tanpa settlement", () => {
    expect(resolveSettlement(base)).toEqual({ ok: true, payload: undefined });
  });

  it("pay_now tunai valid", () => {
    expect(resolveSettlement({ ...base, mode: "pay_now" })).toEqual({
      ok: true,
      payload: { mode: "pay_now", method: "cash", paymentDate: "2026-10-09" },
    });
  });

  it("pay_now transfer wajib referensi", () => {
    const r = resolveSettlement({ ...base, mode: "pay_now", method: "transfer" });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.errors.reference).toBeTruthy();
  });

  it("switch_to_net wajib dueDate", () => {
    const bad = resolveSettlement({ ...base, mode: "switch_to_net" });
    expect(bad.ok).toBe(false);
    const ok = resolveSettlement({ ...base, mode: "switch_to_net", dueDate: "2026-11-01" });
    expect(ok).toEqual({ ok: true, payload: { mode: "switch_to_net", dueDate: "2026-11-01" } });
  });
});

describe("settlementFieldErrors", () => {
  it("memetakan field server ke kunci draft", () => {
    expect(
      settlementFieldErrors([
        { field: "settlement", message: "a" },
        { field: "settlement.dueDate", message: "b" },
        { field: "items", message: "c" },
      ]),
    ).toEqual({ mode: "a", dueDate: "b" });
  });
});

describe("receiveSchema", () => {
  const item = { productId: 1, variantId: 1, uom: "pcs", qty: "2" };

  it("lokasi opsional (default 0) dan nomor SJ supplier maks 60", () => {
    const r = receiveSchema.safeParse({ purchaseOrderId: 5, items: [item] });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.items[0].locationId).toBe(0);
    expect(
      receiveSchema.safeParse({ purchaseOrderId: 5, supplierDeliveryNumber: "x".repeat(61), items: [item] }).success,
    ).toBe(false);
  });

  it("clientRef maks 64 dan minimal 1 baris", () => {
    expect(receiveSchema.safeParse({ purchaseOrderId: 5, clientRef: "x".repeat(65), items: [item] }).success).toBe(false);
    expect(receiveSchema.safeParse({ purchaseOrderId: 5, items: [] }).success).toBe(false);
  });
});
