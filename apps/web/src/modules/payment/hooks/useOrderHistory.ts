import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toApiError } from "@/shared/services/http-client";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";
import { paymentService } from "@/modules/payment/services/payment.service";
import {
  accumulateProgress,
  activeAllocatedTotal,
  deliveryEvents,
  paymentEvents,
  receiptEvents,
  returnEvents,
  sortHistory,
} from "@/modules/payment/lib/order-history";
import type { HistoryEvent, LineProgress } from "@/modules/payment/lib/order-history";

export type OrderKind = "sales" | "purchase";

export interface OrderHistoryData {
  events: HistoryEvent[];
  /** Progres per baris (SJ untuk SO, penerimaan untuk PO); null bila tak boleh dibaca. */
  progress: Map<string, LineProgress> | null;
  /** Bagian riwayat yang gagal dimuat (bukan 403) — ditampilkan, tidak ditelan. */
  warnings: string[];
  /** Bagian yang disembunyikan karena izin. */
  hidden: { docs: boolean; payments: boolean; returns: boolean };
}

const PAGE = 100;
const MAX_LEDGER_PAGES = 10;

/**
 * Riwayat dokumen terkait satu order. Memakai endpoint daftar yang sudah ada
 * (filter order per modul) dan buku pembayaran pihak (alokasi disaring ke
 * order ini). Tiap bagian digerbang izinnya sendiri; kegagalan satu bagian
 * tidak membuang bagian lain.
 */
export function useOrderHistory(
  kind: OrderKind,
  orderId: number,
  opts: { partyId: number; paidAmount: number; enabled: boolean; version?: unknown },
) {
  return useAsyncData<OrderHistoryData>(
    async () => {
      const empty: OrderHistoryData = {
        events: [],
        progress: null,
        warnings: [],
        hidden: { docs: true, payments: true, returns: true },
      };
      if (!opts.enabled || !Number.isInteger(orderId) || orderId <= 0) return empty;

      const can = useAuthStore.getState().can;
      const warnings: string[] = [];
      const events: HistoryEvent[] = [];
      let progress: Map<string, LineProgress> | null = null;

      const docPerm = kind === "sales" ? "delivery.view" : "goodsreceipt.view";
      const retPerm = kind === "sales" ? "salesreturn.view" : "purchasereturn.view";
      const hidden = { docs: !can(docPerm), payments: !can("payment.view"), returns: !can(retPerm) };

      const tasks: Promise<void>[] = [];

      if (!hidden.docs) {
        tasks.push(
          (async () => {
            try {
              if (kind === "sales") {
                const res = await deliveryService.list({ salesOrderId: orderId, page: 1, limit: PAGE });
                events.push(...deliveryEvents(res.items));
                progress = accumulateProgress(res.items, "confirmed", "draft");
              } else {
                const res = await goodsreceiptService.list({ purchaseOrderId: orderId, page: 1, limit: PAGE });
                events.push(...receiptEvents(res.items));
                progress = accumulateProgress(res.items, "posted", "pending");
              }
            } catch (err) {
              warnings.push(`Dokumen barang: ${toApiError(err).message}`);
            }
          })(),
        );
      }

      if (!hidden.returns) {
        tasks.push(
          (async () => {
            try {
              if (kind === "sales") {
                const res = await salesReturnService.list({ salesOrderId: orderId, page: 1, limit: PAGE });
                events.push(...returnEvents("sales", res.items));
              } else {
                const res = await purchaseReturnService.list({ purchaseOrderId: orderId, page: 1, limit: PAGE });
                events.push(...returnEvents("purchase", res.items));
              }
            } catch (err) {
              warnings.push(`Retur: ${toApiError(err).message}`);
            }
          })(),
        );
      }

      // Pembayaran: buku pihak (partyId > 0). Order walk-in/POS tanpa pihak
      // tidak punya buku; pembayarannya terwakili oleh ringkasan tagihan.
      if (!hidden.payments && opts.partyId > 0) {
        tasks.push(
          (async () => {
            try {
              const found = [] as Awaited<ReturnType<typeof paymentService.ledger>>["items"];
              for (let page = 1; page <= MAX_LEDGER_PAGES; page += 1) {
                const res = await paymentService.ledger(opts.partyId, page, PAGE);
                found.push(...res.items);
                const covered = activeAllocatedTotal(found, kind, orderId) >= opts.paidAmount;
                if (covered || page >= res.meta.totalPages) break;
              }
              events.push(...paymentEvents(found, kind, orderId));
            } catch (err) {
              warnings.push(`Pembayaran: ${toApiError(err).message}`);
            }
          })(),
        );
      }

      await Promise.all(tasks);
      return { events: sortHistory(events), progress, warnings, hidden };
    },
    [kind, orderId, opts.partyId, opts.paidAmount, opts.enabled, opts.version],
  );
}
