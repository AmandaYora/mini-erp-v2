import { beforeEach, describe, expect, it, vi } from "vitest";

const apiGet = vi.fn();
const apiPage = vi.fn();
const apiUploadFull = vi.fn();
vi.mock("@/shared/services/http-client", () => ({
  apiGet: (...a: unknown[]) => apiGet(...a),
  apiPage: (...a: unknown[]) => apiPage(...a),
  apiUploadFull: (...a: unknown[]) => apiUploadFull(...a),
  apiDeleteFull: vi.fn(),
  apiPostFull: vi.fn(),
  apiPutFull: vi.fn(),
  httpClient: {},
}));

import { productsService } from "@/modules/products/services/products.service";

beforeEach(() => {
  apiGet.mockReset().mockResolvedValue([]);
  apiPage.mockReset().mockResolvedValue({ items: [], meta: {} });
  apiUploadFull.mockReset().mockResolvedValue({ message: "ok", data: {} });
});

describe("productsService", () => {
  it("list: 'all' (Semua status) dan kosong tidak mengirim filter status; 'archived' dikirim", async () => {
    await productsService.list({ status: "all" });
    expect(apiPage.mock.calls[0][1]).not.toHaveProperty("status");
    await productsService.list({ status: "" });
    expect(apiPage.mock.calls[1][1]).not.toHaveProperty("status");
    await productsService.list({ status: "archived" });
    expect(apiPage.mock.calls[2][1]).toMatchObject({ status: "archived" });
  });

  it("searchOptions: productId dan preferStock hanya dikirim bila dipakai", async () => {
    await productsService.searchOptions({ q: " ab " });
    expect(apiGet.mock.calls[0][1]).toMatchObject({ q: "ab", productId: undefined, preferStock: undefined });
    await productsService.searchOptions({ productId: 7, preferStock: true });
    expect(apiGet.mock.calls[1][1]).toMatchObject({ productId: 7, preferStock: true });
  });

  it("scanLookup memangkas kode dan memanggil /products/scan", async () => {
    await productsService.scanLookup("  8991234 ");
    expect(apiGet).toHaveBeenCalledWith("/api/v1/products/scan", { code: "8991234" });
  });

  it("uploadMedia mengirim is_primary dan sort_order hanya bila diberikan", async () => {
    const file = new File(["x"], "a.jpg", { type: "image/jpeg" });
    await productsService.uploadMedia(5, file);
    let form = apiUploadFull.mock.calls[0][1] as FormData;
    expect(form.has("is_primary")).toBe(false);
    expect(form.has("sort_order")).toBe(false);

    await productsService.uploadMedia(5, file, { isPrimary: true, sortOrder: 3 });
    expect(apiUploadFull.mock.calls[1][0]).toBe("/api/v1/products/5/media");
    form = apiUploadFull.mock.calls[1][1] as FormData;
    expect(form.get("is_primary")).toBe("true");
    expect(form.get("sort_order")).toBe("3");
  });
});
