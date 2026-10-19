import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const profitLoss = vi.fn();
const taxDetail = vi.fn();
const taxSummary = vi.fn();
const downloadTaxPackageRange = vi.fn();
const generalLedger = vi.fn();
const generalLedgerOverview = vi.fn();
const accounts = vi.fn();
const inventoryValue = vi.fn();
const margin = vi.fn();
const marginByProduct = vi.fn();
const fiscalSummary = vi.fn();
vi.mock("@/modules/finance/services/reports.service", () => ({
  financeReportsService: {
    profitLoss: (...a: unknown[]) => profitLoss(...a),
    taxDetail: (...a: unknown[]) => taxDetail(...a),
    taxSummary: (...a: unknown[]) => taxSummary(...a),
    downloadTaxPackage: vi.fn(),
    downloadTaxPackageRange: (...a: unknown[]) => downloadTaxPackageRange(...a),
    downloadTaxDetailCsv: vi.fn(),
    generalLedger: (...a: unknown[]) => generalLedger(...a),
    generalLedgerOverview: (...a: unknown[]) => generalLedgerOverview(...a),
    accounts: (...a: unknown[]) => accounts(...a),
    inventoryValue: (...a: unknown[]) => inventoryValue(...a),
    margin: (...a: unknown[]) => margin(...a),
    marginByProduct: (...a: unknown[]) => marginByProduct(...a),
    fiscalSummary: (...a: unknown[]) => fiscalSummary(...a),
  },
}));

import ProfitLossPage from "@/modules/finance/pages/reports/ProfitLossPage";
import TaxDetailPage from "@/modules/finance/pages/TaxDetailPage";
import GeneralLedgerPage from "@/modules/finance/pages/GeneralLedgerPage";
import InventoryValuePage from "@/modules/finance/pages/reports/InventoryValuePage";
import MarginPage from "@/modules/finance/pages/reports/MarginPage";

function wrap(node: React.ReactNode) {
  return render(<MemoryRouter>{node}</MemoryRouter>);
}

beforeEach(() => {
  for (const m of [profitLoss, taxDetail, taxSummary, downloadTaxPackageRange, generalLedger, generalLedgerOverview, accounts, inventoryValue, margin, marginByProduct, fiscalSummary]) m.mockReset();
  accounts.mockResolvedValue([]);
  useAuthStore.setState({ permissions: ["finance.view", "finance.report.view", "finance.tax_report.view"], ready: true });
});
afterEach(cleanup);

describe("Laba rugi rinci per akun (FN-03)", () => {
  it("menampilkan rincian akun di tiap golongan", async () => {
    profitLoss.mockResolvedValue({
      revenue: 1_000_000, cogs: 600_000, gross: 400_000, expense: 150_000, profit: 250_000,
      revenueLines: [{ code: "4000", name: "Penjualan Barang", amount: 1_000_000 }],
      cogsLines: [{ code: "5000", name: "Harga Pokok Penjualan", amount: 600_000 }],
      expenseLines: [{ code: "6000", name: "Beban Gaji", amount: 100_000 }, { code: "6010", name: "Beban Listrik", amount: 50_000 }],
      from: "2026-10-01", to: "2026-10-31",
    });
    wrap(<ProfitLossPage />);
    expect(await screen.findByText("4000 Penjualan Barang")).toBeInTheDocument();
    expect(screen.getByText("5000 Harga Pokok Penjualan")).toBeInTheDocument();
    expect(screen.getByText("6000 Beban Gaji")).toBeInTheDocument();
    expect(screen.getByText("6010 Beban Listrik")).toBeInTheDocument();
    expect(screen.getByText("Laba Bersih")).toBeInTheDocument();
  });
});

