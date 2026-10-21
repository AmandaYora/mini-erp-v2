import { describe, expect, it } from "vitest";
import {
  formatDocDate,
  formatLongDate,
  formatPhoneGroup,
  formatPhoneLine,
  groupPhonesByLabel,
  printItemName,
  printMethodLabel,
  resolveIdentity,
  resolveShipToContact,
  terbilangRupiah,
} from "@/modules/print/document";

describe("groupPhonesByLabel / formatPhoneGroup", () => {
  it("menggabungkan nomor berlabel sama dalam satu baris", () => {
    const groups = groupPhonesByLabel([
      { label: "HANDOKO", number: "0811" },
      { label: "TOKO", number: "0822" },
      { label: "HANDOKO", number: "0833" },
      { label: "", number: "" },
    ]);
    expect(groups).toEqual([
      { label: "HANDOKO", numbers: ["0811", "0833"] },
      { label: "TOKO", numbers: ["0822"] },
    ]);
    expect(formatPhoneGroup(groups[0])).toBe("HANDOKO (0811) / (0833)");
  });

  it("label kosong menghasilkan nomor polos", () => {
    expect(formatPhoneLine({ label: "", number: "0811" })).toBe("0811");
    expect(formatPhoneLine({ label: "TOKO", number: "0811" })).toBe("TOKO (0811)");
  });
});

describe("resolveShipToContact", () => {
  it("ship-to menang; telepon ship-to kosong menjadi - (bukan telepon pelanggan)", () => {
    expect(
      resolveShipToContact({
        shipToAddress: "Jl. Kirim 1",
        shipToPhone: "",
        fallbackAddress: "Jl. Pelanggan 2",
        fallbackPhone: "0811",
      }),
    ).toEqual({ address: "Jl. Kirim 1", phone: "-" });
  });

  it("tanpa ship-to memakai alamat & telepon pelanggan", () => {
    expect(
      resolveShipToContact({
        shipToAddress: "  ",
        fallbackAddress: "Jl. Pelanggan 2",
        fallbackPhone: "0811",
      }),
    ).toEqual({ address: "Jl. Pelanggan 2", phone: "0811" });
  });
});

describe("resolveIdentity", () => {
  it("fallback ke nama perusahaan sesi lalu ke bawaan", () => {
    expect(resolveIdentity({}, { companyName: "PT X" }).name).toBe("PT X");
    expect(resolveIdentity({}).name).toBe("Mini ERP");
    expect(resolveIdentity({ headerName: "TOKO Y" }, { companyName: "PT X" }).name).toBe("TOKO Y");
  });
});

describe("terbilangRupiah", () => {
  it("mengeja nominal umum", () => {
    expect(terbilangRupiah(1)).toBe("satu rupiah");
    expect(terbilangRupiah(12)).toBe("dua belas rupiah");
    expect(terbilangRupiah(100)).toBe("seratus rupiah");
    expect(terbilangRupiah(1000)).toBe("seribu rupiah");
    expect(terbilangRupiah(125000)).toBe("seratus dua puluh lima ribu rupiah");
    expect(terbilangRupiah(1500000)).toBe("satu juta lima ratus ribu rupiah");
  });

  it("sama dengan sistem lama: nol, negatif, miliar/triliun, pembulatan", () => {
    expect(terbilangRupiah(0)).toBe("nol rupiah");
    expect(terbilangRupiah(-5)).toBe("minus lima rupiah");
    expect(terbilangRupiah(2_500_000_000)).toBe("dua miliar lima ratus juta rupiah");
    expect(terbilangRupiah(1_000_000_000_000)).toBe("satu triliun rupiah");
    expect(terbilangRupiah(1999.6)).toBe("dua ribu rupiah");
    expect(terbilangRupiah(Number.NaN)).toBe("nol rupiah");
  });
});

describe("nama barang tercetak", () => {
  it("varian nyata ditambahkan; varian bawaan tersembunyi tidak", () => {
    expect(printItemName({ productName: "Cat Picolux", productCode: "CAT-1", variantName: "BROWN 923", variantCode: "CAT-1-923" })).toBe(
      "Cat Picolux - BROWN 923",
    );
    expect(printItemName({ productName: "Paku", productCode: "PK", variantName: "Paku", variantCode: "PK" })).toBe("Paku");
    expect(printItemName({ productName: "Paku", productCode: "PK", variantName: "Lama", variantCode: "PK" })).toBe("Paku");
    expect(printItemName({ productName: "Paku", variantName: "" })).toBe("Paku");
  });
});

describe("format tanggal dokumen (sistem lama)", () => {
  it("tanggal panjang & ringkas", () => {
    expect(formatLongDate("2026-06-24")).toBe("24 Juni 2026");
    expect(formatDocDate("2026-06-04")).toBe("04 Jun 2026");
    expect(formatLongDate("")).toBe("-");
    expect(formatLongDate("bukan tanggal")).toBe("-");
  });

  it("label metode bayar istilah lama", () => {
    expect(printMethodLabel("cash")).toBe("Tunai");
    expect(printMethodLabel("transfer")).toBe("Transfer Bank");
    expect(printMethodLabel("check")).toBe("Cek / Giro");
  });
});

describe("kop: cadangan alamat & kota cabang", () => {
  it("profil dokumen menang; kosong -> cabang sesi", () => {
    const fromBranch = resolveIdentity({}, { branchAddress: "Jl. Blora 1", branchCity: "Blora", companyCity: "Semarang" });
    expect(fromBranch.addressLine).toBe("Jl. Blora 1");
    expect(fromBranch.cityLine).toBe("Blora");
    const fromProfile = resolveIdentity({ addressLine: "Jl. Profil", cityLine: "Cepu" }, { branchAddress: "Jl. Blora 1", branchCity: "Blora" });
    expect(fromProfile.addressLine).toBe("Jl. Profil");
    expect(fromProfile.cityLine).toBe("Cepu");
  });
});
