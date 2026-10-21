import { Money } from "@/shared/components/patterns";

/** Sisa tagihan pemasok setelah koreksi; negatif = kelebihan bayar. */
export function BalanceAfter({ outstanding }: { outstanding: number }) {
  if (outstanding === 0) return <span>Lunas</span>;
  if (outstanding > 0) {
    return (
      <span>
        Sisa tagihan <Money value={outstanding} className="font-semibold" />
      </span>
    );
  }
  return (
    <span className="text-warning">
      Kelebihan bayar <Money value={-outstanding} className="font-semibold" /> (tampil sebagai saldo pemasok)
    </span>
  );
}
