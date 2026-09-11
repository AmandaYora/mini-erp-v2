import { Badge } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import type {
  PosCustomerDetail,
  PricingQuote,
} from "@/modules/pos/types";

// Panel pelanggan POS (legacy: member summary panel): nama + telepon,
// badge tipe member, dan total hemat harga member yang berlaku dari
// POST /api/v1/pricing/quote. Presentasional — fetch milik halaman.
export function PosCustomerPanel({
  customer,
  memberName,
  quote,
  quoteStatus,
  quoteError,
  savings,
}: {
  customer: PosCustomerDetail | null;
  memberName: string | null;
  quote: PricingQuote | null;
  quoteStatus: "idle" | "loading" | "error";
  quoteError: string | null;
  savings: number;
}) {
  if (!customer) {
    return (
      <div className="rounded-md border border-hairline bg-surface-subtle px-4 py-3">
        <p className="text-[0.72rem] font-bold uppercase text-muted">Pelanggan</p>
        <p className="font-bold text-ink">Umum / Walk-in</p>
        <p className="mt-0.5 text-[0.78rem] text-muted">
          Harga katalog berlaku — pilih customer untuk harga member.
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-md border border-hairline bg-surface-subtle px-4 py-3">
      <p className="text-[0.72rem] font-bold uppercase text-muted">Pelanggan</p>
      <p className="truncate font-bold text-ink">{customer.name}</p>
      <p className="mt-0.5 text-[0.78rem] text-muted">
        {customer.phone?.trim() ? customer.phone : customer.code}
      </p>
      <div className="mt-1.5 flex flex-wrap items-center gap-2">
        {customer.member ? (
          <Badge tone="info">{memberName ?? customer.member.code}</Badge>
        ) : (
          <Badge tone="neutral">Non-member</Badge>
        )}
        {quote?.memberInactive && <Badge tone="warning">Member nonaktif</Badge>}
      </div>
      {quoteStatus === "loading" && (
        <p className="mt-2 text-[0.8rem] font-semibold text-warn">
          Menghitung harga member…
        </p>
      )}
      {quoteStatus === "error" && quoteError && (
        <p className="mt-2 text-[0.8rem] font-semibold text-bad">{quoteError}</p>
      )}
      {quote && !quote.memberInactive && savings > 0 && (
        <p className="mt-2 text-[0.82rem] font-semibold text-ok">
          Hemat {formatIDR(savings)} dengan harga member
        </p>
      )}
      {quote && !quote.memberInactive && savings === 0 && customer.member && (
        <p className="mt-2 text-[0.78rem] text-muted">
          Harga member sama dengan harga katalog untuk keranjang ini.
        </p>
      )}
    </div>
  );
}
