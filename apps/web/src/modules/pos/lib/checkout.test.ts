import { describe, expect, it } from "vitest";
import {
  NO_PAY_PERMISSION_MESSAGE,
  buildPosOrderPayload,
  remainingSteps,
  resolveStartStep,
  runCheckout,
  type CheckoutApi,
  type CheckoutPlan,
} from "@/modules/pos/lib/checkout";
import type { PosNextStep, PosOrderState } from "@/modules/pos/types";

// Test Tahap I (§7 PLAN): POS "Bayar Nanti" membuat SO dengan
// payment_terms=net + due_date, bukan cod — piutang POS tercatat.
describe("buildPosOrderPayload — Bayar Nanti", () => {
  const items = [{ productId: 1, variantId: 2, uom: "pcs", qty: 3 }];
  const clientRef = "11111111-2222-4333-8444-555555555555";

  it("membuat SO payment_terms=net + due_date untuk Bayar Nanti", () => {
    expect(
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", clientRef, items }),
    ).toEqual({
      channel: "pos",
      partyId: 7,
      paymentTerms: "net",
      dueDate: "2026-09-30",
      clientRef,
      items,
    });
  });

  it("tetap memakai cod tanpa due_date untuk bayar sekarang", () => {
    const payload = buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items });
    expect(payload.paymentTerms).toBe("cod");
    expect(payload.dueDate).toBeUndefined();
  });

  it("menolak Bayar Nanti tanpa jatuh tempo", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "", clientRef, items }),
    ).toThrow("Jatuh tempo wajib diisi");
  });

  it("menolak format jatuh tempo selain YYYY-MM-DD", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "30-09-2026", clientRef, items }),
    ).toThrow("YYYY-MM-DD");
  });

  it("menolak keranjang kosong", () => {
    expect(() =>
      buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", clientRef, items: [] }),
    ).toThrow("Keranjang kosong");
  });
});

// --- mesin langkah + resume -------------------------------------------------

const REF = "11111111-2222-4333-8444-555555555555";

function fakeServer() {
  const calls: string[] = [];
  const order = { id: 10, number: "SO-1", grandTotal: 50000 };
  let failAt: string | null = null;
  const api: CheckoutApi = {
    async createOrder(body) {
      calls.push("createOrder");
      if (failAt === "createOrder") throw new Error("net");
      expect(body.clientRef).toBe(REF);
      return order;
    },
    async confirmOrder(id) {
      calls.push("confirmOrder");
      if (failAt === "confirmOrder") throw new Error("net");
      expect(id).toBe(10);
      return order;
    },
    async createDelivery(body) {
      calls.push("createDelivery");
      if (failAt === "createDelivery") throw new Error("net");
      expect(body.clientRef).toBe(REF);
      // Serah langsung POS tidak bersopir: driver_name kosong adalah penanda yang dipakai
      // alert kesehatan data A11 untuk membedakan SJ reguler (ADR-0025).
      expect(body).not.toHaveProperty("driverName");
      return { id: 20, number: "SJ-1" };
    },
    async confirmDelivery(id, settlement) {
      calls.push(
        settlement ? `confirmDelivery+${settlement.mode}:${settlement.amountTendered ?? "-"}` : "confirmDelivery",
      );
      if (failAt === "confirmDelivery") throw new Error("net");
      expect(id).toBe(20);
      return { id: 20, number: "SJ-1" };
    },
    async createPayment() {
      calls.push("createPayment");
      return { id: 30, number: "PAY-1" };
    },
  };
  return {
    api,
    calls,
    failOn(step: string | null) {
      failAt = step;
    },
  };
}

function plan(over: Partial<CheckoutPlan> = {}): CheckoutPlan {
  return {
    clientRef: REF,
    orderPayload: buildPosOrderPayload({
      partyId: 0,
      mode: "pay_now",
      clientRef: REF,
      items: [{ productId: 1, variantId: 2, uom: "pcs", qty: 1 }],
    }),
    partyId: 0,
    payNow: true,
    method: "cash",
    tendered: 60000,
    canPay: true,
    deliveryItems: () => [{ productId: 1, variantId: 2, uom: "pcs", qty: 1 }],
    ...over,
  };
}

