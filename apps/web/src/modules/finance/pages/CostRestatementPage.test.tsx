import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { RestateReport } from "@/modules/finance/types";

const restatePreview = vi.fn();
const restateCommit = vi.fn();
vi.mock("@/modules/finance/services/corrections.service", () => ({
  correctionsService: {
    restatePreview: (...a: unknown[]) => restatePreview(...a),
    restateCommit: (...a: unknown[]) => restateCommit(...a),
  },
}));

import CostRestatementPage from "@/modules/finance/pages/CostRestatementPage";

const recon = { book: 49000, ledger: 49000, diff: 0, tolerance: 101, pending: 0, pendingValue: 0, ignoredValue: 0, unexplained: 0, missingCost: 0, consistent: true };

function dry(over: Partial<RestateReport> = {}): RestateReport {
  return {
    commit: false,
    branchId: 1,
    positions: 2,
    rowsChanged: 1,
    products: [
      { productId: 7, variantId: 9, productCode: "RST-1", productName: "Barang Uji", rows: 1, valueBefore: 55000, valueAfter: 49000, delta: -6000, offsets: { cogs: -6000 } },
    ],
    skipped: null,
    delta: -6000,
    gates: [
      { key: "trial_balance", ok: true, detail: "debit 1, kredit 1" },
      { key: "inventory_recon", ok: true, detail: "selisih 0" },
      { key: "pending_documents", ok: true, detail: "0 dokumen" },
      { key: "period_open", ok: true, detail: "periode 2026-10" },
    ],
    reconBefore: recon,
    applied: false,
    date: "2026-10-21",
    cutoverMovementId: 120,
    ...over,
  };
}

beforeEach(() => {
  restatePreview.mockReset();
  restateCommit.mockReset();
  restatePreview.mockResolvedValue({ message: "ok", data: dry() });
  restateCommit.mockResolvedValue({
    message: "HPP dihitung ulang",
    data: dry({ commit: true, applied: true, entryId: 31, entryNumber: "JV-TST-2026-0007", reconAfter: recon }),
  });
  useAuthStore.setState({ permissions: ["finance.view", "finance.manage", "finance.close.force"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <CostRestatementPage />
    </MemoryRouter>,
  );
}

describe("CostRestatementPage", () => {
  it("menampilkan gerbang, selisih per produk, dan batas migrasi dari dry run", async () => {
    renderPage();
    expect(await screen.findByText("RST-1")).toBeInTheDocument();
    expect(screen.getByText("Neraca saldo seimbang")).toBeInTheDocument();
    expect(screen.getByText(/mutasi stok #120 ke bawah/)).toBeInTheDocument();
    expect(restatePreview).toHaveBeenCalledTimes(1);
  });

  it("komit menolak alasan kosong lalu mengirim alasan dan menampilkan nomor jurnal", async () => {
    renderPage();
    await screen.findByText("RST-1");
    fireEvent.click(screen.getByRole("button", { name: "Bukukan koreksi" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(dialog.querySelector('button[type="submit"]') as HTMLButtonElement);
    expect(await screen.findByText("Alasan wajib diisi")).toBeInTheDocument();
    expect(restateCommit).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText(/Alasan koreksi/), { target: { value: " HPP basi " } });
    fireEvent.click(dialog.querySelector('button[type="submit"]') as HTMLButtonElement);
    await waitFor(() => expect(restateCommit).toHaveBeenCalledWith("HPP basi", expect.any(String)));
    expect(await screen.findByText("JV-TST-2026-0007")).toBeInTheDocument();
  });

  it("gerbang gagal menonaktifkan tombol bukukan", async () => {
    restatePreview.mockResolvedValue({
      message: "ok",
      data: dry({ gates: [{ key: "pending_documents", ok: false, detail: "3 dokumen belum dijurnal" }] }),
    });
    renderPage();
    expect(await screen.findByText("3 dokumen belum dijurnal")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bukukan koreksi" })).toBeDisabled();
  });

  it("tanpa finance.close.force hanya pratinjau", async () => {
    useAuthStore.setState({ permissions: ["finance.view", "finance.manage"], ready: true });
    renderPage();
    await screen.findByText("RST-1");
    expect(screen.queryByRole("button", { name: "Bukukan koreksi" })).toBeNull();
    expect(screen.getByText(/Halaman ini hanya pratinjau/)).toBeInTheDocument();
  });
});
