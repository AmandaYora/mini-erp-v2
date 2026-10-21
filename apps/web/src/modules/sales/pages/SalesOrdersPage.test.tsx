import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
const downloadExport = vi.fn();
const openDirect = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: {
      list: (...a: unknown[]) => list(...a),
      downloadExport: (...a: unknown[]) => downloadExport(...a),
      openDirectDeliveries: (...a: unknown[]) => openDirect(...a),
    },
  };
});

// Pemilih pelanggan memanggil jaringan sendiri; di sini cukup penanda.
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: ({ value }: { value: number | null }) => (
    <div data-testid="party-picker">{value ?? "semua"}</div>
  ),
}));

import SalesOrdersPage from "@/modules/sales/pages/SalesOrdersPage";

const row = {
  id: 7,
  number: "SO-0007",
  partyId: 0,
  partyName: "Tunai",
  channel: "regular",
  orderDate: "2026-10-01",
  dueDate: "",
  paymentTerms: "cod",
  grandTotal: 150000,
  status: "confirmed",
  paidAmount: 50000,
  outstandingAmount: 100000,
  paymentStatus: "partial",
  isOverdue: false,
  createdByName: "Dewi Pembuat",
};

function Probe() {
  const loc = useLocation();
  return <div data-testid="search">{loc.search}</div>;
}

function renderPage(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SalesOrdersPage />
      <Probe />
    </MemoryRouter>,
  );
}

const directNote = (id: number, partyId: number, deliveryDate: string) => ({
  id,
  number: `SJ-00${id}`,
  deliveryDate,
  partyId,
  partyName: partyId === 3 ? "Budi" : "Citra",
  driverName: "",
  vehiclePlate: "",
  recipientName: "",
  origin: "direct",
  salesOrderId: 0,
  items: [],
});

beforeEach(() => {
  list.mockReset();
  downloadExport.mockReset();
  openDirect.mockReset();
  list.mockResolvedValue({ items: [row], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } });
  downloadExport.mockResolvedValue(undefined);
  openDirect.mockResolvedValue({ items: [], count: 0, oldestDate: "" });
  useAuthStore.setState({ permissions: ["sales.view", "sales.create", "sales.export"], ready: true });
});
afterEach(cleanup);

describe("daftar order jual", () => {
  it("membaca filter dari URL, mengirimnya ke server dengan AbortSignal, dan menampilkan Tunai", async () => {
    renderPage(
      "/sales-orders?status=confirmed,completed&paymentTerms=net&partyId=4&from=2026-10-01&to=2026-10-31&sort=total&dir=asc",
    );
    expect(await screen.findByText("SO-0007")).toBeInTheDocument();
    expect(screen.getByText("Tunai")).toBeInTheDocument();
    const [params, signal] = list.mock.calls[0];
    expect(params).toEqual({
      status: "confirmed,completed",
      paymentTerms: "net",
      paymentStatus: "",
      partyId: 4,
      from: "2026-10-01",
      to: "2026-10-31",
      sort: "total",
      dir: "asc",
      search: "",
      channel: "",
      origin: "",
      page: 1,
      limit: 20,
    });
    expect(signal).toBeInstanceOf(AbortSignal);
    // Filter pelanggan ada di popover "Filter" dan tampil sebagai chip aktif.
    expect(screen.getByText("Pelanggan #4")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Filter/ }));
    expect(await screen.findByTestId("party-picker")).toHaveTextContent("4");
  });

  it("mengubah status menyinkronkan URL, mereset halaman, dan memuat ulang", async () => {
    renderPage("/sales-orders?status=confirmed&page=3");
    await screen.findByText("SO-0007");
    expect(list.mock.calls[0][0]).toMatchObject({ status: "confirmed", page: 3 });
    fireEvent.click(screen.getByRole("button", { name: "Selesai" }));
    await waitFor(() =>
      expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ status: "confirmed,completed", page: 1 }),
    );
    expect(screen.getByTestId("search")).toHaveTextContent("?status=confirmed%2Ccompleted");
  });

  it("ekspor membawa filter yang sama dengan daftar", async () => {
    renderPage("/sales-orders?status=confirmed&paymentTerms=cod&from=2026-10-01");
    await screen.findByText("SO-0007");
    await userEvent.click(screen.getByRole("button", { name: /Ekspor/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Ekspor PDF" }));
    await waitFor(() => expect(downloadExport).toHaveBeenCalledTimes(1));
    const [format, filter] = downloadExport.mock.calls[0];
    expect(format).toBe("pdf");
    expect(filter).toMatchObject({ status: "confirmed", paymentTerms: "cod", from: "2026-10-01" });
    // Filter yang sama dengan yang dikirim ke list (tanpa page/limit).
    const { page: _p, limit: _l, ...listFilter } = list.mock.calls[0][0];
    void _p;
    void _l;
    expect(filter).toMatchObject({ ...listFilter, sort: "", dir: "" });
  });

  it("sortir lewat header kolom menyinkronkan URL dan select Urutkan sudah dihapus", async () => {
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    await userEvent.click(screen.getByRole("button", { name: "Total" }));
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ sort: "total", dir: "asc" }));
    expect(screen.getByTestId("search")).toHaveTextContent("sort=total");
    expect(screen.queryByLabelText("Urutkan")).not.toBeInTheDocument();
  });

  it("menampilkan Dibayar/Sisa/status bayar dan Dibuat oleh (SL-02)", async () => {
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    expect(screen.getByText("Bayar sebagian")).toBeInTheDocument();
    expect(screen.getByText("Dewi Pembuat")).toBeInTheDocument();
    expect(screen.getAllByText(/50\.000/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/100\.000/).length).toBeGreaterThan(0);
  });

  it("filter status bayar dikirim ke server dan tersimpan di URL", async () => {
    renderPage("/sales-orders?paymentStatus=overdue");
    await screen.findByText("SO-0007");
    expect(list.mock.calls[0][0]).toMatchObject({ paymentStatus: "overdue" });
    expect(screen.getByText("Pembayaran: Jatuh tempo")).toBeInTheDocument();
    expect(screen.getByTestId("search")).toHaveTextContent("paymentStatus=overdue");
  });

  it("tombol ekspor disembunyikan tanpa sales.export", async () => {
    useAuthStore.setState({ permissions: ["sales.view"], ready: true });
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    expect(screen.queryByRole("button", { name: /Ekspor/ })).not.toBeInTheDocument();
  });
});

