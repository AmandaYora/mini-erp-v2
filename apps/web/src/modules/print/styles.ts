// Stylesheet cetak (Tahap E) — port dari sales-print-shared.tsx &
// delivery-note-print-page.tsx sistem lama.
//
// Aturan keras warisan tes fisik:
// - @page mode kontinu HARUS persis ukuran profil (continuousPageRule).
// - Breakpoint responsif WAJIB `@media screen and (...)` — tanpa qualifier
//   `screen`, media query ikut aktif di media print dan merusak posisi kop
//   di kertas fisik saat area cetak efektif menyempit.

import {
  contentHeightMm,
  contentWidthMm,
  continuousPageRule,
  fmtMm,
  type ContinuousPaperProfile,
} from "@/modules/print/paper";

const TOOLBAR_CSS = `
  .print-toolbar { background: #f3f4f6; border-bottom: 1px solid #d1d5db; display: flex; flex-wrap: wrap; gap: 8px; justify-content: center; align-items: center; padding: 12px; }
  .print-toolbar button { background: #fff; border: 1px solid #111827; border-radius: 6px; color: #111827; cursor: pointer; font-weight: 700; padding: 8px 14px; }
  .print-toolbar-option { align-items: center; color: #111827; display: inline-flex; font-size: 9.5pt; font-weight: 700; gap: 6px; padding: 8px 10px; }
  .print-toolbar-option input { height: 16px; margin: 0; width: 16px; }
  .print-toolbar-option textarea { font-family: inherit; font-size: 9.5pt; min-width: 220px; padding: 4px 6px; }
  .print-warning { background: #fffbeb; border-bottom: 1px solid #f59e0b; color: #92400e; font-size: 9.5pt; padding: 8px 16px; text-align: center; }
`;

const DOC_TABLE_CSS = `
  .doc-table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .doc-table th, .doc-table td { border: 1px solid #111; padding: 4px 7px; overflow-wrap: anywhere; vertical-align: top; }
  .doc-table th { background: #e5e7eb; text-transform: uppercase; text-align: center; }
  .doc-table td.t-c { text-align: center; }
  .doc-table td.t-r { text-align: right; }
`;

// --- Surat Jalan ---

export function deliveryStyles(): string {
  return `
  @page { size: A4 landscape; margin: 8mm; }
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; }
    .sj-page { box-shadow: none !important; margin: 0 !important; max-width: none !important; }
    * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
  body { background: #f3f4f6; }
  ${TOOLBAR_CSS}
  ${DOC_TABLE_CSS}
  .sj-page { background: #fff; box-shadow: 0 12px 30px rgba(15, 23, 42, .12); color: #111; font-family: Arial, Helvetica, sans-serif; font-size: 10pt; line-height: 1.3; margin: 16px auto; max-width: 1040px; padding: 16px 20px; }
  .sj-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
  .sj-title { font-size: 18pt; font-weight: 800; text-transform: uppercase; text-align: right; white-space: nowrap; }
  .sj-meta { font-size: 9.5pt; text-align: right; }
  .sj-parties { display: flex; justify-content: space-between; gap: 24px; margin: 12px 0 8px; font-size: 9.5pt; }
  .sj-kv { display: grid; grid-template-columns: auto auto 1fr; gap: 2px 6px; align-content: start; }
  .sj-kv .lbl { font-weight: 700; text-transform: uppercase; }
  .sj-table { font-size: 9.5pt; }
  .sj-table th { font-size: 8.5pt; }
  .sj-status { font-size: 9pt; font-weight: 700; margin: 8px 0 4px; }
  .sj-note { font-size: 9pt; margin-bottom: 4px; }
  .sj-foot { display: grid; grid-template-columns: 1fr 2fr; gap: 18px; margin-top: 10px; align-items: start; }
  .sj-cityline { text-align: right; font-size: 9pt; margin-bottom: 6px; }
  .sj-sign-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
  .sj-sign-box { border: 1px solid #111; padding: 8px; text-align: center; font-size: 9pt; }
  .sj-sign-role { font-weight: 700; text-transform: uppercase; }
  .sj-sign-line { border-top: 1px solid #111; margin-top: 42px; padding-top: 3px; font-weight: 700; }
  .sj-footbar { display: flex; justify-content: space-between; gap: 12px; margin-top: 12px; font-size: 8pt; color: #374151; }
  @media screen and (max-width: 760px) {
    .sj-head, .sj-parties, .sj-foot { display: grid; grid-template-columns: 1fr; }
    .sj-title, .sj-meta, .sj-cityline { text-align: left; }
  }
  `;
}

