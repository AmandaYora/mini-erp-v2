import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CorrectPaymentModal } from "@/modules/payment/components/CorrectPaymentModal";
import type { Payment } from "@/modules/payment/types";

afterEach(cleanup);

const payment: Payment = {
  id: 7,
  number: "PAY-0007",
  branchId: 1,
  partyId: 0,
  partyName: "",
  partyType: "customer",
  direction: "in",
  amount: 150000,
  method: "cash",
  paidAt: "2026-10-21 14:05:00",
  notes: "POS",
  status: "active",
  clientRef: "sj:3",
  reference: "",
  amountTendered: 200000,
  change: 50000,
  pairId: 0,
  allocations: [{ orderType: "sales", orderId: 3, orderNumber: "SO-0003", amount: 150000 }],
};

describe("koreksi pembayaran", () => {
  it("terisi nilai saat ini dan mengirim koreksi beserta alasan", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <CorrectPaymentModal payment={payment} submitting={false} serverError={null} onClose={() => undefined} onSubmit={onSubmit} />,
    );
    expect(screen.getByText("SO-0003")).toBeInTheDocument();
    expect(screen.getByLabelText("Nominal koreksi")).toHaveValue("150000");
    expect(screen.getByLabelText("Uang diterima")).toHaveValue("200000");

    await user.selectOptions(screen.getByLabelText("Metode"), "transfer");
    await user.type(screen.getByLabelText("Referensi Transfer"), "TRF-9");
    await user.type(screen.getByLabelText(/Alasan koreksi/), "kasir salah pilih metode");
    await user.click(screen.getByRole("button", { name: "Simpan koreksi" }));

    expect(onSubmit).toHaveBeenCalledWith({
      method: "transfer",
      amount: 150000,
      paidAt: "2026-10-21",
      reference: "TRF-9",
      amountTendered: undefined,
      notes: "POS",
      reason: "kasir salah pilih metode",
    });
  });

  it("menolak tanpa alasan yang cukup", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <CorrectPaymentModal payment={payment} submitting={false} serverError={null} onClose={() => undefined} onSubmit={onSubmit} />,
    );
    await user.type(screen.getByLabelText(/Alasan koreksi/), "abc");
    await user.click(screen.getByRole("button", { name: "Simpan koreksi" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(await screen.findByText(/minimal 5 karakter/)).toBeInTheDocument();
  });

  it("menampilkan pesan server apa adanya", () => {
    render(
      <CorrectPaymentModal
        payment={payment}
        submitting={false}
        serverError="Order SO-0003 (cod) sudah selesai; nominal koreksi 100000 membuatnya belum lunas"
        onClose={() => undefined}
        onSubmit={() => undefined}
      />,
    );
    expect(screen.getByText(/SO-0003 \(cod\) sudah selesai/)).toBeInTheDocument();
  });
});
