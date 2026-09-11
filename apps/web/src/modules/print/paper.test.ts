// Test §7 Tahap E: profil kertas -> string @page + fallback rusak.
import { describe, expect, it } from "vitest";
import {
  CONTINUOUS_PAPER_DEFAULTS,
  contentHeightMm,
  contentWidthMm,
  continuousPageRule,
  parseDocumentProfile,
  readHideLetterhead,
  readPaperMode,
  resolvePaperProfile,
} from "@/modules/print/paper";

describe("contentWidthMm", () => {
  it("default 241.3 x 139.7 menghasilkan lebar konten 197.39", () => {
    // CATATAN: PLAN §7 menulis "199,39 (= 241,3 − 6 − 37,91)" — itu slip
    // aritmetika (241,3 − 6 − 37,91 = 197,39). 199,39 adalah angka lama saat
    // margin kiri masih 4 (lihat komentar di shared.ts legacy). Kalibrasi
    // print head yang mengikat adalah rumusnya (lebar − kiri − kanan),
    // bukan hapalannya — kedua angka diuji di bawah.
    expect(contentWidthMm(CONTINUOUS_PAPER_DEFAULTS)).toBeCloseTo(197.39, 2);
  });

  it("profil lama bermargin kiri 4 tetap menghasilkan 199.39 warisan", () => {
    expect(
      contentWidthMm({ ...CONTINUOUS_PAPER_DEFAULTS, marginLeftMm: 4 }),
    ).toBeCloseTo(199.39, 2);
  });

  it("tinggi konten default = 139.7 - 4 - 7 = 128.7", () => {
    expect(contentHeightMm(CONTINUOUS_PAPER_DEFAULTS)).toBeCloseTo(128.7, 2);
  });
});

describe("continuousPageRule", () => {
  it("memakai ukuran profil persis (widthMm tidak disempitkan)", () => {
    const rule = continuousPageRule(CONTINUOUS_PAPER_DEFAULTS);
    expect(rule).toContain("size: 241.3mm 139.7mm");
    expect(rule).toContain("margin: 4mm 37.91mm 7mm 6mm");
  });
});

describe("parseDocumentProfile + resolvePaperProfile", () => {
  it("profil kosong/hilang jatuh ke default tanpa corrupt", () => {
    for (const raw of [undefined, "", "   ", null]) {
      const { profile, corrupt } = parseDocumentProfile(raw);
      expect(corrupt).toBe(false);
      expect(resolvePaperProfile(profile.continuousNota)).toEqual(
        CONTINUOUS_PAPER_DEFAULTS,
      );
    }
  });

  it("profil rusak jatuh ke default dengan flag corrupt, cetak tetap jalan", () => {
    for (const raw of ["{bukan json", "[1,2]", "42", '"teks"', "null"]) {
      const { profile, corrupt } = parseDocumentProfile(raw);
      expect(corrupt).toBe(true);
      expect(resolvePaperProfile(profile.continuousNota)).toEqual(
        CONTINUOUS_PAPER_DEFAULTS,
      );
      // Pembuat @page tidak boleh melempar untuk profil rusak.
      expect(() => continuousPageRule(profile.continuousNota ?? {})).not.toThrow();
    }
  });

  it("angka tak valid per field diganti default-nya masing-masing", () => {
    const { profile, corrupt } = parseDocumentProfile(
      JSON.stringify({
        continuousNota: {
          widthMm: -5,
          heightMm: "lebar",
          marginRightMm: Number.NaN,
          marginLeftMm: 10,
        },
      }),
    );
    expect(corrupt).toBe(false);
    const paper = resolvePaperProfile(profile.continuousNota);
    expect(paper.widthMm).toBe(241.3);
    expect(paper.heightMm).toBe(139.7);
    expect(paper.marginRightMm).toBe(37.91);
    expect(paper.marginLeftMm).toBe(10);
  });

  it("profil valid dipakai apa adanya", () => {
    const { profile, corrupt } = parseDocumentProfile(
      JSON.stringify({ headerName: "TOKO X", continuousNota: { widthMm: 210, heightMm: 297 } }),
    );
    expect(corrupt).toBe(false);
    expect(profile.headerName).toBe("TOKO X");
    const paper = resolvePaperProfile(profile.continuousNota);
    expect(paper.widthMm).toBe(210);
    expect(paper.heightMm).toBe(297);
    expect(continuousPageRule(paper)).toContain("size: 210mm 297mm");
  });
});

describe("readPaperMode / readHideLetterhead", () => {
  it("membaca ?paper= bila valid, fallback bila tidak", () => {
    expect(readPaperMode(new URLSearchParams("paper=continuous"), "a4")).toBe("continuous");
    expect(readPaperMode(new URLSearchParams("paper=a4"), "continuous")).toBe("a4");
    expect(readPaperMode(new URLSearchParams("paper=lebar"), "a4")).toBe("a4");
    expect(readPaperMode(new URLSearchParams(""), "continuous")).toBe("continuous");
  });

  it("membaca ?hide_letterhead= 1/0, fallback ke profil", () => {
    expect(readHideLetterhead(new URLSearchParams("hide_letterhead=1"), false)).toBe(true);
    expect(readHideLetterhead(new URLSearchParams("hide_letterhead=0"), true)).toBe(false);
    expect(readHideLetterhead(new URLSearchParams(""), true)).toBe(true);
    expect(readHideLetterhead(new URLSearchParams(""), false)).toBe(false);
  });
});
