import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const creditBalances = vi.fn();
vi.mock("@/modules/payment/services/payment.service", () => ({
  paymentService: { creditBalances: (...a: unknown[]) => creditBalances(...a) },
}));
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: ({ onChange }: { onChange: (id: number | null) => void }) => (
    <button type="button" onClick={() => onChange(5)}>
      pilih-pihak
    </button>
  ),
}));
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({
  SalesOrderSearchSelect: ({ onChange }: { onChange: (id: number | null) => void }) => (
    <button type="button" onClick={() => onChange(11)}>
      pilih-tagihan
    </button>
  ),
}));
vi.mock("@/modules/purchasing/components/PurchaseOrderSearchSelect", () => ({
  PurchaseOrderSearchSelect: () => <div />,
}));

import { UseCreditModal } from "@/modules/payment/components/UseCreditModal";

beforeEach(() => {
  creditBalances.mockReset();
  creditBalances.mockResolvedValue({
    items: [
      { returnType: "sales_return", returnId: 8, number: "SR-0008", credited: 100000, used: 30000, remaining: 70000 },
    ],
    totalRemaining: 70000,
  });
});
afterEach(cleanup);

async function pickHolder(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByText("pilih-pihak"));
  await waitFor(() => expect(screen.getByText(/SR-0008/)).toBeInTheDocument());
  const selects = screen.getAllByRole("combobox");
  await user.selectOptions(selects[selects.length - 1]!, "8");
  await user.click(screen.getByText("pilih-tagihan"));
}

describe("pakai saldo uang muka (N-41)", () => {
  it("menawarkan sisa saldo retur dan mengirim payload yang benar", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<UseCreditModal open submitting={false} serverError={null} onClose={() => undefined} onSubmit={onSubmit} />);
    await pickHolder(user);
    expect(creditBalances).toHaveBeenCalledWith(5);
    await user.click(screen.getByRole("button", { name: "Pakai saldo" }));
    expect(onSubmit).toHaveBeenCalledWith({
      partyId: 5,
      returnType: "sales_return",
      returnId: 8,
      orderType: "sales",
      orderId: 11,
      amount: 70000,
      notes: undefined,
    });
  });

  it("menolak nominal di atas sisa saldo", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<UseCreditModal open submitting={false} serverError={null} onClose={() => undefined} onSubmit={onSubmit} />);
    await pickHolder(user);
    const amount = screen.getByLabelText("Nominal dipakai");
    await user.clear(amount);
    await user.type(amount, "80000");
    await user.click(screen.getByRole("button", { name: "Pakai saldo" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(await screen.findByText(/Melebihi sisa saldo/)).toBeInTheDocument();
  });
});
