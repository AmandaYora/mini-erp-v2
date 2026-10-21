import { describe, expect, it } from "vitest";
import {
  deliveryItemLine,
  deliveryPartyBlock,
  deliveryStatusLabel,
  productLinesForNote,
} from "@/modules/print/lib/delivery-document";

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

  it("SJ Langsung belum diorder = Belum Diorder; batal tetap DIBATALKAN", () => {
    expect(deliveryStatusLabel("confirmed", undefined, undefined, true)).toBe("Belum Diorder");
    expect(deliveryStatusLabel("cancelled", undefined, undefined, true)).toBe("DIBATALKAN");
  });
});

describe("SJ Langsung tanpa SO (ADR-0022): pihak dan nama barang dari SJ/master", () => {
  it("pihak dari SO bila ada, else snapshot pelanggan di SJ, else strip", () => {
    expect(deliveryPartyBlock({ orderPartyName: "Budi", orderShipToRecipient: "Ani", notePartyName: "PT Maju" })).toEqual({
      partyName: "Budi",
      shipToRecipient: "Ani",
    });
    expect(deliveryPartyBlock({ notePartyName: "PT Maju", noteShipToRecipient: "Pak Anton" })).toEqual({
      partyName: "PT Maju",
      shipToRecipient: "Pak Anton",
    });
    expect(deliveryPartyBlock({})).toEqual({ partyName: "-", shipToRecipient: "" });
  });

  it("baris cetak dari master produk: varian tersembunyi tidak ikut nama; produk tak terbaca dilewati", () => {
    const products = [
      {
        id: 1,
        code: "SMN",
        name: "Semen",
        variants: [
          { id: 10, code: "SMN", name: "Semen", isHidden: true },
          { id: 11, code: "SMN-50", name: "50 KG", isHidden: false },
        ],
      },
    ];
    const lines = productLinesForNote(
      [
        { productId: 1, variantId: 10, uom: "sak" },
        { productId: 1, variantId: 11, uom: "sak" },
        { productId: 2, variantId: 20, uom: "pcs" },
      ],
      products,
    );
    expect(lines).toHaveLength(2);
    expect(lines[0]).toMatchObject({ productName: "Semen", productCode: "SMN", variantName: "", variantCode: "" });
    expect(lines[1]).toMatchObject({ productName: "Semen", variantName: "50 KG", variantCode: "SMN-50" });
    expect(deliveryItemLine({ productId: 1, variantId: 11, uom: "sak" }, lines)?.variantName).toBe("50 KG");
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
