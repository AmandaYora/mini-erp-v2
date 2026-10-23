import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resolveIdentity } from "@/modules/print/document";
import { CONTINUOUS_PAPER_DEFAULTS, resolvePaperProfile } from "@/modules/print/paper";

// Dokumen yang dipegang pelanggan harus tercetak sama dengan sistem lama
// (identitas perusahaan): judul, label, urutan kolom, dan kaki dokumen.

const profileState = {
  profile: { continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } },
  paper: resolvePaperProfile(undefined),
  identity: resolveIdentity(
    {
      continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS },
      sellingPoints: ["Semen", "Cat"],
      returnNote: "Barang yang sudah dibeli tidak dapat dikembalikan.",
    },
    { companyName: "Toko Uji", companyCity: "Jakarta" },
  ),
  corrupt: false,
  warning: null as string | null,
  loading: false,
};

vi.mock("@/modules/print/hooks/usePrintProfile", () => ({
  usePrintProfile: () => profileState,
}));

const fetchSales = vi.fn();
const fetchReceipt = vi.fn();
const fetchAlloc = vi.fn();
const fetchPos = vi.fn();
const fetchDelivery = vi.fn();
vi.mock("@/modules/print/services/print-fetch", () => ({
  fetchSalesOrderForPrint: (...a: unknown[]) => fetchSales(...a),
  fetchPaymentReceipt: (...a: unknown[]) => fetchReceipt(...a),
  fetchAllocationBilling: (...a: unknown[]) => fetchAlloc(...a),
  fetchPosReceipt: (...a: unknown[]) => fetchPos(...a),
  fetchDeliveryPrint: (...a: unknown[]) => fetchDelivery(...a),
}));

import SalesInvoicePrintPage from "@/modules/print/pages/SalesInvoicePrintPage";
import PaymentReceiptPrintPage from "@/modules/print/pages/PaymentReceiptPrintPage";
import PosThermalReceiptPage from "@/modules/print/pages/PosThermalReceiptPage";
import DeliveryNotePrintPage from "@/modules/print/pages/DeliveryNotePrintPage";

