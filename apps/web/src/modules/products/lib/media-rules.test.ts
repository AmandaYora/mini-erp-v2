import { describe, expect, it } from "vitest";
import {
  MEDIA_ACCEPT_ATTR,
  MEDIA_MAX_BYTES,
  formatDimensions,
  validatePhotoFile,
} from "@/modules/products/lib/media-rules";
import { clampSearchLimit } from "@/modules/products/services/products.service";

describe("validatePhotoFile", () => {
  it("menerima jpeg, png, webp", () => {
    expect(validatePhotoFile({ name: "a.jpg", size: 1000, type: "image/jpeg" })).toBeNull();
    expect(validatePhotoFile({ name: "a.JPEG", size: 1000, type: "image/jpeg" })).toBeNull();
    expect(validatePhotoFile({ name: "a.png", size: 1000, type: "image/png" })).toBeNull();
    expect(validatePhotoFile({ name: "a.webp", size: 1000, type: "image/webp" })).toBeNull();
  });

  it("menolak pdf, gif, bmp", () => {
    expect(validatePhotoFile({ name: "a.pdf", size: 1000, type: "application/pdf" })).toMatch(/JPEG, PNG, atau WebP/);
    expect(validatePhotoFile({ name: "a.gif", size: 1000, type: "image/gif" })).not.toBeNull();
    expect(validatePhotoFile({ name: "a.bmp", size: 1000, type: "image/bmp" })).not.toBeNull();
  });

  it("menolak ekstensi benar dengan MIME salah", () => {
    expect(validatePhotoFile({ name: "a.png", size: 1000, type: "image/gif" })).not.toBeNull();
  });

  it("batas 5 MB dan berkas kosong", () => {
    expect(validatePhotoFile({ name: "a.png", size: MEDIA_MAX_BYTES, type: "image/png" })).toBeNull();
    expect(validatePhotoFile({ name: "a.png", size: MEDIA_MAX_BYTES + 1, type: "image/png" })).toMatch(/5 MB/);
    expect(validatePhotoFile({ name: "a.png", size: 0, type: "image/png" })).toMatch(/kosong/);
  });

  it("atribut accept tidak memuat pdf", () => {
    expect(MEDIA_ACCEPT_ATTR).not.toMatch(/pdf/);
    expect(MEDIA_ACCEPT_ATTR).toMatch(/image\/webp/);
  });
});

describe("formatDimensions", () => {
  it("menampilkan dimensi asli bila ada", () => {
    expect(formatDimensions(1600, 1200)).toBe("1600 × 1200 px");
  });
  it("baris lama tanpa dimensi = null", () => {
    expect(formatDimensions(0, 0)).toBeNull();
    expect(formatDimensions(undefined, undefined)).toBeNull();
  });
});

describe("clampSearchLimit (search-options 1-30)", () => {
  it("default 20 dan penjepitan", () => {
    expect(clampSearchLimit(undefined)).toBe(20);
    expect(clampSearchLimit(0)).toBe(1);
    expect(clampSearchLimit(-5)).toBe(1);
    expect(clampSearchLimit(30)).toBe(30);
    expect(clampSearchLimit(100)).toBe(30);
    expect(clampSearchLimit(7.9)).toBe(7);
    expect(clampSearchLimit(Number.NaN)).toBe(20);
  });
});
