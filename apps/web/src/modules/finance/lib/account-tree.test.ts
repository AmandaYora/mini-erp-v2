import { describe, expect, it } from "vitest";
import { buildAccountTree, filterAccounts } from "@/modules/finance/lib/account-tree";
import type { Account } from "@/modules/finance/types";

const acc = (id: number, code: string, name: string, type: string, status = "active"): Account => ({
  id,
  code,
  name,
  type,
  isCash: false,
  status,
});

const accounts = [
  acc(1, "1100", "Piutang Usaha", "aset"),
  acc(2, "1000", "Kas Tunai", "aset"),
  acc(3, "2000", "Hutang Usaha", "kewajiban"),
  acc(4, "5100", "Beban Listrik", "beban", "archived"),
];

describe("pohon akun", () => {
  it("mengelompokkan menurut tipe dan mengurutkan kode secara numerik", () => {
    const rows = buildAccountTree(accounts, new Set());
    expect(rows.map((r) => (r.kind === "group" ? r.type : r.account.code))).toEqual([
      "aset",
      "1000",
      "1100",
      "kewajiban",
      "2000",
      "beban",
      "5100",
    ]);
  });

  it("golongan yang dilipat menyembunyikan akunnya tetapi tetap menampilkan jumlah", () => {
    const rows = buildAccountTree(accounts, new Set(["aset"]));
    const group = rows[0];
    expect(group.kind === "group" && group.expanded).toBe(false);
    expect(group.kind === "group" && group.count).toBe(2);
    expect(rows.some((r) => r.kind === "account" && r.account.code === "1000")).toBe(false);
  });

  it("forceOpen membuka semua golongan walau dilipat", () => {
    expect(buildAccountTree(accounts, new Set(["aset"]), true).some((r) => r.kind === "account" && r.account.code === "1000")).toBe(true);
  });

  it("daftar null tidak crash", () => {
    expect(buildAccountTree(null, new Set())).toEqual([]);
  });

  it("filter menurut kode/nama dan status", () => {
    expect(filterAccounts(accounts, "kas", "").map((a) => a.code)).toEqual(["1000"]);
    expect(filterAccounts(accounts, "", "archived").map((a) => a.code)).toEqual(["5100"]);
    expect(filterAccounts(accounts, "2000", "active").map((a) => a.code)).toEqual(["2000"]);
  });
});
