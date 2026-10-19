import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
const detail = vi.fn();
const permissionCatalog = vi.fn();
const setPermissions = vi.fn();
vi.mock("@/modules/users/services/role.service", () => ({
  roleService: {
    list: (...a: unknown[]) => list(...a),
    detail: (...a: unknown[]) => detail(...a),
    permissionCatalog: (...a: unknown[]) => permissionCatalog(...a),
    setPermissions: (...a: unknown[]) => setPermissions(...a),
  },
}));

import RolesPage from "@/modules/users/pages/RolesPage";

const roles = [
  { id: 7, code: "kasir", name: "Kasir", description: "", isSystem: false },
  { id: 1, code: "owner", name: "Pemilik", description: "", isSystem: true },
];
const catalog = [
  { code: "sales.view", name: "Lihat penjualan" },
  { code: "sales.create", name: "Buat penjualan" },
  { code: "stock.view", name: "Lihat stok" },
];

beforeEach(() => {
  for (const m of [list, detail, permissionCatalog, setPermissions]) m.mockReset();
  list.mockResolvedValue(roles);
  permissionCatalog.mockResolvedValue(catalog);
  setPermissions.mockResolvedValue({ message: "Izin role disimpan", data: null });
  useAuthStore.setState({ permissions: ["roles.view", "roles.manage"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <RolesPage />
    </MemoryRouter>,
  );
}

async function openPerms(name: string) {
  const row = (await screen.findByText(name)).closest("tr")!;
  fireEvent.click(row.querySelector("button")!);
}

describe("halaman role — matriks izin", () => {
  it("mengisi centang awal dari data server dan tidak lagi menampilkan banner 'belum menyediakan'", async () => {
    detail.mockResolvedValue({ ...roles[0], locked: false, permissions: ["sales.view", "stock.view"] });
    renderPage();
    await openPerms("Kasir");
    await waitFor(() => expect(screen.getByLabelText(/Lihat penjualan/)).toBeChecked());
    expect(screen.getByLabelText(/Lihat stok/)).toBeChecked();
    expect(screen.getByLabelText(/Buat penjualan/)).not.toBeChecked();
    expect(screen.queryByText(/belum menyediakan/i)).not.toBeInTheDocument();
    expect(detail).toHaveBeenCalledWith(7);
  });

  it("tanpa perubahan tombol simpan nonaktif sehingga izin tidak tertimpa", async () => {
    detail.mockResolvedValue({ ...roles[0], locked: false, permissions: ["sales.view"] });
    renderPage();
    await openPerms("Kasir");
    await waitFor(() => expect(screen.getByLabelText(/Lihat penjualan/)).toBeChecked());
    expect(screen.getByRole("button", { name: "Simpan izin" })).toBeDisabled();
    expect(setPermissions).not.toHaveBeenCalled();
  });

  it("menambah izin menampilkan selisih dan menyimpan seluruh set tanpa konfirmasi pencabutan", async () => {
    detail.mockResolvedValue({ ...roles[0], locked: false, permissions: ["sales.view"] });
    renderPage();
    await openPerms("Kasir");
    await waitFor(() => expect(screen.getByLabelText(/Lihat penjualan/)).toBeChecked());
    fireEvent.click(screen.getByLabelText(/Lihat stok/));
    expect(screen.getByTestId("perm-diff")).toHaveTextContent("+1");
    fireEvent.click(screen.getByRole("button", { name: "Simpan izin" }));
    await waitFor(() => expect(setPermissions).toHaveBeenCalledWith(7, ["sales.view", "stock.view"]));
  });

  it("mencabut izin meminta konfirmasi sebelum menyimpan", async () => {
    detail.mockResolvedValue({ ...roles[0], locked: false, permissions: ["sales.view", "stock.view"] });
    renderPage();
    await openPerms("Kasir");
    await waitFor(() => expect(screen.getByLabelText(/Lihat stok/)).toBeChecked());
    fireEvent.click(screen.getByLabelText(/Lihat stok/));
    fireEvent.click(screen.getByRole("button", { name: "Simpan izin" }));
    expect(setPermissions).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "Simpan perubahan" }));
    await waitFor(() => expect(setPermissions).toHaveBeenCalledWith(7, ["sales.view"]));
  });

  it("role terkunci tampil baca-saja tanpa tombol simpan", async () => {
    detail.mockResolvedValue({ ...roles[1], locked: true, permissions: ["sales.view", "stock.view", "sales.create"] });
    renderPage();
    await openPerms("Pemilik");
    await waitFor(() => expect(screen.getByLabelText(/Lihat penjualan/)).toBeChecked());
    expect(screen.getByLabelText(/Lihat penjualan/)).toBeDisabled();
    expect(screen.getByText(/hanya-baca/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Simpan izin" })).not.toBeInTheDocument();
  });
});
