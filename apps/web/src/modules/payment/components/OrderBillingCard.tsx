import type { ReactNode } from "react";
import { Badge, SectionCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { billingBadge } from "@/modules/payment/lib/order-billing";
import type { BillingView } from "@/modules/payment/lib/order-billing";

interface OrderBillingCardProps {
  billing: BillingView;
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  grandTotal: number;
  /** Pengaruh retur (SL-03); hanya SO. Dihilangkan = tanpa baris retur. */
  returns?: {
    adjustment: number;
    netTotal: number;
    refundDue: number;
    exchangeChargeDue: number;
  };
  /** "Tagihan" untuk SO, "Hutang ke supplier" untuk PO. */
  title?: string;
}

function Row({
  label,
  children,
  strong,
  tone,
}: {
  label: string;
  children: ReactNode;
  strong?: boolean;
  tone?: "bad" | "ok";
}) {
  const color = tone === "bad" ? "text-destructive" : tone === "ok" ? "text-success" : "text-foreground";
  return (
    <div className={cn("flex items-baseline justify-between gap-3 text-sm", strong && "border-t border-border pt-3")}>
      <dt className={strong ? "font-semibold text-foreground" : "text-muted-foreground"}>{label}</dt>
      <dd className={cn(color, strong && "font-display text-xl font-semibold")}>{children}</dd>
    </div>
  );
}

/**
 * Kartu "Ringkasan" (gaya T2) satu order: pecahan total + dibayar / sisa / jatuh tempo.
 * Domain-agnostik terhadap SO/PO (hanya angka). Bila server tidak mengirim
 * ringkasan pembayaran, hanya pecahan total yang tampil.
 */
export function OrderBillingCard({
  billing,
  subtotal,
  discountTotal,
  taxTotal,
  grandTotal,
  returns,
  title = "Ringkasan",
}: OrderBillingCardProps) {
  const hasReturns = returns !== undefined && returns.adjustment !== 0;
  const badge = billingBadge(billing);
  return (
    <SectionCard
      title={title}
      actions={billing.available ? <Badge tone={badge.tone}>{badge.label}</Badge> : undefined}
    >
      <dl className="space-y-2">
        <Row label="Subtotal">
          <Money value={subtotal} />
        </Row>
        <Row label="Diskon">
          <Money value={discountTotal} />
        </Row>
        <Row label="Pajak">
          <Money value={taxTotal} />
        </Row>
        <Row label="Total tagihan" strong>
          <Money value={grandTotal} />
        </Row>
        {hasReturns && returns && (
          <>
            <Row label={returns.adjustment < 0 ? "Retur terkonfirmasi" : "Tukar (selisih tambah)"}>
              <Money value={returns.adjustment} />
            </Row>
            <Row label="Total bersih setelah retur" strong>
              <Money value={returns.netTotal} />
            </Row>
          </>
        )}
        {billing.available && (
          <>
            <Row label="Sudah dibayar" tone={billing.paid > 0 ? "ok" : undefined}>
              <Money value={billing.paid} />
            </Row>
            <Row label="Sisa tagihan" tone={billing.outstanding > 0 ? "bad" : "ok"}>
              <Money value={billing.outstanding} />
            </Row>
            {returns && returns.refundDue > 0 && (
              <Row label="Harus dikembalikan ke pelanggan" tone="bad">
                <Money value={returns.refundDue} />
              </Row>
            )}
            {returns && returns.exchangeChargeDue > 0 && (
              <Row label="Selisih tukar belum dipungut" tone="bad">
                <Money value={returns.exchangeChargeDue} />
              </Row>
            )}
            {billing.dueDate !== "" && (
              <Row label={billing.overdue ? "Jatuh tempo (terlewat)" : "Jatuh tempo"} tone={billing.overdue ? "bad" : undefined}>
                {formatDate(billing.dueDate)}
              </Row>
            )}
          </>
        )}
      </dl>
      <p className="mt-4 text-xs text-muted-foreground">
        {returns
          ? "Sisa tagihan = total bersih setelah retur − uang yang benar-benar diterima. Tidak pernah negatif; kelebihan bayar tampil sebagai dana yang harus dikembalikan."
          : "Sisa tagihan = total − pembayaran aktif."}
      </p>
    </SectionCard>
  );
}