function stateAt(nextStep: PosNextStep, over: Partial<PosOrderState> = {}): PosOrderState {
  return {
    order: { id: 10, number: "SO-1", grandTotal: 50000 },
    deliveries: [],
    paidAmount: 0,
    outstanding: 50000,
    nextStep,
    ...over,
  };
}

describe("urutan langkah checkout", () => {
  it("tanpa state mulai dari create_order; selain itu mengikuti nextStep", () => {
    expect(resolveStartStep(null)).toBe("create_order");
    expect(resolveStartStep(stateAt("confirm_delivery"))).toBe("confirm_delivery");
  });

  it("urutan sisa dari tiap titik mulai", () => {
    expect(remainingSteps("create_order", true)).toEqual([
      "create_order",
      "confirm",
      "create_delivery",
      "confirm_delivery",
    ]);
    expect(remainingSteps("create_delivery", true)).toEqual(["create_delivery", "confirm_delivery"]);
    expect(remainingSteps("pay", true)).toEqual(["pay"]);
    expect(remainingSteps("done", true)).toEqual([]);
  });
});

describe("runCheckout", () => {
  it("bayar sekarang: 4 langkah, settlement dalam SATU panggilan konfirmasi SJ", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan(), null);
    expect(out.ok).toBe(true);
    expect(srv.calls).toEqual([
      "createOrder",
      "confirmOrder",
      "createDelivery",
      "confirmDelivery+pay_now:60000",
    ]);
  });

  it("Bayar Nanti: konfirmasi SJ tanpa settlement, tanpa pembayaran", async () => {
    const srv = fakeServer();
    await runCheckout(srv.api, plan({ payNow: false }), null);
    expect(srv.calls.at(-1)).toBe("confirmDelivery");
    expect(srv.calls).not.toContain("createPayment");
  });

  it("transfer: settlement tanpa amountTendered", async () => {
    const srv = fakeServer();
    await runCheckout(srv.api, plan({ method: "transfer" }), null);
    expect(srv.calls.at(-1)).toBe("confirmDelivery+pay_now:-");
  });

  it("tanpa izin bayar: bayar sekarang ditolak sebelum dokumen apa pun dibuat (K1)", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan({ canPay: false }), null);
    expect(srv.calls).toEqual([]);
    expect(out.ok).toBe(false);
    expect(!out.ok && out.blocked).toBe(NO_PAY_PERMISSION_MESSAGE);
    expect(!out.ok && out.orderCreated).toBe(false);
  });

  it("tanpa izin bayar: Bayar Nanti tetap selesai tanpa settlement", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan({ canPay: false, payNow: false }), null);
    expect(srv.calls.at(-1)).toBe("confirmDelivery");
    expect(out.ok && out.paymentPending).toBe(false);
  });

  it("gagal di tiap langkah lalu dilanjutkan dari nextStep sampai selesai (tiap langkah sekali)", async () => {
    const cases: [string, PosOrderState | null][] = [
      ["createOrder", null],
      ["confirmOrder", stateAt("confirm")],
      ["createDelivery", stateAt("create_delivery")],
      ["confirmDelivery", stateAt("confirm_delivery", { deliveries: [{ id: 20, number: "SJ-1", status: "draft" }] })],
    ];
    for (const [step, stateAfterFailure] of cases) {
      const srv = fakeServer();
      srv.failOn(step);
      const first = await runCheckout(srv.api, plan(), null);
      expect(first.ok).toBe(false);
      srv.failOn(null);
      const before = srv.calls.length;
      const second = await runCheckout(srv.api, plan(), stateAfterFailure);
      expect(second.ok).toBe(true);
      // lanjutan tidak mengulang langkah yang sudah sukses
      const resumed = srv.calls.slice(before);
      expect(resumed[0]).toBe(
        step === "confirmDelivery" ? "confirmDelivery+pay_now:60000" : step,
      );
      expect(resumed.filter((c) => c === "createOrder").length).toBe(step === "createOrder" ? 1 : 0);
    }
  });

  it("resume di langkah pay: pembayaran dibuat sekali dengan clientRef", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan(), stateAt("pay", { outstanding: 50000 }));
    expect(out.ok).toBe(true);
    expect(srv.calls).toEqual(["createPayment"]);
  });

  it("nextStep done: tidak ada panggilan baru", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan(), stateAt("done", { outstanding: 0 }));
    expect(out.ok).toBe(true);
    expect(srv.calls).toEqual([]);
  });

  it("order dibatalkan: gagal tanpa panggilan", async () => {
    const srv = fakeServer();
    const out = await runCheckout(srv.api, plan(), stateAt("cancelled"));
    expect(out.ok).toBe(false);
    expect(srv.calls).toEqual([]);
  });

  it("stok kurang menghentikan checkout SEBELUM order dikonfirmasi", async () => {
    const srv = fakeServer();
    const out = await runCheckout(
      srv.api,
      plan({ beforeConfirm: async () => "Stok Gula kurang 2" }),
      null,
    );
    expect(out.ok).toBe(false);
    if (!out.ok) {
      expect(out.blocked).toBe("Stok Gula kurang 2");
      expect(out.step).toBe("confirm");
    }
    expect(srv.calls).toEqual(["createOrder"]);
  });
});

