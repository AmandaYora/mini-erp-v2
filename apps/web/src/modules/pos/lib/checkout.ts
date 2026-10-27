import type {
  CheckoutMode,
  PayMethod,
  PosCreatedDelivery,
  PosCreatedOrder,
  PosCreatedPayment,
  PosDeliveryLinePayload,
  PosDeliveryPayload,
  PosOrderLinePayload,
  PosOrderPayload,
  PosOrderState,
  PosPaymentPayload,
  PosSettlement,
} from "@/modules/pos/types";

// Pembangun payload SO untuk checkout POS — murni (tanpa IO) agar bisa diuji.
// Aturan disalin dari sales/application/service.go: validateHeader:
// paymentTerms hanya cod|net; net WAJIB dueDate YYYY-MM-DD.
// Bayar Nanti (pay_later) = termin net + jatuh tempo, TANPA langkah pembayaran.
// Bayar sekarang (pay_now) = termin cod; pembayaran lunas dicatat terpisah.

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export interface BuildPosOrderInput {
  partyId: number;
  mode: CheckoutMode;
  /** YYYY-MM-DD — wajib bila mode pay_later. */
  dueDate?: string;
  /** UUID keranjang (kunci idempoten SO). */
  clientRef: string;
  /** PPN opsional (K-03): "none"/kosong = tanpa pajak; tarif 0â€“100 wajib > 0 bila kena pajak. */
  taxType?: string;
  taxRate?: number;
  /** Catatan transaksi (opsional, maks 1000 karakter). */
  notes?: string;
  /** Alamat kirim opt-in (id buku alamat pelanggan); diabaikan untuk walk-in. */
  shipToAddressId?: number | null;
  items: PosOrderLinePayload[];
}

export const POS_NOTES_MAX = 1000;

/** Bagian opsional payload: catatan dan alamat kirim (SL-07). */
function optionalFields(
  input: BuildPosOrderInput,
): Pick<PosOrderPayload, "notes" | "shipToAddressId" | "taxType" | "taxRate"> {
  const out: Pick<PosOrderPayload, "notes" | "shipToAddressId" | "taxType" | "taxRate"> = {};
  if (input.taxType === "include" || input.taxType === "exclude") {
    const rate = input.taxRate ?? 0;
    if (!Number.isFinite(rate) || rate <= 0 || rate > 100) {
      throw new Error("Tarif PPN harus lebih dari 0 dan paling banyak 100.");
    }
    out.taxType = input.taxType;
    out.taxRate = rate;
  }
  const notes = (input.notes ?? "").trim();
  if (notes.length > POS_NOTES_MAX) {
    throw new Error(`Catatan maksimal ${POS_NOTES_MAX} karakter.`);
  }
  if (notes !== "") out.notes = notes;
  if (input.partyId > 0 && input.shipToAddressId != null && input.shipToAddressId > 0) {
    out.shipToAddressId = input.shipToAddressId;
  }
  return out;
}

// Termin net (Bayar Nanti) hanya untuk pelanggan terdaftar yang bisa dihubungi:
// sales/application/service.go validateHeader menolak net tanpa partyId dan net
// untuk customer tanpa nomor telepon. Opsi ini dinonaktifkan di POS sebelum kasir
// menekan Bayar, bukan dibiarkan ditolak server.
export const PAY_LATER_NEEDS_CUSTOMER_MESSAGE =
  "Bayar Nanti hanya untuk pelanggan terdaftar — pilih pelanggan dulu, atau gunakan Bayar Sekarang untuk penjualan umum/walk-in.";
export const PAY_LATER_NEEDS_PHONE_MESSAGE =
  "Pelanggan ini belum punya nomor telepon — Bayar Nanti butuh nomor telepon. Lengkapi data pelanggan, atau gunakan Bayar Sekarang.";

/** Alasan Bayar Nanti tidak tersedia (null = tersedia). `customerPhone` undefined =
 * detail pelanggan belum termuat (jangan memblokir; server tetap otoritas). */
export function payLaterBlockReason(input: {
  customerId: number | null;
  customerPhone?: string | null;
}): string | null {
  if (input.customerId === null || input.customerId <= 0) return PAY_LATER_NEEDS_CUSTOMER_MESSAGE;
  if (input.customerPhone !== undefined && (input.customerPhone ?? "").trim() === "") {
    return PAY_LATER_NEEDS_PHONE_MESSAGE;
  }
  return null;
}

/** Bangun body POST /sales-orders channel pos. Melempar Error berbahasa
 * Indonesia bila input melanggar aturan termin backend. */
