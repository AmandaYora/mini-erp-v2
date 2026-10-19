import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

const list = vi.fn();
vi.mock("@/modules/salesreturn/services/sales-return.service", () => ({
  salesReturnService: { list: (...a: unknown[]) => list(...a) },
}));
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({
  SalesOrderSearchSelect: ({ value }: { value: number | null }) => <div data-testid="so-picker">{value ?? "semua"}</div>,
}));

vi.setConfig({ testTimeout: 60000 });

import SalesReturnsPage from "@/modules/salesreturn/pages/SalesReturnsPage";

const row = {
  id: 11, number: "RJ-0011", orderNumber: "SO-0007", salesOrderId: 7, returnDate: "2026-10-05",
  total: 75000, status: "confirmed",
};
const meta = (total: number) => ({ page: 1, limit: 20, total, totalPages: Math.max(1, Math.ceil(total / 20)) });

function Probe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname + loc.search}</div>;
}
function renderPage(path: string = ROUTE_PATHS.salesReturns) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SalesReturnsPage />
      <Probe />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  list.mockReset();
  list.mockResolvedValue({ items: [row], meta: meta(1) });
  useAuthStore.setState({ permissions: ["salesreturn.view", "salesreturn.create"], ready: true });
});
afterEach(cleanup);

describe("daftar retur jual", () => {
  it("render: baris retur dengan nomor, order, total, status; memanggil server dengan limit 20 dan AbortSignal", async () => {
    renderPage();
    expect(await screen.findByText("RJ-0011")).toBeInTheDocument();
    expect(screen.getByText("SO-0007")).toBeInTheDocument();
    expect(screen.getByText(/75\.000/)).toBeInTheDocument();
    const [params, signal] = list.mock.calls[0];
    expect(params).toMatchObject({ page: 1, limit: 20 });
    expect(signal).toBeInstanceOf(AbortSignal);
    expect(screen.getByRole("link", { name: "Buat retur" })).toHaveAttribute("href", `${ROUTE_PATHS.salesReturns}/new`);
  });

  it("filter dibaca dari URL dan diteruskan ke server; chip filter aktif bisa dihapus", async () => {
    const user = userEvent.setup();
    renderPage(`${ROUTE_PATHS.salesReturns}?status=draft,confirmed&salesOrderId=7&from=2026-10-01&to=2026-10-31`);
    await screen.findByText("RJ-0011");
    expect(list.mock.calls[0][0]).toMatchObject({
      status: "draft,confirmed", salesOrderId: 7, from: "2026-10-01", to: "2026-10-31",
    });
    expect(screen.getByText("Order jual dipilih")).toBeInTheDocument();
    // Hapus satu status lewat chip: URL ikut berubah dan daftar dimuat ulang.
    await user.click(screen.getByRole("button", { name: /Hapus filter Status: Draf/i }));
    await waitFor(() => expect(screen.getByTestId("loc")).not.toHaveTextContent("draft"));
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ status: "confirmed" }));
  });

  it("klik baris membuka detail retur", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByText("SO-0007"));
    await waitFor(() => expect(screen.getByTestId("loc")).toHaveTextContent(`${ROUTE_PATHS.salesReturns}/11`));
  });

  it("kosong tanpa filter: ajakan Buat retur pertama; kosong dengan filter: pesan filter", async () => {
    list.mockResolvedValue({ items: [], meta: meta(0) });
    renderPage();
    expect(await screen.findByText("Belum ada retur penjualan")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Buat retur pertama" })).toBeInTheDocument();
    cleanup();
    renderPage(`${ROUTE_PATHS.salesReturns}?status=cancelled`);
    expect(await screen.findByText("Tidak ada retur yang cocok")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Buat retur pertama" })).not.toBeInTheDocument();
  });

  it("tanpa izin salesreturn.create tidak ada tombol Buat retur", async () => {
    useAuthStore.setState({ permissions: ["salesreturn.view"], ready: true });
    renderPage();
    await screen.findByText("RJ-0011");
    expect(screen.queryByRole("link", { name: "Buat retur" })).not.toBeInTheDocument();
  });

  it("galat memuat menampilkan pesan server dan Coba lagi memuat ulang", async () => {
    const user = userEvent.setup();
    list.mockRejectedValueOnce({ status: 500, message: "Server retur sedang sibuk" });
    renderPage();
    expect(await screen.findByText(/Server retur sedang sibuk/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Coba lagi/i }));
    expect(await screen.findByText("RJ-0011")).toBeInTheDocument();
  });

  it("paginasi: halaman 2 diminta ke server", async () => {
    const user = userEvent.setup();
    list.mockResolvedValue({ items: [row], meta: meta(45) });
    renderPage();
    await screen.findByText("RJ-0011");
    await user.click(screen.getByRole("button", { name: /Berikutnya|Next|Halaman berikut/i }));
    await waitFor(() => expect(list.mock.calls.at(-1)?.[1]).toBeInstanceOf(AbortSignal));
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ page: 2, limit: 20 }));
  });
});
