import { describe, expect, it } from "vitest";
import { deliveryConfirmSchema, toConfirmPayload } from "@/modules/delivery/schemas/delivery-confirm.schema";

describe("deliveryConfirmSchema", () => {
  it("konfirmasi polos valid dan menghasilkan body kosong", () => {
    const r = deliveryConfirmSchema.safeParse({});
    expect(r.success).toBe(true);
    if (r.success) expect(toConfirmPayload(r.data)).toBeUndefined();
  });

  it("status tanda terima butuh nama; missing butuh alasan", () => {
    expect(deliveryConfirmSchema.safeParse({ recipientSignatureStatus: "signed" }).success).toBe(false);
    expect(
      deliveryConfirmSchema.safeParse({ recipientName: "Budi", recipientSignatureStatus: "missing" }).success,
    ).toBe(false);
    expect(
      deliveryConfirmSchema.safeParse({
        recipientName: "Budi",
        recipientSignatureStatus: "missing",
        recipientSignatureMissingReason: "Hujan",
      }).success,
    ).toBe(true);
  });

  it("switch_to_net wajib dueDate YYYY-MM-DD", () => {
    expect(deliveryConfirmSchema.safeParse({ settlement: { mode: "switch_to_net" } }).success).toBe(false);
    expect(
      deliveryConfirmSchema.safeParse({ settlement: { mode: "switch_to_net", dueDate: "10/10/2026" } }).success,
    ).toBe(false);
    const ok = deliveryConfirmSchema.safeParse({ settlement: { mode: "switch_to_net", dueDate: "2026-10-30" } });
    expect(ok.success).toBe(true);
    if (ok.success) {
      expect(toConfirmPayload(ok.data)).toEqual({
        settlement: { mode: "switch_to_net", dueDate: "2026-10-30" },
      });
    }
  });

  it("pay_now transfer butuh referensi; cash tidak", () => {
    expect(
      deliveryConfirmSchema.safeParse({ settlement: { mode: "pay_now", method: "transfer" } }).success,
    ).toBe(false);
    const ok = deliveryConfirmSchema.safeParse({
      settlement: { mode: "pay_now", method: "cash", amountTendered: "150000", paymentDate: "2026-10-09" },
    });
    expect(ok.success).toBe(true);
    if (ok.success) {
      expect(toConfirmPayload(ok.data)).toEqual({
        settlement: { mode: "pay_now", method: "cash", paymentDate: "2026-10-09", amountTendered: 150000 },
      });
    }
  });

  it("amountTendered non-integer ditolak", () => {
    expect(
      deliveryConfirmSchema.safeParse({ settlement: { mode: "pay_now", amountTendered: "1.5" } }).success,
    ).toBe(false);
  });
});
