import { describe, expect, it } from "vitest";
import {
  PAY_LATER_NEEDS_CUSTOMER_MESSAGE,
  PAY_LATER_NEEDS_PHONE_MESSAGE,
  buildPosOrderPayload,
  payLaterBlockReason,
} from "@/modules/pos/lib/checkout";
import { posCheckoutSchema } from "@/modules/pos/schemas/pos-checkout.schema";

// Server (sales validateHeader) selalu menolak termin net untuk walk-in dan untuk
// customer tanpa nomor telepon — POS tidak boleh menawarkannya.
const items = [{ productId: 1, variantId: 2, uom: "pcs", qty: 1 }];
const clientRef = "11111111-2222-4333-8444-555555555555";

describe("Bayar Nanti hanya untuk pelanggan terdaftar", () => {
  it("payLaterBlockReason: walk-in diblokir, pelanggan bertelepon boleh", () => {
    expect(payLaterBlockReason({ customerId: null })).toBe(PAY_LATER_NEEDS_CUSTOMER_MESSAGE);
    expect(payLaterBlockReason({ customerId: 7, customerPhone: "0812" })).toBeNull();
    // Detail belum termuat: jangan memblokir (server tetap otoritas).
    expect(payLaterBlockReason({ customerId: 7 })).toBeNull();
  });

  it("payLaterBlockReason: pelanggan tanpa nomor telepon diblokir", () => {
    expect(payLaterBlockReason({ customerId: 7, customerPhone: null })).toBe(PAY_LATER_NEEDS_PHONE_MESSAGE);
    expect(payLaterBlockReason({ customerId: 7, customerPhone: "  " })).toBe(PAY_LATER_NEEDS_PHONE_MESSAGE);
  });

  it("buildPosOrderPayload menolak Bayar Nanti tanpa pelanggan", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 0, mode: "pay_later", dueDate: "2026-11-30", clientRef, items }),
    ).toThrow(PAY_LATER_NEEDS_CUSTOMER_MESSAGE);
  });

  it("posCheckoutSchema menolak pay_later dengan partyId 0, menerima bayar sekarang", () => {
    const later = posCheckoutSchema.safeParse({ method: "cash", tendered: "0", mode: "pay_later", dueDate: "2026-11-30", partyId: 0 });
    expect(later.success).toBe(false);
    expect(later.error?.issues[0]?.path[0]).toBe("partyId");
    const now = posCheckoutSchema.safeParse({ method: "cash", tendered: "5000", mode: "pay_now", dueDate: "", partyId: 0 });
    expect(now.success).toBe(true);
    const member = posCheckoutSchema.safeParse({ method: "cash", tendered: "0", mode: "pay_later", dueDate: "2026-11-30", partyId: 7 });
    expect(member.success).toBe(true);
  });
});
