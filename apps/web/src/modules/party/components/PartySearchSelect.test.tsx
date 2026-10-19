import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const customerList = vi.fn();
const supplierList = vi.fn();
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: {
    list: (...a: unknown[]) => customerList(...a),
    get: vi.fn(),
  },
  supplierService: {
    list: (...a: unknown[]) => supplierList(...a),
    get: vi.fn(),
  },
}));

import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";

const party = (id: number, name: string, type = "customer") => ({
  id,
  code: `C${id}`,
  type,
  name,
  phone: "0812",
  email: null,
  address: null,
  notes: null,
  member: null,
  status: "active",
});

beforeEach(() => {
  customerList.mockReset();
  supplierList.mockReset();
  customerList.mockResolvedValue({ items: [party(1, "Budi")], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } });
  supplierList.mockResolvedValue({
    items: [party(9, "PT Sumber", "supplier")],
    meta: { page: 1, limit: 10, total: 1, totalPages: 1 },
  });
});
afterEach(cleanup);

describe("PartySearchSelect", () => {
  it("ketikan 1 huruf tidak memanggil pencarian pihak; 2 huruf memanggil dengan search + signal", async () => {
    const user = userEvent.setup();
    render(<PartySearchSelect partyKind="customer" value={null} onChange={vi.fn()} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    // Daftar awal tanpa kata kunci.
    await waitFor(() => expect(customerList).toHaveBeenCalledTimes(1));
    expect(customerList.mock.calls[0][0]).toMatchObject({ search: undefined, status: "active", limit: 20 });

    await user.type(screen.getByPlaceholderText("Cari nama, kode, atau kontak…"), "b");
    // Tunggu lewat debounce bawaan (250 ms) — tetap tidak boleh ada panggilan baru.
    await new Promise((r) => setTimeout(r, 400));
    expect(customerList).toHaveBeenCalledTimes(1);

    await user.type(screen.getByPlaceholderText("Cari nama, kode, atau kontak…"), "u");
    await waitFor(() => expect(customerList).toHaveBeenCalledTimes(2));
    expect(customerList.mock.calls[1][0]).toMatchObject({ search: "bu" });
    expect(customerList.mock.calls[1][1]).toBeInstanceOf(AbortSignal);
  });

  it("partyKind=any memanggil customer dan supplier paralel dan memberi label jenis", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<PartySearchSelect partyKind="any" value={null} onChange={onChange} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(customerList).toHaveBeenCalled());
    expect(supplierList).toHaveBeenCalled();
    await user.click(await screen.findByRole("option", { name: /PT Sumber/ }));
    expect(onChange).toHaveBeenCalledWith(9, expect.objectContaining({ type: "supplier" }));
  });
});
