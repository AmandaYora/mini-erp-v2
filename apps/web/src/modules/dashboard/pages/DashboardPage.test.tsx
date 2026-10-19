import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const summary = vi.fn();
const salesTrend = vi.fn();
const criticalStock = vi.fn();
const priorityOrders = vi.fn();
const orderStatus = vi.fn();
const topSellers = vi.fn();
const topMargin = vi.fn();
const whatsappChannel = vi.fn();
vi.mock("@/modules/dashboard/services/dashboard.service", () => ({
  dashboardService: {
    summary: (...a: unknown[]) => summary(...a),
    salesTrend: (...a: unknown[]) => salesTrend(...a),
    criticalStock: (...a: unknown[]) => criticalStock(...a),
    priorityOrders: (...a: unknown[]) => priorityOrders(...a),
    orderStatus: (...a: unknown[]) => orderStatus(...a),
    topSellers: (...a: unknown[]) => topSellers(...a),
    topMargin: (...a: unknown[]) => topMargin(...a),
    whatsappChannel: (...a: unknown[]) => whatsappChannel(...a),
  },
}));

import DashboardPage from "@/modules/dashboard/pages/DashboardPage";

const period = { from: "2026-10-01", to: "2026-10-19", revenue: 1_000_000, expense: 200_000, profit: 800_000 };

function makeSummary(over: Record<string, unknown> = {}) {
  return {
    branchId: 1,
    today: period,
    monthToDate: period,
    balances: { cash: 5_000_000, receivable: 1_000_000, payable: 500_000, inventory: 3_000_000 },
    inventoryValue: 3_000_000,
    operations: {
      salesToday: 0, salesMonth: 0, salesOrdersToday: 0, salesOrdersMonth: 0, ordersToday: 0, ordersMonth: 0,
      purchasesToday: 0, purchasesMonth: 0, outstandingReceivable: 0, outstandingPayable: 0,
      criticalCount: 3, trackedCount: 42, missingCostCount: 2, costBasis: "estimated",
      ...over,
    },
  };
}

beforeEach(() => {
  for (const m of [summary, salesTrend, criticalStock, priorityOrders, orderStatus, topSellers, topMargin, whatsappChannel]) m.mockReset();
  summary.mockResolvedValue(makeSummary());
  salesTrend.mockResolvedValue([]);
  criticalStock.mockResolvedValue([]);
  priorityOrders.mockResolvedValue([
    { id: 1, number: "SO-1", grandTotal: 100, status: "confirmed", partyId: 1, partyName: "Budi", orderDate: "2026-10-01", dueDate: "2026-10-10", score: 130, kind: "sales", reason: "Lewat jatuh tempo", ageDays: 18 },
    { id: 9, number: "PO-9", grandTotal: 100, status: "draft", partyId: 2, partyName: "Sup A", orderDate: "2026-10-15", dueDate: "", score: 64, kind: "purchase", reason: "Menunggu keputusan", ageDays: 4 },
  ]);
  orderStatus.mockResolvedValue({ sales: {}, purchasing: {} });
  topSellers.mockResolvedValue([]);
  topMargin.mockResolvedValue([]);
  whatsappChannel.mockResolvedValue({ connected: true, phone: "628123" });
  useAuthStore.setState({
    permissions: ["dashboard.view", "reporting.view", "assistant.view"],
    ready: true,
    user: { fullName: "Dimas Prasetio" } as never,
  });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <DashboardPage />
    </MemoryRouter>,
  );
}

describe("DashboardPage", () => {
  it("menyapa pengguna dan menampilkan KPI pembukuan", async () => {
    renderPage();
    expect(await screen.findByText(/Dimas/)).toBeInTheDocument();
    expect(await screen.findByText("Omzet hari ini")).toBeInTheDocument();
  });

  it("menampilkan produk dipantau, basis biaya margin, dan status WhatsApp (DB-01)", async () => {
    renderPage();
    expect(await screen.findByText("Produk dipantau")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText(/3 di bawah stok minimum/)).toBeInTheDocument();
    expect(screen.getByText("Basis biaya margin")).toBeInTheDocument();
    expect(screen.getByText("Estimasi")).toBeInTheDocument();
    expect(screen.getByText(/2 produk memakai HPP estimasi/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Terhubung")).toBeInTheDocument());
    expect(screen.getByText("628123")).toBeInTheDocument();
  });

  it("kartu WhatsApp menunjukkan menghubungkan saat QR menunggu dipindai", async () => {
    whatsappChannel.mockResolvedValue({ connected: false, qrDataUrl: "data:image/png;base64,xx" });
    renderPage();
    await waitFor(() => expect(screen.getByText("Menghubungkan")).toBeInTheDocument());
  });

  it("tanpa izin asisten kartu WhatsApp tidak tampil dan kanal tidak dipanggil", async () => {
    useAuthStore.setState({ permissions: ["dashboard.view", "reporting.view"], ready: true });
    renderPage();
    expect(await screen.findByText("Produk dipantau")).toBeInTheDocument();
    expect(screen.queryByText("WhatsApp asisten")).not.toBeInTheDocument();
    expect(whatsappChannel).not.toHaveBeenCalled();
  });

  it("basis biaya lengkap tampil hijau tanpa peringatan estimasi", async () => {
    summary.mockResolvedValue(makeSummary({ missingCostCount: 0, costBasis: "complete" }));
    renderPage();
    expect(await screen.findByText("Lengkap")).toBeInTheDocument();
    expect(screen.getByText("Semua HPP berdasar biaya riil")).toBeInTheDocument();
  });

  it("Perlu tindakan memuat pesanan jual dan beli dengan alasan dan umur", async () => {
    renderPage();
    expect(await screen.findByText(/SO-1/)).toBeInTheDocument();
    expect(screen.getByText(/Lewat jatuh tempo · terlambat \d+ hari · umur 18 hari/)).toBeInTheDocument();
    expect(screen.getByText(/PO-9/)).toBeInTheDocument();
    expect(screen.getByText(/Pembelian: Menunggu keputusan · umur 4 hari/)).toBeInTheDocument();
  });
});
