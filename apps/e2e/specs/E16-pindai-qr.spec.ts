import { test, expect, chromium, type Page } from "@playwright/test";
import { writeFileSync } from "node:fs";
import path from "node:path";
import zxing from "@zxing/library";
import { ok, productByCode, session } from "../helpers/api";
import { BASE_URL, RUN_DIR } from "../helpers/env";

// Label produk dicetak sebagai QR (`GET /products/{id}/qr`, halaman /print/labels). Spec ini membuktikan
// rantai label → pindai → produk tertutup di sistem sungguhan:
//   1. isi QR hasil server bisa didekode dan `POST /stock/scan-product` mengenalinya (barcode varian,
//      kode varian, atau kode produk — sesuai pilihan isi di product/application/qr.go);
//   2. kamera POS (pemindai @zxing/browser yang sama dengan tablet kasir) membaca QR label itu dari
//      kamera dan memasukkan barangnya ke keranjang. Kamera diganti umpan video palsu Chromium.

// @zxing/library dipaketkan CommonJS: ambil ekspornya dari default import (spec berjalan sebagai ESM).
const { BinaryBitmap, HybridBinarizer, QRCodeReader, RGBLuminanceSource } = zxing;

interface Pixels {
  w: number;
  h: number;
  rgba: number[];
}

/** Ambil PNG label QR dari server (perlu izin products.view). */
async function qrPng(productId: number, variantId?: number): Promise<Buffer> {
  const s = await session("admin");
  const q = variantId ? `?variantId=${variantId}` : "";
  const res = await fetch(`${BASE_URL}/api/v1/products/${productId}/qr${q}`, {
    headers: { Authorization: `Bearer ${s.access}` },
  });
  expect(res.status, `GET /products/${productId}/qr${q}`).toBe(200);
  expect(res.headers.get("content-type")).toBe("image/png");
  return Buffer.from(await res.arrayBuffer());
}

/** Dekode PNG ke piksel RGBA lewat canvas browser (tanpa dependensi PNG di Node). */
async function pngPixels(page: Page, png: Buffer): Promise<Pixels> {
  return page.evaluate(async (b64) => {
    const img = new Image();
    img.src = `data:image/png;base64,${b64}`;
    await img.decode();
    const c = document.createElement("canvas");
    c.width = img.naturalWidth;
    c.height = img.naturalHeight;
    const ctx = c.getContext("2d")!;
    ctx.drawImage(img, 0, 0);
    return { w: c.width, h: c.height, rgba: Array.from(ctx.getImageData(0, 0, c.width, c.height).data) };
  }, png.toString("base64"));
}

const luma = (r: number, g: number, b: number) => Math.round(0.299 * r + 0.587 * g + 0.114 * b);

/** Dekode QR dengan pustaka yang sama dengan pemindai aplikasi (@zxing). */
function decodeQr({ w, h, rgba }: Pixels): string {
  const lum = new Uint8ClampedArray(w * h);
  for (let i = 0; i < w * h; i++) lum[i] = luma(rgba[i * 4], rgba[i * 4 + 1], rgba[i * 4 + 2]);
  return new QRCodeReader().decode(new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(lum, w, h)))).getText();
}

/**
 * Umpan kamera palsu Chromium (Y4M, 4:2:0): label QR di tengah bidang putih 640×480, seperti label yang
 * diarahkan ke kamera tablet. Y4M ditulis langsung dari piksel — tanpa alat konversi video.
 */
function writeY4m(file: string, qr: Pixels, scale = 1): void {
  const W = 640;
  const H = 480;
  const Y = new Uint8Array(W * H).fill(235);
  const U = new Uint8Array((W / 2) * (H / 2)).fill(128);
  const V = new Uint8Array((W / 2) * (H / 2)).fill(128);
  const qw = Math.min(qr.w * scale, W - 40);
  const qh = Math.min(qr.h * scale, H - 40);
  const ox = Math.floor((W - qw) / 2);
  const oy = Math.floor((H - qh) / 2);
  for (let y = 0; y < qh; y++) {
    for (let x = 0; x < qw; x++) {
      const sx = Math.floor((x * qr.w) / qw);
      const sy = Math.floor((y * qr.h) / qh);
      const i = (sy * qr.w + sx) * 4;
      const l = luma(qr.rgba[i], qr.rgba[i + 1], qr.rgba[i + 2]);
      Y[(oy + y) * W + ox + x] = Math.round(16 + (l * 219) / 255);
    }
  }
  const header = Buffer.from(`YUV4MPEG2 W${W} H${H} F10:1 Ip A1:1 C420jpeg\n`);
  const frame = Buffer.concat([Buffer.from("FRAME\n"), Buffer.from(Y), Buffer.from(U), Buffer.from(V)]);
  // Beberapa bingkai identik; Chromium memutar ulang berkas umpan.
  writeFileSync(file, Buffer.concat([header, frame, frame, frame, frame, frame]));
}

