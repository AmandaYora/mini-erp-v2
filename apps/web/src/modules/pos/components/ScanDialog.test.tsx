import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import { SCAN_SOUND_SRC } from "@/modules/pos/lib/scan-sound";

const play = vi.fn().mockResolvedValue(undefined);
const created: string[] = [];

beforeEach(() => {
  play.mockClear();
  created.length = 0;
  localStorage.clear();
  vi.stubGlobal(
    "Audio",
    class {
      volume = 1;
      constructor(src: string) {
        created.push(src);
      }
      play = play;
    },
  );
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function scanManual(code: string) {
  fireEvent.change(screen.getByLabelText("Kode manual"), { target: { value: code } });
  fireEvent.click(screen.getByRole("button", { name: "Tambah" }));
}

describe("ScanDialog — bunyi sukses (SL-08)", () => {
  it("memutar bunyi saat kode dikenal dan tidak saat tidak dikenal", async () => {
    const onScan = vi
      .fn()
      .mockResolvedValueOnce({ kind: "added", name: "Barang A", qty: 1 })
      .mockResolvedValueOnce({ kind: "unknown", code: "X", detail: "" });
    render(<ScanDialog open paused={false} onClose={() => undefined} onScan={onScan} />);
    await scanManual("111");
    await waitFor(() => expect(play).toHaveBeenCalledTimes(1));
    expect(created[0]).toBe(SCAN_SOUND_SRC);
    await scanManual("999");
    await waitFor(() => expect(onScan).toHaveBeenCalledTimes(2));
    expect(play).toHaveBeenCalledTimes(1);
  });

  it("tombol bisukan mematikan bunyi dan preferensi tersimpan", async () => {
    const onScan = vi.fn().mockResolvedValue({ kind: "added", name: "Barang A", qty: 1 });
    render(<ScanDialog open paused={false} onClose={() => undefined} onScan={onScan} />);
    fireEvent.click(screen.getByRole("button", { name: "Bisukan bunyi pindai" }));
    expect(localStorage.getItem("scan-sound-muted")).toBe("1");
    await scanManual("111");
    await waitFor(() => expect(onScan).toHaveBeenCalled());
    expect(play).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Nyalakan bunyi pindai" })).toBeInTheDocument();
  });

  it("bawaan bisu bagi pengguna prefers-reduced-motion tanpa pilihan tersimpan", async () => {
    vi.stubGlobal("matchMedia", () => ({ matches: true }));
    const onScan = vi.fn().mockResolvedValue({ kind: "added", name: "Barang A", qty: 1 });
    render(<ScanDialog open paused={false} onClose={() => undefined} onScan={onScan} />);
    await scanManual("111");
    await waitFor(() => expect(onScan).toHaveBeenCalled());
    expect(play).not.toHaveBeenCalled();
  });
});