export function continuousDeliveryStyles(
  profile: ContinuousPaperProfile,
): string {
  const w = fmtMm(contentWidthMm(profile));
  const h = fmtMm(contentHeightMm(profile));
  return `
  ${continuousPageRule(profile)}
  @media print {
    .sj-page.continuous { box-shadow: none !important; margin: 0 !important; max-width: none !important; width: ${w}mm !important; }
  }
  .sj-page.continuous { display: flex; flex-direction: column; font-size: 9pt; line-height: 1.15; margin: 16px auto; max-width: none; min-height: ${h}mm; padding: 0; width: ${w}mm; --print-name-size: 11pt; --print-detail-size: 7.5pt; }
  .sj-page.continuous .sj-title { font-size: 13pt; }
  .sj-page.continuous .sj-meta { font-size: 8pt; }
  .sj-page.continuous .sj-parties { font-size: 8.5pt; margin: 6px 0 6px; }
  .sj-page.continuous .sj-table { font-size: 8.5pt; }
  .sj-page.continuous .sj-table th { font-size: 7.5pt; }
  .sj-page.continuous .sj-table th, .sj-page.continuous .sj-table td { padding: 2px 5px; }
  .sj-page.continuous .sj-status { font-size: 8pt; margin: 4px 0 3px; }
  .sj-page.continuous .sj-foot { font-size: 7.5pt; gap: 10px; margin-top: auto; padding-top: 6px; }
  .sj-page.continuous .sj-cityline { font-size: 7.5pt; margin-bottom: 3px; }
  .sj-page.continuous .sj-sign-grid { gap: 6px; }
  .sj-page.continuous .sj-sign-box { font-size: 7.5pt; padding: 4px; }
  .sj-page.continuous .sj-sign-line { margin-top: 9mm; }
  .sj-page.continuous .sj-footbar { font-size: 7pt; margin-top: 6px; }
  .sj-page.continuous .sj-table thead { display: table-header-group; }
  .sj-page.continuous .sj-table tr { break-inside: avoid; page-break-inside: avoid; }
  `;
}

// --- Faktur / Nota Penjualan ---

export function invoiceStyles(): string {
  return `
  @page { size: A4 landscape; margin: 8mm; }
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; }
    .nota-page { box-shadow: none !important; margin: 0 !important; max-width: none !important; }
    * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
  body { background: #f3f4f6; }
  ${TOOLBAR_CSS}
  ${DOC_TABLE_CSS}
  .nota-page { background: #fff; box-shadow: 0 12px 30px rgba(15, 23, 42, .12); color: #111; font-family: Arial, Helvetica, sans-serif; font-size: 10pt; line-height: 1.3; margin: 16px auto; max-width: 1040px; padding: 16px 20px; }
  .nota-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
  .nota-title { font-size: 18pt; font-weight: 800; text-transform: uppercase; text-align: right; white-space: nowrap; }
  .nota-meta { font-size: 9.5pt; text-align: right; }
  .nota-parties { display: flex; justify-content: space-between; gap: 24px; margin: 12px 0 8px; font-size: 9.5pt; }
  .nota-kv { display: grid; grid-template-columns: auto auto 1fr; gap: 2px 6px; align-content: start; }
  .nota-kv .lbl { font-weight: 700; text-transform: uppercase; }
  .nota-table { font-size: 9.5pt; }
  .nota-table th { font-size: 8.5pt; }
  .nota-sumbar { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-top: 4px; font-size: 9.5pt; }
  .nota-terbilang { font-style: italic; padding-top: 4px; }
  .nota-paysum { margin-left: auto; width: 280px; border-collapse: collapse; font-size: 9pt; }
  .nota-paysum td { border: 1px solid #111; padding: 3px 7px; }
  .nota-paysum td:last-child { text-align: right; white-space: nowrap; }
  .nota-paysum tr.bold td { font-weight: 800; background: #e5e7eb; }
  .nota-foot { display: grid; grid-template-columns: 1.2fr .9fr 1.1fr; gap: 18px; margin-top: 12px; font-size: 9pt; }
  .nota-buyer { text-align: center; }
  .nota-buyer-line { margin: 26px auto 4px; }
  .nota-sell-title { font-weight: 700; }
  .nota-sell ul { margin: 4px 0 0; padding-left: 16px; }
  .nota-footbar { display: flex; justify-content: space-between; gap: 12px; margin-top: 12px; font-size: 8pt; color: #374151; }
  @media screen and (max-width: 760px) {
    .nota-head, .nota-parties, .nota-foot { display: grid; grid-template-columns: 1fr; }
    .nota-title { text-align: left; }
    .nota-paysum { width: 100%; }
  }
  `;
}

