import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { MemberType } from "@/modules/party/types";

const list = vi.fn();
const customerCount = vi.fn();
const archive = vi.fn();
vi.mock("@/modules/party/services/party.service", () => ({
  memberTypeService: {
    list: (...a: unknown[]) => list(...a),
    customerCount: (...a: unknown[]) => customerCount(...a),
    archive: (...a: unknown[]) => archive(...a),
    restore: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
  },
}));

import MemberTypesPage from "@/modules/party/pages/MemberTypesPage";
import { archiveConfirmMessage } from "@/modules/party/lib/member-type-impact";

const gold: MemberType = {
  id: 3,
  code: "GOLD",
  name: "Gold",
  description: null,
  basis: "selling_price",
  direction: "minus",
  type: "percent",
  value: 5,
  roundingMode: "none",
  roundingStep: 0,
  status: "active",
};

beforeEach(() => {
  for (const m of [list, customerCount, archive]) m.mockReset();
  list.mockResolvedValue([gold]);
  archive.mockResolvedValue({ message: "Tipe member diarsipkan", data: null });
  useAuthStore.setState({ permissions: ["member_types.view", "member_types.manage"], ready: true });
});
afterEach(cleanup);

async function openArchiveDialog() {
  const user = userEvent.setup();
  render(
    <MemoryRouter>
      <MemberTypesPage />
    </MemoryRouter>,
  );
  await screen.findByText("Gold");
  await user.click(screen.getByRole("button", { name: "Aksi untuk Gold" }));
  await user.click(await screen.findByRole("menuitem", { name: /Arsipkan/ }));
  return user;
}

describe("halaman tipe member — dampak arsip (D3)", () => {
  it("daftar dibuka dengan filter Aktif", async () => {
    render(
      <MemoryRouter>
        <MemberTypesPage />
      </MemoryRouter>,
    );
    await screen.findByText("Gold");
    expect(list).toHaveBeenCalledWith("", "active");
  });

  it("dialog arsip menampilkan jumlah pelanggan aktif yang akan turun ke harga standar", async () => {
    customerCount.mockResolvedValue({ memberTypeId: 3, activeCustomers: 12 });
    await openArchiveDialog();
    const dialog = await screen.findByRole("alertdialog");
    expect(await within(dialog).findByText(/12 pelanggan aktif akan turun ke harga standar/)).toBeInTheDocument();
    expect(customerCount).toHaveBeenCalledWith(3);
  });

  it("arsip tetap diizinkan setelah dikonfirmasi (tidak diblokir)", async () => {
    customerCount.mockResolvedValue({ memberTypeId: 3, activeCustomers: 2 });
    const user = await openArchiveDialog();
    const dialog = await screen.findByRole("alertdialog");
    await within(dialog).findByText(/2 pelanggan aktif/);
    await user.click(within(dialog).getByRole("button", { name: "Arsipkan" }));
    expect(archive).toHaveBeenCalledWith(3);
  });

  it("gagal memuat jumlah: peringatan tetap muncul dan arsip tidak terhalang", async () => {
    customerCount.mockRejectedValue(new Error("gagal"));
    await openArchiveDialog();
    const dialog = await screen.findByRole("alertdialog");
    expect(await within(dialog).findByText(/Jumlah pelanggan terdampak tidak dapat dimuat/)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Arsipkan" })).toBeEnabled();
  });
});

describe("archiveConfirmMessage", () => {
  const base = { loading: false, error: null };
  it("0 pelanggan, memuat, dan jumlah", () => {
    expect(archiveConfirmMessage("GOLD", { ...base, activeCustomers: 0 })).toContain("Tidak ada pelanggan aktif");
    expect(archiveConfirmMessage("GOLD", { loading: true, error: null, activeCustomers: null })).toContain("Memeriksa");
    expect(archiveConfirmMessage("GOLD", { ...base, activeCustomers: 1 })).toContain("1 pelanggan aktif akan turun ke harga standar.");
  });
});
