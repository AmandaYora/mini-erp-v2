import { describe, expect, it } from "vitest";
import {
  editAction,
  headerEditDirty,
  headerEditInitial,
  headerEditPayload,
  shortCloseSummary,
} from "@/modules/sales/lib/order-correction";
import { headerEditSchema, requiredReasonSchema } from "@/modules/sales/schemas/order-correction.schema";
import type { SalesOrder, ShortClosePlan } from "@/modules/sales/types";

function order(over: Partial<SalesOrder> = {}): SalesOrder {
  return {
    id: 9,
    number: "SO-0009",
    branchId: 1,
    partyId: 3,
    partyName: "Budi",
    channel: "pos",
    memberCode: "",
    orderDate: "2026-10-01 09:30:00",
    dueDate: "2026-10-31 00:00:00",
    paymentTerms: "net",
    taxType: "exclude",
    taxRate: 11,
    subtotal: 30000,
    discountTotal: 0,
    taxTotal: 3300,
    grandTotal: 33300,
    status: "confirmed",
    notes: "lama",
    shipToAddressId: 4,
    shipToLabel: "Gudang",
    shipToRecipient: "Pak Joko",
    shipToPhone: "0812",
    shipToAddress: "Jl. Buku 1",
    taxInvoiceNumber: "",
    taxInvoiceDate: "",
    assignedUserId: 0,
    items: [
      {
        id: 1,
        productId: 5,
        variantId: 6,
        productCode: "P5",
        productName: "Semen",
        uom: "sak",
        uomFactor: 1,
        qty: 2,
        qtyBase: 2,
        unitPrice: 15000,
        discountPct: 0,
        discountNominal: 0,
        lineTotal: 33300,
        stockTracked: true,
        itemType: "barang",
        variantName: "",
        variantCode: "",
        baseUom: "sak",
        pricingSource: "manual",
        basePrice: 16000,
        discountReason: "langganan",
        note: "",
        taxable: true,
        taxBase: 30000,
        taxAmount: 3300,
      },
    ],
    ...over,
  };
}

describe("editAction", () => {
  it("draf selalu form; terkonfirmasi mengikuti editMode server; final tidak bisa", () => {
    expect(editAction({ status: "draft" })).toBe("form");
    expect(editAction({ status: "confirmed", editMode: "full" })).toBe("form");
    expect(editAction({ status: "confirmed", editMode: "header" })).toBe("header");
    expect(editAction({ status: "confirmed" })).toBeNull();
    expect(editAction({ status: "completed", editMode: "none" })).toBeNull();
    expect(editAction({ status: "cancelled" })).toBeNull();
  });
});

describe("headerEditPayload", () => {
  it("mengirim kolom ekonomi persis seperti tersimpan dan menandai headerOnly", () => {
    const o = order();
    const initial = headerEditInitial(o);
    expect(initial.dueDate).toBe("2026-10-31");
    const body = headerEditPayload(o, initial, { ...initial, notes: "baru", reason: "minta pelanggan" });
    expect(body).toMatchObject({
      partyId: 3,
      channel: "pos",
      orderDate: "2026-10-01",
      paymentTerms: "net",
      taxType: "exclude",
      taxRate: 11,
      notes: "baru",
      dueDate: "2026-10-31",
      shipToAddressId: 4,
      headerOnly: true,
      reason: "minta pelanggan",
    });
    expect(body.shipToAddress).toBeUndefined();
    expect(body.items).toEqual([
      {
        productId: 5,
        variantId: 6,
        uom: "sak",
        qty: 2,
        discountPct: 0,
        discountNominal: 0,
        taxable: true,
        unitPrice: 15000,
        discountReason: "langganan",
      },
    ]);
  });

  it("alamat buku yang disunting menjadi alamat sekali pakai", () => {
    const o = order();
    const initial = headerEditInitial(o);
    const body = headerEditPayload(o, initial, { ...initial, shipToAddress: "Jl. Baru 2" });
    expect(body.shipToAddressId).toBe(0);
    expect(body.shipToAddress).toBe("Jl. Baru 2");
    expect(body.shipToRecipient).toBe("Pak Joko");
    expect(body.reason).toBeUndefined();
  });

  it("dirty hanya bila kolom selain alasan berubah", () => {
    const initial = headerEditInitial(order());
    expect(headerEditDirty(initial, { ...initial, reason: "x" })).toBe(false);
    expect(headerEditDirty(initial, { ...initial, assignedUserId: 8 })).toBe(true);
  });
});

describe("skema koreksi", () => {
  it("alasan wajib, maksimal 255", () => {
    expect(requiredReasonSchema.safeParse("  ").success).toBe(false);
    expect(requiredReasonSchema.safeParse("x".repeat(256)).success).toBe(false);
    expect(requiredReasonSchema.safeParse(" stok habis ").data).toBe("stok habis");
  });

  it("termin net wajib jatuh tempo; rincian alamat tanpa alamat ditolak", () => {
    const base = headerEditInitial(order());
    const noDue = headerEditSchema.safeParse({ ...base, dueDate: "" });
    expect(noDue.success).toBe(false);
    expect(noDue.error?.issues[0].path).toEqual(["dueDate"]);
    const noAddr = headerEditSchema.safeParse({ ...base, shipToAddress: "" });
    expect(noAddr.error?.issues.map((i) => i.path.join("."))).toContain("shipToAddress");
    expect(headerEditSchema.safeParse({ ...base, paymentTerms: "cod", dueDate: "" }).success).toBe(true);
  });
});

describe("shortCloseSummary", () => {
  it("menghitung baris terpotong, terhapus, dan pengurangan total", () => {
    const plan: ShortClosePlan = {
      orderId: 9,
      number: "SO-0009",
      lines: [
        { itemId: 1, productCode: "A", productName: "A", variantName: "", uom: "pcs", orderedQty: 10, deliveredQty: 4, oldLineTotal: 100, newLineTotal: 40, removed: false },
        { itemId: 2, productCode: "B", productName: "B", variantName: "", uom: "pcs", orderedQty: 5, deliveredQty: 0, oldLineTotal: 50, newLineTotal: 0, removed: true },
        { itemId: 3, productCode: "C", productName: "C", variantName: "", uom: "pcs", orderedQty: 1, deliveredQty: 1, oldLineTotal: 10, newLineTotal: 10, removed: false },
      ],
      oldGrandTotal: 160,
      newSubtotal: 50,
      newDiscountTotal: 0,
      newTaxTotal: 0,
      newGrandTotal: 50,
      paidAmount: 0,
      outstandingAfter: 50,
      refundDueAfter: 0,
    };
    expect(shortCloseSummary(plan)).toEqual({ cut: 1, removed: 1, reduction: 110 });
  });
});
