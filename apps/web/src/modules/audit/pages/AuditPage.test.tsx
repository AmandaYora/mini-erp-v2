import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
vi.mock("@/modules/audit/services/audit.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/audit/services/audit.service")>();
  return { ...actual, auditService: { list: (...a: unknown[]) => list(...a) } };
});
vi.mock("@/modules/auth/hooks/use-accessible-branches", () => ({
  useAccessibleBranches: () => ({
    branches: [{ id: 2, code: "B2", name: "Cabang Dua", status: "active" }],
    loading: false,
  }),
}));
vi.mock("@/modules/users/components/UserSearchSelect", () => ({
  UserSearchSelect: () => <div data-testid="user-picker" />,
}));

import AuditPage from "@/modules/audit/pages/AuditPage";

const records = [
  {
    id: 1,
    action: "products.update",
    entity: "product",
    entityId: 5,
    branchId: 2,
    branchName: "Cabang Dua",
    actorId: 9,
    actorType: "user",
    actorName: "Siti",
    note: "",
    createdAt: "2026-10-09T10:00:00+07:00",
    before: { name: "A" },
    after: { name: "B" },
    metadata: null,
  },
  {
    id: 2,
    action: "job.sweep",
    entity: "",
    entityId: 0,
    branchId: 0,
    actorId: 0,
    actorType: "system",
    note: "otomatis",
    createdAt: "2026-10-09T11:00:00+07:00",
    before: null,
    after: null,
    metadata: null,
  },
];

beforeEach(() => {
  list.mockReset();
  list.mockResolvedValue({ items: records, meta: { page: 1, limit: 20, total: 2, totalPages: 1 } });
  useAuthStore.setState({ permissions: ["audit.view", "users.view"], ready: true });
});
afterEach(cleanup);

function renderPage(path = "/audit-logs") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuditPage />
    </MemoryRouter>,
  );
}

describe("halaman audit", () => {
  it("menampilkan nama aktor/cabang, Sistem, dan rincian hanya untuk entri yang punya before/after", async () => {
    renderPage();
    expect(await screen.findByText("Siti")).toBeInTheDocument();
    expect(screen.getByText("Sistem")).toBeInTheDocument();
    expect(screen.getByText("Global")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Rincian" })).toHaveLength(1);
  });

  it("menampilkan label Indonesia untuk aksi/entitas dikenal (kunci mentah di tooltip) dan kunci mentah untuk yang tak dikenal", async () => {
    renderPage();
    await screen.findByText("Siti");
    const cell = within(screen.getByRole("table")).getByText("Ubah produk");
    expect(cell).toHaveAttribute("title", "products.update");
    expect(screen.getByText("Produk #5")).toBeInTheDocument();
    expect(within(screen.getByRole("table")).queryByText("products.update")).not.toBeInTheDocument();
    expect(within(screen.getByRole("table")).getByText("job.sweep")).toBeInTheDocument();
  });

  it("filter aksi memakai label Indonesia dan mengirim kunci backend", async () => {
    renderPage();
    await screen.findByText("Siti");
    fireEvent.change(screen.getByLabelText("Aksi"), { target: { value: "sales_orders.confirm" } });
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ action: "sales_orders.confirm" }));
    expect(screen.getByText("Aksi: Konfirmasi order jual")).toBeInTheDocument();
  });

  it("Rincian membuka panel samping berisi diff sebelum/sesudah, Tutup menutupnya", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Rincian" }));
    const detail = await screen.findByTestId("audit-detail");
    expect(detail).toHaveTextContent("name");
    expect(detail).toHaveTextContent("Diubah");
    expect(detail).toHaveTextContent("Sebelum: A");
    expect(detail).toHaveTextContent("Sesudah: B");
    fireEvent.click(screen.getByRole("button", { name: "Tutup" }));
    await waitFor(() => expect(screen.queryByTestId("audit-detail")).not.toBeInTheDocument());
  });

  it("filter dari URL dikirim ke server: cabang semua, aksi, aktor, tanggal", async () => {
    renderPage("/audit-logs?branchId=all&action=products.&actorId=9&from=2026-10-01&to=2026-10-09&entityId=5");
    await screen.findByText("Siti");
    const [params, signal] = list.mock.calls[0];
    expect(params).toMatchObject({
      branchId: "all",
      action: "products.",
      actorId: 9,
      from: "2026-10-01",
      to: "2026-10-09",
      entityId: 5,
      limit: 20,
    });
    expect(signal).toBeInstanceOf(AbortSignal);
  });

  it("memilih cabang tertentu mengirim branchId angka", async () => {
    renderPage();
    await screen.findByText("Siti");
    fireEvent.change(screen.getByLabelText("Cabang"), { target: { value: "2" } });
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ branchId: "2" }));
  });

  it("pemilih pengguna disembunyikan tanpa users.view", async () => {
    useAuthStore.setState({ permissions: ["audit.view"], ready: true });
    renderPage();
    await screen.findByText("Siti");
    expect(screen.queryByTestId("user-picker")).not.toBeInTheDocument();
  });
});
