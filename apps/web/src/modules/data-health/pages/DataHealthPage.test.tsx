import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { makeAlert, makeReport, minPriceAlert } from "@/modules/data-health/test/fixtures";

const report = vi.fn();
const items = vi.fn();
const snooze = vi.fn();
const unsnooze = vi.fn();
vi.mock("@/modules/data-health/services/data-health.service", () => ({
  dataHealthService: {
    report: (...a: unknown[]) => report(...a),
    items: (...a: unknown[]) => items(...a),
    snooze: (...a: unknown[]) => snooze(...a),
    unsnooze: (...a: unknown[]) => unsnooze(...a),
  },
}));

import DataHealthPage from "@/modules/data-health/pages/DataHealthPage";

beforeEach(() => {
  for (const m of [report, items, snooze, unsnooze]) m.mockReset();
  report.mockResolvedValue(makeReport());
  items.mockResolvedValue({ items: [], meta: { page: 1, limit: 20, total: 0, totalPages: 0 } });
  useAuthStore.setState({ permissions: ["dashboard.alerts.view_all", "products.view"], ready: true });
  try {
    localStorage.clear();
  } catch {
    // tanpa storage: jejak penyelesaian tidak dirender
  }
});
afterEach(cleanup);

function renderPage(url = "/data-health") {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <DataHealthPage />
    </MemoryRouter>,
  );
}

describe("DataHealthPage", () => {
  it("mengelompokkan alert per area, area beres jadi satu baris tenang", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { level: 2, name: "Persediaan" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Master" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 2, name: "Keuangan" })).not.toBeInTheDocument();
    expect(screen.getByText(/Keuangan: semua pemeriksaan lolos \(2 pemeriksaan\)/)).toBeInTheDocument();
    // Ringkasan diumumkan sebagai status.
    expect(screen.getByRole("status")).toHaveTextContent("2 kritis · 1 perhatian · 1 info");
  });

  it("alert keamanan tanpa tombol Tunda, dengan petunjuk; tombol aksi disembunyikan bila tanpa izin", async () => {
    useAuthStore.setState({ permissions: ["dashboard.alerts.view_all"], ready: true });
    renderPage();
    expect(await screen.findByText("Superadmin masih memakai sandi bawaan")).toBeInTheDocument();
    expect(screen.getByText("Tidak bisa ditunda")).toBeInTheDocument();
    expect(screen.getByText(/Minta pemegang akun superadmin/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Perbaiki harga/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tunda: Harga minimum di atas harga jual" })).toBeInTheDocument();
  });

  it("dialog Tunda: alasan wajib ≥5 karakter, opsi 14 hari tidak ada untuk alert kritis", async () => {
    snooze.mockResolvedValue({ data: makeAlert({ snooze: { ...makeAlert().snooze, snoozedUntil: "2026-10-24T03:00:00Z" } }), message: "Alert ditunda" });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Tunda: Harga modal > 1,2× harga jual" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByLabelText(/14 hari/)).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(/7 hari/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Tunda" }));
    expect(await within(dialog).findByText("Alasan wajib diisi")).toBeInTheDocument();
    fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "abc" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Tunda" }));
    expect(await within(dialog).findByText("Alasan minimal 5 karakter")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Menunggu pemasok" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Tunda" }));
    await waitFor(() => expect(snooze).toHaveBeenCalledWith("inventory.cost_above_price", { days: 1, reason: "abc. Menunggu pemasok" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(report).toHaveBeenCalledTimes(2);
  });

  it("galat API tetap di dialog dengan pesan server", async () => {
    snooze.mockRejectedValue({ isAxiosError: true, response: { status: 409, data: { success: false, message: "Alert ini sudah selesai, tidak perlu ditunda" } } });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Tunda: Harga minimum di atas harga jual" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/14 hari/)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "Sedang dikerjakan" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Tunda" }));
    expect(await within(dialog).findByText(/Alert ini sudah selesai/)).toBeInTheDocument();
  });

  it("tab Ditunda menampilkan penunda dan Aktifkan lagi", async () => {
    const snoozed = { ...minPriceAlert, snooze: { ...minPriceAlert.snooze, snoozedAt: "2026-10-23T03:00:00Z", snoozedUntil: "2026-10-26T03:00:00Z", reason: "Menunggu pemasok", byName: "Rina", countAtSnooze: 40 } };
    report.mockResolvedValue(makeReport({ snoozed: [snoozed] }));
    unsnooze.mockResolvedValue({ data: { alertId: snoozed.id, snoozed: false }, message: "Alert diaktifkan lagi" });
    renderPage("/data-health?tab=ditunda");
    expect(await screen.findByText(/Ditunda oleh Rina/)).toBeInTheDocument();
    expect(screen.getByText("Berkurang 2 sejak ditunda 23 Okt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Aktifkan lagi: Harga minimum di atas harga jual" }));
    await waitFor(() => expect(unsnooze).toHaveBeenCalledWith("master.min_price_above_selling"));
  });

  it("?alert= membuka daftar rinci dengan peringatan jurnal 5100", async () => {
    renderPage("/data-health?alert=inventory.cost_above_price");
    expect(await screen.findByText(/menjurnal lawan 5100/)).toBeInTheDocument();
    await waitFor(() => expect(items).toHaveBeenCalledWith("inventory.cost_above_price", 1, 20, expect.anything()));
  });

  it("semua lolos hanya bila tidak ada alert, galat, dan pemeriksaan tertunda", async () => {
    const empty = makeReport().areas.map((a) => ({ ...a, items: [] }));
    report.mockResolvedValue(makeReport({ areas: empty, bySeverity: { critical: 0, warning: 0, info: 0 }, pending: true }));
    renderPage();
    expect(await screen.findByText("Belum ada masalah ditemukan")).toBeInTheDocument();
    expect(screen.getByText(/Sebagian pemeriksaan masih dihitung/)).toBeInTheDocument();
  });

  it("pemeriksaan gagal tampil netral di areanya, tidak pernah sebagai lolos", async () => {
    report.mockResolvedValue(
      makeReport({
        failedChecks: [{ id: "finance.period_unclosed", label: "Periode yang belum ditutup", message: "Pemeriksaan tidak dapat dijalankan, coba Periksa ulang" }],
        checks: [{ id: "finance.period_unclosed", label: "Periode yang belum ditutup", area: "keuangan", status: "error", tier: "fast", computedAt: "" }],
      }),
    );
    renderPage();
    expect(await screen.findByText("Periode yang belum ditutup tidak terperiksa")).toBeInTheDocument();
    expect(screen.queryByText(/Keuangan: semua pemeriksaan lolos/)).not.toBeInTheDocument();
  });

  it("galat memuat menampilkan pesan API", async () => {
    report.mockRejectedValue({ isAxiosError: true, response: { status: 500, data: { success: false, message: "Terjadi kesalahan internal" } } });
    renderPage();
    expect(await screen.findByText("Gagal memuat kesehatan data")).toBeInTheDocument();
  });
});
