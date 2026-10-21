import { useState } from "react";
import { Button } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { financeService } from "@/modules/finance/services/finance.service";
import type { UncostedItem } from "@/modules/finance/types";

interface MissingCostNoticeProps {
  /** Pesan konflik server ("Harga modal produk X belum tersedia"). */
  message: string;
  canPost: boolean;
}

// "Biaya belum lengkap": posting ditolak karena produk belum punya harga
// modal. Pemulihan = sinkronkan ledger biaya (costing/sync) lalu cek daftar
// produk yang masih belum berbiaya (costing/uncosted), kemudian ulangi posting.
export function MissingCostNotice({ message, canPost }: MissingCostNoticeProps) {
  const [busy, setBusy] = useState(false);
  const [items, setItems] = useState<UncostedItem[] | null>(null);

  async function syncAndCheck() {
    setBusy(true);
    try {
      const sync = await financeService.costingSync();
      toast.fromServer(sync.message, "Sinkronisasi biaya selesai");
      setItems((await financeService.uncosted()) ?? []);
    } catch (err) {
      toast.danger("Gagal sinkron biaya", formatApiError(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Notice tone="warning" title="Biaya belum lengkap">
      <p>
        {message}
        {" "}Posting ditahan sampai harga modal tersedia — terima
        barang (penerimaan/saldo awal stok) atau sinkronkan biaya, lalu ulangi
        posting.
      </p>
      <div className="mt-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={!canPost || busy}
          onClick={() => void syncAndCheck()}
        >
          {busy ? "Menyinkronkan…" : "Sinkron & Cek Produk Belum Berbiaya"}
        </Button>
      </div>
      {items && (
        <div className="mt-2">
          {items.length === 0 ? (
            <p>Semua produk sudah berbiaya — silakan ulangi posting.</p>
          ) : (
            <ul className="list-disc pl-5">
              {items.map((it) => (
                <li key={`${it.productId}-${it.variantId}`}>
                  {it.productCode} — {it.productName}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </Notice>
  );
}
