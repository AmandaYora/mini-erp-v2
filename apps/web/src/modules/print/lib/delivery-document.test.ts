import { describe, expect, it } from "vitest";
import {
  deliveryItemLine,
  deliveryPartyBlock,
  deliveryStatusLabel,
  paginateDeliveryLines,
  productLinesForNote,
  type SheetChunk,
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

describe("pemecahan halaman surat jalan (temuan 3)", () => {
  const rows = (n: number, name = "Semen Tiga Roda 50kg") => Array.from({ length: n }, (_, i) => ({ id: i + 1, name }));
  const shape = (chunks: SheetChunk<{ id: number }>[]) => chunks.map((c) => [c.items.length, c.firstNo, c.lastNo, c.fillers]);
  const sj = (n: number, paper: "continuous" | "a4" = "continuous") =>
    paginateDeliveryLines(rows(n), { nameOf: (r) => r.name, paper });

  it.each([
    [0, [[0, 0, 0, 8]]],
    [1, [[1, 1, 1, 7]]],
    [8, [[8, 1, 8, 0]]],
    [9, [[8, 1, 8, 0], [1, 9, 9, 7]]],
    [16, [[8, 1, 8, 0], [8, 9, 16, 0]]],
    [17, [[8, 1, 8, 0], [8, 9, 16, 0], [1, 17, 17, 7]]],
    [20, [[8, 1, 8, 0], [8, 9, 16, 0], [4, 17, 20, 4]]],
  ])("%i baris → lembar [qty, no awal, no akhir, pengisi]", (n, want) => {
    expect(shape(sj(n))).toEqual(want);
    expect(shape(sj(n, "a4"))).toEqual(want);
  });

  it("41 baris = 6 lembar; total & indeks benar", () => {
    const c = sj(41);
    expect(c.map((x) => x.items.length)).toEqual([8, 8, 8, 8, 8, 1]);
    expect(c.every((x, i) => x.index === i && x.total === 6)).toBe(true);
    expect(c[5].firstNo).toBe(41);
  });

  it("nama panjang memakan anggaran: barang berikutnya pindah lembar, No tetap berlanjut", () => {
    const long = "x".repeat(150); // 3 baris teks di kertas kontinu (60/baris) = 2 tambahan
    const lines = [...rows(2, long).map((r) => ({ ...r, id: r.id + 100 })), ...rows(18)];
    const c = paginateDeliveryLines(lines, { nameOf: (r) => r.name, paper: "continuous" });
    // Nama panjang kedua (2 tambahan) melampaui sisa anggaran (3 − 2): pindah ke lembar 2.
    expect(c.map((x) => x.items.length)).toEqual([1, 8, 8, 3]);
    expect(c.reduce((n, x) => n + x.items.length, 0)).toBe(20);
    expect(c.map((x) => x.firstNo)).toEqual(c.map((_, i) => c.slice(0, i).reduce((n, x) => n + x.items.length, 0) + 1));
    // A4 lebih lebar (95/baris, anggaran 6): tetap persis 8.
    expect(paginateDeliveryLines(lines, { nameOf: (r) => r.name, paper: "a4" })[0].items.length).toBe(8);
  });

  it("catatan SJ multi-baris mengurangi anggaran setiap lembar; nama ≤ 1 baris tetap 8", () => {
    const notes = "n".repeat(250); // 3 baris di kontinu = 2 tambahan → anggaran 1
    expect(paginateDeliveryLines(rows(20), { nameOf: (r) => r.name, paper: "continuous", notes }).map((c) => c.items.length)).toEqual([8, 8, 4]);
    const oneLong = [...rows(1, "y".repeat(130)), ...rows(9)]; // 2 tambahan > anggaran 1
    expect(paginateDeliveryLines(oneLong, { nameOf: (r) => r.name, paper: "continuous", notes }).map((c) => c.items.length)).toEqual([1, 8, 1]);
  });
});
