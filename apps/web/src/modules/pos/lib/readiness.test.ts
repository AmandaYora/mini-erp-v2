import { describe, expect, it } from "vitest";
import { posReadiness } from "@/modules/pos/lib/readiness";

const all = () => true;
const only = (...perms: string[]) => (p: string) => perms.includes(p);

describe("posReadiness", () => {
  it("semua izin: siap, tanpa catatan", () => {
    const r = posReadiness(all);
    expect(r.canCheckout).toBe(true);
    expect(r.notices).toEqual([]);
    expect(r.missingCore).toEqual([]);
  });

  it("izin inti kurang memblokir checkout", () => {
    const r = posReadiness(only("sales.create", "sales.update"));
    expect(r.canCheckout).toBe(false);
    expect(r.missingCore.map((m) => m.perm)).toEqual(["delivery.create"]);
  });

  it("izin opsional kurang hanya memberi catatan", () => {
    const r = posReadiness(only("sales.create", "sales.update", "delivery.create"));
    expect(r.canCheckout).toBe(true);
    expect(r.notices.map((n) => n.perm)).toEqual([
      "customers.view",
      "products.view",
      "stock.view",
      "pricing.quote",
      "member_types.view",
    ]);
  });

  it("member_types.manage cukup menggantikan view", () => {
    const r = posReadiness(only("sales.create", "sales.update", "delivery.create", "member_types.manage"));
    expect(r.notices.some((n) => n.perm === "member_types.view")).toBe(false);
  });
});
