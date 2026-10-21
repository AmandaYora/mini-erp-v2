import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { todayWIB } from "@/shared/lib/format";

const periods = vi.fn();
const taxPeriods = vi.fn();
const reopenPeriod = vi.fn();
const reopenTaxPeriod = vi.fn();
const readiness = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    periods: (...a: unknown[]) => periods(...a),
    taxPeriods: (...a: unknown[]) => taxPeriods(...a),
    reopenPeriod: (...a: unknown[]) => reopenPeriod(...a),
    reopenTaxPeriod: (...a: unknown[]) => reopenTaxPeriod(...a),
    closePeriod: vi.fn(),
    closeTaxPeriod: vi.fn(),
    safeClose: vi.fn(),
    readiness: (...a: unknown[]) => readiness(...a),
  },
}));

import PeriodsPage from "@/modules/finance/pages/PeriodsPage";

const year = Number(todayWIB().slice(0, 4));

beforeEach(() => {
  for (const m of [periods, taxPeriods, reopenPeriod, reopenTaxPeriod, readiness]) m.mockReset();
  periods.mockResolvedValue([{ year, month: 3, status: "closed" }]);
  taxPeriods.mockResolvedValue([{ year, month: 3, status: "closed" }]);
  reopenPeriod.mockResolvedValue({ message: "Periode dibuka", data: null });
  reopenTaxPeriod.mockResolvedValue({ message: "Periode pajak dibuka", data: null });
  useAuthStore.setState({ permissions: ["finance.view", "finance.manage", "finance.close.force"], ready: true });
});
afterEach(cleanup);

function renderPage(entry = "/") {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <PeriodsPage />
    </MemoryRouter>,
  );
}

describe("PeriodsPage: buka ulang periode butuh alasan (FN-05)", () => {
  it("menolak buka ulang tanpa alasan dan tidak memanggil API", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: new RegExp(`Maret ${year}: terkunci`) }));
    expect(await screen.findByText("Buka ulang periode?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Buka ulang" }));
    expect(await screen.findByText("Alasan membuka kembali periode wajib diisi")).toBeInTheDocument();
    expect(reopenPeriod).not.toHaveBeenCalled();
  });

  it("mengirim alasan ke API fiskal", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: new RegExp(`Maret ${year}: terkunci`) }));
    fireEvent.change(await screen.findByLabelText(/Alasan/), { target: { value: "  Koreksi jurnal penyesuaian  " } });
    fireEvent.click(screen.getByRole("button", { name: "Buka ulang" }));
    await waitFor(() => expect(reopenPeriod).toHaveBeenCalledTimes(1));
    expect(reopenPeriod).toHaveBeenCalledWith({ year, month: 3, reason: "Koreksi jurnal penyesuaian" });
  });

  it("periode pajak juga mengirim alasan", async () => {
    renderPage("/?tab=pajak");
    fireEvent.click(await screen.findByRole("button", { name: new RegExp(`Maret ${year}: terkunci`) }));
    fireEvent.change(await screen.findByLabelText(/Alasan/), { target: { value: "Salah snapshot" } });
    fireEvent.click(screen.getByRole("button", { name: "Buka ulang" }));
    await waitFor(() => expect(reopenTaxPeriod).toHaveBeenCalledWith({ year, month: 3, reason: "Salah snapshot" }));
  });
});

describe("PeriodsPage: daftar periksa kesiapan tutup periode", () => {
  it("merender gerbang 'direct_unordered' dengan label dan keterangan dari server (ADR-0022)", async () => {
    readiness.mockResolvedValue({
      year, month: 9, closed: false, unposted: 2, unbalanced: 0, missingTaxInvoice: 0, missingSupplierInvoice: 0,
      latePeriodDocs: 0, orphans: 0, ready: false,
      blockers: ["2 surat jalan langsung belum diorder"], warnings: [],
      checks: [
        { key: "unposted", label: "Dokumen sudah diposting", status: "failed", detail: "2 dokumen belum diposting", count: 2 },
        { key: "direct_unordered", label: "Surat jalan langsung sudah diorder", status: "failed", detail: "2 surat jalan langsung belum diorder (tertua 2026-09-03)", count: 2 },
      ],
    });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Cek dan tutup aman" }));
    expect(await screen.findByText("Surat jalan langsung sudah diorder")).toBeInTheDocument();
    expect(screen.getByText("2 surat jalan langsung belum diorder (tertua 2026-09-03)")).toBeInTheDocument();
  });
});
