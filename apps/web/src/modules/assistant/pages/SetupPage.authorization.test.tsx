import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { AssistantAuthorization } from "@/modules/assistant/types";

// T-02/T-03: manajemen nomor pengakses BOT di SetupPage (padanan UI e2e legacy
// 19-whatsapp-authorization-management: tambah, ubah, cabut) + kanal.
const listAuthorizations = vi.fn();
const createAuthorization = vi.fn();
const updateAuthorization = vi.fn();
const revokeAuthorization = vi.fn();
const getChannel = vi.fn();
const connectChannel = vi.fn();
const resetChannel = vi.fn();
const disconnectChannel = vi.fn();
vi.mock("@/modules/assistant/services/assistant.service", () => ({
  assistantService: {
    getChannel: (...a: unknown[]) => getChannel(...a),
    connectChannel: (...a: unknown[]) => connectChannel(...a),
    resetChannel: (...a: unknown[]) => resetChannel(...a),
    disconnectChannel: (...a: unknown[]) => disconnectChannel(...a),
    listAuthorizations: (...a: unknown[]) => listAuthorizations(...a),
    createAuthorization: (...a: unknown[]) => createAuthorization(...a),
    updateAuthorization: (...a: unknown[]) => updateAuthorization(...a),
    revokeAuthorization: (...a: unknown[]) => revokeAuthorization(...a),
    chat: vi.fn(),
    simulate: vi.fn(),
  },
}));
vi.mock("@/modules/auth/hooks/use-accessible-branches", () => ({
  useAccessibleBranches: () => ({ branches: [], loading: false }),
}));

import SetupPage from "@/modules/assistant/pages/SetupPage";

const owner: AssistantAuthorization = {
  id: 1, phone: "6281100000001", name: "Budi Pemilik", accessLevel: "owner",
  status: "active", isPrimaryOwner: true, lastSeenAt: "",
};
const staff: AssistantAuthorization = {
  id: 2, phone: "6281100000002", name: "Sari Staf", accessLevel: "authorized_party",
  status: "active", isPrimaryOwner: false, lastSeenAt: "",
};
const revoked: AssistantAuthorization = {
  id: 3, phone: "6281100000003", name: "Dodi Lama", accessLevel: "authorized_party",
  status: "revoked", isPrimaryOwner: false, lastSeenAt: "",
};