export function buildPosOrderPayload(input: BuildPosOrderInput): PosOrderPayload {
  if (input.clientRef.trim() === "" || input.clientRef.length > 64) {
    throw new Error("Referensi keranjang tidak valid.");
  }
  if (input.items.length === 0) {
    throw new Error("Keranjang kosong — minimal 1 baris item.");
  }
  if (input.mode === "pay_later") {
    if (input.partyId <= 0) {
      throw new Error(PAY_LATER_NEEDS_CUSTOMER_MESSAGE);
    }
    const dueDate = (input.dueDate ?? "").trim();
    if (dueDate === "") {
      throw new Error("Jatuh tempo wajib diisi untuk Bayar Nanti.");
    }
    if (!DATE_RE.test(dueDate)) {
      throw new Error("Format jatuh tempo harus YYYY-MM-DD.");
    }
    return {
      channel: "pos",
      partyId: input.partyId,
      paymentTerms: "net",
      dueDate,
      clientRef: input.clientRef,
      ...optionalFields(input),
      items: input.items,
    };
  }
  return {
    channel: "pos",
    partyId: input.partyId,
    paymentTerms: "cod",
    clientRef: input.clientRef,
    ...optionalFields(input),
    items: input.items,
  };
}

// --- mesin langkah checkout (idempoten, bisa dilanjutkan) -------------------
//
// Urutan: (1) buat order (2) konfirmasi order (3) buat SJ draf (4) konfirmasi
// SJ — bayar sekarang membawa settlement dalam SATU panggilan; Bayar Nanti
// (termin net) tanpa settlement. Titik mulai SELALU diambil dari server
// (GET .../by-client-ref/{ref}/state → nextStep), bukan dari ingatan klien,
// sehingga reload dan kegagalan di tengah jalan lanjut dari langkah yang tepat.

export type CheckoutStep =
  | "create_order"
  | "confirm"
  | "create_delivery"
  | "confirm_delivery"
  | "pay"
  | "done"
  | "cancelled";

/** Langkah awal: tanpa state (order belum pernah sampai server) = buat order. */
export function resolveStartStep(state: PosOrderState | null): CheckoutStep {
  return state === null ? "create_order" : state.nextStep;
}

/** Urutan langkah yang masih harus dijalankan dari `from` (untuk tampilan/uji). */
export function remainingSteps(from: CheckoutStep, payNow: boolean): CheckoutStep[] {
  const order: CheckoutStep[] = [
    "create_order",
    "confirm",
    "create_delivery",
    "confirm_delivery",
  ];
  if (from === "pay") return payNow ? ["pay"] : [];
  if (from === "done" || from === "cancelled") return [];
  return order.slice(order.indexOf(from));
}

export interface CheckoutApi {
  createOrder(body: PosOrderPayload): Promise<PosCreatedOrder>;
  confirmOrder(id: number): Promise<PosCreatedOrder>;
  createDelivery(body: PosDeliveryPayload): Promise<PosCreatedDelivery>;
  confirmDelivery(id: number, settlement?: PosSettlement): Promise<PosCreatedDelivery>;
  createPayment(body: PosPaymentPayload): Promise<PosCreatedPayment>;
  /** Pembayaran tersimpan untuk order (kembalian dari data server); boleh null. */
  findPayment?(orderId: number): Promise<PosCreatedPayment | null>;
}

export interface CheckoutPlan {
  clientRef: string;
  orderPayload: PosOrderPayload;
  partyId: number;
  /** true = bayar sekarang; false = Bayar Nanti (net, tanpa settlement). */
  payNow: boolean;
  method: PayMethod;
  /** Uang diterima (tunai). Diabaikan untuk transfer. */
  tendered: number;
  /** Rekening transfer (ADR-0026); diabaikan untuk tunai. null = rekening order/default. */
  cashAccountId?: number | null;
  /** Punya izin payment.create. Tanpa izin, bayar sekarang ditolak SEBELUM dokumen apa pun dibuat:
   * konfirmasi SJ order tunai mewajibkan settlement, jadi order akan tertinggal setengah jalan
   * (KI legacy BR-18 / e2e K1). Bayar Nanti tetap bisa. */
  canPay: boolean;
  deliveryItems: () => PosDeliveryLinePayload[];
  /** Dipanggil sebelum order dikonfirmasi; string = hentikan dengan pesan itu
   * (mis. stok kurang) agar tidak meninggalkan SO terkonfirmasi tanpa SJ. */
  beforeConfirm?: () => Promise<string | null>;
}

export type CheckoutOutcome =
  | {
      ok: true;
      order: PosCreatedOrder;
      /** Pembayaran belum tercatat (tanpa izin) — harus dicatat manual. */
      paymentPending: boolean;
      payment: PosCreatedPayment | null;
    }
  | {
      ok: false;
      step: CheckoutStep;
      error: unknown;
      /** Alasan henti terencana (beforeConfirm) — bukan kegagalan API. */
      blocked: string | null;
      order: PosCreatedOrder | null;
      /** Order mungkin sudah ada di server → kunci isi keranjang. */
      orderCreated: boolean;
    };

