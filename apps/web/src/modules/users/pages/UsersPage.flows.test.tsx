import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

// T-03: alur kritis halaman pengguna — buat pengguna (validasi + payload),
// ganti kata sandi (PasswordInput + konfirmasi), ubah status.
const list = vi.fn();
const create = vi.fn();
const changePassword = vi.fn();
const setStatus = vi.fn();
vi.mock("@/modules/users/services/user.service", () => ({
  userService: {
    list: (...a: unknown[]) => list(...a),
    create: (...a: unknown[]) => create(...a),
    changePassword: (...a: unknown[]) => changePassword(...a),
    setStatus: (...a: unknown[]) => setStatus(...a),
    get: vi.fn(),
    update: vi.fn(),
    branchOptions: vi.fn().mockResolvedValue([]),
  },
}));
vi.mock("@/modules/users/services/role.service", () => ({
  roleService: {
    list: vi.fn().mockResolvedValue([
      { id: 7, name: "Kasir", code: "KSR", description: "", permissions: [] },
      { id: 8, name: "Gudang", code: "GDG", description: "", permissions: [] },
    ]),
  },
}));

vi.setConfig({ testTimeout: 60000 });

import UsersPage from "@/modules/users/pages/UsersPage";

const sinta = {
  id: 1, username: "sinta", email: "sinta@toko.id", fullName: "Sinta Kasir", status: "active",
  roles: [{ id: 7, name: "Kasir" }], branches: [], lastLoginAt: null,
};

beforeEach(() => {
  for (const m of [list, create, changePassword, setStatus]) m.mockReset();
  list.mockResolvedValue({ items: [sinta], meta: { page: 1, limit: 25, total: 1, totalPages: 1 } });
  create.mockResolvedValue({ message: "Pengguna dibuat", data: { ...sinta, id: 5 } });
  changePassword.mockResolvedValue({ message: "Kata sandi diganti", data: null });
  setStatus.mockResolvedValue({ message: "Status diubah", data: null });
  useAuthStore.setState({ permissions: ["users.view", "users.create", "users.update", "users.archive"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <UsersPage />
    </MemoryRouter>,
  );
}

async function openCreate() {
  const user = userEvent.setup();
  renderPage();
  await screen.findByText("sinta");
  await user.click(screen.getByRole("button", { name: "Tambah pengguna" }));
  await screen.findByLabelText(/^Username/);
  return user;
}

describe("halaman pengguna — buat pengguna", () => {
  it("submit kosong: semua isian wajib ditandai dan server tidak dipanggil", async () => {
    const user = await openCreate();
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Username wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Nama lengkap wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Password minimal 8 karakter")).toBeInTheDocument();
    expect(screen.getByText("Pengguna wajib memiliki minimal satu role")).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("tanpa role dipilih ditolak walau isian lain sah", async () => {
    const user = await openCreate();
    await user.type(screen.getByLabelText(/^Username/), "budi");
    await user.type(screen.getByLabelText(/^Nama lengkap/), "Budi Gudang");
    await user.type(screen.getByLabelText(/^Kata sandi/), "rahasia123");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Pengguna wajib memiliki minimal satu role")).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("isian sah + role: payload dirapikan dikirim dan daftar dimuat ulang", async () => {
    const user = await openCreate();
    await user.type(screen.getByLabelText(/^Username/), "  budi  ");
    await user.type(screen.getByLabelText(/^Nama lengkap/), "Budi Gudang");
    await user.type(screen.getByLabelText(/^Kata sandi/), "rahasia123");
    await user.click(await screen.findByLabelText(/^Gudang/));
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        username: "budi", email: undefined, fullName: "Budi Gudang", phone: "", displayTitle: "",
        password: "rahasia123", roleIds: [8], branches: [],
      }),
    );
    await waitFor(() => expect(list.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it("username duplikat dari server tampil pada isian username", async () => {
    create.mockRejectedValue({
      status: 409, message: "Username sudah dipakai",
      errors: [{ field: "username", message: "Username sudah dipakai" }],
    });
    const user = await openCreate();
    await user.type(screen.getByLabelText(/^Username/), "sinta");
    await user.type(screen.getByLabelText(/^Nama lengkap/), "Sinta Dua");
    await user.type(screen.getByLabelText(/^Kata sandi/), "rahasia123");
    await user.click(await screen.findByLabelText(/^Kasir/));
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect((await screen.findAllByText("Username sudah dipakai")).length).toBeGreaterThan(0);
    // Form tetap terbuka dengan isian utuh.
    expect(screen.getByLabelText(/^Username/)).toHaveValue("sinta");
  });

  it("kata sandi bisa diintip lewat tombol mata", async () => {
    const user = await openCreate();
    const pw = screen.getByLabelText(/^Kata sandi/);
    expect(pw).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "Tampilkan password" }));
    expect(pw).toHaveAttribute("type", "text");
  });

  it("tanpa izin users.create tidak ada tombol Tambah pengguna", async () => {
    useAuthStore.setState({ permissions: ["users.view"], ready: true });
    renderPage();
    await screen.findByText("sinta");
    expect(screen.queryByRole("button", { name: "Tambah pengguna" })).not.toBeInTheDocument();
  });
});

describe("halaman pengguna — ganti kata sandi & status", () => {
  async function openRowAction(name: RegExp | string) {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("sinta");
    await user.click(screen.getByRole("button", { name: /Aksi untuk/ }));
    await user.click(await screen.findByRole("menuitem", { name }));
    return user;
  }

  it("konfirmasi tidak sama ditolak; sama dikirim ke id pengguna", async () => {
    const user = await openRowAction("Ganti kata sandi");
    const pw = await screen.findByLabelText(/^Kata sandi baru/);
    await user.type(pw, "sandi-baru-1");
    await user.type(screen.getByLabelText(/^Konfirmasi kata sandi/), "sandi-lain-22");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Konfirmasi password tidak sama")).toBeInTheDocument();
    expect(changePassword).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText(/^Konfirmasi kata sandi/));
    await user.type(screen.getByLabelText(/^Konfirmasi kata sandi/), "sandi-baru-1");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(changePassword).toHaveBeenCalledWith(1, "sandi-baru-1"));
  });

  it("kata sandi terlalu pendek ditolak di klien", async () => {
    const user = await openRowAction("Ganti kata sandi");
    await user.type(await screen.findByLabelText(/^Kata sandi baru/), "pendek");
    await user.type(screen.getByLabelText(/^Konfirmasi kata sandi/), "pendek");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Password minimal 8 karakter")).toBeInTheDocument();
    expect(changePassword).not.toHaveBeenCalled();
  });

  it("Nonaktifkan meminta konfirmasi lalu memanggil server dengan status inactive", async () => {
    const user = await openRowAction("Nonaktifkan");
    const dialog = await screen.findByRole("alertdialog");
    expect(setStatus).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: /Nonaktifkan/ }));
    await waitFor(() => expect(setStatus).toHaveBeenCalledWith(1, "inactive"));
  });
});
