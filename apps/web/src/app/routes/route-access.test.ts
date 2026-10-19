import { describe, expect, it } from "vitest";
import { isPathPermitted, requiredPermForPath } from "@/app/routes/route-access";

describe("route-access", () => {
  it("menu persis memakai izin menunya", () => {
    expect(requiredPermForPath("/stock/transfers")).toBe("stock.transfer");
    expect(requiredPermForPath("/finance/expenses")).toBe("finance.expense.view");
  });

  it("rute detail mewarisi menu induk terpanjang", () => {
    expect(requiredPermForPath("/finance/journals/12")).toBe("finance.journal.view");
  });

  it("path tak dikenal diizinkan", () => {
    expect(requiredPermForPath("/tidak-ada")).toBeNull();
    expect(isPathPermitted("/tidak-ada", () => false)).toBe(true);
  });

  it("izin dicek lewat can()", () => {
    const can = (p: string) => p === "stock.adjust";
    expect(isPathPermitted("/stock/damaged", can)).toBe(true);
    expect(isPathPermitted("/stock/transfers", can)).toBe(false);
  });

  it("kode izin lama tidak lagi dipakai di registry", () => {
    expect(requiredPermForPath("/stock/opening")).toBe("stock.adjust");
  });
});
