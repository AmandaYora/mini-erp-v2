import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReasonDialog } from "@/modules/purchasing/components/ReasonDialog";

afterEach(cleanup);

describe("ReasonDialog", () => {
  it("menolak tanpa alasan dan mengirim alasan yang dirapikan", () => {
    const onConfirm = vi.fn();
    render(
      <ReasonDialog
        open
        title="Batalkan order"
        message="Batalkan order PO-1?"
        confirmLabel="Batalkan order"
        submitting={false}
        serverError={null}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Batalkan order" }));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(screen.getByRole("alert").textContent).toContain("Alasan wajib diisi");
    fireEvent.change(screen.getByLabelText(/Alasan/), { target: { value: "  order dobel  " } });
    fireEvent.click(screen.getByRole("button", { name: "Batalkan order" }));
    expect(onConfirm).toHaveBeenCalledWith("order dobel");
  });

  it("menampilkan pesan server apa adanya", () => {
    render(
      <ReasonDialog
        open
        title="Batalkan order"
        message="Batalkan?"
        confirmLabel="Batalkan order"
        submitting={false}
        serverError="Order sudah memiliki penerimaan barang yang terposting"
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.getByText("Order sudah memiliki penerimaan barang yang terposting")).toBeTruthy();
  });
});
