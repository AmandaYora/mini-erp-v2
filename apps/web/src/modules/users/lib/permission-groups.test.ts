import { describe, expect, it } from "vitest";
import { groupPermissions } from "./permission-groups";

describe("groupPermissions", () => {
  it("mengelompokkan per prefiks kunci dan mengurutkan modul", () => {
    const groups = groupPermissions([
      { code: "sales.create", name: "Buat order" },
      { code: "audit.view", name: "Lihat audit" },
      { code: "sales.update", name: "Ubah order" },
      { code: "roles", name: "Tanpa titik" },
    ]);
    expect(groups.map((g) => g.module)).toEqual(["audit", "lainnya", "sales"]);
    expect(groups[2].items.map((p) => p.code)).toEqual(["sales.create", "sales.update"]);
  });

  it("katalog kosong menghasilkan daftar kosong", () => {
    expect(groupPermissions([])).toEqual([]);
  });
});
