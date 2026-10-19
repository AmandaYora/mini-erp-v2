import { beforeEach, describe, expect, it, vi } from "vitest";

const get = vi.fn();
vi.mock("@/shared/services/http-client", async (orig) => {
  const actual = await orig<typeof import("@/shared/services/http-client")>();
  return { ...actual, httpClient: { get: (...a: unknown[]) => get(...a) } };
});

import { financeReportsService } from "@/modules/finance/services/reports.service";

beforeEach(() => {
  get.mockReset();
  get.mockResolvedValue({ data: new Blob(["x"]), headers: {} });
  // jsdom has no object URLs.
  URL.createObjectURL = vi.fn(() => "blob:x");
  URL.revokeObjectURL = vi.fn();
});

describe("paket pajak lintas cabang", () => {
  it("bulan: meneruskan ?branch=all, dan tidak mengirim branch bila kosong", async () => {
    await financeReportsService.downloadTaxPackage(2026, 10, "actual", "all");
    expect(get.mock.calls[0][1].params).toMatchObject({ year: 2026, month: 10, variant: "actual", branch: "all" });
    await financeReportsService.downloadTaxPackage(2026, 10, "capped", "");
    expect(get.mock.calls[1][1].params.branch).toBeUndefined();
  });

  it("rentang bebas: meneruskan ?branch=<id>", async () => {
    await financeReportsService.downloadTaxPackageRange("2026-10-01", "2026-10-31", "capped", "7");
    expect(get.mock.calls[0][1].params).toMatchObject({ from: "2026-10-01", to: "2026-10-31", variant: "capped", branch: "7" });
  });
});
