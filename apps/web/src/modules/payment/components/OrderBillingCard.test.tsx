import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { OrderBillingCard } from "@/modules/payment/components/OrderBillingCard";
import { billingView } from "@/modules/payment/lib/order-billing";

afterEach(cleanup);

const billing = billingView({
  status: "completed",
  grandTotal: 30000,
  paidAmount: 30000,
  outstandingAmount: 0,
  paymentStatus: "paid",
});

describe("OrderBillingCard — retur (SL-03)", () => {
  it("menampilkan retur, total bersih, dan dana yang harus dikembalikan; tanpa teks 'dikelola terpisah'", () => {
    render(
      <OrderBillingCard
        billing={billing}
        subtotal={30000}
        discountTotal={0}
        taxTotal={0}
        grandTotal={30000}
        returns={{ adjustment: -15000, netTotal: 15000, refundDue: 15000, exchangeChargeDue: 0 }}
      />,
    );
    expect(screen.getByText("Retur terkonfirmasi")).toBeInTheDocument();
    expect(screen.getByText("Total bersih setelah retur")).toBeInTheDocument();
    expect(screen.getByText("Harus dikembalikan ke pelanggan")).toBeInTheDocument();
    expect(screen.queryByText(/dikelola terpisah/)).not.toBeInTheDocument();
  });

  it("tanpa retur tidak ada baris retur", () => {
    render(
      <OrderBillingCard
        billing={billing}
        subtotal={30000}
        discountTotal={0}
        taxTotal={0}
        grandTotal={30000}
        returns={{ adjustment: 0, netTotal: 30000, refundDue: 0, exchangeChargeDue: 0 }}
      />,
    );
    expect(screen.queryByText("Total bersih setelah retur")).not.toBeInTheDocument();
  });
});
