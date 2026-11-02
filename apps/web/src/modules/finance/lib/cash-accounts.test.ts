import { beforeEach, describe, expect, it } from "vitest";
import type { CashAccountOption } from "@/modules/finance/types";
import {
  cashAccountChoiceShown,
  cashAccountForPayload,
  initialCashAccount,
  rememberCashAccount,
} from "@/modules/finance/lib/cash-accounts";

const kas = (accountId: number, code: string, over: Partial<CashAccountOption> = {}): CashAccountOption => ({
  accountId, code, name: code, isDefault: false, status: "active", ...over,
});
const tunai = kas(1, "1000", { name: "Kas Tunai", isDefault: true });
const besar = kas(2, "1002", { name: "Kas Besar" });

beforeEach(() => {
  try {
    localStorage.clear();
  } catch {
    // tanpa storage
  }
});

describe("akun kas pembayaran tunai", () => {
  it("pemilih hanya tampil bila ada lebih dari satu kas aktif", () => {
    expect(cashAccountChoiceShown([tunai])).toBe(false);
    expect(cashAccountChoiceShown([tunai, { ...besar, status: "archived" }])).toBe(false);
    expect(cashAccountChoiceShown([tunai, besar])).toBe(true);
  });

  it("nilai awal: kas diminta → kas terakhir per arah → Kas Tunai", () => {
    expect(initialCashAccount([tunai, besar], "out")).toBe(1);
    rememberCashAccount("out", 2);
    expect(initialCashAccount([tunai, besar], "out")).toBe(2);
    expect(initialCashAccount([tunai, besar], "in")).toBe(1);
    expect(initialCashAccount([tunai, besar], "out", 1)).toBe(1);
    // kas terakhir sudah diarsipkan → kembali ke default
    expect(initialCashAccount([tunai, { ...besar, status: "archived" }], "out")).toBe(1);
  });

  it("payload hanya membawa kas bila pemilih tampil", () => {
    expect(cashAccountForPayload([tunai], 1)).toBeUndefined();
    expect(cashAccountForPayload([tunai, besar], 2)).toBe(2);
    expect(cashAccountForPayload([tunai, besar], null)).toBeUndefined();
  });
});
