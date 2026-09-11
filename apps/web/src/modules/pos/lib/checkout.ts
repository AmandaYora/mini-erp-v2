import type {
  CheckoutMode,
  PosOrderLinePayload,
  PosOrderPayload,
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
  items: PosOrderLinePayload[];
}

/** Bangun body POST /sales-orders channel pos. Melempar Error berbahasa
 * Indonesia bila input melanggar aturan termin backend. */
export function buildPosOrderPayload(input: BuildPosOrderInput): PosOrderPayload {
  if (input.items.length === 0) {
    throw new Error("Keranjang kosong — minimal 1 baris item.");
  }
  if (input.mode === "pay_later") {
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
      items: input.items,
    };
  }
  return {
    channel: "pos",
    partyId: input.partyId,
    paymentTerms: "cod",
    items: input.items,
  };
}
