import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account, BankAccount } from "@/modules/finance/types";

const bankAccounts = vi.fn();
const createBankAccount = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    bankAccounts: (...a: unknown[]) => bankAccounts(...a),
    createBankAccount: (...a: unknown[]) => createBankAccount(...a),
    updateBankAccount: vi.fn(),
    archiveBankAccount: vi.fn(),
    restoreBankAccount: vi.fn(),
    bankAccountOptions: vi.fn().mockResolvedValue([]),
  },
}));

import { BankAccountsPanel } from "@/modules/finance/components/BankAccountsPanel";

const accounts: Account[] = [
  { id: 1, code: "1000", name: "Kas Tunai", type: "aset", isCash: true, status: "active" },
  { id: 2, code: "1010", name: "Rekening Bank", type: "aset", isCash: true, status: "active" },
  { id: 3, code: "4000", name: "Penjualan", type: "pendapatan", isCash: false, status: "active" },
];
const bca: BankAccount = {
  id: 1, accountId: 2, accountCode: "1010", accountName: "Rekening Bank", bankName: "BCA", accountNumber: "123",
  accountHolder: "PT Uji", showOnDocuments: true, isDefault: true, sortOrder: 0, status: "active",
};

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => cleanup());

describe("panel rekening perusahaan (ADR-0026)", () => {
  it("menampilkan rekening dengan akun buku besar dan penanda default", async () => {
    bankAccounts.mockResolvedValue([bca]);
    render(<BankAccountsPanel accounts={accounts} cashAccountId={1} canManage onChartChanged={vi.fn()} />);
    expect(await screen.findByText("BCA 123")).toBeInTheDocument();
    expect(screen.getByText("Default")).toBeInTheDocument();
    expect(screen.getByText(/wajib memilih rekening/)).toBeInTheDocument();
  });

  it("tambah rekening dengan akun kas/bank baru: kode akun berikutnya terisi, payload lengkap", async () => {
    bankAccounts.mockResolvedValue([]);
    createBankAccount.mockResolvedValue({ message: "Rekening ditambahkan", data: { ...bca, accountCode: "1011" } });
    const onChartChanged = vi.fn();
    render(<BankAccountsPanel accounts={accounts} cashAccountId={1} canManage onChartChanged={onChartChanged} />);
    await screen.findByText("Belum ada rekening");
    await userEvent.click(screen.getByRole("button", { name: "Tambah rekening" }));
    await userEvent.type(screen.getByLabelText(/^Bank/), "Mandiri");
    await userEvent.type(screen.getByLabelText(/Nomor rekening/), "456");
    await userEvent.type(screen.getByLabelText(/Atas nama/), "PT Uji");
    // Kas Tunai (pemetaan "cash") tidak ditawarkan sebagai akun rekening.
    const ledger = screen.getByLabelText(/Akun buku besar/) as HTMLSelectElement;
    expect(Array.from(ledger.options).map((o) => o.textContent)).toEqual(["Buat akun kas/bank baru", "1010 — Rekening Bank"]);
    expect((screen.getByLabelText(/Kode akun baru/) as HTMLInputElement).value).toBe("1011");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(createBankAccount).toHaveBeenCalled());
    expect(createBankAccount.mock.calls[0][0]).toEqual({
      bankName: "Mandiri", accountNumber: "456", accountHolder: "PT Uji", accountCode: "1011",
      isDefault: true, showOnDocuments: true, sortOrder: 0,
    });
    expect(onChartChanged).toHaveBeenCalled();
  });

  it("tanpa izin kelola, tombol tambah tidak tampil", async () => {
    bankAccounts.mockResolvedValue([bca]);
    render(<BankAccountsPanel accounts={accounts} cashAccountId={1} canManage={false} onChartChanged={vi.fn()} />);
    await screen.findByText("BCA 123");
    expect(screen.queryByRole("button", { name: "Tambah rekening" })).not.toBeInTheDocument();
  });
});
