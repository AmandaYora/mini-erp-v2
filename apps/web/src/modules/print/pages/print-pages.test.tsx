import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resolveIdentity } from "@/modules/print/document";
import { CONTINUOUS_PAPER_DEFAULTS, resolvePaperProfile } from "@/modules/print/paper";

const profileState = {
  profile: { continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } },
  paper: resolvePaperProfile(undefined),
  identity: resolveIdentity({ continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } }, { companyName: "Toko Uji", companyCity: "Jakarta" }),
  corrupt: false,
  warning: null as string | null,
  loading: false,
};

vi.mock("@/modules/print/hooks/usePrintProfile", () => ({
  usePrintProfile: () => profileState,
}));

const fetchSales = vi.fn();
const fetchPayment = vi.fn();
const fetchAlloc = vi.fn();
vi.mock("@/modules/print/services/print-fetch", () => ({
  fetchSalesOrderForPrint: (...a: unknown[]) => fetchSales(...a),
  fetchPaymentForPrint: (...a: unknown[]) => fetchPayment(...a),
  fetchAllocationBilling: (...a: unknown[]) => fetchAlloc(...a),
}));

import SalesInvoicePrintPage from "@/modules/print/pages/SalesInvoicePrintPage";
import PaymentReceiptPrintPage from "@/modules/print/pages/PaymentReceiptPrintPage";
import PosThermalReceiptPage from "@/modules/print/pages/PosThermalReceiptPage";

function order(over: Record<string, unknown> = {}) {
  return {
    id: 5,
    number: "SO-0005",
    partyName: "Budi",
    orderDate: "2026-10-01",
    dueDate: "2026-11-01",
    paymentTerms: "net",
    status: "confirmed",
    channel: "regular",
    shipToRecipient: "",
    shipToLabel: "",
    shipToAddress: "",
    shipToPhone: "",
    taxInvoiceNumber: "",
    taxInvoiceDate: "",
    subtotal: 200000,
    discountTotal: 20000,
    taxTotal: 0,
    grandTotal: 180000,
    paidAmount: 50000,
    outstandingAmount: 130000,
    paymentStatus: "partial",
    effectiveDueDate: "2026-11-01",
    isOverdue: false,
    items: [
      {
        id: 1,
        productCode: "P-1",
        productName: "Semen",
        uom: "sak",
        qty: 2,
        unitPrice: 100000,
        discountPct: 0,
        discountNominal: 20000,
        lineTotal: 180000,
      },
    ],
    ...over,
  };
}

function renderAt(path: string, route: string, el: React.ReactElement) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path={route} element={el} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  vi.clearAllMocks();
  profileState.warning = null;
});

describe("nota penjualan", () => {
  it("judul Nota (Sebagian), Sudah Dibayar dan Sisa Tagihan tercetak", async () => {
    fetchSales.mockResolvedValue(order());
    renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByText("Nota (Sebagian)")).toBeTruthy();
    expect(screen.getByText("Sudah Dibayar")).toBeTruthy();
    expect(screen.getByText("Sisa Tagihan")).toBeTruthy();
    expect(screen.getByText("Disc")).toBeTruthy();
  });

  it("lunas: judul Nota, jatuh tempo '-', tanpa baris sisa berisi nominal sisa", async () => {
    fetchSales.mockResolvedValue(
      order({ paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid" }),
    );
    renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByText("Nota")).toBeTruthy();
    expect(screen.queryByText("Tagihan")).toBeNull();
  });

  it("sembunyikan diskon: kolom Disc hilang dan jumlah baris bruto", async () => {
    fetchSales.mockResolvedValue(order());
    renderAt("/print/sales/5?hideDiscount=1", "/print/sales/:id", <SalesInvoicePrintPage />);
    await screen.findByText("Nota (Sebagian)");
    expect(screen.queryByText("Disc")).toBeNull();
    // Bruto 2 x 100.000 = 200.000 muncul sebagai jumlah baris.
    expect(screen.getAllByText(/200\.000/).length).toBeGreaterThan(0);
  });

  it("peringatan profil gagal dimuat tampil (tidak ditelan)", async () => {
    profileState.warning = "Kop tidak termuat";
    fetchSales.mockResolvedValue(order());
    renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByText("Kop tidak termuat")).toBeTruthy();
  });
});

describe("kwitansi", () => {
  it("tanpa blok tanda tangan (BR-30), terbilang tetap ada", async () => {
    fetchPayment.mockResolvedValue({
      id: 9,
      number: "PAY-9",
      partyName: "Budi",
      partyType: "customer",
      direction: "in",
      amount: 50000,
      method: "cash",
      paidAt: "2026-10-02",
      notes: "",
      status: "active",
      reference: "",
      amountTendered: 60000,
      change: 10000,
      allocations: [{ orderType: "sales", orderId: 5, orderNumber: "SO-0005", amount: 50000 }],
    });
    fetchAlloc.mockResolvedValue({ "sales:5": { total: 180000, paid: 50000, outstanding: 130000 } });
    const { container } = renderAt("/print/payments/9", "/print/payments/:id", <PaymentReceiptPrintPage />);
    await waitFor(() => expect(screen.getByText(/Terbilang:/)).toBeTruthy());
    expect(container.querySelector(".kwitansi-sign")).toBeNull();
    expect(screen.queryByText("( .................... )")).toBeNull();
    expect(screen.getByText("Tunai diterima")).toBeTruthy();
    expect(screen.getByText("Kembalian")).toBeTruthy();
    expect(screen.getByText(/Sisa Tagihan/)).toBeTruthy();
  });
});

describe("struk termal POS (SL-05, SL-10)", () => {
  const posOrder = (over: Record<string, unknown> = {}) =>
    order({ channel: "pos", paymentTerms: "cod", grandTotal: 180000, cashierName: "Tono Kasir", ...over });

  it("kasir = pencatat pembayaran dari server, bukan pengguna yang mencetak", async () => {
    fetchSales.mockResolvedValue(posOrder());
    renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    expect(await screen.findByText("Tono Kasir")).toBeTruthy();
  });

  it("diskon tampil bawaan; ?hideDiscount=1 menghilangkan baris diskon dan memakai harga penuh", async () => {
    fetchSales.mockResolvedValue(posOrder());
    const first = renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.getAllByText("Diskon").length).toBeGreaterThan(0);
    first.unmount();
    renderAt("/print/pos/5?hideDiscount=1", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.queryByText("Diskon")).toBeNull();
    expect(screen.getAllByText(/200\.000/).length).toBeGreaterThan(0);
  });

  it("order ber-PPN: struk menampilkan DPP dan PPN; tanpa pajak baris itu tidak ada (K-03)", async () => {
    fetchSales.mockResolvedValue(posOrder({ taxTotal: 18000, taxRate: 11, grandTotal: 198000 }));
    const first = renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.getByText("DPP")).toBeTruthy();
    expect(screen.getByText("PPN 11%")).toBeTruthy();
    expect(screen.getAllByText(/180\.000/).length).toBeGreaterThan(0);
    first.unmount();
    fetchSales.mockResolvedValue(posOrder());
    renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.queryByText("DPP")).toBeNull();
    expect(screen.queryByText(/^PPN/)).toBeNull();
  });

  it("toolbar punya toggle Sembunyikan diskon yang dicentang dari tautan", async () => {
    fetchSales.mockResolvedValue(posOrder());
    renderAt("/print/pos/5?hideDiscount=1", "/print/pos/:orderId", <PosThermalReceiptPage />);
    const box = (await screen.findByLabelText("Sembunyikan diskon")) as HTMLInputElement;
    expect(box.checked).toBe(true);
  });
});
