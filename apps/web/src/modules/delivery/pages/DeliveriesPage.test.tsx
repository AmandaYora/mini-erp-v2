import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
vi.mock("@/modules/delivery/services/delivery.service", async () => {
  const actual = await vi.importActual<typeof import("@/modules/delivery/services/delivery.service")>(
    "@/modules/delivery/services/delivery.service",
  );
  return {
    ...actual,
    deliveryService: { list: (...a: unknown[]) => list(...a) },
  };
});
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({
  SalesOrderSearchSelect: () => <div>pemilih order</div>,
}));

import DeliveriesPage from "@/modules/delivery/pages/DeliveriesPage";

const base = {
  branchId: 1,
  deliveryDate: "2026-10-10 00:00:00",
  notes: "",
  driverName: "Budi",
  vehiclePlate: "",
  warehouseStaffName: "Sari",
  recipientName: "",
  recipientSignatureStatus: "",
  recipientSignatureMissingReason: "",
  dropLocationNote: "",
  dispatchedAt: null,
  dispatchedBy: null,
  confirmedAt: null,
  confirmedBy: null,
  documentKind: "order",
  items: [],
};
const orderNote = { ...base, id: 1, number: "SJ-0001", salesOrderId: 7, orderNumber: "SO-0007", status: "draft", origin: "order", partyName: "" };
const directNote = {
  ...base,
  id: 2,
  number: "SJ-0002",
  salesOrderId: 0,
  orderNumber: "",
  status: "confirmed",
  origin: "direct",
  partyName: "PT Maju",
};

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname + loc.search}</div>;
}

function open(path = "/deliveries") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/deliveries" element={<DeliveriesPage />} />
        <Route path="/deliveries/new" element={<div>form dari order</div>} />
        <Route path="/deliveries/new-direct" element={<div>form sj langsung</div>} />
      </Routes>
      <Where />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["delivery.view", "delivery.create"], user: null, ready: true });
  list.mockResolvedValue({ items: [orderNote, directNote], meta: { total: 2 } });
});
afterEach(() => cleanup());

describe("DeliveriesPage — SJ Langsung (ADR-0022, W-01)", () => {
  it("tanpa izin delivery.create_direct tombol buat tetap tautan ke form dari order", async () => {
    open();
    await screen.findByText("SJ-0001");
    const link = screen.getByRole("link", { name: "Buat surat jalan" });
    expect(link.getAttribute("href")).toBe("/deliveries/new");
    expect(screen.queryByText("Tanpa order (SJ Langsung)")).toBeNull();
  });

  it("dengan izin, tombol bercabang menawarkan Dari order jual dan Tanpa order (SJ Langsung)", async () => {
    useAuthStore.setState({ permissions: ["delivery.view", "delivery.create", "delivery.create_direct"], user: null, ready: true });
    open();
    await screen.findByText("SJ-0001");
    const trigger = screen.getByRole("button", { name: "Buat surat jalan" });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    expect(await screen.findByRole("menuitem", { name: "Dari order jual" })).toBeTruthy();
    fireEvent.click(screen.getByRole("menuitem", { name: "Tanpa order (SJ Langsung)" }));
    expect(await screen.findByText("form sj langsung")).toBeTruthy();
  });

  it("SJ Langsung tanpa order memakai nama pelanggan di kolom order dan berlencana Belum diorder", async () => {
    open();
    await screen.findByText("SJ-0002");
    expect(screen.getByText("PT Maju")).toBeTruthy();
    expect(screen.getByText("SJ Langsung")).toBeTruthy();
    expect(screen.getByText("Belum diorder")).toBeTruthy();
    // SJ order biasa tetap seperti sebelumnya.
    expect(screen.getByText("SO-0007")).toBeTruthy();
    expect(screen.getByText("Order")).toBeTruthy();
  });

  it("filter Asal dan Belum diorder dari URL dikirim ke server sebagai origin dan unordered", async () => {
    open("/deliveries?origin=direct&unordered=1");
    await waitFor(() =>
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ origin: "direct", unordered: true }),
        expect.anything(),
      ),
    );
    expect(await screen.findByText("Asal: SJ Langsung")).toBeTruthy();
  });
});
