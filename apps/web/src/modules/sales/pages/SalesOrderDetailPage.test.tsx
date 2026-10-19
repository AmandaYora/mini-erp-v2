import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { SalesOrder } from "@/modules/sales/types";

const getOrder = vi.fn();
const cancelOrder = vi.fn();
vi.mock("@/modules/sales/services/sales.service", () => ({
  salesService: {
    get: (...a: unknown[]) => getOrder(...a),
    cancel: (...a: unknown[]) => cancelOrder(...a),
    confirm: vi.fn(),
    approveCredit: vi.fn(),
  },
}));

vi.mock("@/modules/payment/hooks/useOrderHistory", () => ({
  useOrderHistory: () => ({
    data: {
      events: [],
      progress: null,
      warnings: [],
      hidden: { docs: false, payments: false, returns: false },
    },
    loading: false,
    error: null,
    errorStatus: null,
    reload: vi.fn(),
  }),
}));

// Modal berat diganti penanda sederhana: yang diuji adalah gerbang aksinya.
vi.mock("@/modules/payment/components/OrderPaymentModal", () => ({
  OrderPaymentModal: () => null,
}));

import SalesOrderDetailPage from "@/modules/sales/pages/SalesOrderDetailPage";

function order(over: Partial<SalesOrder> = {}): SalesOrder {
  return {
    id: 7,
    number: "SO-0007",
    branchId: 1,
    partyId: 3,
    partyName: "Budi",
    channel: "regular",
    memberCode: "",
    orderDate: "2026-10-01",
    dueDate: "",
    paymentTerms: "cod",
    taxType: "none",
    taxRate: 0,
    subtotal: 100000,
    discountTotal: 0,
    taxTotal: 0,
    grandTotal: 100000,
    status: "confirmed",
    notes: "",
    shipToAddressId: 0,
    shipToLabel: "",
    shipToRecipient: "",
    shipToPhone: "",
    shipToAddress: "",
    taxInvoiceNumber: "",
    taxInvoiceDate: "",
    paidAmount: 0,
    outstandingAmount: 100000,
    paymentStatus: "unpaid",
    effectiveDueDate: "",
    isOverdue: false,
    items: [],
    ...over,
  };
}

function renderPage(path = "/sales-orders/7") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/sales-orders/:id" element={<SalesOrderDetailPage />} />
        <Route path="/sales-orders" element={<div>daftar</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

const ALL = [
  "sales.view",
  "sales.update",
  "sales.archive",
  "sales.approve",
  "delivery.create",
  "payment.create",
  "salesreturn.create",
];

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ALL, user: null, ready: true });
});
afterEach(() => cleanup());

async function openMenu() {
  await userEvent.click(await screen.findByRole("button", { name: "Aksi lainnya" }));
}

