import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ConfirmDialog } from "./confirm-dialog";

afterEach(cleanup);

describe("ConfirmDialog — fokus & keyboard (U7-2)", () => {
  it("fokus pindah ke Batal saat terbuka; Tab hanya berputar di dalam dialog", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button type="button">Di belakang</button>
        <ConfirmDialog open title="Konfirmasi order" message="Yakin?" tone="primary" onConfirm={() => {}} onCancel={() => {}} />
      </>,
    );
    const cancel = screen.getByRole("button", { name: "Batal" });
    await waitFor(() => expect(cancel).toHaveFocus());
    await user.tab();
    expect(screen.getByRole("button", { name: "Konfirmasi" })).toHaveFocus();
    await user.tab();
    expect(cancel).toHaveFocus();
  });

  it("Batal memanggil onCancel sekali; Enter pada tombol konfirmasi hanya memanggil onConfirm", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    const onConfirm = vi.fn();
    const { rerender } = render(
      <ConfirmDialog open title="Hapus?" message="Yakin?" onConfirm={onConfirm} onCancel={onCancel} />,
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "Batal" })).toHaveFocus());
    await user.tab();
    await user.keyboard("{Enter}");
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();

    rerender(<ConfirmDialog open title="Hapus?" message="Yakin?" onConfirm={onConfirm} onCancel={onCancel} />);
    await user.click(screen.getByRole("button", { name: "Batal" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
