import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { statusLabel } from "@/shared/lib/format";
import { salesService } from "@/modules/sales/services/sales.service";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import { paymentService } from "@/modules/payment/services/payment.service";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";

// "Lompat ke dokumen" di command palette (PLAN redesain-ui §6.3, U6-2): ketik nomor dokumen →
// cari lewat endpoint daftar yang sudah menerima `search` (semuanya mencocokkan kolom `number`).
// Tanpa endpoint baru (U11). Penerimaan barang (GR-) dan transfer stok (TRF-) dilewati karena
// daftar keduanya tidak punya parameter pencarian.
//
// Bentuk nomor (branch/application/service.go NextDocumentNumber):
//   ORD-<cabang>/PJ/<thn>/<bln>/NNNNN  order jual     ORD-<cabang>/PB/…  order beli
//   PAY-…  pembayaran   SJ-…  surat jalan   RTR-…  retur jual   RTB-…  retur beli

export interface DocumentHit {
  key: string;
  /** Jenis dokumen untuk dibaca pengguna, mis. "Order jual". */
  kind: string;
  number: string;
  /** Keterangan singkat: pihak/order asal + status. */
  detail: string;
  path: string;
}

interface DocumentSource {
  id: string;
  kind: string;
  perm: string;
  /** Awalan nomor yang pasti milik sumber ini (huruf besar, tanpa spasi). */
  owns: (q: string) => boolean;
  search: (q: string, limit: number, signal: AbortSignal) => Promise<DocumentHit[]>;
}

const join = (...parts: (string | undefined)[]) => parts.filter(Boolean).join(" · ");

const SOURCES: DocumentSource[] = [
  {
    id: "so",
    kind: "Order jual",
    perm: "sales.view",
    owns: (q) => q.startsWith("ORD-") && !q.includes("/PB"),
    search: async (q, limit, signal) =>
      (await salesService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `so-${r.id}`,
        kind: "Order jual",
        number: r.number,
        detail: join(r.partyName || "Tunai", statusLabel(r.status)),
        path: `${ROUTE_PATHS.salesOrders}/${r.id}`,
      })),
  },
  {
    id: "po",
    kind: "Order beli",
    perm: "purchasing.view",
    owns: (q) => q.startsWith("ORD-") && !q.includes("/PJ"),
    search: async (q, limit, signal) =>
      (await purchaseOrderService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `po-${r.id}`,
        kind: "Order beli",
        number: r.number,
        detail: join(r.partyName, statusLabel(r.status)),
        path: `${ROUTE_PATHS.purchaseOrders}/${r.id}`,
      })),
  },
  {
    id: "pay",
    kind: "Pembayaran",
    perm: "payment.view",
    owns: (q) => q.startsWith("PAY-"),
    search: async (q, limit, signal) =>
      (await paymentService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `pay-${r.id}`,
        kind: "Pembayaran",
        number: r.number,
        detail: join(r.partyName, statusLabel(r.status)),
        path: `${ROUTE_PATHS.payments}/${r.id}`,
      })),
  },
  {
    id: "sj",
    kind: "Surat jalan",
    perm: "delivery.view",
    owns: (q) => q.startsWith("SJ-"),
    search: async (q, limit, signal) =>
      (await deliveryService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `sj-${r.id}`,
        kind: "Surat jalan",
        number: r.number,
        detail: join(r.orderNumber, statusLabel(r.status)),
        path: `${ROUTE_PATHS.deliveries}/${r.id}`,
      })),
  },
  {
    id: "rtr",
    kind: "Retur jual",
    perm: "salesreturn.view",
    owns: (q) => q.startsWith("RTR-"),
    search: async (q, limit, signal) =>
      (await salesReturnService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `rtr-${r.id}`,
        kind: "Retur jual",
        number: r.number,
        detail: join(r.orderNumber, statusLabel(r.status)),
        path: `${ROUTE_PATHS.salesReturns}/${r.id}`,
      })),
  },
  {
    id: "rtb",
    kind: "Retur beli",
    perm: "purchasereturn.view",
    owns: (q) => q.startsWith("RTB-"),
    search: async (q, limit, signal) =>
      (await purchaseReturnService.list({ search: q, page: 1, limit }, signal)).items.map((r) => ({
        key: `rtb-${r.id}`,
        kind: "Retur beli",
        number: r.number,
        detail: join(r.orderNumber, statusLabel(r.status)),
        path: `${ROUTE_PATHS.purchaseReturns}/${r.id}`,
      })),
  },
];

/** Hasil per sumber bila awalan dikenali vs bila menebak ke semua sumber. */
const LIMIT_OWNED = 8;
const LIMIT_GUESS = 3;

/** Teks yang layak dicari sebagai nomor dokumen: ≥ 3 karakter dan berisi angka atau awalan dikenal. */
export function looksLikeDocumentNumber(raw: string): boolean {
  const q = raw.trim().toUpperCase();
  if (q.length < 3) return false;
  return /\d/.test(q) || SOURCES.some((s) => s.owns(q));
}

/** Sumber yang dicari untuk `raw`: pemilik awalan bila ada, selain itu semua yang diizinkan. */
export function documentSourcesFor(raw: string, can: (perm: string) => boolean): { ids: string[]; owned: boolean } {
  const q = raw.trim().toUpperCase();
  const allowed = SOURCES.filter((s) => can(s.perm));
  const owners = allowed.filter((s) => s.owns(q));
  const picked = owners.length > 0 ? owners : allowed;
  return { ids: picked.map((s) => s.id), owned: owners.length > 0 };
}

/**
 * Cari dokumen bernomor `raw` di sumber yang diizinkan peran. Sumber yang gagal (jaringan, 403)
 * dilewati diam-diam: palet tetap berguna dengan hasil sumber lain. Nomor yang persis sama
 * diurutkan paling atas.
 */
export async function searchDocuments(
  raw: string,
  can: (perm: string) => boolean,
  signal: AbortSignal,
): Promise<DocumentHit[]> {
  const q = raw.trim();
  if (!looksLikeDocumentNumber(q)) return [];
  const { ids, owned } = documentSourcesFor(q, can);
  const limit = owned ? LIMIT_OWNED : LIMIT_GUESS;
  const settled = await Promise.allSettled(
    SOURCES.filter((s) => ids.includes(s.id)).map((s) => s.search(q, limit, signal)),
  );
  const upper = q.toUpperCase();
  return settled
    .flatMap((r) => (r.status === "fulfilled" ? r.value : []))
    .sort((a, b) => Number(b.number.toUpperCase() === upper) - Number(a.number.toUpperCase() === upper));
}