describe("daftar order jual — SJ Langsung (ADR-0022 W-05)", () => {
  it("panel SJ menunggu order tersembunyi bila jumlahnya 0 dan tanpa izin delivery.view", async () => {
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    expect(openDirect).not.toHaveBeenCalled();
    expect(screen.queryByText(/SJ menunggu order/)).not.toBeInTheDocument();

    cleanup();
    useAuthStore.setState({ permissions: ["sales.view", "sales.create", "delivery.view"], ready: true });
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    await waitFor(() => expect(openDirect).toHaveBeenCalledTimes(1));
    expect(openDirect.mock.calls[0][0]).toBeNull();
    expect(openDirect.mock.calls[0][1]).toBe(20);
    expect(screen.queryByText(/SJ menunggu order/)).not.toBeInTheDocument();
  });

  it("panel menampilkan SJ dengan umur, tautan Buat order per baris, dan gabungan hanya satu pelanggan", async () => {
    useAuthStore.setState({ permissions: ["sales.view", "sales.create", "delivery.view"], ready: true });
    openDirect.mockResolvedValue({
      items: [directNote(11, 3, "2026-10-01"), directNote(12, 3, "2026-10-02"), directNote(13, 4, "2026-10-03")],
      count: 3,
      oldestDate: "2026-10-01",
    });
    renderPage("/sales-orders");
    expect(await screen.findByText(/SJ menunggu order \(3\) · tertua \d+ hari/)).toBeInTheDocument();
    expect(screen.getByText("SJ-0011")).toBeInTheDocument();
    const links = screen.getAllByRole("link", { name: "Buat order" });
    expect(links[0]).toHaveAttribute("href", "/sales-orders/new?fromDeliveries=11");
    // Centang dua SJ pelanggan yang sama → tombol gabung aktif dengan kedua id.
    const boxes = screen.getAllByRole("checkbox", { name: /Pilih baris/ });
    await userEvent.click(boxes[0]);
    await userEvent.click(boxes[1]);
    const bulk = await screen.findByRole("button", { name: "Buat order dari 2 SJ" });
    expect(bulk).toBeEnabled();
    // Tambah SJ pelanggan lain → ditolak di klien.
    await userEvent.click(boxes[2]);
    expect(await screen.findByRole("button", { name: "Buat order dari 3 SJ" })).toBeDisabled();
    expect(screen.getByText(/satu pelanggan yang sama/)).toBeInTheDocument();
  });

  it("tombol Tambah order bercabang: Order biasa / Dari surat jalan membuka pemilih", async () => {
    useAuthStore.setState({ permissions: ["sales.view", "sales.create", "delivery.view"], ready: true });
    renderPage("/sales-orders");
    await screen.findByText("SO-0007");
    expect(screen.getByRole("link", { name: "Tambah order" })).toHaveAttribute("href", "/sales-orders/new");
    await userEvent.click(screen.getByRole("button", { name: "Pilihan tambah order" }));
    expect(await screen.findByRole("menuitem", { name: "Order biasa" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("menuitem", { name: "Dari surat jalan" }));
    expect(await screen.findByRole("dialog", { name: "Order dari surat jalan" })).toBeInTheDocument();
  });

  it("filter Asal dikirim ke server, tampil sebagai chip, dan baris dari SJ berlencana", async () => {
    list.mockResolvedValue({
      items: [{ ...row, origin: "delivery" }],
      meta: { page: 1, limit: 20, total: 1, totalPages: 1 },
    });
    renderPage("/sales-orders?origin=delivery");
    await screen.findByText("SO-0007");
    expect(list.mock.calls[0][0]).toMatchObject({ origin: "delivery" });
    expect(screen.getByText("Asal: SJ Langsung")).toBeInTheDocument();
    expect(screen.getByText("Dari SJ")).toBeInTheDocument();
  });
});
