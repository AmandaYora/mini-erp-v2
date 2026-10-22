import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import { SCAN_SOUND_SRC } from "@/modules/pos/lib/scan-sound";

const zxing = vi.hoisted(() => ({ decode: vi.fn() }));
vi.mock("@zxing/browser", () => ({
  BrowserMultiFormatReader: class {
    decodeFromConstraints = zxing.decode;
  },
}));

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
  zxing.decode.mockReset();
  Reflect.deleteProperty(navigator, "mediaDevices");
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

type Opened = { deviceId: string; facingMode?: string };

/** Kamera palsu: `open` memetakan constraint -> track terbuka (atau galat). `log` mencatat urutan buka/tutup. */
function stubCamera(devices: { deviceId: string; label: string }[], open: (video: MediaTrackConstraints) => Opened) {
  const log: string[] = [];
  const listeners: (() => void)[] = [];
  const media = {
    getUserMedia: vi.fn(),
    enumerateDevices: vi.fn().mockResolvedValue(devices.map((d) => ({ kind: "videoinput", groupId: "", ...d }))),
    addEventListener: (_type: string, fn: () => void) => listeners.push(fn),
    removeEventListener: vi.fn(),
  };
  vi.stubGlobal("isSecureContext", true);
  Object.defineProperty(navigator, "mediaDevices", { configurable: true, value: media });
  zxing.decode.mockImplementation(async (constraints: MediaStreamConstraints) => {
    const track = open(constraints.video as MediaTrackConstraints);
    log.push(`buka:${track.deviceId}`);
    return { stop: () => void log.push(`tutup:${track.deviceId}`), streamVideoSettingsGet: () => track };
  });
  return { log, media, fireDeviceChange: () => listeners.forEach((fn) => fn()) };
}

const facing = (v: MediaTrackConstraints) => v.facingMode as { ideal?: string; exact?: string } | undefined;
const phoneOpen = (v: MediaTrackConstraints): Opened => {
  const side = facing(v)?.exact ?? facing(v)?.ideal;
  return side === "user" ? { deviceId: "front", facingMode: "user" } : { deviceId: "back", facingMode: "environment" };
};
const PHONE = [
  { deviceId: "back", label: "Kamera Belakang" },
  { deviceId: "front", label: "Kamera Depan" },
];
const noop = () => undefined;
const onScanNoop = () => Promise.resolve({ kind: "added" as const, name: "A", qty: 1 });

describe("ScanDialog — ganti kamera depan/belakang", () => {
  it("membuka kamera belakang dulu, lalu ganti ke depan setelah kamera lama ditutup", async () => {
    const cam = stubCamera(PHONE, phoneOpen);
    render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    const toFront = await screen.findByRole("button", { name: "Ganti ke kamera depan" });
    expect(zxing.decode.mock.calls[0][0]).toEqual({ video: { facingMode: { ideal: "environment" } } });
    expect(document.querySelector("video")).not.toHaveClass("-scale-x-100");

    fireEvent.click(toFront);
    const toBack = await screen.findByRole("button", { name: "Ganti ke kamera belakang" });
    expect(zxing.decode.mock.calls[1][0]).toEqual({ video: { facingMode: { exact: "user" } } });
    expect(cam.log).toEqual(["buka:back", "tutup:back", "buka:front"]);
    expect(document.querySelector("video")).toHaveClass("-scale-x-100");
    expect(localStorage.getItem("scan-camera-side")).toBe("user");
    expect(screen.getByText("Kamera depan aktif")).toBeInTheDocument();
    expect(toBack).toBeEnabled();
  });

  it("memakai sisi tersimpan saat dibuka", async () => {
    localStorage.setItem("scan-camera-side", "user");
    stubCamera(PHONE, phoneOpen);
    render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    await screen.findByRole("button", { name: "Ganti ke kamera belakang" });
    expect(zxing.decode.mock.calls[0][0]).toEqual({ video: { facingMode: { ideal: "user" } } });
  });

  it("facingMode exact ditolak: jatuh ke deviceId kamera sisi tujuan", async () => {
    const cam = stubCamera(PHONE, (v) => {
      if (facing(v)?.exact) throw Object.assign(new Error("exact"), { name: "OverconstrainedError" });
      const id = (v.deviceId as { exact?: string } | undefined)?.exact;
      return id === "front" ? { deviceId: "front" } : { deviceId: "back" };
    });
    render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    fireEvent.click(await screen.findByRole("button", { name: "Ganti ke kamera depan" }));
    await screen.findByRole("button", { name: "Ganti ke kamera belakang" });
    expect(zxing.decode.mock.calls.slice(1).map((c) => c[0])).toEqual([
      { video: { facingMode: { exact: "user" } } },
      { video: { deviceId: { exact: "front" } } },
    ]);
    expect(cam.log).toEqual(["buka:back", "tutup:back", "buka:front"]);
  });

  it("stream baru gagal sekali (iOS): diulang setelah jeda", async () => {
    let failed = false;
    const cam = stubCamera(PHONE, (v) => {
      if (facing(v)?.exact === "user" && !failed) {
        failed = true;
        throw Object.assign(new Error("busy"), { name: "NotReadableError" });
      }
      return phoneOpen(v);
    });
    render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    fireEvent.click(await screen.findByRole("button", { name: "Ganti ke kamera depan" }));
    await screen.findByRole("button", { name: "Ganti ke kamera belakang" });
    expect(zxing.decode).toHaveBeenCalledTimes(3);
    expect(cam.log).toEqual(["buka:back", "tutup:back", "buka:front"]);
    expect(screen.queryByText(/Kamera tidak dapat dibuka/)).toBeNull();
  });

  it("satu webcam: tombol tidak dirender; webcam kedua dipasang -> tombol muncul", async () => {
    const cam = stubCamera([{ deviceId: "w1", label: "HD Webcam" }], () => ({ deviceId: "w1" }));
    render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    await waitFor(() => expect(cam.media.enumerateDevices).toHaveBeenCalled());
    await act(async () => undefined);
    expect(screen.queryByRole("button", { name: /Ganti/ })).toBeNull();

    cam.media.enumerateDevices.mockResolvedValue([
      { kind: "videoinput", groupId: "", deviceId: "w1", label: "HD Webcam" },
      { kind: "videoinput", groupId: "", deviceId: "w2", label: "USB Camera" },
    ]);
    act(() => cam.fireDeviceChange());
    expect(await screen.findByRole("button", { name: "Ganti kamera" })).toBeInTheDocument();
  });

  it("menutup dialog menghentikan kamera", async () => {
    const cam = stubCamera(PHONE, phoneOpen);
    const view = render(<ScanDialog open paused={false} onClose={noop} onScan={onScanNoop} />);
    await screen.findByRole("button", { name: "Ganti ke kamera depan" });
    view.unmount();
    await waitFor(() => expect(cam.log).toEqual(["buka:back", "tutup:back"]));
    expect(cam.media.removeEventListener).toHaveBeenCalledWith("devicechange", expect.any(Function));
  });
});
