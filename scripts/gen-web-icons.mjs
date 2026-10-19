#!/usr/bin/env node
// Membangkitkan ikon PNG dari apps/web/public/favicon.svg (OP-02) memakai Chromium milik
// Playwright (sudah ada untuk e2e) — tanpa mengunduh aset. Jalankan ulang bila SVG berubah:
//   node scripts/gen-web-icons.mjs
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { chromium } from "@playwright/test";

const PUBLIC = new URL("../apps/web/public/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");
const svg = readFileSync(join(PUBLIC, "favicon.svg"), "utf8");
const TARGETS = [
  ["favicon-32.png", 32],
  ["apple-touch-icon.png", 180],
  ["icon-192.png", 192],
  ["icon-512.png", 512],
];

const browser = await chromium.launch();
try {
  for (const [name, size] of TARGETS) {
    const page = await browser.newPage({ viewport: { width: size, height: size } });
    await page.setContent(
      `<style>html,body{margin:0;background:transparent}svg{display:block;width:${size}px;height:${size}px}</style>${svg}`,
    );
    const buf = await page.screenshot({ omitBackground: true, clip: { x: 0, y: 0, width: size, height: size } });
    writeFileSync(join(PUBLIC, name), buf);
    await page.close();
    console.log(`${name} (${size}px, ${buf.length} B)`);
  }
} finally {
  await browser.close();
}
