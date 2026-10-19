import { describe, expect, it } from "vitest";
import { parseFilters } from "@/shared/hooks/use-url-filters";
import {
  PAYMENT_LIST_DEFAULTS,
  paymentListFilterSchema,
  toPaymentFilterParams,
} from "@/modules/payment/schemas/payment-list-filter.schema";
import { buildPaymentListQuery } from "@/modules/payment/services/payment.service";
import {
  DELIVERY_LIST_DEFAULTS,
  deliveryListFilterSchema,
  toDeliveryFilterParams,
} from "@/modules/delivery/schemas/delivery-list-filter.schema";
import { buildDeliveryListQuery } from "@/modules/delivery/services/delivery.service";
import {
  SALESRETURN_LIST_DEFAULTS,
  salesreturnListFilterSchema,
  toSalesReturnFilterParams,
} from "@/modules/salesreturn/schemas/sales-return-list-filter.schema";
import { buildSalesReturnListQuery } from "@/modules/salesreturn/services/sales-return.service";
import {
  PURCHASERETURN_LIST_DEFAULTS,
  purchasereturnListFilterSchema,
  toPurchaseReturnFilterParams,
} from "@/modules/purchasereturn/schemas/purchase-return-list-filter.schema";
import { buildPurchaseReturnListQuery } from "@/modules/purchasereturn/services/purchase-return.service";
import {
  AUDIT_LIST_DEFAULTS,
  auditListFilterSchema,
} from "@/modules/audit/schemas/audit-list-filter.schema";
import { buildAuditListQuery } from "@/modules/audit/services/audit.service";
import {
  TRANSFER_LIST_DEFAULTS,
  transferListFilterSchema,
} from "@/modules/stock/schemas/transfer-list-filter.schema";

const q = (s: string) => new URLSearchParams(s);

describe("filter Pembayaran", () => {
  it("memetakan semua filter ke query server", () => {
    const f = parseFilters(
      paymentListFilterSchema,
      PAYMENT_LIST_DEFAULTS,
      q("search=budi&method=transfer&direction=in&status=active&partyId=3&orderType=sales&orderId=12&from=2026-10-01&to=2026-10-31&page=2"),
    );
    expect(buildPaymentListQuery({ ...toPaymentFilterParams(f), page: f.page, limit: 20 })).toEqual({
      search: "budi",
      method: "transfer",
      direction: "in",
      status: "active",
      partyId: 3,
      orderType: "sales",
      orderId: 12,
      from: "2026-10-01",
      to: "2026-10-31",
      page: 2,
      limit: 20,
    });
  });

  it("orderType tanpa orderId (dan sebaliknya) tidak dikirim — server menolak orderId tanpa jenis", () => {
    const onlyType = parseFilters(paymentListFilterSchema, PAYMENT_LIST_DEFAULTS, q("orderType=purchase"));
    expect(toPaymentFilterParams(onlyType)).toMatchObject({ orderType: "", orderId: 0 });
    const onlyId = parseFilters(paymentListFilterSchema, PAYMENT_LIST_DEFAULTS, q("orderId=5"));
    expect(toPaymentFilterParams(onlyId)).toMatchObject({ orderType: "", orderId: 0 });
  });
});

describe("filter Pengiriman", () => {
  it("status CSV, salesOrderId, salesReturnId, documentKind, tanggal", () => {
    const f = parseFilters(
      deliveryListFilterSchema,
      DELIVERY_LIST_DEFAULTS,
      q("search=sopir&status=draft,confirmed&salesOrderId=5&salesReturnId=2&documentKind=replacement&from=2026-10-01&to=2026-10-02"),
    );
    expect(buildDeliveryListQuery({ ...toDeliveryFilterParams(f), page: 1, limit: 20 })).toEqual({
      search: "sopir",
      status: "draft,confirmed",
      salesOrderId: 5,
      salesReturnId: 2,
      documentKind: "replacement",
      from: "2026-10-01",
      to: "2026-10-02",
      page: 1,
      limit: 20,
    });
  });
});

describe("filter Retur", () => {
  it("retur jual: search, status CSV, salesOrderId, tanggal", () => {
    const f = parseFilters(
      salesreturnListFilterSchema,
      SALESRETURN_LIST_DEFAULTS,
      q("search=RJ&status=confirmed&salesOrderId=7&from=2026-10-01"),
    );
    expect(buildSalesReturnListQuery({ ...toSalesReturnFilterParams(f), page: 1, limit: 20 })).toEqual({
      search: "RJ",
      status: "confirmed",
      salesOrderId: 7,
      from: "2026-10-01",
      page: 1,
      limit: 20,
    });
  });

  it("retur beli: purchaseOrderId", () => {
    const f = parseFilters(
      purchasereturnListFilterSchema,
      PURCHASERETURN_LIST_DEFAULTS,
      q("status=draft,cancelled&purchaseOrderId=4&to=2026-10-31"),
    );
    expect(buildPurchaseReturnListQuery({ ...toPurchaseReturnFilterParams(f), page: 2, limit: 20 })).toEqual({
      status: "draft,cancelled",
      purchaseOrderId: 4,
      to: "2026-10-31",
      page: 2,
      limit: 20,
    });
  });
});

describe("filter Transfer", () => {
  it("status tunggal, arah (ST-06) dan page", () => {
    expect(parseFilters(transferListFilterSchema, TRANSFER_LIST_DEFAULTS, q("status=dispatched&page=2"))).toEqual({
      status: "dispatched",
      direction: "",
      page: 2,
    });
    expect(parseFilters(transferListFilterSchema, TRANSFER_LIST_DEFAULTS, q("status=nope"))).toEqual(
      TRANSFER_LIST_DEFAULTS,
    );
  });
});

describe("filter Audit", () => {
  it("branchId menerima all atau angka, menolak lainnya", () => {
    expect(parseFilters(auditListFilterSchema, AUDIT_LIST_DEFAULTS, q("branchId=all")).branchId).toBe("all");
    expect(parseFilters(auditListFilterSchema, AUDIT_LIST_DEFAULTS, q("branchId=12")).branchId).toBe("12");
    expect(parseFilters(auditListFilterSchema, AUDIT_LIST_DEFAULTS, q("branchId=semua")).branchId).toBe("");
  });

  it("query: branchId|all, awalan aksi, entity, entityId, actorId, from/to; kosong dibuang", () => {
    const f = parseFilters(
      auditListFilterSchema,
      AUDIT_LIST_DEFAULTS,
      q("branchId=all&action=product.&entity=product&entityId=5&actorId=2&from=2026-10-01&to=2026-10-09"),
    );
    expect(buildAuditListQuery({ ...f, limit: 20 })).toEqual({
      branchId: "all",
      action: "product.",
      entity: "product",
      entityId: 5,
      actorId: 2,
      from: "2026-10-01",
      to: "2026-10-09",
      page: 1,
      limit: 20,
    });
    expect(buildAuditListQuery({ ...AUDIT_LIST_DEFAULTS, limit: 20 })).toEqual({ page: 1, limit: 20 });
  });
});
