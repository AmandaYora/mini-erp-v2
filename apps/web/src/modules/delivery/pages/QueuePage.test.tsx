import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const workQueue = vi.fn();
vi.mock("@/modules/delivery/services/delivery.service", () => ({
  deliveryService: { workQueue: (...a: unknown[]) => workQueue(...a) },
}));

import QueuePage from "@/modules/delivery/pages/QueuePage";

const order = (id: number, orderDate: string) => ({
  orderId: id,
  number: `SO-${id}`,
  partyName: "Budi",
  orderDate,
  dueDate: "",
  paymentTerms: "cod",
  grandTotal: 100000,
  lines: [
    { productId: 1, variantId: 1, productCode: "P1", productName: "Semen", uom: "sak", ordered: 10, delivered: 0, remaining: 10 },
    { productId: 2, variantId: 2, productCode: "P2", productName: "Paku", uom: "kg", ordered: 5, delivered: 0, remaining: 5 },
  ],
  totalOrdered: 15,
  totalDelivered: 0,
  pendingDrafts: 0,
  itemCount: 2,
});

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["delivery.view", "delivery.create"], user: null, ready: true });
  workQueue.mockResolvedValue({
    createSj: [order(1, "2026-01-05 00:00:00")],
    waitingReturn: [
      { id: 9, number: "SJ-9", salesOrderId: 1, orderNumber: "SO-1", partyName: "Budi", deliveryDate: "2026-01-06 00:00:00", ageDays: 3, driverName: "Joko", vehiclePlate: "B 1", hasSignedDocument: false },
    ],
    limit: 100,
  });
});
afterEach(() => cleanup());

function open() {
  return render(
    <MemoryRouter initialEntries={["/deliveries/queue"]}>
      <QueuePage />
    </MemoryRouter>,
  );
}

describe("QueuePage (DL-02)", () => {
  it("kartu order menampilkan jumlah item", async () => {
    open();
    expect(await screen.findByText("2 item")).toBeTruthy();
  });

  it("tidak menyaring tanggal order di klien: baris dari server ditampilkan apa adanya", async () => {
    // order bertanggal 2026-01-05 tetap tampil walau filter 'Dari' diisi — server yang memfilter.
    open();
    await screen.findByText("SO-1");
    const from = screen.getByLabelText("Dari") as HTMLInputElement;
    fireEvent.change(from, { target: { value: "2026-06-01" } });
    await waitFor(() => expect(workQueue).toHaveBeenLastCalledWith(expect.objectContaining({ from: "2026-06-01" })));
    expect(await screen.findByText("SO-1")).toBeTruthy();
  });

  it("pencarian dikirim ke server (bukan disaring di klien)", async () => {
    open();
    await screen.findByText("SO-1");
    fireEvent.change(screen.getByLabelText("Cari antrean"), { target: { value: "SO-777" } });
    await waitFor(() => expect(workQueue).toHaveBeenLastCalledWith(expect.objectContaining({ search: "SO-777" })), { timeout: 3000 });
  });

  it("tab Menunggu menandai SJ yang scan tanda tangannya belum diunggah", async () => {
    open();
    fireEvent.mouseDown(await screen.findByRole("tab", { name: /Menunggu konfirmasi/ }), { button: 0 });
    fireEvent.click(screen.getByRole("tab", { name: /Menunggu konfirmasi/ }));
    expect(await screen.findByText("SJ ttd belum diunggah")).toBeTruthy();
  });
});
