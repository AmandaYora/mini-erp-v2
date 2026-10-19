import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const taxAdjustments = vi.fn();
const accounts = vi.fn();
const taxAdjustmentFiles = vi.fn();
const attachTaxAdjustment = vi.fn();
const fiscalSummary = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    taxAdjustments: (...a: unknown[]) => taxAdjustments(...a),
    accounts: (...a: unknown[]) => accounts(...a),
    taxAdjustmentFiles: (...a: unknown[]) => taxAdjustmentFiles(...a),
    attachTaxAdjustment: (...a: unknown[]) => attachTaxAdjustment(...a),
    createTaxAdjustment: vi.fn(),
  },
}));
vi.mock("@/modules/finance/services/reports.service", () => ({
  financeReportsService: { fiscalSummary: (...a: unknown[]) => fiscalSummary(...a) },
}));

import TaxAdjustmentsPage from "@/modules/finance/pages/TaxAdjustmentsPage";

const adjustments = [
  {
    id: 21, number: "JV-21", date: "2026-10-05", memo: "Koreksi fiskal: beban non-deductible", status: "posted",
    classification: "positive", classLabel: "Koreksi positif", note: "Sumbangan tanpa bukti",
  },
  {
    id: 22, number: "JV-22", date: "2026-10-06", memo: "Koreksi fiskal: lama", status: "posted",
    classification: "", classLabel: "Belum diklasifikasi", note: "",
  },
];

beforeEach(() => {
  for (const m of [taxAdjustments, accounts, taxAdjustmentFiles, attachTaxAdjustment, fiscalSummary]) m.mockReset();
  taxAdjustments.mockResolvedValue(adjustments);
  accounts.mockResolvedValue([]);
  taxAdjustmentFiles.mockResolvedValue([{ id: 1, name: "bukti.png", mime: "image/png", sizeBytes: 1000, url: "/media/bukti.png" }]);
  attachTaxAdjustment.mockResolvedValue({ message: "ok", data: {} });
  fiscalSummary.mockResolvedValue({
    commercial: { revenue: 0, cogs: 0, gross: 0, expense: 0, profit: 1_000_000, from: "", to: "" },
    lines: [],
    classes: [
      { classification: "positive", label: "Koreksi positif", entries: 1, effect: -1000 },
      { classification: "final_income", label: "Penghasilan final", entries: 1, effect: -400 },
    ],
    fiscalProfit: 998_600, from: "", to: "",
  });
  useAuthStore.setState({ permissions: ["finance.view", "finance.tax_adjustment.view", "finance.tax_adjustment.manage"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <TaxAdjustmentsPage />
    </MemoryRouter>,
  );
}

describe("TaxAdjustmentsPage (FN-07)", () => {
  it("riwayat menampilkan jenis dan catatan koreksi", async () => {
    renderPage();
    expect(await screen.findByText("Sumbangan tanpa bukti")).toBeInTheDocument();
    expect(screen.getAllByText("Koreksi positif").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Belum diklasifikasi").length).toBeGreaterThan(0);
  });

  it("rekonsiliasi memisahkan kelas koreksi", async () => {
    renderPage();
    expect(await screen.findByText("Penghasilan final")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Koreksi fiskal per jenis" })).toBeInTheDocument();
  });

  it("lampiran dapat dilihat dan ditambah", async () => {
    renderPage();
    const buttons = await screen.findAllByRole("button", { name: "Lihat" });
    fireEvent.click(buttons[0]);
    expect(await screen.findByAltText("bukti.png")).toBeInTheDocument();
    expect(taxAdjustmentFiles).toHaveBeenCalledWith(21);
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(["x"], "baru.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => expect(attachTaxAdjustment).toHaveBeenCalledWith(21, file));
  });
});
