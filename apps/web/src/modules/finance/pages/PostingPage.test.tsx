import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const postingSources = vi.fn();
const orphans = vi.fn();
const cutover = vi.fn();
const dailyOverview = vi.fn();
const ignorePosting = vi.fn();
const restorePosting = vi.fn();
const closeDay = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    postingSources: (...a: unknown[]) => postingSources(...a),
    orphans: (...a: unknown[]) => orphans(...a),
    cutover: (...a: unknown[]) => cutover(...a),
    dailyOverview: (...a: unknown[]) => dailyOverview(...a),
    ignorePosting: (...a: unknown[]) => ignorePosting(...a),
    restorePosting: (...a: unknown[]) => restorePosting(...a),
    postBatch: vi.fn(),
    closeDay: (...a: unknown[]) => closeDay(...a),
    setCutover: vi.fn(),
    preview: vi.fn(),
    post: vi.fn(),
  },
}));

import PostingPage from "@/modules/finance/pages/PostingPage";

const queue = [
  {
    docType: "delivery", docId: 11, number: "SJ-0011", date: "2026-10-12", periodClosed: false,
    amount: 1_500_000, amountKind: "order", partyId: 3, partyName: "Toko Budi", summary: "Pesanan SO-0007",
  },
  {
    docType: "payment", docId: 5, number: "PAY-0005", date: "2026-10-11", periodClosed: false,
    amount: 250_000, amountKind: "document", partyName: "Toko Ani", summary: "Pembayaran masuk (cash)",
  },
  {
    docType: "goods_receipt", docId: 8, number: "Penerimaan GR-0008", date: "2026-10-10", periodClosed: false,
    ignored: true, ignoreReason: "Barang uji coba", amount: 0,
  },
];

const overview = {
  date: "2026-10-12",
  status: "selesai_catatan",
  activity: 4,
  totals: {
    sales: { amount: 3_000_000, count: 2 }, purchases: { amount: 800_000, count: 1 },
    returns: { amount: 100_000, count: 1 }, cashIn: 2_000_000, cashOut: 500_000,
  },
  ready: { today: 0, backlog: 1 },
  tasks: [
    {
      code: "missing_cost_basis", label: "Harga modal belum ada", hint: "Lengkapi harga modal produk", count: 1,
      items: [{ docType: "delivery", docId: 11, number: "SJ-0011", date: "2026-10-12", amount: 0, message: "Harga modal produk X belum tersedia" }],
    },
  ],
  backlog: { count: 2, ready: 1, needsInput: 1, tasks: [] },
  ignored: 1,
  dataWarning: { invalidDateCount: 0 },
  probed: 1,
  probeCapped: false,
};

beforeEach(() => {
  for (const m of [postingSources, orphans, cutover, dailyOverview, ignorePosting, restorePosting, closeDay]) m.mockReset();
  postingSources.mockResolvedValue(queue);
  orphans.mockResolvedValue([]);
  cutover.mockResolvedValue({});
  dailyOverview.mockResolvedValue(overview);
  ignorePosting.mockResolvedValue({ message: "Dokumen diabaikan dari antrian posting", data: {} });
  restorePosting.mockResolvedValue({ message: "Dokumen dikembalikan ke antrian posting", data: null });
  useAuthStore.setState({ permissions: ["finance.view", "finance.posting.view", "finance.posting.post"], ready: true });
});
afterEach(cleanup);

function renderPage(entry = "/") {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <PostingPage />
    </MemoryRouter>,
  );
}

