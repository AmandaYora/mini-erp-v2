// Cerminan 1:1 dari view backend (payment/presentation/handler.go):
// paymentView, PartyBalance (contracts/client.go), dan Proofs (service.go).
// Semua kunci JSON camelCase.

export interface PaymentAllocation {
  orderType: string;
  orderId: number;
  orderNumber: string;
  amount: number;
}

export interface Payment {
  id: number;
  number: string;
  branchId: number;
  partyId: number;
  partyName: string;
  partyType: string;
  direction: string;
  amount: number;
  method: string;
  paidAt: string;
  notes: string;
  status: string;
  allocations: PaymentAllocation[];
}

/** Pihak untuk SearchSelect — gabungan customers + suppliers aktif. */
export interface PartyLookup {
  id: number;
  name: string;
  /** "customer" | "supplier" (party/contracts: PartyCustomer/PartySupplier). */
  type: string;
}

// --- payload keluar (kunci persis struct Create/SettleCredit di handler.go) ---

export interface AllocationInput {
  orderType: string;
  orderId: number;
  amount: number;
}

export interface PaymentCreatePayload {
  partyId: number;
  amount: number;
  method?: string;
  paidAt?: string;
  notes?: string;
  /** Kosong = backend alokasi FIFO otomatis (handler.go: Create). */
  allocations?: AllocationInput[];
}

export interface SettleCreditPayload {
  partyId: number;
  returnType: string;
  returnId: number;
  orderType: string;
  orderId: number;
  amount: number;
  notes?: string;
}

// --- saldo & ledger (GET party-balances / party-ledger) ---

export interface OrderBalance {
  orderType: string;
  orderId: number;
  number: string;
  grandTotal: number;
  paid: number;
  outstanding: number;
}

export interface PartyBalance {
  partyId: number;
  partyName: string;
  totalBilled: number;
  totalPaid: number;
  totalReturned: number;
  totalRefunded: number;
  outstanding: number;
  orders: OrderBalance[];
  returns: OrderBalance[];
}

export interface PaymentProof {
  id: number;
  originalName: string;
  mime: string;
  sizeBytes: number;
  url: string;
}

// --- label Indonesia ---

export function methodLabel(method: string | null | undefined): string {
  switch ((method ?? "").toLowerCase()) {
    case "cash":
      return "Tunai";
    case "transfer":
      return "Transfer";
    case "offset":
      return "Offset";
    default:
      return method ?? "-";
  }
}

export function directionLabel(direction: string | null | undefined): string {
  switch ((direction ?? "").toLowerCase()) {
    case "in":
      return "Masuk";
    case "out":
      return "Keluar";
    default:
      return direction ?? "-";
  }
}

export function orderTypeLabel(t: string | null | undefined): string {
  switch (t) {
    case "purchase":
      return "Order Beli";
    case "sales":
      return "Order Jual";
    case "sales_return":
      return "Retur Jual";
    case "purchase_return":
      return "Retur Beli";
    default:
      return t ?? "-";
  }
}
