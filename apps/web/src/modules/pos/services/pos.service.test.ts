import { beforeEach, describe, expect, it, vi } from "vitest";

const apiPage = vi.fn();
vi.mock("@/shared/services/http-client", () => ({
  apiGet: vi.fn(),
  apiPage: (...a: unknown[]) => apiPage(...a),
  apiPost: vi.fn(),
  toApiError: (e: unknown) => e,
}));

import { findPosPayment, posService } from "@/modules/pos/services/pos.service";

beforeEach(() => {
  apiPage.mockReset();
});

describe("posService.products (katalog berhalaman)", () => {
  it("Semua: tanpa categoryId, status aktif, halaman & limit diteruskan, signal ikut", async () => {
    apiPage.mockResolvedValue({ items: [], meta: { page: 2, limit: 48, total: 0, totalPages: 0 } });
    const ctrl = new AbortController();
    await posService.products({ search: "  baut ", categoryId: null, page: 2, limit: 48 }, ctrl.signal);
    const [url, params, signal] = apiPage.mock.calls[0];
    expect(url).toBe("/api/v1/products");
    expect(params).toEqual({ search: "baut", categoryId: undefined, status: "active", page: 2, limit: 48 });
    expect(signal).toBe(ctrl.signal);
  });

  it("per kategori: categoryId terkirim, kata kunci kosong tidak dikirim", async () => {
    apiPage.mockResolvedValue({ items: [], meta: {} });
    await posService.products({ search: " ", categoryId: 7, page: 1, limit: 48 });
    expect(apiPage.mock.calls[0][1]).toEqual({ search: undefined, categoryId: 7, status: "active", page: 1, limit: 48 });
  });
});

describe("findPosPayment", () => {
  it("menyaring di server per order (orderType=sales&orderId), bukan 20 pembayaran terbaru", async () => {
    apiPage.mockResolvedValue({
      items: [{ id: 9, amountTendered: 100000, change: 10000, allocations: [{ orderId: 42 }] }],
      meta: {},
    });
    const p = await findPosPayment(0, 42);
    expect(apiPage).toHaveBeenCalledTimes(1);
    const [url, params] = apiPage.mock.calls[0];
    expect(url).toBe("/api/v1/payments");
    expect(params).toMatchObject({ orderType: "sales", orderId: 42, status: "active" });
    expect(params.partyId).toBeUndefined();
    expect(p?.id).toBe(9);
  });

  it("membawa partyId pelanggan dan mengembalikan null bila tak ada alokasi ke order itu", async () => {
    apiPage.mockResolvedValue({ items: [{ id: 3, allocations: [{ orderId: 7 }] }], meta: {} });
    expect(await findPosPayment(5, 42)).toBeNull();
    expect(apiPage.mock.calls[0][1]).toMatchObject({ partyId: 5, orderId: 42 });
  });
});
