import { SectionCard } from "@/shared/components/ui";
import { Money, Qty } from "@/shared/components/patterns";
import { formatDate, formatIDR } from "@/shared/lib/format";
import type { PurchaseOrderCorrection } from "@/modules/purchasing/types";

const KIND_LABEL: Record<PurchaseOrderCorrection["kind"], string> = {
  price: "Koreksi harga",
  short_close: "Tutup sisa order",
};

/** Riwayat koreksi order: apa yang berubah, kapan, oleh siapa, dan kenapa. */
export function CorrectionHistoryCard({ corrections }: { corrections: PurchaseOrderCorrection[] }) {
  if (corrections.length === 0) return null;
  return (
    <SectionCard title={`Riwayat koreksi (${corrections.length})`}>
      <ol className="flex flex-col gap-4">
        {corrections.map((c) => (
          <li key={c.id} className="flex flex-col gap-1.5 border-l-2 border-border pl-3">
            <p className="text-sm">
              <span className="font-medium">{KIND_LABEL[c.kind] ?? c.kind}</span>{" "}
              <span className="text-muted-foreground">
                {formatDate(c.date)}
                {c.createdByName ? ` oleh ${c.createdByName}` : ""}
              </span>
            </p>
            <p className="text-sm text-foreground">{c.reason}</p>
            <ul className="text-xs text-muted-foreground">
              {c.lines.map((l) => (
                <li key={`${c.id}-${l.itemId}`}>
                  {l.productName}:{" "}
                  {c.kind === "short_close" ? (
                    <>
                      qty <Qty value={l.oldQty} /> menjadi <Qty value={l.newQty} unit={l.uom} />
                    </>
                  ) : (
                    <>
                      harga {formatIDR(l.oldUnitPrice)} menjadi {formatIDR(l.newUnitPrice)}
                      {l.oldDiscountPct !== l.newDiscountPct ? `, diskon ${l.oldDiscountPct}% menjadi ${l.newDiscountPct}%` : ""}
                      {l.oldDiscountNominal !== l.newDiscountNominal
                        ? `, potongan ${formatIDR(l.oldDiscountNominal)} menjadi ${formatIDR(l.newDiscountNominal)}`
                        : ""}
                    </>
                  )}
                </li>
              ))}
            </ul>
            <p className="text-xs text-muted-foreground">
              Total order <Money value={c.oldGrandTotal} /> menjadi <Money value={c.newGrandTotal} />
              {c.valueDelta + c.taxDelta !== 0 ? (
                <>
                  ; jurnal koreksi <Money value={c.valueDelta + c.taxDelta} />
                </>
              ) : null}
            </p>
          </li>
        ))}
      </ol>
    </SectionCard>
  );
}
