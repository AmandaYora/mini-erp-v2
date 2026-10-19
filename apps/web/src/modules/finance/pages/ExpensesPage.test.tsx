import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const expenses = vi.fn();
const accounts = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    expenses: (...a: unknown[]) => expenses(...a),
    accounts: (...a: unknown[]) => accounts(...a),
    createExpense: vi.fn(),
    cancelExpense: vi.fn(),
  },
}));

import ExpensesPage from "@/modules/finance/pages/ExpensesPage";

beforeEach(() => {
  expenses.mockReset();
  accounts.mockReset();
  accounts.mockResolvedValue([]);
  expenses.mockResolvedValue({
    items: [
      { id: 1, number: "EXP-1", branchId: 1, expenseAccountId: 5, payAccountId: 6, amount: 55_000, date: "2026-10-12", notes: "", journalEntryId: 9, status: "posted", partyId: 3, partyName: "PLN Uji" },
      { id: 2, number: "EXP-2", branchId: 1, expenseAccountId: 5, payAccountId: 6, amount: 10_000, date: "2026-10-13", notes: "", journalEntryId: 10, status: "posted" },
    ],
    meta: { page: 1, limit: 20, total: 2, totalPages: 1 },
  });
  useAuthStore.setState({ permissions: ["finance.view", "finance.expense.view", "finance.expense.create"], ready: true });
});
afterEach(cleanup);

describe("ExpensesPage: pihak lawan (FN-10)", () => {
  it("menampilkan pihak lawan pada daftar biaya dan tanda hubung bila tidak ada", async () => {
    render(
      <MemoryRouter>
        <ExpensesPage />
      </MemoryRouter>,
    );
    expect(await screen.findByText("PLN Uji")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Pihak" })).toBeInTheDocument();
    expect(expenses).toHaveBeenCalled();
  });
});
