import { describe, expect, it } from "vitest";
import {
  activeBankAccounts,
  bankAccountByAccountId,
  bankAccountLabel,
  defaultBankAccountId,
  initialPaymentAccount,
  methodUsesBankAccount,
} from "@/modules/finance/lib/bank-accounts";
import type { BankAccount } from "@/modules/finance/types";

const base: BankAccount = {
  id: 1, accountId: 11, accountCode: "1011", accountName: "Bank BCA", bankName: "BCA", accountNumber: "123",
  accountHolder: "PT Uji", showOnDocuments: true, isDefault: true, sortOrder: 0, status: "active",
};
const mandiri: BankAccount = { ...base, id: 2, accountId: 12, bankName: "Mandiri", accountNumber: "456", isDefault: false };
const archived: BankAccount = { ...base, id: 3, accountId: 13, bankName: "BRI", accountNumber: "789", isDefault: false, status: "archived" };
const list = [base, mandiri, archived];

describe("rekening perusahaan (ADR-0026)", () => {
  it("label satu baris", () => {
    expect(bankAccountLabel(base)).toBe("BCA 123 a.n. PT Uji");
    expect(bankAccountLabel({ ...base, accountHolder: " " })).toBe("BCA 123");
  });

  it("hanya rekening aktif yang bisa dipilih; default dari yang aktif", () => {
    expect(activeBankAccounts(list).map((b) => b.id)).toEqual([1, 2]);
    expect(defaultBankAccountId(list)).toBe(11);
    expect(defaultBankAccountId([{ ...base, status: "archived" }])).toBeNull();
  });

  it("cari dari akun buku besar, termasuk yang diarsipkan (label dokumen lama)", () => {
    expect(bankAccountByAccountId(list, 13)?.bankName).toBe("BRI");
    expect(bankAccountByAccountId(list, 0)).toBeUndefined();
    expect(bankAccountByAccountId(list, null)).toBeUndefined();
  });

  it("transfer dan cek membawa rekening; tunai tidak", () => {
    expect(methodUsesBankAccount("transfer")).toBe(true);
    expect(methodUsesBankAccount("check")).toBe(true);
    expect(methodUsesBankAccount("cash")).toBe(false);
  });

  it("nilai awal pembayaran: rekening tagihan bila aktif, lalu default", () => {
    expect(initialPaymentAccount(list, 12)).toBe(12);
    expect(initialPaymentAccount(list, 13)).toBe(11); // rekening tagihan diarsipkan → default
    expect(initialPaymentAccount(list, null)).toBe(11);
    expect(initialPaymentAccount([], null)).toBeNull();
  });
});
