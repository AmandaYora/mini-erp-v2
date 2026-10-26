import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { BankAccount } from "@/modules/finance/types";
import type { BankAccountsState } from "@/modules/finance/hooks/use-bank-accounts";

const saveSettings = vi.fn();
vi.mock("@/modules/company/services/company.service", () => ({
  companyService: { saveSettings: (...a: unknown[]) => saveSettings(...a) },
}));

let bankState: BankAccountsState;
vi.mock("@/modules/finance/hooks/use-bank-accounts", () => ({
  useBankAccounts: () => bankState,
}));

import { useAuthStore } from "@/modules/auth/stores/auth.store";
import DocumentProfileForm from "@/modules/company/components/DocumentProfileForm";

const bca: BankAccount = {
  id: 1, accountId: 11, accountCode: "1011", accountName: "Bank BCA", bankName: "BCA", accountNumber: "123",
  accountHolder: "PT Uji", showOnDocuments: true, isDefault: true, sortOrder: 0, status: "active",
};
const legacyRaw = JSON.stringify({
  headerName: "TOKO UJI",
  bankAccounts: [
    { bank: "BNI", holder: "Toko Lama", number: "999" },
    { bank: "BCA", holder: "PT Uji", number: "123" },
  ],
});

function state(all: BankAccount[]): BankAccountsState {
  const active = all.filter((b) => b.status === "active");
  return { all, active, defaultAccountId: active.find((b) => b.isDefault)?.accountId ?? null, required: active.length > 0, loading: false };
}

function renderForm(raw = legacyRaw) {
  return render(
    <MemoryRouter>
      <DocumentProfileForm raw={raw} canManage onSaved={vi.fn()} />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  bankState = state([]);
  useAuthStore.setState({ permissions: ["company.manage", "finance.manage"], ready: true });
});
afterEach(cleanup);

describe("Pengaturan › Kontak dan rekening: rekening satu sumber (analysis/004)", () => {
  it("tidak ada lagi input rekening; master kosong menampilkan rekening lama sebagai tampilan", () => {
    renderForm();
    expect(screen.queryByLabelText("Bank 1")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Nomor rekening 1")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tambah rekening" })).not.toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Rekening di dokumen" });
    expect(within(list).getByText("BNI 999 a.n. Toko Lama")).toBeInTheDocument();
    expect(screen.getByText(/Dipindahkan otomatis ke Keuangan/)).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Kelola rekening" });
    expect(link).toHaveAttribute("href", "/finance/accounts?tab=rekening");
  });

  it("master berisi: hanya rekening master yang tampil di dokumen, teks lama tidak", () => {
    bankState = state([bca, { ...bca, id: 2, accountId: 12, bankName: "BRI", accountNumber: "555", showOnDocuments: false, isDefault: false }]);
    renderForm();
    const list = screen.getByRole("list", { name: "Rekening di dokumen" });
    expect(within(list).getByText("BCA 123 a.n. PT Uji")).toBeInTheDocument();
    expect(within(list).getByText("Default")).toBeInTheDocument();
    expect(within(list).queryByText(/BRI/)).not.toBeInTheDocument();
    expect(within(list).queryByText(/BNI/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Dari data sebelumnya/)).not.toBeInTheDocument();
  });

  it("tanpa izin kelola keuangan: tidak ada tautan kelola", () => {
    useAuthStore.setState({ permissions: ["company.manage"], ready: true });
    renderForm();
    expect(screen.queryByRole("link", { name: "Kelola rekening" })).not.toBeInTheDocument();
  });

  it("menyimpan profil tidak menghapus rekening lama (cadangan cetak selama master kosong)", async () => {
    saveSettings.mockResolvedValue({ message: "Pengaturan disimpan", data: {} });
    renderForm();
    await userEvent.clear(screen.getByLabelText("Nama header"));
    await userEvent.type(screen.getByLabelText("Nama header"), "TOKO BARU");
    await userEvent.click(screen.getByRole("button", { name: /Simpan/ }));
    await waitFor(() => expect(saveSettings).toHaveBeenCalled());
    const saved = JSON.parse(saveSettings.mock.calls[0][0].document_profile);
    expect(saved.headerName).toBe("TOKO BARU");
    expect(saved.bankAccounts).toEqual([
      { bank: "BNI", holder: "Toko Lama", number: "999" },
      { bank: "BCA", holder: "PT Uji", number: "123" },
    ]);
  });
});
