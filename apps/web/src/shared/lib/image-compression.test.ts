import { afterEach, describe, expect, it, vi } from "vitest";
import { compressImageForUpload, containWithin } from "@/shared/lib/image-compression";

function fakeFile(name: string, type: string, bytes: number): File {
  return new File([new Uint8Array(bytes)], name, { type });
}

/** Pasang decoder + canvas palsu; `outBytes` = ukuran blob hasil, `outType` = tipe yang dikembalikan canvas. */
function stubBrowser(opts: { w: number; h: number; outBytes?: number; outType?: string; drawn?: { w: number; h: number }[] }) {
  vi.stubGlobal(
    "createImageBitmap",
    vi.fn(async () => ({ width: opts.w, height: opts.h, close: vi.fn() })),
  );
  const real = document.createElement.bind(document);
  vi.spyOn(document, "createElement").mockImplementation(((tag: string) => {
    if (tag !== "canvas") return real(tag);
    const canvas = {
      width: 0,
      height: 0,
      getContext: () => ({
        drawImage: () => opts.drawn?.push({ w: canvas.width, h: canvas.height }),
      }),
      toBlob: (cb: (b: Blob | null) => void, type: string) =>
        cb(new Blob([new Uint8Array(opts.outBytes ?? 1000)], { type: opts.outType ?? type })),
    };
    return canvas as unknown as HTMLElement;
  }) as typeof document.createElement);
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("containWithin", () => {
  it("memperkecil sisi terpanjang dan menjaga rasio", () => {
    expect(containWithin(4000, 3000, 1600)).toEqual({ width: 1600, height: 1200 });
    expect(containWithin(3000, 4000, 1600)).toEqual({ width: 1200, height: 1600 });
  });
  it("tidak membesarkan gambar kecil", () => {
    expect(containWithin(800, 600, 1600)).toEqual({ width: 800, height: 600 });
  });
  it("dimensi tak valid → null", () => {
    expect(containWithin(0, 10, 1600)).toBeNull();
    expect(containWithin(NaN, 10, 1600)).toBeNull();
  });
});

describe("compressImageForUpload", () => {
  it("tipe selain jpeg/png/webp (pdf) dikembalikan apa adanya", async () => {
    const f = fakeFile("a.pdf", "application/pdf", 5_000_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });

  it("tanpa createImageBitmap (jsdom) jatuh ke berkas asli", async () => {
    const f = fakeFile("a.jpg", "image/jpeg", 5_000_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });

  it("foto besar mengecil: dimensi dibatasi, format & nama tetap", async () => {
    const drawn: { w: number; h: number }[] = [];
    stubBrowser({ w: 4000, h: 3000, outBytes: 200_000, drawn });
    const f = fakeFile("bukti.jpg", "image/jpeg", 5_000_000);
    const out = await compressImageForUpload(f);
    expect(out).not.toBe(f);
    expect(out.size).toBe(200_000);
    expect(out.type).toBe("image/jpeg");
    expect(out.name).toBe("bukti.jpg");
    expect(drawn).toEqual([{ w: 1600, h: 1200 }]);
  });

  it("png tetap png (tidak diubah formatnya)", async () => {
    stubBrowser({ w: 3200, h: 1600, outBytes: 100_000 });
    const out = await compressImageForUpload(fakeFile("scan.png", "image/png", 2_000_000));
    expect(out.type).toBe("image/png");
    expect(out.name).toBe("scan.png");
  });

  it("berkas kecil yang dimensinya sudah muat tidak disentuh", async () => {
    stubBrowser({ w: 800, h: 600 });
    const f = fakeFile("kecil.jpg", "image/jpeg", 100_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });

  it("hasil tidak lebih kecil → berkas asli", async () => {
    stubBrowser({ w: 4000, h: 3000, outBytes: 6_000_000 });
    const f = fakeFile("a.jpg", "image/jpeg", 5_000_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });

  it("peramban jatuh ke tipe lain (mis. PNG) → berkas asli", async () => {
    stubBrowser({ w: 4000, h: 3000, outBytes: 100_000, outType: "image/png" });
    const f = fakeFile("a.webp", "image/webp", 5_000_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });

  it("decoder melempar galat → berkas asli", async () => {
    vi.stubGlobal("createImageBitmap", vi.fn(async () => { throw new Error("rusak"); }));
    const f = fakeFile("a.jpg", "image/jpeg", 5_000_000);
    expect(await compressImageForUpload(f)).toBe(f);
  });
});