describe("PostingPage: antrian", () => {
  it("menampilkan pihak, ringkasan, dan nominal tiap dokumen (FN-10)", async () => {
    renderPage();
    expect(await screen.findByText("Toko Budi")).toBeInTheDocument();
    expect(screen.getByText("Pesanan SO-0007")).toBeInTheDocument();
    expect(screen.getByText("total pesanan")).toBeInTheDocument();
    expect(screen.getByText("Toko Ani")).toBeInTheDocument();
    expect(screen.getByText(/250\.000/)).toBeInTheDocument();
  });

  it("dokumen diabaikan tampil terpisah dengan alasan dan tidak ikut hitungan menunggu (FN-01)", async () => {
    renderPage();
    expect((await screen.findAllByText("Diabaikan")).length).toBeGreaterThan(0);
    expect(screen.getByText("Barang uji coba")).toBeInTheDocument();
    // Dua dokumen siap (delivery + payment); yang diabaikan tidak dihitung.
    expect(screen.getAllByText("2").length).toBeGreaterThan(0);
    expect(screen.queryByText("3")).not.toBeInTheDocument();
  });

  it("mengabaikan dokumen wajib beralasan lalu memanggil API", async () => {
    renderPage();
    await screen.findByText("Toko Budi");
    fireEvent.click(screen.getAllByRole("button", { name: /Abaikan/ })[0]);
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Abaikan" }));
    expect(await within(dialog).findByText("Alasan wajib diisi")).toBeInTheDocument();
    expect(ignorePosting).not.toHaveBeenCalled();
    fireEvent.change(within(dialog).getByLabelText(/Alasan/), { target: { value: "  SJ duplikat  " } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Abaikan" }));
    await waitFor(() => expect(ignorePosting).toHaveBeenCalledWith("delivery", 11, "SJ duplikat"));
  });

  it("memulihkan dokumen yang diabaikan", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Pulihkan/ }));
    await waitFor(() => expect(restorePosting).toHaveBeenCalledWith("goods_receipt", 8));
  });
});

describe("PostingPage: ikhtisar tutup hari (FN-02)", () => {
  it("menampilkan status hari, angka, tugas butuh data pemilik, dan backlog", async () => {
    renderPage("/?tab=tutup-hari");
    expect(await screen.findByText("Selesai dengan catatan")).toBeInTheDocument();
    expect(screen.getByText("Penjualan")).toBeInTheDocument();
    expect(screen.getByText(/3\.000\.000/)).toBeInTheDocument();
    expect(screen.getByText("Harga modal belum ada")).toBeInTheDocument();
    expect(screen.getByText("Harga modal produk X belum tersedia")).toBeInTheDocument();
    expect(screen.getByText(/Backlog hari sebelumnya/)).toBeInTheDocument();
    expect(dailyOverview).toHaveBeenCalled();
  });
});

describe("PostingPage: tutup hari melaporkan sisa antrean (N-52)", () => {
  async function runCloseDay() {
    renderPage("/?tab=tutup-hari");
    fireEvent.click(await screen.findByRole("button", { name: /^Tutup hari \(/ }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /^Posting \d+ dokumen/ }));
  }

  it("menampilkan 'N diposting, M tersisa — jalankan lagi' bila antrean belum habis", async () => {
    closeDay.mockResolvedValue({
      message: "Tutup harian: 1 dokumen diposting, 1 gagal, 3 tersisa",
      data: { results: [{ docType: "delivery", docId: 11, entryId: 1 }, { docType: "payment", docId: 5, error: "Akun belum dipetakan" }], posted: 1, failed: 1, remaining: 3 },
    });
    await runCloseDay();
    await waitFor(() => expect(closeDay).toHaveBeenCalled());
    expect(await screen.findByText("1 diposting, 3 tersisa — jalankan lagi")).toBeInTheDocument();
    expect(screen.getByText(/1 dokumen gagal diposting/)).toBeInTheDocument();
  });

  it("tanpa sisa tidak ada peringatan", async () => {
    closeDay.mockResolvedValue({
      message: "Tutup harian: 2 dokumen diposting",
      data: { results: [{ docType: "delivery", docId: 11, entryId: 1 }, { docType: "payment", docId: 5, entryId: 2 }], posted: 2, failed: 0, remaining: 0 },
    });
    await runCloseDay();
    await waitFor(() => expect(closeDay).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByText(/tersisa — jalankan lagi/)).not.toBeInTheDocument());
  });
});
