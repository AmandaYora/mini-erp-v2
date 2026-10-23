import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { AlertActionCard } from "@/modules/dashboard/components/ActionCard";
import { makeAlert, minPriceAlert, seedAlert } from "@/modules/data-health/test/fixtures";
import type { AlertSummary } from "@/modules/data-health/types";

function summary(over: Partial<AlertSummary> = {}): AlertSummary {
  return {
    items: [seedAlert, makeAlert(), minPriceAlert],
    totalActive: 8,
    bySeverity: { critical: 2, warning: 6, info: 2 },
    snoozedCount: 2,
    failedChecks: [],
    pending: false,
    computedAt: "2026-10-23T03:00:00Z",
    ...over,
  };
}

function renderCard(data: AlertSummary | null, extra: { loading?: boolean; error?: string | null } = {}) {
  return render(
    <MemoryRouter>
      <AlertActionCard
        alerts={{ data, loading: extra.loading ?? false, error: extra.error ?? null, reload: () => undefined }}
        today="2026-10-23"
      />
    </MemoryRouter>,
  );
}

beforeEach(() => useAuthStore.setState({ permissions: ["dashboard.alerts.view_all"], ready: true }));
afterEach(cleanup);

describe("AlertActionCard", () => {
  it("menampilkan alert dengan nama aksesibel lengkap dan tautan Lihat semua / ditunda", () => {
    renderCard(summary());
    expect(screen.getByRole("heading", { name: "Perlu tindakan (8)" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^Kritis: Harga modal > 1,2× harga jual, 64 produk · Rp/ })).toHaveAttribute(
      "href",
      "/data-health?alert=inventory.cost_above_price",
    );
    expect(screen.getByRole("link", { name: "Lihat semua (8)" })).toHaveAttribute("href", "/data-health");
    expect(screen.getByRole("link", { name: "2 ditunda" })).toHaveAttribute("href", "/data-health?tab=ditunda");
  });

  it("tanpa izin tujuan aksi, baris menuju halaman Kesehatan Data", () => {
    renderCard(summary());
    expect(screen.getByRole("link", { name: /Harga minimum di atas harga jual/ })).toHaveAttribute(
      "href",
      "/data-health?alert=master.min_price_above_selling",
    );
    useAuthStore.setState({ permissions: ["dashboard.alerts.view_all", "products.view"] });
    cleanup();
    renderCard(summary());
    expect(screen.getByRole("link", { name: /Harga minimum di atas harga jual/ })).toHaveAttribute(
      "href",
      "/products?priceIssue=min_above_selling",
    );
  });

  it("semua lolos hanya bila tidak ada pemeriksaan gagal atau tertunda", () => {
    renderCard(summary({ items: [], totalActive: 0, snoozedCount: 0 }));
    expect(screen.getByText("Semua pemeriksaan lolos")).toBeInTheDocument();
    cleanup();
    renderCard(summary({ items: [], totalActive: 0, pending: true, failedChecks: [{ id: "x", label: "X", message: "m" }] }));
    expect(screen.getByText("Belum ada masalah ditemukan")).toBeInTheDocument();
    expect(screen.getByText("1 pemeriksaan tidak terperiksa · Sebagian pemeriksaan masih dihitung")).toBeInTheDocument();
  });

  it("galat memuat tampil di dalam kartu", () => {
    renderCard(null, { error: "Anda tidak memiliki akses" });
    expect(screen.getByText("Gagal memuat alert")).toBeInTheDocument();
  });
});
