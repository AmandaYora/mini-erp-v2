import { beforeEach, describe, expect, it, vi } from "vitest";

const apiPage = vi.fn();
vi.mock("@/shared/services/http-client", () => ({
  apiGet: vi.fn(),
  apiPage: (...a: unknown[]) => apiPage(...a),
  apiPost: vi.fn(),
  toApiError: (e: unknown) => e,
}));

import { findPosPayment } from "@/modules/pos/services/pos.service";

beforeEach(() => {
  apiPage.mockReset();
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