export function continuousInvoiceStyles(
  profile: ContinuousPaperProfile,
): string {
  const w = fmtMm(contentWidthMm(profile));
  const h = fmtMm(contentHeightMm(profile));
  return `
  ${continuousPageRule(profile)}
  @media print {
    .nota-page.continuous { box-shadow: none !important; margin: 0 !important; max-width: none !important; width: ${w}mm !important; }
  }
  .nota-page.continuous { display: flex; flex-direction: column; font-size: 9pt; line-height: 1.15; margin: 16px auto; max-width: none; min-height: ${h}mm; padding: 0; width: ${w}mm; --print-name-size: 11pt; --print-detail-size: 7.5pt; }
  .nota-page.continuous .nota-title { font-size: 13pt; }
  .nota-page.continuous .nota-parties { font-size: 8.5pt; margin: 6px 0 6px; }
  .nota-page.continuous .nota-table { font-size: 8.5pt; }
  .nota-page.continuous .nota-table th { font-size: 7.5pt; }
  .nota-page.continuous .nota-table th, .nota-page.continuous .nota-table td { padding: 2px 5px; }
  .nota-page.continuous .nota-sumbar { font-size: 8pt; margin-top: 4px; }
  .nota-page.continuous .nota-paysum { font-size: 8pt; }
  .nota-page.continuous .nota-paysum td { padding: 2px 6px; }
  .nota-page.continuous .nota-foot { font-size: 7.5pt; gap: 10px; margin-top: auto; padding-top: 6px; }
  .nota-page.continuous .nota-buyer-line { margin: 8mm auto 3px; }
  .nota-page.continuous .nota-footbar { font-size: 7pt; margin-top: 6px; }
  .nota-page.continuous .nota-table thead { display: table-header-group; }
  .nota-page.continuous .nota-table tr { break-inside: avoid; page-break-inside: avoid; }
  `;
}

// --- Kwitansi Pembayaran ---

