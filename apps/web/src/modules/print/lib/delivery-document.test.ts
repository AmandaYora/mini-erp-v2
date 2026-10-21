import { describe, expect, it } from "vitest";
import { deliveryItemLine, deliveryStatusLabel } from "@/modules/print/lib/delivery-document";

describe("STATUS surat jalan (status bayar order, istilah sistem lama)", () => {
  it("lunas / sebagian / COD / tempo / belum lunas", () => {
    expect(deliveryStatusLabel("confirmed", "paid", "cod")).toBe("Lunas");
    expect(deliveryStatusLabel("confirmed", "partial", "net")).toBe("Bayar Sebagian");
    expect(deliveryStatusLabel("draft", "unpaid", "cod")).toBe("Bayar Saat Serah (COD)");
    expect(deliveryStatusLabel("confirmed", "unpaid", "net")).toBe("Tempo");
    expect(deliveryStatusLabel("confirmed", undefined, undefined)).toBe("Belum Lunas");
  });

  it("surat jalan batal tidak pernah tercetak seolah sah", () => {
    expect(deliveryStatusLabel("cancelled", "paid", "cod")).toBe("DIBATALKAN");
  });
});

describe("baris SO untuk baris surat jalan", () => {
  const lines = [
    { productId: 1, variantId: 10, uom: "dus", productName: "Paku (dus)" },
    { productId: 1, variantId: 10, uom: "pcs", productName: "Paku (pcs)" },
  ];

  it("cocok produk + varian + satuan, lalu produk + varian", () => {
    expect(deliveryItemLine({ productId: 1, variantId: 10, uom: "pcs" }, lines)?.productName).toBe("Paku (pcs)");
    expect(deliveryItemLine({ productId: 1, variantId: 10, uom: "kg" }, lines)?.productName).toBe("Paku (dus)");
    expect(deliveryItemLine({ productId: 2, variantId: 20, uom: "pcs" }, lines)).toBeNull();
  });
});
