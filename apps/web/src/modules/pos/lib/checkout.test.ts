import { describe, expect, it } from "vitest";
import { buildPosOrderPayload } from "@/modules/pos/lib/checkout";

// Test Tahap I (§7 PLAN): POS "Bayar Nanti" membuat SO dengan
// payment_terms=net + due_date, bukan cod — piutang POS tercatat.
describe("buildPosOrderPayload — Bayar Nanti", () => {
  const items = [{ productId: 1, variantId: 2, uom: "pcs", qty: 3 }];

  it("membuat SO payment_terms=net + due_date untuk Bayar Nanti", () => {
    expect(
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", items }),
    ).toEqual({
      channel: "pos",
      partyId: 7,
      paymentTerms: "net",
      dueDate: "2026-09-30",
      items,
    });
  });

  it("tetap memakai cod tanpa due_date untuk bayar sekarang", () => {
    const payload = buildPosOrderPayload({ partyId: 0, mode: "pay_now", items });
    expect(payload.paymentTerms).toBe("cod");
    expect(payload.dueDate).toBeUndefined();
  });

  it("menolak Bayar Nanti tanpa jatuh tempo", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "", items }),
    ).toThrow("Jatuh tempo wajib diisi");
  });

  it("menolak format jatuh tempo selain YYYY-MM-DD", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "30-09-2026", items }),
    ).toThrow("YYYY-MM-DD");
  });

  it("menolak keranjang kosong", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", items: [] }),
    ).toThrow("Keranjang kosong");
  });
});
