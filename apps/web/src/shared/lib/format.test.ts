import { describe, expect, it } from "vitest";
import {
  formatDate,
  formatDateTime,
  formatIDR,
  formatNumber,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";

describe("formatIDR", () => {
  // Intl id-ID memakai non-breaking space (U+00A0) setelah "Rp" — cocokkan
  // persis seperti yang dirender browser, bukan spasi biasa.
  const NBSP = " ";
  it("formats zero and positives with dot thousands", () => {
    expect(formatIDR(0)).toBe(`Rp${NBSP}0`);
    expect(formatIDR(500)).toBe(`Rp${NBSP}500`);
    expect(formatIDR(15500)).toBe(`Rp${NBSP}15.500`);
    expect(formatIDR(1234567)).toBe(`Rp${NBSP}1.234.567`);
  });
  it("formats negatives with leading minus", () => {
    expect(formatIDR(-93000)).toBe(`-Rp${NBSP}93.000`);
  });
  it("renders dash for nullish input", () => {
    expect(formatIDR(null)).toBe("-");
    expect(formatIDR(undefined)).toBe("-");
  });
});

describe("formatNumber", () => {
  it("groups thousands id-ID", () => {
    expect(formatNumber(1000000)).toBe("1.000.000");
  });
  it("renders dash for nullish input", () => {
    expect(formatNumber(null)).toBe("-");
  });
});

describe("formatDate / formatDateTime", () => {
  it("formats YYYY-MM-DD to Indonesian short date", () => {
    expect(formatDate("2026-09-10")).toBe("10 Sep 2026");
  });
  it("renders dash for empty input", () => {
    expect(formatDate("")).toBe("-");
    expect(formatDateTime(null)).toBe("-");
  });
  it("includes hour and minute", () => {
    expect(formatDateTime("2026-09-10 14:30:00")).toContain("10 Sep 2026");
    expect(formatDateTime("2026-09-10 14:30:00")).toContain("14:30");
  });
});

describe("statusTone / statusLabel", () => {
  it("maps lifecycle states to tones", () => {
    expect(statusTone("completed")).toBe("success");
    expect(statusTone("confirmed")).toBe("info");
    expect(statusTone("draft")).toBe("warning");
    expect(statusTone("cancelled")).toBe("danger");
    expect(statusTone("whatever")).toBe("neutral");
  });
  it("labels in Indonesian", () => {
    expect(statusLabel("draft")).toBe("Draf");
    expect(statusLabel("completed")).toBe("Selesai");
    expect(statusLabel("cancelled")).toBe("Dibatalkan");
  });
});