describe("Rincian PPN (FN-04, FN-06)", () => {
  const summary = {
    year: 2026, month: 10, ppnOut: 3300, ppnIn: 4950, payable: -1650, salesDpp: 30000, purchaseDpp: 45000,
    exceptions: [{ orderId: 4, orderNumber: "SO-0004", kind: "sales", date: "2026-10-12", partyName: "Toko Budi", dpp: 30000, ppn: 3300 }],
    exceptionsMeta: { page: 1, limit: 20, total: 1, totalPages: 1 },
  };
  const row = (over: Record<string, unknown>) => ({
    entryId: 1, kind: "sales", ppnIn: 0, date: "2026-10-12", number: "JV-1", memo: "Kirim", revenue: 30000, ppn: 3300,
    taxInvoiceNumber: "", taxInvoiceDate: "", dpp: 30000, missingInvoice: true, ...over,
  });

  beforeEach(() => {
    taxSummary.mockResolvedValue(summary);
    taxDetail.mockResolvedValue({
      items: [
        row({}),
        row({ entryId: 2, kind: "purchase", number: "JV-2", revenue: 0, ppn: 0, ppnIn: 4950, dpp: 45000, taxInvoiceNumber: "FP-9", missingInvoice: false }),
        row({ entryId: 3, kind: "purchase_return", number: "JV-3", revenue: 0, ppn: 0, ppnIn: -550, dpp: -5000, missingInvoice: false }),
      ],
      meta: { page: 1, limit: 200, total: 3, totalPages: 1 },
    });
    downloadTaxPackageRange.mockResolvedValue(undefined);
  });

  it("menandai baris tanpa faktur, menampilkan DPP jual/beli dan hitungan, serta baris retur beli", async () => {
    wrap(<TaxDetailPage />);
    expect((await screen.findAllByText("Tanpa faktur")).length).toBeGreaterThan(0);
    expect(screen.getByText("DPP penjualan")).toBeInTheDocument();
    expect(screen.getByText("DPP pembelian")).toBeInTheDocument();
    expect(screen.getByText("Transaksi kena pajak tanpa nomor faktur")).toBeInTheDocument();
    expect(screen.getByText("Retur Beli")).toBeInTheDocument();
    expect(screen.getByText("SO-0004")).toBeInTheDocument();
    expect(screen.getByText("FP-9")).toBeInTheDocument();
  });

  it("mengunduh paket untuk rentang tanggal bebas", async () => {
    useAuthStore.setState({ permissions: ["finance.view", "finance.report.view"], ready: true });
    wrap(<TaxDetailPage />);
    await screen.findByText("Paket pajak rentang bebas");
    const buttons = screen.getAllByRole("button", { name: "Paket data riil" });
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => expect(downloadTaxPackageRange).toHaveBeenCalledTimes(1));
    const [from, to, variant, branch] = downloadTaxPackageRange.mock.calls[0];
    expect(branch).toBe("");
    expect(from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(variant).toBe("actual");
  });
});

describe("Buku besar semua akun (FN-08)", () => {
  it("tanpa memilih akun menampilkan ikhtisar semua akun dengan saldo awal", async () => {
    generalLedgerOverview.mockResolvedValue({
      rows: [
        { code: "1000", name: "Kas Tunai", type: "aset", normal: "debit", opening: 100_000, debit: 50_000, credit: 20_000, closing: 130_000 },
        { code: "4000", name: "Penjualan Barang", type: "pendapatan", normal: "credit", opening: 0, debit: 0, credit: 50_000, closing: 50_000 },
      ],
      totalDebit: 50_000, totalCredit: 70_000,
      meta: { page: 1, limit: 100, total: 2, totalPages: 1 },
    });
    wrap(<GeneralLedgerPage />);
    const show = screen.getByRole("button", { name: "Tampilkan" });
    await waitFor(() => expect(show).toBeEnabled());
    fireEvent.click(show);
    await waitFor(() => expect(generalLedgerOverview).toHaveBeenCalled());
    expect(await screen.findByText("Kas Tunai")).toBeInTheDocument();
    expect(screen.getByText("Penjualan Barang")).toBeInTheDocument();
    expect(screen.getByText("Saldo awal")).toBeInTheDocument();
    expect(generalLedgerOverview).toHaveBeenCalledTimes(1);
    expect(generalLedger).not.toHaveBeenCalled();
  });
});

describe("Lintas cabang (FN-09)", () => {
  it("nilai persediaan menampilkan pecahan per cabang", async () => {
    inventoryValue.mockResolvedValue({
      positions: [{ productId: 1, variantId: 1, productCode: "P1", productName: "Gula", qty: 10, avgCost: 1000, value: 10_000 }],
      total: 16_000,
      byBranch: [
        { branchId: 1, branchName: "Pusat", total: 10_000, positions: 1 },
        { branchId: 2, branchName: "Bandung", total: 6_000, positions: 1 },
      ],
    });
    wrap(<InventoryValuePage />);
    expect(await screen.findByText("Per cabang")).toBeInTheDocument();
    expect(screen.getByText("Pusat")).toBeInTheDocument();
    expect(screen.getByText("Bandung")).toBeInTheDocument();
  });

  it("margin menampilkan pecahan per cabang", async () => {
    margin.mockResolvedValue({
      revenue: 100_000, cogs: 60_000, gross: 40_000, percent: 40,
      byBranch: [
        { branchId: 1, branchName: "Pusat", revenue: 70_000, cogs: 40_000, gross: 30_000, percent: 42.8 },
        { branchId: 2, branchName: "Bandung", revenue: 30_000, cogs: 20_000, gross: 10_000, percent: 33.3 },
      ],
    });
    marginByProduct.mockResolvedValue([]);
    wrap(<MarginPage />);
    expect((await screen.findAllByText("Marjin per cabang")).length).toBeGreaterThan(0);
    expect(screen.getByText("Bandung")).toBeInTheDocument();
  });
});
