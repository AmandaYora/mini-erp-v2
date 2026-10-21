import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReasonDialog } from "@/shared/components/patterns/reason-dialog";
import { reasonSchema } from "@/shared/components/patterns/reason-schema";
import { cancellationItems } from "@/shared/components/patterns/cancellation";

afterEach(() => cleanup());

function Harness({ onSubmit }: { onSubmit: (r: string) => void }) {
  const [open, setOpen] = useState(true);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Buka
      </button>
      <ReasonDialog
        open={open}
        title="Batalkan surat jalan"
        description="SJ-1 dibatalkan."
        confirmLabel="Batalkan surat jalan"
        onClose={() => setOpen(false)}
        onSubmit={onSubmit}
      />
    </>
  );
}

describe("ReasonDialog — alasan wajib untuk koreksi", () => {
  it("menolak alasan kosong/spasi tanpa memanggil onSubmit", async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);
    await userEvent.click(screen.getByRole("button", { name: "Batalkan surat jalan" }));
    expect(await screen.findByText("Alasan wajib diisi")).toBeTruthy();
    await userEvent.type(screen.getByLabelText(/Alasan/), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Batalkan surat jalan" }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("mengirim alasan yang sudah dirapikan", async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);
    await userEvent.type(screen.getByLabelText(/Alasan/), "  salah jumlah  ");
    await userEvent.click(screen.getByRole("button", { name: "Batalkan surat jalan" }));
    expect(onSubmit).toHaveBeenCalledWith("salah jumlah");
  });

  it("dibuka ulang dalam keadaan kosong", async () => {
    render(<Harness onSubmit={vi.fn()} />);
    await userEvent.type(screen.getByLabelText(/Alasan/), "draf lama");
    await userEvent.click(screen.getByRole("button", { name: "Kembali" }));
    expect(screen.queryByLabelText(/Alasan/)).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Buka" }));
    expect((screen.getByLabelText(/Alasan/) as HTMLTextAreaElement).value).toBe("");
  });

  it("skema membatasi 255 karakter", () => {
    expect(reasonSchema.safeParse({ reason: "a".repeat(255) }).success).toBe(true);
    expect(reasonSchema.safeParse({ reason: "a".repeat(256) }).success).toBe(false);
  });
});

describe("cancellationItems", () => {
  it("kosong untuk dokumen yang tidak dibatalkan", () => {
    expect(cancellationItems({})).toEqual([]);
    expect(cancellationItems({ cancelReason: "", cancelledAt: "" })).toEqual([]);
  });

  it("menampilkan waktu, pelaku, dan alasan", () => {
    const items = cancellationItems({ cancelReason: "salah gudang", cancelledAt: "2026-10-21 14:05:00", cancelledBy: 7 });
    expect(items.map((i) => i.label)).toEqual(["Dibatalkan", "Alasan batal"]);
    expect(String(items[0].value)).toContain("#7");
    expect(items[1].value).toBe("salah gudang");
  });
});
