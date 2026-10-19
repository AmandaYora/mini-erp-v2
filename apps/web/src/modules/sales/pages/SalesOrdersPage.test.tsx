import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
const downloadExport = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: {
      list: (...a: unknown[]) => list(...a),
      downloadExport: (...a: unknown[]) => downloadExport(...a),
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

beforeEach(() => {
  list.mockReset();
  downloadExport.mockReset();
  list.mockResolvedValue({ items: [row], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } });
  downloadExport.mockResolvedValue(undefined);
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