export function receiptStyles(): string {
  return `
  @page { size: A4; margin: 14mm; }
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; }
    .kwitansi-page { box-shadow: none !important; margin: 0 !important; max-width: none !important; }
    * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
  body { background: #f3f4f6; }
  ${TOOLBAR_CSS}
  .kwitansi-page { background: #fff; box-shadow: 0 12px 30px rgba(15, 23, 42, .12); color: #111; font-family: Arial, Helvetica, sans-serif; font-size: 11pt; line-height: 1.35; margin: 16px auto; max-width: 860px; padding: 24px; }
  .kwitansi-head { display: flex; justify-content: space-between; gap: 24px; border-bottom: 2px solid #111; padding-bottom: 14px; margin-bottom: 16px; }
  .kwitansi-title { font-size: 20pt; font-weight: 800; margin: 0 0 6px; text-transform: uppercase; }
  .kwitansi-company { font-size: 12pt; font-weight: 700; }
  .kwitansi-muted { color: #4b5563; }
  .kwitansi-meta { min-width: 240px; text-align: right; font-size: 10.5pt; }
  .kwitansi-meta div { margin-bottom: 3px; }
  .kwitansi-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 28px; margin-bottom: 16px; }
  .kwitansi-box { border: 1px solid #d1d5db; border-radius: 8px; padding: 12px; }
  .kwitansi-box h2 { font-size: 10pt; text-transform: uppercase; margin: 0 0 8px; color: #374151; }
  .kwitansi-kv { display: grid; grid-template-columns: 105px 1fr; gap: 4px 8px; }
  .kwitansi-kv span:nth-child(odd) { color: #4b5563; }
  .kwitansi-amount { border: 1px solid #111; padding: 10px 12px; margin: 14px 0; }
  .kwitansi-amount .terbilang { font-style: italic; margin-top: 4px; }
  .kwitansi-alloc { width: 100%; border-collapse: collapse; margin: 14px 0; font-size: 10.5pt; }
  .kwitansi-alloc th, .kwitansi-alloc td { border: 1px solid #d1d5db; padding: 7px 8px; vertical-align: top; }
  .kwitansi-alloc th { background: #f3f4f6; text-align: left; font-weight: 800; }
  .kwitansi-alloc td.num { text-align: right; white-space: nowrap; }
  .kwitansi-sign { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-top: 18px; font-size: 10pt; }
  .kwitansi-sign .box { text-align: center; }
  .kwitansi-sign .line { border-top: 1px solid #111; margin-top: 56px; padding-top: 3px; font-weight: 700; }
  .kwitansi-note { border-top: 1px dashed #9ca3af; margin-top: 18px; padding-top: 10px; color: #374151; font-size: 10pt; }
  .kwitansi-status { display: inline-block; border: 1px solid #111827; border-radius: 999px; padding: 3px 10px; font-weight: 800; font-size: 9.5pt; text-transform: uppercase; }
  @media screen and (max-width: 720px) {
    .kwitansi-head, .kwitansi-grid, .kwitansi-sign { display: grid; grid-template-columns: 1fr; }
    .kwitansi-meta { text-align: left; min-width: 0; }
    .kwitansi-page { padding: 16px; }
  }
  `;
}

export function continuousReceiptStyles(
  profile: ContinuousPaperProfile,
): string {
  const w = fmtMm(contentWidthMm(profile));
  return `
  ${continuousPageRule(profile)}
  @media print {
    .kwitansi-page.continuous { box-shadow: none !important; margin: 0 !important; max-width: none !important; padding: 0 !important; width: ${w}mm !important; }
  }
  .kwitansi-page.continuous { font-size: 9pt; line-height: 1.15; max-width: none; padding: 0; width: ${w}mm; }
  .kwitansi-page.continuous .kwitansi-head { padding-bottom: 7px; margin-bottom: 9px; }
  .kwitansi-page.continuous .kwitansi-title { font-size: 12pt; margin: 0 0 3px; }
  .kwitansi-page.continuous .kwitansi-company { font-size: 9pt; }
  .kwitansi-page.continuous .kwitansi-meta { font-size: 8pt; min-width: 0; }
  .kwitansi-page.continuous .kwitansi-grid { gap: 7px 12px; margin-bottom: 9px; }
  .kwitansi-page.continuous .kwitansi-box { padding: 7px; }
  .kwitansi-page.continuous .kwitansi-box h2 { font-size: 7.5pt; margin: 0 0 4px; }
  .kwitansi-page.continuous .kwitansi-kv { font-size: 8pt; grid-template-columns: 75px 1fr; }
  .kwitansi-page.continuous .kwitansi-amount { font-size: 8.5pt; margin: 9px 0; padding: 7px; }
  .kwitansi-page.continuous .kwitansi-alloc { font-size: 8pt; margin: 9px 0; }
  .kwitansi-page.continuous .kwitansi-sign { font-size: 8pt; margin-top: 10px; }
  .kwitansi-page.continuous .kwitansi-note { font-size: 7.5pt; margin-top: 9px; padding-top: 7px; }
  `;
}

