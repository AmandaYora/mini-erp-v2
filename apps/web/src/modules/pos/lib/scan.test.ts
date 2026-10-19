import { describe, expect, it } from "vitest";
import { scanAction, scanErrorMessage, variantChoiceLabel } from "@/modules/pos/lib/scan";
import type { ScanResult } from "@/modules/pos/types";

const base: ScanResult = {
  status: "ok",
  via: "barcode",
  product: { id: 10, code: "P-1", name: "Kopi" },
  variant: { id: 101, code: "P-1-A", name: "Hitam" },
  balances: [],
};

describe("scanAction", () => {
  it("ok -> tambah varian pindaian", () => {
    expect(scanAction(base, "899")).toEqual({ kind: "add", productId: 10, variantId: 101 });
  });

  it("variant_required -> pemilih varian dengan daftar dari server", () => {
    const a = scanAction(
      {
        ...base,
        status: "variant_required",
        via: "product_code",
        variant: null,
        variants: [
          { id: 1, code: "A", name: "Merah", barcode: "" },
          { id: 2, code: "B", name: "Biru", barcode: "B-1" },
        ],
      },
      "P-1",
    );
    expect(a.kind).toBe("choose");
    if (a.kind === "choose") {
      expect(a.productId).toBe(10);
      expect(a.variants).toHaveLength(2);
    }
  });

  it("variant_required tanpa varian aktif -> pesan, bukan pemilih kosong", () => {
    const a = scanAction({ ...base, status: "variant_required", variant: null, variants: [] }, "P-1");
    expect(a).toMatchObject({ kind: "message", tone: "warning" });
  });

  it("inactive -> pesan jelas tanpa menambah ke keranjang", () => {
    const a = scanAction({ ...base, status: "inactive", variant: null }, "P-1");
    expect(a.kind).toBe("message");
    if (a.kind === "message") expect(a.text).toMatch(/tidak aktif/);
  });

  it("ok tanpa varian (kontrak rusak) tidak menambah diam-diam", () => {
    expect(scanAction({ ...base, variant: null }, "X").kind).toBe("message");
  });
});

describe("scanErrorMessage", () => {
  it("404 menyebut tak dikenal atau ambigu", () => {
    const m = scanErrorMessage(404, "Produk tidak ditemukan", "ABC");
    expect(m).toMatch(/ABC/);
    expect(m).toMatch(/ambigu/);
  });
  it("galat lain memakai pesan server apa adanya", () => {
    expect(scanErrorMessage(500, "Terjadi kesalahan", "ABC")).toBe("Terjadi kesalahan");
  });
});

describe("variantChoiceLabel", () => {
  it("kode, nama, barcode", () => {
    expect(variantChoiceLabel({ id: 1, code: "A", name: "Merah", barcode: "" })).toBe("A — Merah");
    expect(variantChoiceLabel({ id: 1, code: "A", name: "A", barcode: "899" })).toBe("A (899)");
  });
});