test.describe.serial("E16 pindai QR label produk", () => {
  test("QR label (barcode varian) dikenali scan-product sebagai produk + varian yang sama", async ({ page }) => {
    const p = await productByCode("P004");
    expect(p.barcode, "seed: P004 punya barcode varian").toBeTruthy();
    for (const variantId of [undefined, p.variantId]) {
      const text = decodeQr(await pngPixels(page, await qrPng(p.id, variantId)));
      expect(text).toBe(p.barcode);
      const hit = await ok<any>("admin", "POST", "/stock/scan-product", { barcode: text });
      expect(hit.status).toBe("ok");
      expect(hit.product.id).toBe(p.id);
      expect(hit.variant.id).toBe(p.variantId);
    }
  });

  test("produk tanpa barcode: QR berisi kode produk dan tetap dikenali scan-product", async ({ page }) => {
    const code = `QR-${Date.now() % 1_000_000}`;
    const created = await ok<any>("admin", "POST", "/products", {
      code,
      name: `Produk QR ${code}`,
      baseUom: "pcs",
      purchaseUom: "pcs",
      salesUom: "pcs",
      purchaseFactor: 1,
      salesFactor: 1,
      purchasePrice: 5000,
      sellingPrice: 8000,
    });
    const detail = await ok<any>("admin", "GET", `/products/${created.id}`);
    expect(detail.variants[0].barcode ?? "").toBe("");
    const text = decodeQr(await pngPixels(page, await qrPng(created.id)));
    expect(text).toBe(code);
    const hit = await ok<any>("admin", "POST", "/stock/scan-product", { barcode: text });
    expect(hit.status).toBe("ok");
    expect(hit.product.id).toBe(created.id);
  });

  test("kamera POS membaca QR label dan memasukkan barangnya ke keranjang", async ({ page }, testInfo) => {
    test.setTimeout(120_000);
    const p = await productByCode("P005");
    const feed = path.join(RUN_DIR, "qr-feed.y4m");
    writeY4m(feed, await pngPixels(page, await qrPng(p.id)), 1);

    const browser = await chromium.launch({
      args: [
        "--use-fake-device-for-media-stream",
        "--use-fake-ui-for-media-stream",
        `--use-file-for-fake-video-capture=${feed}`,
      ],
    });
    try {
      const ctx = await browser.newContext({ baseURL: BASE_URL, viewport: { width: 1280, height: 800 }, locale: "id-ID" });
      await ctx.grantPermissions(["camera"], { origin: BASE_URL });
      const s = await session("kasir");
      await ctx.addInitScript(([a, r]) => {
        localStorage.setItem("mini-erp.access-token", a);
        localStorage.setItem("mini-erp.refresh-token", r);
      }, [s.access, s.refresh]);
      const pos = await ctx.newPage();
      await pos.goto("/pos");
      const cart = pos.getByRole("complementary", { name: "Keranjang" });
      await expect(cart.getByText(p.name)).toHaveCount(0);

      const consoleLines: string[] = [];
      pos.on("console", (m) => consoleLines.push(`${m.type()}: ${m.text()}`));
      await pos.getByRole("button", { name: "Pindai dengan kamera" }).click();
      const dialog = pos.getByRole("dialog");
      try {
        // Pemindaian beruntun: QR yang sama terus terlihat selama 30 dtk umpan diputar ulang, tetapi
        // debounce pemindai menahan bacaan ganda — tetap satu barang.
        await expect(dialog.getByText(`${p.name} (1)`)).toBeVisible({ timeout: 30_000 });
        await pos.waitForTimeout(2_000);
        await expect(dialog.getByRole("button", { name: "Selesai (1 barang)" })).toBeVisible();
        await dialog.getByRole("button", { name: "Selesai (1 barang)" }).click();
        await expect(dialog).toBeHidden();
        await expect(cart.getByText(p.name).first()).toBeVisible();
      } catch (err) {
        await testInfo.attach("pos-kamera.png", { body: await pos.screenshot(), contentType: "image/png" });
        const text = await dialog.innerText().catch(() => "(tanpa dialog)");
        const errors = consoleLines.filter((l) => l.startsWith("error"));
        throw new Error(`${String(err)}\n--- dialog ---\n${text}\n--- console error ---\n${errors.join("\n")}`);
      }
    } finally {
      await browser.close();
    }
  });
});
