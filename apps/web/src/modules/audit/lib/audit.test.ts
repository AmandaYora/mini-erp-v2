import { describe, expect, it } from "vitest";
import { actorLabel, branchLabel, entityLabel, isSystemActor } from "@/modules/audit/lib/audit";

describe("label audit", () => {
  it("aktor: sistem, nama, atau nomor pengguna", () => {
    expect(isSystemActor({ actorType: "system", actorId: 5 })).toBe(true);
    expect(isSystemActor({ actorType: "user", actorId: 0 })).toBe(true);
    expect(actorLabel({ actorType: "user", actorId: 9, actorName: "Siti" })).toBe("Siti");
    expect(actorLabel({ actorType: "user", actorId: 9 })).toBe("Pengguna #9");
    expect(actorLabel({ actorType: "system", actorId: 0 })).toBe("Sistem");
  });

  it("cabang: Global, nama, atau nomor", () => {
    expect(branchLabel({ branchId: 0 })).toBe("Global");
    expect(branchLabel({ branchId: 2, branchName: "Cabang Dua" })).toBe("Cabang Dua");
    expect(branchLabel({ branchId: 2 })).toBe("#2");
  });

  it("entitas dengan/tanpa id", () => {
    expect(entityLabel({ entity: "product", entityId: 5 })).toBe("Produk #5");
    expect(entityLabel({ entity: "product", entityId: 0 })).toBe("Produk");
    expect(entityLabel({ entity: "entitas_lama", entityId: 3 })).toBe("entitas_lama #3");
    expect(entityLabel({ entity: "", entityId: 0 })).toBe("-");
  });
});
