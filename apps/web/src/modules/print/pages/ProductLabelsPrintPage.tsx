// Label QR produk untuk cetak-tempel (J2): ?ids=1,2,3 (maks 20 produk).
// Tiap label = QR + nama + kode, dicetak dari browser di kertas biasa.
// Bukan PDF server-side (keputusan D5): layout hidup di sini.

import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { productsService } from "@/modules/products/services/products.service";
import { expandLabelTargets } from "@/modules/print/lib/labels";
import { useAsyncData } from "@/shared/hooks/use-async-data";

interface LabelRow {
  key: string;
  name: string;
  code: string;
  imgUrl: string;
}

const MAX_PRODUCTS = 20;

const LABEL_CSS = `
  @page { size: A4 portrait; margin: 10mm; }
  @media print {
    .no-print { display: none !important; }
    html, body { background: #fff !important; }
    .labels-page { margin: 0 !important; }
    * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
  body { background: #f3f4f6; }
  .print-toolbar { background: #f3f4f6; border-bottom: 1px solid #d1d5db; display: flex; flex-wrap: wrap; gap: 8px; justify-content: center; align-items: center; padding: 12px; }
  .print-toolbar button { background: #fff; border: 1px solid #111827; border-radius: 6px; color: #111827; cursor: pointer; font-weight: 700; padding: 8px 14px; }
  .print-toolbar .hint { color: #111827; font-size: 9.5pt; }
  .labels-page { margin: 16px auto; max-width: 800px; }
  .label-grid { display: flex; flex-wrap: wrap; gap: 6mm; justify-content: flex-start; }
  .qr-label { background: #fff; border: 1px dashed #111; break-inside: avoid; page-break-inside: avoid; width: 58mm; padding: 3mm; text-align: center; font-family: Arial, Helvetica, sans-serif; }
  .qr-label img { width: 100%; height: auto; display: block; }
  .qr-label .qr-name { font-size: 9pt; font-weight: 700; margin-top: 2mm; overflow-wrap: anywhere; }
  .qr-label .qr-code { font-size: 8pt; font-family: monospace; color: #444; margin-top: 1mm; }
`;

export default function ProductLabelsPrintPage() {
  const [searchParams] = useSearchParams();
  const idsParam = searchParams.get("ids") ?? "";

  const { data, loading, error } = useAsyncData(
    async (): Promise<LabelRow[]> => {
      const ids = idsParam
        .split(",")
        .map((s) => Number(s.trim()))
        .filter((n) => Number.isInteger(n) && n > 0)
        .slice(0, MAX_PRODUCTS);
      if (ids.length === 0) {
        throw new Error("Parameter ids kosong — buka halaman ini dari tombol Cetak Label.");
      }
      const out: LabelRow[] = [];
      for (const id of ids) {
        const p = await productsService.detail(id);
        for (const t of expandLabelTargets(p)) {
          const imgUrl = await productsService.qrPngUrl(t.productId, t.variantId ?? undefined);
          out.push({
            key: `${t.productId}/${t.variantId ?? 0}`,
            name: t.name,
            code: t.code,
            imgUrl,
          });
        }
      }
      return out;
    },
    [idsParam],
  );
  const rows: LabelRow[] = data ?? [];

  useEffect(() => {
    return () => {
      for (const r of rows) URL.revokeObjectURL(r.imgUrl);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows.length]);

  return (
    <>
      <style>{LABEL_CSS}</style>
      <div className="no-print print-toolbar">
        <span className="hint">
          {loading ? "Memuat label…" : `${rows.length} label — potong mengikuti garis putus-putus.`}
        </span>
        <button type="button" onClick={() => window.print()}>
          Cetak / Print
        </button>
      </div>
      {loading ? (
        <div style={{ padding: "2rem" }}>Memuat label…</div>
      ) : error !== null ? (
        <div style={{ padding: "2rem" }}>{error}</div>
      ) : (
        <main className="labels-page">
          <div className="label-grid">
            {rows.map((r) => (
              <div key={r.key} className="qr-label">
                <img src={r.imgUrl} alt={r.code} />
                <div className="qr-name">{r.name}</div>
                <div className="qr-code">{r.code}</div>
              </div>
            ))}
          </div>
        </main>
      )}
    </>
  );
}