// SL-07: catatan transaksi dan alamat kirim opt-in ikut ke payload order POS.
describe("buildPosOrderPayload — catatan dan alamat kirim (SL-07)", () => {
  const items = [{ productId: 1, variantId: 2, uom: "pcs", qty: 3 }];
  const clientRef = "11111111-2222-4333-8444-555555555555";

  it("tanpa catatan/alamat: payload tidak memuat kuncinya", () => {
    const p = buildPosOrderPayload({ partyId: 7, mode: "pay_now", clientRef, items, notes: "  ", shipToAddressId: null });
    expect("notes" in p).toBe(false);
    expect("shipToAddressId" in p).toBe(false);
  });

  it("catatan dipangkas dan alamat dikirim untuk pelanggan terdaftar (cod maupun net)", () => {
    const cod = buildPosOrderPayload({ partyId: 7, mode: "pay_now", clientRef, items, notes: " antar sore ", shipToAddressId: 12 });
    expect(cod).toMatchObject({ paymentTerms: "cod", notes: "antar sore", shipToAddressId: 12 });
    const net = buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", clientRef, items, shipToAddressId: 12 });
    expect(net).toMatchObject({ paymentTerms: "net", shipToAddressId: 12 });
  });

  it("walk-in tidak membawa alamat buku alamat; catatan tetap boleh", () => {
    const p = buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, notes: "bungkus rapi", shipToAddressId: 12 });
    expect(p.notes).toBe("bungkus rapi");
    expect("shipToAddressId" in p).toBe(false);
  });

  it("menolak catatan lebih dari 1000 karakter", () => {
    expect(() => buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, notes: "x".repeat(1001) })).toThrow(
      "Catatan maksimal",
    );
  });
});

// K-03: PPN opsional dari kasir ikut ke payload (default tanpa pajak).
describe("buildPosOrderPayload — PPN (K-03)", () => {
  const items = [{ productId: 1, variantId: 2, uom: "pcs", qty: 3 }];
  const clientRef = "11111111-2222-4333-8444-555555555555";

  it("tanpa pajak (bawaan): payload tidak memuat taxType/taxRate", () => {
    const p = buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items });
    expect("taxType" in p).toBe(false);
    expect("taxRate" in p).toBe(false);
    const none = buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, taxType: "none", taxRate: 0 });
    expect("taxType" in none).toBe(false);
  });

  it("exclude dan include membawa tarif, untuk bayar sekarang maupun Bayar Nanti", () => {
    const ex = buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, taxType: "exclude", taxRate: 11 });
    expect(ex).toMatchObject({ paymentTerms: "cod", taxType: "exclude", taxRate: 11 });
    const inc = buildPosOrderPayload({ partyId: 7, mode: "pay_later", dueDate: "2026-09-30", clientRef, items, taxType: "include", taxRate: 11 });
    expect(inc).toMatchObject({ paymentTerms: "net", taxType: "include", taxRate: 11 });
  });

  it("menolak tarif nol atau di atas 100 bila kena pajak", () => {
    expect(() => buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, taxType: "exclude", taxRate: 0 })).toThrow("Tarif PPN");
    expect(() => buildPosOrderPayload({ partyId: 0, mode: "pay_now", clientRef, items, taxType: "include", taxRate: 101 })).toThrow("Tarif PPN");
  });
});