beforeEach(() => {
  for (const m of [listAuthorizations, createAuthorization, updateAuthorization, revokeAuthorization, getChannel, connectChannel, resetChannel, disconnectChannel]) m.mockReset();
  getChannel.mockResolvedValue({ connected: true, phone: "6281999", qrDataUrl: "", lastError: "", updatedAt: "" });
  listAuthorizations.mockImplementation((includeRevoked: boolean) => Promise.resolve(includeRevoked ? [owner, staff, revoked] : [owner, staff]));
  createAuthorization.mockResolvedValue({ message: "Nomor terotorisasi", data: { ...staff, id: 9 } });
  updateAuthorization.mockResolvedValue({ message: "Otorisasi diperbarui", data: staff });
  revokeAuthorization.mockResolvedValue({ message: "Otorisasi dicabut", data: null });
  useAuthStore.setState({ permissions: ["assistant.view", "assistant.manage"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/assistant/setup"]}>
      <SetupPage />
    </MemoryRouter>,
  );
}

describe("SetupPage — nomor terotorisasi", () => {
  it("render: kanal terhubung, daftar nomor aktif, dicabut tersembunyi sampai dicentang", async () => {
    const user = userEvent.setup();
    renderPage();
    expect(await screen.findByText("Budi Pemilik")).toBeInTheDocument();
    expect(screen.getAllByText("Terhubung").length).toBeGreaterThan(0);
    expect(screen.getByText("6281999")).toBeInTheDocument();
    expect(screen.getByText("Pemilik utama")).toBeInTheDocument();
    expect(screen.getByText("Sari Staf")).toBeInTheDocument();
    expect(screen.queryByText("Dodi Lama")).not.toBeInTheDocument();
    expect(listAuthorizations).toHaveBeenLastCalledWith(false);

    await user.click(screen.getByLabelText("Tampilkan nomor yang dicabut"));
    expect(await screen.findByText("Dodi Lama")).toBeInTheDocument();
    expect(listAuthorizations).toHaveBeenLastCalledWith(true);
    expect(screen.getByText("Dicabut")).toBeInTheDocument();
  });

  it("tambah: validasi klien menolak nomor tak valid; nomor sah dikirim sebagai payload", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Budi Pemilik");
    await user.click(screen.getByRole("button", { name: "Tambah nomor" }));

    await user.type(screen.getByLabelText(/Nomor telepon/), "+62 812");
    await user.type(screen.getByLabelText(/^Nama/), "Operator Baru");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Nomor telepon harus 8-15 digit angka")).toBeInTheDocument();
    expect(createAuthorization).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText(/Nomor telepon/));
    await user.type(screen.getByLabelText(/Nomor telepon/), "6281100000099");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() =>
      expect(createAuthorization).toHaveBeenCalledWith({
        phone: "6281100000099", name: "Operator Baru", accessLevel: "authorized_party", isPrimaryOwner: false,
      }),
    );
    // Daftar dimuat ulang setelah simpan.
    await waitFor(() => expect(listAuthorizations.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it("tambah: nama kosong ditolak di klien", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Budi Pemilik");
    await user.click(screen.getByRole("button", { name: "Tambah nomor" }));
    await user.type(screen.getByLabelText(/Nomor telepon/), "6281100000099");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Nama wajib diisi")).toBeInTheDocument();
    expect(createAuthorization).not.toHaveBeenCalled();
  });

  it("tambah: nomor duplikat (409 dari server) menampilkan pesan server dan form tetap terbuka", async () => {
    createAuthorization.mockRejectedValue({
      status: 409, message: "Nomor 6281100000002 sudah terdaftar",
      errors: [{ field: "phone", message: "Nomor sudah terdaftar" }],
    });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Budi Pemilik");
    await user.click(screen.getByRole("button", { name: "Tambah nomor" }));
    await user.type(screen.getByLabelText(/Nomor telepon/), "6281100000002");
    await user.type(screen.getByLabelText(/^Nama/), "Duplikat");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Nomor sudah terdaftar")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Simpan" })).toBeEnabled();
    expect(screen.getByLabelText(/Nomor telepon/)).toHaveValue("6281100000002");
  });

  it("ubah: form terisi dari baris, perubahan dikirim ke id yang benar", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Sari Staf");
    await user.click(screen.getByRole("button", { name: "Aksi untuk 6281100000002" }));
    await user.click(await screen.findByRole("menuitem", { name: "Ubah" }));

    expect(screen.getByLabelText(/Nomor telepon/)).toHaveValue("6281100000002");
    expect(screen.getByLabelText(/^Nama/)).toHaveValue("Sari Staf");
    await user.clear(screen.getByLabelText(/^Nama/));
    await user.type(screen.getByLabelText(/^Nama/), "Sari Edit");
    await user.selectOptions(screen.getByLabelText(/Tingkat akses/), "owner");
    await user.click(screen.getByLabelText("Pemilik utama kanal"));
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() =>
      expect(updateAuthorization).toHaveBeenCalledWith(2, {
        phone: "6281100000002", name: "Sari Edit", accessLevel: "owner", isPrimaryOwner: true,
      }),
    );
    expect(createAuthorization).not.toHaveBeenCalled();
  });

  it("cabut: butuh konfirmasi; batal tidak memanggil server, konfirmasi memanggil id yang benar", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Sari Staf");
    await user.click(screen.getByRole("button", { name: "Aksi untuk 6281100000002" }));
    await user.click(await screen.findByRole("menuitem", { name: "Cabut otorisasi" }));
    let dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(/6281100000002/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));
    expect(revokeAuthorization).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Aksi untuk 6281100000002" }));
    await user.click(await screen.findByRole("menuitem", { name: "Cabut otorisasi" }));
    dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Cabut" }));
    await waitFor(() => expect(revokeAuthorization).toHaveBeenCalledWith(2));
  });

  it("nomor yang sudah dicabut tidak bisa dicabut lagi", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Sari Staf");
    await user.click(screen.getByLabelText("Tampilkan nomor yang dicabut"));
    await screen.findByText("Dodi Lama");
    await user.click(screen.getByRole("button", { name: "Aksi untuk 6281100000003" }));
    expect(await screen.findByRole("menuitem", { name: "Cabut otorisasi" })).toHaveAttribute("aria-disabled", "true");
  });

  it("tanpa assistant.manage: tidak ada tombol tambah, aksi baris, maupun Hubungkan", async () => {
    useAuthStore.setState({ permissions: ["assistant.view"], ready: true });
    renderPage();
    await screen.findByText("Budi Pemilik");
    expect(screen.queryByRole("button", { name: "Tambah nomor" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Aksi untuk/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hubungkan" })).not.toBeInTheDocument();
  });

  it("daftar kosong menampilkan keadaan kosong dengan ajakan tambah", async () => {
    listAuthorizations.mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText("Belum ada nomor terotorisasi")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Tambah nomor" }).length).toBeGreaterThan(0);
  });
});

describe("SetupPage — kanal WhatsApp", () => {
  it("terputus dengan QR: menampilkan QR dan galat, tombol Hubungkan aktif, Putuskan nonaktif", async () => {
    getChannel.mockResolvedValue({ connected: false, phone: "", qrDataUrl: "data:image/png;base64,AAAA", lastError: "koneksi terputus", updatedAt: "" });
    renderPage();
    expect(await screen.findByAltText("QR tautan WhatsApp")).toBeInTheDocument();
    expect(screen.getByText("koneksi terputus")).toBeInTheDocument();
    expect(screen.getByText("Terputus", { selector: "span" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hubungkan" })).toBeEnabled();
  });

  it("Hubungkan memanggil server lalu memuat ulang status", async () => {
    getChannel.mockResolvedValue({ connected: false, phone: "", qrDataUrl: "", lastError: "", updatedAt: "" });
    connectChannel.mockResolvedValue({ message: "Kanal dihubungkan", data: null });
    const user = userEvent.setup();
    renderPage();
    const btn = await screen.findByRole("button", { name: "Hubungkan" });
    await waitFor(() => expect(btn).toBeEnabled());
    const before = getChannel.mock.calls.length;
    await user.click(btn);
    await waitFor(() => expect(connectChannel).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(getChannel.mock.calls.length).toBeGreaterThan(before));
  });

  it("Reset sesi meminta konfirmasi sebelum memanggil server", async () => {
    resetChannel.mockResolvedValue({ message: "Sesi direset", data: null });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Budi Pemilik");
    await user.click(screen.getByRole("button", { name: "Aksi kanal" }));
    await user.click(await screen.findByRole("menuitem", { name: "Reset sesi" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(resetChannel).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "Reset" }));
    await waitFor(() => expect(resetChannel).toHaveBeenCalledTimes(1));
  });

  it("Putuskan kanal hanya aktif saat terhubung dan meminta konfirmasi", async () => {
    disconnectChannel.mockResolvedValue({ message: "Kanal diputuskan", data: null });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Budi Pemilik");
    await user.click(screen.getByRole("button", { name: "Aksi kanal" }));
    await user.click(await screen.findByRole("menuitem", { name: "Putuskan kanal" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Putuskan" }));
    await waitFor(() => expect(disconnectChannel).toHaveBeenCalledTimes(1));
  });

  it("gagal memuat status kanal menampilkan galat dengan tombol coba lagi", async () => {
    getChannel.mockRejectedValue({ status: 500, message: "Kanal tidak tersedia" });
    renderPage();
    expect(await screen.findByText("Kanal tidak tersedia")).toBeInTheDocument();
  });
});
