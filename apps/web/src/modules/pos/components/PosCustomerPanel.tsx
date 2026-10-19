import { UserRound } from "lucide-react";
import { Badge, Button } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import type { PosCustomerDetail, PricingQuote } from "@/modules/pos/types";

// Kepala keranjang POS (legacy: member summary panel): nama pelanggan + tombol Ganti,
// badge tipe member, dan total hemat harga member dari POST /api/v1/pricing/quote.
// Presentasional — fetch milik hook. Ganti pelanggan = Sheet pencarian (CustomerSheet).
export function PosCustomerPanel({
  customer,
  memberName,
  quote,
  quoteStatus,
  quoteError,
  savings,
  locked,
  onChange,
}: {
  customer: PosCustomerDetail | null;
  memberName: string | null;
  quote: PricingQuote | null;
  quoteStatus: "idle" | "loading" | "error";
  quoteError: string | null;
  savings: number;
  locked: boolean;
  onChange: () => void;
}) {
  return (
    <div className="border-b border-border px-4 py-3">
      <div className="flex items-center gap-3">
        <span aria-hidden className="flex size-10 flex-none items-center justify-center rounded-full bg-muted text-muted-foreground">
          <UserRound className="size-5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-2xs font-medium tracking-wide text-muted-foreground uppercase">Pelanggan</p>
          <p className="truncate text-base font-semibold text-foreground">{customer ? customer.name : "Umum / Walk-in"}</p>
        </div>
        <Button
          variant="secondary"
          className="h-12 px-4 text-base"
          disabled={locked}
          onClick={onChange}
          aria-label="Ganti pelanggan"
        >
          Ganti
        </Button>
      </div>
      {customer ? (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <span className="text-xs text-muted-foreground">{customer.phone?.trim() ? customer.phone : customer.code}</span>
          {customer.member ? (
            <Badge tone="info">{memberName ?? customer.member.code}</Badge>
          ) : (
            <Badge tone="neutral">Non-member</Badge>
          )}
          {quote?.memberInactive && <Badge tone="warning">Member nonaktif</Badge>}
        </div>
      ) : (
        <p className="mt-1.5 text-xs text-muted-foreground">Harga katalog berlaku — pilih pelanggan untuk harga member.</p>
      )}
      {quoteStatus === "loading" && <p className="mt-1.5 text-xs font-semibold text-warning">Menghitung harga member…</p>}
      {quoteStatus === "error" && quoteError && <p className="mt-1.5 text-xs font-semibold text-destructive">{quoteError}</p>}
      {customer && quote && !quote.memberInactive && savings > 0 && (
        <p className="mt-1.5 text-sm font-semibold text-success">Hemat {formatIDR(savings)} dengan harga member</p>
      )}
      {customer && quote && !quote.memberInactive && savings === 0 && customer.member && (
        <p className="mt-1.5 text-xs text-muted-foreground">Harga member sama dengan harga katalog untuk keranjang ini.</p>
      )}
    </div>
  );
}