function order(over: Record<string, unknown> = {}) {
  return {
    id: 5,
    number: "SO-0005",
    partyId: 7,
    partyName: "Budi",
    partyPhone: "0811-111",
    partyAddress: "Jl. Pelanggan 2",
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
    taxRate: 0,
    subtotal: 180000,
    discountTotal: 20000,
    taxTotal: 0,
    grandTotal: 180000,
    paidAmount: 50000,
    outstandingAmount: 130000,
    paymentStatus: "partial",
    effectiveDueDate: "2026-11-01",
    isOverdue: false,
    createdByName: "Sari Admin",
    items: [
      {
        id: 1,
        productId: 11,
        variantId: 110,
        productCode: "SMN-1",
        productName: "Semen",
        variantName: "Semen",
        variantCode: "SMN-1",
        uom: "sak",
        qty: 2,
        unitPrice: 100000,
        discountPct: 0,
        discountNominal: 20000,
        lineTotal: 180000,
        pricingSource: "catalog",
        basePrice: 100000,
        note: "",
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

describe("nota penjualan (sama dengan sistem lama)", () => {
  it("judul, label pihak, kolom, ringkasan, dan kaki dokumen", async () => {
    fetchSales.mockResolvedValue(order());
    const { container } = renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByText("Tagihan - Bayar Sebagian")).toBeTruthy();
    // Pihak: Nama / No HP / Alamat dari pelanggan bila tanpa alamat kirim.
    expect(screen.getByText("Nama")).toBeTruthy();
    expect(screen.getByText("0811-111")).toBeTruthy();
    expect(screen.getByText("Jl. Pelanggan 2")).toBeTruthy();
    expect(screen.getByText("No Nota")).toBeTruthy();
    expect(screen.getByText("Staff")).toBeTruthy();
    expect(screen.getByText("Sari Admin")).toBeTruthy();
    expect(screen.getByText("01 Oktober 2026")).toBeTruthy();
    expect(screen.getByText("01 November 2026")).toBeTruthy();
    const heads = Array.from(container.querySelectorAll("thead th")).map((th) => th.textContent);
    expect(heads).toEqual(["No", "Qty", "Satuan", "Produk", "Harga Satuan", "Disc", "Harga Jual"]);
    // Varian bawaan tidak tercetak ("Semen", bukan "Semen - Semen").
    expect(container.querySelector("tbody tr td:nth-child(4)")?.textContent).toBe("Semen");
    expect(container.querySelector("tbody tr td:nth-child(6)")?.textContent).toMatch(/^Rp\s20\.000$/);
    expect(screen.getByText("Jumlah (Rp)")).toBeTruthy();
    expect(screen.getByText("Sudah Dibayar")).toBeTruthy();
    expect(screen.getByText("Sisa Tagihan")).toBeTruthy();
    expect(screen.queryByText("PPN")).toBeNull();
    expect(screen.getByText(/^terbilang : seratus delapan puluh ribu rupiah$/)).toBeTruthy();
    expect(screen.getByText("TOKO UJI MENJUAL:")).toBeTruthy();
    expect(screen.getByText("Barang yang sudah dibeli tidak dapat dikembalikan.")).toBeTruthy();
    expect(screen.getByText("(................)")).toBeTruthy();
    // Sistem lama tidak mencetak blok rekening di nota.
    expect(screen.queryByText("REKENING TRANSFER MELALUI:")).toBeNull();
  });

  it("lunas: Nota Penjualan Lunas dan Tgl. Jtp '-'", async () => {
    fetchSales.mockResolvedValue(order({ paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid" }));
    renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByText("Nota Penjualan Lunas")).toBeTruthy();
    expect(screen.queryByText("01 November 2026")).toBeNull();
  });

  it("nota lunas + sembunyikan diskon: kolom Disc hilang, harga penuh, total naik", async () => {
    fetchSales.mockResolvedValue(order({ paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid" }));
    renderAt("/print/sales/5?hideDiscount=1", "/print/sales/:id", <SalesInvoicePrintPage />);
    await screen.findByText("Nota Penjualan Lunas");
    expect(screen.queryByText("Disc")).toBeNull();
    expect(screen.queryByText("Diskon")).toBeNull();
    expect(screen.getByText(/^terbilang : dua ratus ribu rupiah$/)).toBeTruthy();
    expect((screen.getByLabelText(/Sembunyikan diskon/) as HTMLInputElement).disabled).toBe(false);
  });

  it("tagihan bersisa: diskon tetap tercetak walau diminta disembunyikan, centang nonaktif + alasan (D12)", async () => {
    fetchSales.mockResolvedValue(order());
    renderAt("/print/sales/5?hideDiscount=1", "/print/sales/:id", <SalesInvoicePrintPage />);
    await screen.findByText("Tagihan - Bayar Sebagian");
    expect(screen.getByText("Disc")).toBeTruthy();
    expect(screen.getByText("Diskon")).toBeTruthy();
    expect(screen.getByText(/^terbilang : seratus delapan puluh ribu rupiah$/)).toBeTruthy();
    const box = screen.getByLabelText(/Sembunyikan diskon/) as HTMLInputElement;
    expect(box.disabled).toBe(true);
    expect(box.checked).toBe(false);
    expect(screen.getByText(/Tagihan harus menunjukkan jumlah yang benar-benar terutang/)).toBeTruthy();
  });

  it("peringatan profil gagal dimuat tampil (tidak ditelan)", async () => {
    profileState.warning = "Kop tidak termuat";
    fetchSales.mockResolvedValue(order());
    renderAt("/print/sales/5", "/print/sales/:id", <SalesInvoicePrintPage />);
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByText("Kop tidak termuat")).toBeTruthy();
  });
});

describe("surat jalan (sama dengan sistem lama)", () => {
  function note(over: Record<string, unknown> = {}) {
    return {
      id: 3,
      number: "SJ-0003",
      salesOrderId: 5,
      orderNumber: "SO-0005",
      deliveryDate: "2026-10-03",
      status: "confirmed",
      driverName: "Joko",
      vehiclePlate: "",
      warehouseStaffName: "",
      recipientName: "",
      dropLocationNote: "",
      notes: "",
      items: [{ id: 1, productId: 11, variantId: 110, locationId: 1, uom: "sak", uomFactor: 1, qty: 2, qtyBase: 2 }],
      ...over,
    };
  }

  it("STATUS = status bayar order; Staff; tanda tangan; tembusan karbon", async () => {
    fetchDelivery.mockResolvedValue({ note: note(), order: order() });
    const { container } = renderAt("/print/delivery/3", "/print/delivery/:id", <DeliveryNotePrintPage />);
    expect(await screen.findByText("SURAT JALAN")).toBeTruthy();
    expect(screen.getByText("STATUS : Bayar Sebagian")).toBeTruthy();
    expect(screen.getByText("Staff")).toBeTruthy();
    expect(screen.getByText("Sari Admin")).toBeTruthy();
    expect(screen.getByText("03 Oktober 2026")).toBeTruthy();
    expect(screen.getByText("Jl. Pelanggan 2")).toBeTruthy();
    // Kendaraan hanya bila ada pelat.
    expect(screen.queryByText("Kendaraan")).toBeNull();
    const heads = Array.from(container.querySelectorAll("thead th")).map((th) => th.textContent);
    expect(heads).toEqual(["No", "Qty", "Satuan", "Produk", "Keterangan"]);
    expect(screen.getByText("Petugas Gudang")).toBeTruthy();
    expect(screen.getByText("—")).toBeTruthy();
    expect(screen.getByText("PUTIH : SOPIR MERAH : PELANGGAN", { normalizer: (t) => t.replace(/\s+/g, " ").trim() })).toBeTruthy();
    expect(screen.queryByText("REKENING TRANSFER MELALUI:")).toBeNull();
  });

  it("20 barang = 3 lembar lengkap (kop, pihak, tanda tangan, Halaman X dari 3); No berlanjut; Bersambung kecuali terakhir", async () => {
    const items = Array.from({ length: 20 }, (_, i) => ({ id: i + 1, productId: 11, variantId: 110, locationId: 1, uom: "sak", uomFactor: 1, qty: i + 1, qtyBase: i + 1 }));
    fetchDelivery.mockResolvedValue({ note: note({ items }), order: order() });
    const { container } = renderAt("/print/delivery/3", "/print/delivery/:id", <DeliveryNotePrintPage />);
    await screen.findAllByText("SURAT JALAN");
    const sheets = Array.from(container.querySelectorAll<HTMLElement>(".sj-sheet"));
    expect(sheets).toHaveLength(3);
    sheets.forEach((sheet, i) => {
      const t = within(sheet);
      expect(t.getByText("SURAT JALAN")).toBeTruthy();
      expect(t.getByText("SJ-0003")).toBeTruthy();
      expect(t.getByText("Sari Admin")).toBeTruthy();
      expect(sheet.querySelectorAll(".sj-sign-box")).toHaveLength(3);
      expect(t.getByText(`Halaman ${i + 1} dari 3`)).toBeTruthy();
      expect(sheet.querySelectorAll("tbody tr")).toHaveLength(8); // 8 slot, pengisi di lembar terakhir
      expect(t.queryByText(/Bersambung ke halaman/) !== null).toBe(i < 2);
    });
    expect(sheets[1].querySelector("tbody td")?.textContent).toBe("9");
    expect(within(sheets[1]).getByText("Barang 9–16 dari 20")).toBeTruthy();
    expect(within(sheets[2]).getByText("Barang 17–20 dari 20")).toBeTruthy();
    expect(sheets[2].querySelectorAll("tr.sj-filler")).toHaveLength(4);
    expect(screen.getByText("3 halaman")).toBeTruthy();
  });

  it("satu barang tetap Halaman 1 dari 1 dengan 8 slot", async () => {
    fetchDelivery.mockResolvedValue({ note: note(), order: order() });
    const { container } = renderAt("/print/delivery/3", "/print/delivery/:id", <DeliveryNotePrintPage />);
    expect(await screen.findByText("Halaman 1 dari 1")).toBeTruthy();
    expect(container.querySelectorAll(".sj-sheet")).toHaveLength(1);
    expect(container.querySelectorAll("tbody tr")).toHaveLength(8);
    expect(screen.queryByText(/Bersambung/)).toBeNull();
  });

  it("varian nyata tercetak; surat jalan batal ditandai DIBATALKAN", async () => {
    const o = order();
    o.items[0] = { ...o.items[0], variantName: "50 KG", variantCode: "SMN-1-50" };
    fetchDelivery.mockResolvedValue({ note: note({ status: "cancelled", vehiclePlate: "K 1234 AB" }), order: o });
    renderAt("/print/delivery/3", "/print/delivery/:id", <DeliveryNotePrintPage />);
    expect(await screen.findByText("Semen - 50 KG")).toBeTruthy();
    expect(screen.getByText("STATUS : DIBATALKAN")).toBeTruthy();
    expect(screen.getByText("K 1234 AB")).toBeTruthy();
  });

  it("SJ Langsung tanpa order: pelanggan/penerima/alamat dari SJ, No Nota kosong, nama barang dari master (ADR-0022)", async () => {
    fetchDelivery.mockResolvedValue({
      note: note({
        salesOrderId: 0,
        orderNumber: "",
        origin: "direct",
        partyName: "PT Maju",
        shipToRecipient: "Pak Anton",
        shipToPhone: "0812-999",
        shipToAddress: "Jl. Proyek 1",
      }),
      order: null,
      products: [{ id: 11, code: "SMN", name: "Semen", variants: [{ id: 110, code: "SMN", name: "Semen", isHidden: true }] }],
    });
    const { container } = renderAt("/print/delivery/3", "/print/delivery/:id", <DeliveryNotePrintPage />);
    expect(await screen.findByText("SURAT JALAN")).toBeTruthy();
    expect(screen.getAllByText("PT Maju").length).toBeGreaterThan(0);
    expect(screen.getByText("Pak Anton")).toBeTruthy();
    expect(screen.getByText("0812-999")).toBeTruthy();
    expect(screen.getByText("Jl. Proyek 1")).toBeTruthy();
    expect(screen.getByText("Semen")).toBeTruthy();
    expect(screen.getByText("STATUS : Belum Diorder")).toBeTruthy();
    // Tata letak sama: label No Nota tetap ada, isinya kosong.
    expect(screen.getByText("No Nota")).toBeTruthy();
    expect(screen.queryByText("SO-0005")).toBeNull();
    const heads = Array.from(container.querySelectorAll("thead th")).map((th) => th.textContent);
    expect(heads).toEqual(["No", "Qty", "Satuan", "Produk", "Keterangan"]);
  });
});

describe("bukti pembayaran (sama dengan sistem lama)", () => {
  const payment = {
    id: 9,
    number: "PAY-9",
    partyId: 7,
    partyName: "Budi",
    partyType: "customer",
    partyPhone: "0811-111",
    partyAddress: "Jl. Pelanggan 2",
    direction: "in",
    amount: 50000,
    method: "transfer",
    paidAt: "2026-10-02T09:00:00+07:00",
    notes: "",
    status: "active",
    reference: "",
    amountTendered: 0,
    change: 0,
    allocations: [{ orderType: "sales", orderId: 5, orderNumber: "SO-0005", amount: 50000 }],
  };

  it("judul, pihak, rincian, dan ringkasan setelah pembayaran ini", async () => {
    fetchReceipt.mockResolvedValue({
      payment,
      order: { number: "SO-0005", total: 180000 },
      history: [
        { ...payment, id: 8, number: "PAY-8", amount: 30000, paidAt: "2026-10-01T09:00:00+07:00", allocations: [{ ...payment.allocations[0], amount: 30000 }] },
        payment,
      ],
    });
    const { container } = renderAt("/print/payments/9", "/print/payments/:id", <PaymentReceiptPrintPage />);
    expect(await screen.findByText("Bukti Pembayaran Customer")).toBeTruthy();
    expect(screen.getByText("Diterima Dari")).toBeTruthy();
    expect(screen.getByText("0811-111")).toBeTruthy();
    expect(screen.getByText("SO-0005")).toBeTruthy();
    expect(screen.getByText("Transfer Bank")).toBeTruthy();
    expect(screen.getByText("Ada Sisa")).toBeTruthy();
    const rows = Array.from(container.querySelectorAll(".kwitansi-summary tr")).map((tr) => tr.textContent?.replace(/\s+/g, " "));
    expect(rows).toEqual([
      "Total TagihanRp 180.000",
      "Jumlah Pembayaran IniRp 50.000",
      "Total Terbayar Setelah IniRp 80.000",
      "Sisa Setelah PembayaranRp 100.000",
    ]);
    expect(screen.getByText(/Bukti pembayaran ini dicatat dari sistem/)).toBeTruthy();
    // Sistem lama: tanpa tanda tangan (BR-30) dan tanpa terbilang.
    expect(container.querySelector(".kwitansi-sign")).toBeNull();
    expect(screen.queryByText(/Terbilang/i)).toBeNull();
  });

  it("pembayaran batal ditandai Dibatalkan", async () => {
    fetchReceipt.mockResolvedValue({ payment: { ...payment, status: "cancelled" }, order: null, history: [] });
    renderAt("/print/payments/9", "/print/payments/:id", <PaymentReceiptPrintPage />);
    expect(await screen.findByText("Dibatalkan")).toBeTruthy();
  });

  it("alokasi ke beberapa order: tabel alokasi menggantikan ringkasan satu order", async () => {
    fetchReceipt.mockResolvedValue({
      payment: {
        ...payment,
        allocations: [
          { orderType: "sales", orderId: 5, orderNumber: "SO-0005", amount: 20000 },
          { orderType: "sales", orderId: 6, orderNumber: "SO-0006", amount: 30000 },
        ],
      },
      order: null,
      history: [],
    });
    fetchAlloc.mockResolvedValue({ "sales:5": { total: 180000, paid: 20000, outstanding: 160000 } });
    renderAt("/print/payments/9", "/print/payments/:id", <PaymentReceiptPrintPage />);
    expect(await screen.findByText("Sisa Tagihan (saat cetak)")).toBeTruthy();
    expect(screen.getByText("SO-0005, SO-0006")).toBeTruthy();
    expect(screen.queryByText("Total Terbayar Setelah Ini")).toBeNull();
  });
});

describe("struk termal POS (sama dengan sistem lama)", () => {
  const posOrder = (over: Record<string, unknown> = {}) =>
    order({ channel: "pos", paymentTerms: "cod", paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid", cashierName: "Tono Kasir", ...over });
  const cashPayment = {
    id: 9,
    number: "PAY-9",
    method: "cash",
    paidAt: "2026-10-01T10:00:00+07:00",
    status: "active",
    amount: 180000,
    amountTendered: 200000,
    allocations: [],
  };

  it("cap LUNAS, No Bayar, kasir dari server, baris bruto + diskon (tidak dobel)", async () => {
    fetchPos.mockResolvedValue({ order: posOrder(), payments: [cashPayment] });
    renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    expect(await screen.findByText("Tono Kasir")).toBeTruthy();
    expect(screen.getByText("LUNAS")).toBeTruthy();
    expect(screen.getByText("PAY-9")).toBeTruthy();
    // Baris barang: 2 sak x Rp 100.000 = Rp 200.000, lalu Diskon -Rp 20.000.
    expect(screen.getByText(/2 sak x Rp\s100\.000/)).toBeTruthy();
    expect(screen.getByText(/-Rp\s20\.000/)).toBeTruthy();
    // "Tunai" = baris uang diterima dan metode bayar.
    expect(screen.getAllByText("Tunai")).toHaveLength(2);
    expect(screen.getByText("Kembalian")).toBeTruthy();
  });

  it("piutang tanpa pembayaran: PIUTANG, No Bayar '-', metode Tempo", async () => {
    fetchPos.mockResolvedValue({
      order: posOrder({ paymentTerms: "net", paidAmount: 0, outstandingAmount: 180000, paymentStatus: "unpaid" }),
      payments: [],
    });
    renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    expect(await screen.findByText("PIUTANG")).toBeTruthy();
    expect(screen.getByText("Tempo")).toBeTruthy();
    expect(screen.getByText("Sisa")).toBeTruthy();
  });

  it("?hideDiscount=1 menghilangkan baris diskon dan memakai harga penuh", async () => {
    fetchPos.mockResolvedValue({ order: posOrder(), payments: [cashPayment] });
    renderAt("/print/pos/5?hideDiscount=1", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.queryByText("Diskon")).toBeNull();
    const box = screen.getByLabelText("Sembunyikan diskon (harga penuh)") as HTMLInputElement;
    expect(box.checked).toBe(true);
  });

  it("struk piutang: ?hideDiscount=1 diabaikan, diskon tetap tercetak (D12)", async () => {
    fetchPos.mockResolvedValue({
      order: posOrder({ paymentTerms: "net", paidAmount: 0, outstandingAmount: 180000, paymentStatus: "unpaid" }),
      payments: [],
    });
    renderAt("/print/pos/5?hideDiscount=1", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("PIUTANG");
    expect(screen.getAllByText("Diskon").length).toBeGreaterThan(0);
    expect((screen.getByLabelText(/Sembunyikan diskon/) as HTMLInputElement).disabled).toBe(true);
  });

  it("order ber-PPN (K-03): struk merinci DPP dan PPN; tanpa pajak baris itu tidak ada", async () => {
    fetchPos.mockResolvedValue({ order: posOrder({ taxTotal: 18000, taxRate: 11, grandTotal: 198000 }), payments: [cashPayment] });
    const first = renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.getByText("DPP")).toBeTruthy();
    expect(screen.getByText("PPN 11%")).toBeTruthy();
    first.unmount();
    fetchPos.mockResolvedValue({ order: posOrder(), payments: [cashPayment] });
    renderAt("/print/pos/5", "/print/pos/:orderId", <PosThermalReceiptPage />);
    await screen.findByText("Tono Kasir");
    expect(screen.queryByText("DPP")).toBeNull();
    expect(screen.queryByText(/^PPN/)).toBeNull();
  });
});