/** Pesan bila bayar sekarang diminta tanpa izin payment.create (K1). */
export const NO_PAY_PERMISSION_MESSAGE =
  "Akun ini belum punya izin mencatat pembayaran, jadi pembayaran langsung tidak bisa diproses. Pilih Bayar Nanti, atau minta administrator menambahkan izin “Pembayaran: Tambah” ke role Anda.";

/** Rekening transfer pada settlement/pembayaran (ADR-0026); kosong untuk tunai. */
function transferAccount(plan: CheckoutPlan): { cashAccountId?: number } {
  return plan.method !== "cash" && plan.cashAccountId != null && plan.cashAccountId > 0
    ? { cashAccountId: plan.cashAccountId }
    : {};
}

/** Jalankan checkout dari titik yang ditentukan `state` (null = awal). */
export async function runCheckout(
  api: CheckoutApi,
  plan: CheckoutPlan,
  state: PosOrderState | null,
): Promise<CheckoutOutcome> {
  let step = resolveStartStep(state);
  let order: PosCreatedOrder | null = state?.order ?? null;
  let deliveryId: number | null =
    state?.deliveries.find((d) => d.status === "draft")?.id ?? null;
  let outstanding = state?.outstanding ?? 0;
  let orderCreated = state !== null;
  let paymentPending = false;
  let payment: PosCreatedPayment | null = null;

  const fail = (at: CheckoutStep, error: unknown, blocked: string | null = null): CheckoutOutcome => ({
    ok: false,
    step: at,
    error,
    blocked,
    order,
    orderCreated,
  });

  // K1: tanpa izin pembayaran, bayar sekarang pasti gagal di konfirmasi SJ — hentikan di awal.
  if (plan.payNow && !plan.canPay && step !== "done") {
    return fail(step, null, NO_PAY_PERMISSION_MESSAGE);
  }

  for (;;) {
    switch (step) {
      case "cancelled":
        return fail(step, new Error("Order ini sudah dibatalkan — mulai transaksi baru."));
      case "create_order":
        try {
          order = await api.createOrder(plan.orderPayload);
          orderCreated = true;
          step = "confirm";
        } catch (error) {
          return fail(step, error);
        }
        break;
      case "confirm":
        if (plan.beforeConfirm) {
          const blocked = await plan.beforeConfirm();
          if (blocked) return fail(step, null, blocked);
        }
        try {
          order = await api.confirmOrder((order as PosCreatedOrder).id);
          step = "create_delivery";
        } catch (error) {
          return fail(step, error);
        }
        break;
      case "create_delivery":
        try {
          const note = await api.createDelivery({
            salesOrderId: (order as PosCreatedOrder).id,
            clientRef: plan.clientRef,
            items: plan.deliveryItems(),
          });
          deliveryId = note.id;
          step = "confirm_delivery";
        } catch (error) {
          return fail(step, error);
        }
        break;
      case "confirm_delivery":
        try {
          if (deliveryId === null) throw new Error("Surat jalan draf tidak ditemukan.");
          const settle = plan.payNow && plan.canPay;
          const settlement: PosSettlement | undefined = settle
            ? {
                mode: "pay_now",
                method: plan.method,
                ...(plan.method === "cash" ? { amountTendered: plan.tendered } : {}),
                ...transferAccount(plan),
              }
            : undefined;
          await api.confirmDelivery(deliveryId, settlement);
          paymentPending = plan.payNow && !plan.canPay;
          step = "done";
        } catch (error) {
          return fail(step, error);
        }
        break;
      case "pay":
        if (!plan.payNow || !plan.canPay) {
          paymentPending = plan.payNow;
          step = "done";
          break;
        }
        try {
          const o = order as PosCreatedOrder;
          const amount = outstanding > 0 ? outstanding : o.grandTotal;
          payment = await api.createPayment({
            partyId: plan.partyId,
            method: plan.method,
            amount,
            allocations: [{ orderType: "sales", orderId: o.id, amount }],
            clientRef: plan.clientRef,
            ...(plan.method === "cash" ? { amountTendered: plan.tendered } : {}),
            ...transferAccount(plan),
          });
          outstanding = 0;
          step = "done";
        } catch (error) {
          return fail(step, error);
        }
        break;
      case "done": {
        if (payment === null && plan.payNow && plan.canPay && api.findPayment) {
          try {
            payment = await api.findPayment((order as PosCreatedOrder).id);
          } catch {
            payment = null; // struk tetap bisa dicetak tanpa id pembayaran
          }
        }
        return { ok: true, order: order as PosCreatedOrder, paymentPending, payment };
      }
    }
  }
}

/** Nama langkah untuk pesan ke kasir. */
export const CHECKOUT_STEP_LABEL: Record<CheckoutStep, string> = {
  create_order: "membuat order",
  confirm: "mengonfirmasi order",
  create_delivery: "membuat surat jalan",
  confirm_delivery: "mengonfirmasi surat jalan / pembayaran",
  pay: "mencatat pembayaran",
  done: "selesai",
  cancelled: "dibatalkan",
};