// --- Struk termal POS (58/80mm) ---

export type ThermalWidth = 58 | 80;

export function readThermalWidth(searchParams: URLSearchParams): ThermalWidth {
  return searchParams.get("w") === "58" ? 58 : 80;
}

export function thermalStyles(widthMm: ThermalWidth): string {
  const content = widthMm - 8;
  return `
  @page { size: ${widthMm}mm 297mm; margin: 0; }
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; margin: 0 !important; padding: 0 !important; width: ${widthMm}mm; }
    .thermal-receipt { box-shadow: none !important; margin: 0 !important; width: ${content}mm !important; }
  }
  body { background: #f3f4f6; }
  .thermal-toolbar { background: #f3f4f6; border-bottom: 1px solid #d1d5db; display: flex; flex-wrap: wrap; gap: 8px; justify-content: center; align-items: center; padding: 12px; }
  .thermal-toolbar button { background: #fff; border: 1px solid #111827; border-radius: 6px; color: #111827; cursor: pointer; font-weight: 700; padding: 8px 14px; }
  .thermal-toolbar-option { align-items: center; color: #111827; display: inline-flex; font-size: 10.5px; font-weight: 700; gap: 6px; padding: 8px 10px; }
  .thermal-toolbar-option input { height: 15px; margin: 0; width: 15px; }
  .thermal-logo { display: block; height: auto; margin: 0 auto 4px; width: 30mm; max-width: 60%; }
  .thermal-receipt { background: #fff; box-shadow: 0 16px 40px rgba(15, 23, 42, 0.14); color: #000; font-family: "Courier New", Courier, monospace; font-size: 10.5px; line-height: 1.25; margin: 16px auto; padding: 4mm; width: ${content}mm; }
  .thermal-center { text-align: center; }
  .thermal-store { font-size: 13px; font-weight: 800; text-transform: uppercase; }
  .thermal-tagline { font-weight: 700; }
  .thermal-phone { font-size: 10px; }
  .thermal-separator { border-top: 1px dashed #000; margin: 6px 0; }
  .thermal-row { display: flex; gap: 8px; justify-content: space-between; }
  .thermal-row span:last-child { flex: 0 0 auto; text-align: right; white-space: nowrap; }
  .thermal-item { margin: 6px 0; }
  .thermal-item-name { font-weight: 700; overflow-wrap: anywhere; }
  .thermal-item-meta { display: flex; gap: 8px; justify-content: space-between; padding-left: 8px; }
  .thermal-total { font-size: 12px; font-weight: 800; }
  .thermal-status { border: 1px solid #000; display: inline-block; font-weight: 800; margin-top: 4px; padding: 2px 8px; }
  .thermal-rekening { font-size: 10px; margin-top: 6px; overflow-wrap: anywhere; }
  .thermal-note { font-size: 10px; margin-top: 6px; }
  .thermal-footer { font-weight: 700; margin-top: 8px; text-align: center; }
  `;
}

// --- Halaman uji kalibrasi (E7) ---

export function calibrationTestStyles(
  profile: ContinuousPaperProfile,
): string {
  const w = fmtMm(contentWidthMm(profile));
  const h = fmtMm(contentHeightMm(profile));
  return `
  ${continuousPageRule(profile)}
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; }
    .calib-sheet { box-shadow: none !important; margin: 0 !important; max-width: none !important; width: ${w}mm !important; }
  }
  body { background: #f3f4f6; }
  ${TOOLBAR_CSS}
  .calib-sheet { background: #fff; box-shadow: 0 12px 30px rgba(15, 23, 42, .12); color: #111; font-family: Arial, Helvetica, sans-serif; font-size: 9pt; line-height: 1.2; margin: 16px auto; max-width: none; min-height: ${h}mm; padding: 0; width: ${w}mm; }
  .calib-sheet table { width: 100%; border-collapse: collapse; }
  .calib-sheet th, .calib-sheet td { border: 1px solid #111; padding: 2px 5px; text-align: left; }
  `;
}