describe("detail order jual", () => {
  it("peran penuh: aksi di header sesuai status, termin, dan sisa tagihan", async () => {
    getOrder.mockResolvedValue(order());
    renderPage();
    // Satu primary (langkah berikut), sekunder terlihat, sisanya di menu.
    expect(await screen.findByRole("link", { name: "Terbitkan surat jalan" })).toHaveAttribute(
      "href",
      "/deliveries/new?from=7",
    );
    expect(screen.getByRole("button", { name: "Catat pembayaran" })).toBeTruthy();
    expect(screen.getByText("Cetak Tagihan")).toBeTruthy();
    expect(screen.getByText("Sisa tagihan")).toBeTruthy();
    // Tidak ada kartu "Aksi Order" maupun tombol kembali terpisah.
    expect(screen.queryByText("Aksi Order")).toBeNull();
    expect(screen.queryByText("Kembali ke daftar")).toBeNull();
    await openMenu();
    expect(screen.getByRole("menuitem", { name: /Setujui kredit/ })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Batalkan order" })).toBeTruthy();
    // Retur butuh barang yang sudah bergerak: belum ada dokumen terposting.
    expect(screen.queryByRole("menuitem", { name: "Retur / tukar" })).toBeNull();
  });

  it("rel dokumen menampilkan tahapan order", async () => {
    getOrder.mockResolvedValue(order());
    renderPage();
    await screen.findByRole("heading", { name: "SO-0007" });
    const steps = screen.getByRole("list", { name: "Tahapan" });
    expect(steps).toHaveTextContent("Draf");
    expect(steps).toHaveTextContent("Terkonfirmasi");
    expect(steps).toHaveTextContent("Dikirim");
    expect(steps).toHaveTextContent("Lunas");
    expect(steps).toHaveTextContent("Selesai");
  });

  it("draf: Konfirmasi order jadi primary", async () => {
    getOrder.mockResolvedValue(order({ status: "draft" }));
    renderPage();
    expect(await screen.findByRole("button", { name: "Konfirmasi order" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Ubah" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Terbitkan surat jalan" })).toBeNull();
  });

  it("kasir tanpa izin: tidak ada tombol aksi", async () => {
    useAuthStore.setState({ permissions: ["sales.view"] });
    getOrder.mockResolvedValue(order());
    renderPage();
    await screen.findByRole("heading", { name: "SO-0007" });
    expect(screen.queryByText("Terbitkan surat jalan")).toBeNull();
    expect(screen.queryByText("Catat pembayaran")).toBeNull();
    expect(screen.queryByRole("button", { name: "Aksi lainnya" })).toBeNull();
  });

  it("order prabayar belum lunas: surat jalan disembunyikan dan dijelaskan", async () => {
    getOrder.mockResolvedValue(order({ paymentTerms: "prepaid" }));
    renderPage();
    await screen.findByRole("heading", { name: "SO-0007" });
    expect(screen.queryByText("Terbitkan surat jalan")).toBeNull();
    expect(screen.getByText(/Surat jalan baru bisa diterbitkan setelah tagihan lunas/)).toBeTruthy();
    // Tanpa surat jalan, Catat pembayaran menjadi aksi utama.
    expect(screen.getByRole("button", { name: "Catat pembayaran" })).toBeTruthy();
  });

  it("lunas: tidak ada Catat pembayaran, judul cetak Nota", async () => {
    getOrder.mockResolvedValue(order({ paidAmount: 100000, outstandingAmount: 0, paymentStatus: "paid" }));
    renderPage();
    await screen.findByRole("heading", { name: "SO-0007" });
    expect(screen.queryByText("Catat pembayaran")).toBeNull();
    expect(screen.getByText("Cetak Nota")).toBeTruthy();
  });

  it("batal ditolak server: pesan penjaga tampil menetap", async () => {
    getOrder.mockResolvedValue(order());
    cancelOrder.mockRejectedValue({ status: 409, message: "Masih ada surat jalan aktif SJ-1" });
    renderPage();
    await openMenu();
    await userEvent.click(screen.getByRole("menuitem", { name: "Batalkan order" }));
    await userEvent.click(await screen.findByText("Ya, batalkan"));
    await waitFor(() => expect(screen.getByText(/Masih ada surat jalan aktif SJ-1/)).toBeTruthy());
  });

  it("id tidak ada: halaman 404 yang jelas", async () => {
    getOrder.mockRejectedValue({ status: 404, message: "Sales order tidak ditemukan" });
    renderPage();
    expect(await screen.findByText("Order jual tidak ditemukan")).toBeTruthy();
    expect(screen.getByText("404")).toBeTruthy();
  });

  it("id tidak valid: 404 tanpa memanggil server", async () => {
    renderPage("/sales-orders/abc");
    expect(await screen.findByText("Order jual tidak ditemukan")).toBeTruthy();
    expect(getOrder).not.toHaveBeenCalled();
  });
});

describe("detail order jual — orang, kredit, retur, alamat (SL-01/03/09)", () => {
  it("menampilkan dibuat oleh, penanggung jawab, persetujuan kredit, dan alamat sekali pakai", async () => {
    getOrder.mockResolvedValue(
      order({
        paymentTerms: "net",
        createdByName: "Dewi Pembuat",
        assignedUserName: "Rudi PJ",
        creditApprovedByName: "Ani Penyetuju",
        creditApprovedAt: "2026-10-05 10:00:00",
        shipToAddressId: 0,
        shipToRecipient: "Pak Joko",
        shipToPhone: "0812",
        shipToAddress: "Jl. Sementara 5",
      }),
    );
    renderPage();
    expect(await screen.findByText("Dewi Pembuat")).toBeInTheDocument();
    expect(screen.getByText("Rudi PJ")).toBeInTheDocument();
    expect(screen.getByText(/Ani Penyetuju/)).toBeInTheDocument();
    expect(screen.getByText("Alamat kirim (sekali pakai)")).toBeInTheDocument();
    expect(screen.getByText(/Pak Joko · 0812 · Jl\. Sementara 5/)).toBeInTheDocument();
  });

  it("order dengan retur: total bersih dan dana yang harus dikembalikan tampil", async () => {
    getOrder.mockResolvedValue(
      order({
        status: "completed",
        paidAmount: 100000,
        outstandingAmount: 0,
        paymentStatus: "paid",
        returnAdjustment: -40000,
        netTotal: 60000,
        refundDue: 40000,
      }),
    );
    renderPage();
    expect(await screen.findByText("Total bersih setelah retur")).toBeInTheDocument();
    expect(screen.getByText("Harus dikembalikan ke pelanggan")).toBeInTheDocument();
  });
});
