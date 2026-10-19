import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AddressFormModal } from "@/modules/party/components/AddressFormModal";
import type { Address } from "@/modules/party/types";

vi.setConfig({ testTimeout: 60000 });
afterEach(cleanup);

const existing: Address = {
  id: 7, label: "Kantor", recipient: "Budi", phone: "0812345678", text: "Jl. Merdeka 1",
  isPrimary: true, sortOrder: 2, status: "active",
};

function renderModal(over: Partial<React.ComponentProps<typeof AddressFormModal>> = {}) {
  const onSubmit = vi.fn();
  const onClose = vi.fn();
  render(
    <AddressFormModal open initial={null} submitting={false} serverError={null} onClose={onClose} onSubmit={onSubmit} {...over} />,
  );
  return { onSubmit, onClose };
}

describe("AddressFormModal (pemilih/buku alamat)", () => {
  it("tambah: judul Tambah alamat; alamat lengkap wajib dan tidak dikirim bila kosong", async () => {
    const user = userEvent.setup();
    const { onSubmit } = renderModal();
    expect(screen.getByText("Tambah alamat")).toBeInTheDocument();
    await user.type(screen.getByLabelText(/^Label/), "Rumah");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Alamat wajib diisi")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("tambah: nilai dirapikan dan dikirim dengan bawaan urutan 0 dan bukan utama", async () => {
    const user = userEvent.setup();
    const { onSubmit } = renderModal();
    await user.type(screen.getByLabelText(/^Penerima/), "  Siti  ");
    await user.type(screen.getByLabelText(/^Alamat lengkap/), "  Jl. Mawar 5, Bandung ");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        label: "", recipient: "Siti", phone: "", text: "Jl. Mawar 5, Bandung", isPrimary: false, sortOrder: 0,
      }),
    );
  });

  it("telepon tidak valid ditolak di klien (urutan negatif dicegah validasi bawaan peramban)", async () => {
    const user = userEvent.setup();
    const { onSubmit } = renderModal();
    await user.type(screen.getByLabelText(/^Alamat lengkap/), "Jl. X");
    await user.type(screen.getByLabelText(/^Telepon/), "abc123");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText("Format telepon tidak valid")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("ubah: form terisi dari alamat; menandai utama ikut terkirim", async () => {
    const user = userEvent.setup();
    const { onSubmit } = renderModal({ initial: { ...existing, isPrimary: false } });
    expect(screen.getByText("Ubah alamat")).toBeInTheDocument();
    expect(screen.getByLabelText(/^Label/)).toHaveValue("Kantor");
    expect(screen.getByLabelText(/^Penerima/)).toHaveValue("Budi");
    expect(screen.getByLabelText(/^Telepon/)).toHaveValue("0812345678");
    expect(screen.getByLabelText(/^Alamat lengkap/)).toHaveValue("Jl. Merdeka 1");
    expect(screen.getByLabelText(/^Urutan/)).toHaveValue(2);
    await user.click(screen.getByLabelText("Jadikan alamat utama"));
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        label: "Kantor", recipient: "Budi", phone: "0812345678", text: "Jl. Merdeka 1", isPrimary: true, sortOrder: 2,
      }),
    );
  });

  it("galat server tampil di dalam form; Batal menutup tanpa mengirim", async () => {
    const user = userEvent.setup();
    const { onSubmit, onClose } = renderModal({ initial: existing, serverError: "Customer diarsipkan" });
    expect(screen.getByText("Customer diarsipkan")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Batal" }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
