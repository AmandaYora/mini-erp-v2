import { describe, expect, it } from "vitest";
import { parseFilters } from "@/shared/hooks/use-url-filters";
import {
  SALES_LIST_DEFAULTS,
  salesListFilterSchema,
  toSalesFilterParams,
} from "@/modules/sales/schemas/sales-list-filter.schema";
import {
  buildSalesFilterQuery,
  buildSalesListQuery,
} from "@/modules/sales/services/sales.service";
import {
  PURCHASE_LIST_DEFAULTS,
  purchaseListFilterSchema,
  toPurchaseFilterParams,
} from "@/modules/purchasing/schemas/purchase-list-filter.schema";
import {
  buildPurchaseFilterQuery,
  buildPurchaseListQuery,
} from "@/modules/purchasing/services/purchase-order.service";

describe("filter Order Jual", () => {
  it("memparse URL: status CSV, termin, pihak, tanggal, urutan", () => {
    const f = parseFilters(
      salesListFilterSchema,
      SALES_LIST_DEFAULTS,
      new URLSearchParams(
        "search=budi&status=confirmed,completed&paymentTerms=net&paymentStatus=overdue&channel=pos&partyId=4&from=2026-10-01&to=2026-10-31&sort=total&dir=asc&page=2",
      ),
    );
    expect(f).toEqual({
      search: "budi",
      status: "confirmed,completed",
      paymentTerms: "net",
      paymentStatus: "overdue",
      channel: "pos",
      partyId: 4,
      from: "2026-10-01",
      to: "2026-10-31",
      sort: "total",
      dir: "asc",
      page: 2,
    });
  });

  it("nilai tidak sah (status, termin, tanggal, sort) jatuh ke default", () => {
    const f = parseFilters(
      salesListFilterSchema,
      SALES_LIST_DEFAULTS,
      new URLSearchParams("status=confirmed,bogus&paymentTerms=kredit&from=1-10-2026&sort=hack&dir=up"),
    );
    expect(f).toEqual(SALES_LIST_DEFAULTS);
  });

  it("query list dan ekspor memakai SATU sumber filter yang sama", () => {
    const filters = {
      ...SALES_LIST_DEFAULTS,
      search: " budi ",
      status: "confirmed,completed",
      paymentTerms: "cod" as const,
      paymentStatus: "partial" as const,
      channel: "regular" as const,
      partyId: 4,
      from: "2026-10-01",
      to: "2026-10-31",
      sort: "date" as const,
      dir: "asc" as const,
    };
    const params = toSalesFilterParams(filters);
    const exportQuery = buildSalesFilterQuery(params);
    expect(exportQuery).toEqual({
      search: "budi",
      status: "confirmed,completed",
      channel: "regular",
      paymentTerms: "cod",
      paymentStatus: "partial",
      partyId: 4,
      from: "2026-10-01",
      to: "2026-10-31",
      sort: "date",
      dir: "asc",
    });
    expect(buildSalesListQuery({ ...params, page: 3, limit: 20 })).toEqual({
      ...exportQuery,
      page: 3,
      limit: 20,
    });
  });

  it("urutan bawaan (id desc) dan filter kosong tidak dikirim", () => {
    const params = toSalesFilterParams(SALES_LIST_DEFAULTS);
    expect(buildSalesFilterQuery(params)).toEqual({});
    expect(buildSalesListQuery({ ...params, page: 1, limit: 20 })).toEqual({ page: 1, limit: 20 });
  });
});

describe("filter Order Beli", () => {
  it("tidak punya channel dan memakai query yang sama untuk list dan ekspor", () => {
    const f = parseFilters(
      purchaseListFilterSchema,
      PURCHASE_LIST_DEFAULTS,
      new URLSearchParams("status=draft,confirmed&paymentTerms=prepaid&partyId=9&sort=number&dir=desc"),
    );
    const params = toPurchaseFilterParams(f);
    const exportQuery = buildPurchaseFilterQuery(params);
    expect(exportQuery).toEqual({
      status: "draft,confirmed",
      paymentTerms: "prepaid",
      partyId: 9,
      sort: "number",
      dir: "desc",
    });
    expect(buildPurchaseListQuery({ ...params, page: 1, limit: 20 })).toEqual({
      ...exportQuery,
      page: 1,
      limit: 20,
    });
    expect("channel" in exportQuery).toBe(false);
  });
});
