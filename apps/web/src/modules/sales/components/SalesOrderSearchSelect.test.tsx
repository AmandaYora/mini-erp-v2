import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const list = vi.fn();
const get = vi.fn();
vi.mock("@/modules/sales/services/sales.service", () => ({
  salesService: {
    list: (...a: unknown[]) => list(...a),
    get: (...a: unknown[]) => get(...a),
  },
}));

import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";

const order = (id: number, number: string, partyName: string) => ({
  id,
  number,
  partyName,
  orderDate: "2026-10-01",
  grandTotal: 150000,
});

beforeEach(() => {
  list.mockReset();
  get.mockReset();
  list.mockResolvedValue({
    items: [order(7, "SO-0007", "Budi"), order(8, "SO-0008", "")],
    meta: { page: 1, limit: 20, total: 2, totalPages: 1 },
  });
});
afterEach(cleanup);

describe("SalesOrderSearchSelect", () => {
  it("mencari server-side dengan search, status CSV, partyId dan limit 20", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SalesOrderSearchSelect value={null} onChange={onChange} status="confirmed,completed" partyId={3} />,
    );
    await user.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1));
    expect(list.mock.calls[0][0]).toEqual({
      search: "",
      status: "confirmed,completed",
      partyId: 3,
      page: 1,
      limit: 20,
    });
    expect(list.mock.calls[0][1]).toBeInstanceOf(AbortSignal);

    await user.type(screen.getByPlaceholderText("Ketik nomor atau nama pelanggan…"), "bud");
    await waitFor(() => expect(list.mock.calls.at(-1)?.[0]).toMatchObject({ search: "bud" }));

    // Walk-in tampil sebagai "Tunai"; memilih mengembalikan id + objek order.
    await user.click(await screen.findByRole("option", { name: /SO-0008 — Tunai/ }));
    expect(onChange).toHaveBeenCalledWith(8, expect.objectContaining({ number: "SO-0008" }));
  });

  it("mengambil satu order untuk label bila nilai awal tanpa seed", async () => {
    get.mockResolvedValue(order(7, "SO-0007", "Budi"));
    render(<SalesOrderSearchSelect value={7} onChange={vi.fn()} />);
    expect(await screen.findByText("SO-0007 — Budi")).toBeInTheDocument();
    expect(get).toHaveBeenCalledWith(7);
  });
});
