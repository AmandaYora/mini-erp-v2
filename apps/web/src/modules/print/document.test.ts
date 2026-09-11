import { describe, expect, it } from "vitest";
import {
  formatPhoneGroup,
  formatPhoneLine,
  groupPhonesByLabel,
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

  it("menolak nol/negatif", () => {
    expect(terbilangRupiah(0)).toBe("-");
    expect(terbilangRupiah(-5)).toBe("-");
  });
});
