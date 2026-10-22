import { afterEach, describe, expect, it } from "vitest";
import {
  cameraActiveMessage,
  cameraAttempts,
  cameraFallbackMessage,
  cameraUnavailableReason,
  classifyCameras,
  loadCameraSide,
  nextCamera,
  saveCameraSide,
  sideFromLabel,
  switchCameraLabel,
  type CameraInfo,
} from "@/modules/pos/lib/camera";

describe("cameraUnavailableReason", () => {
  it("null bila secure context + getUserMedia ada", () => {
    expect(
      cameraUnavailableReason({
        isSecureContext: true,
        navigator: { mediaDevices: { getUserMedia: () => {} } },
      }),
    ).toBeNull();
  });

  it("insecure_context bila bukan secure context", () => {
    expect(
      cameraUnavailableReason({
        isSecureContext: false,
        navigator: { mediaDevices: { getUserMedia: () => {} } },
      }),
    ).toBe("insecure_context");
  });

  it("no_camera bila getUserMedia tidak ada", () => {
    expect(cameraUnavailableReason({ isSecureContext: true, navigator: {} })).toBe(
      "no_camera",
    );
  });

  it("pesan fallback berbahasa Indonesia", () => {
    expect(cameraFallbackMessage("no_camera")).toContain("ketik manual");
    expect(cameraFallbackMessage("insecure_context")).toContain("HTTPS");
  });
});

describe("preferensi sisi kamera", () => {
  afterEach(() => localStorage.clear());
  it("bawaan kamera belakang", () => {
    expect(loadCameraSide()).toBe("environment");
  });
  it("menyimpan dan membaca kamera depan", () => {
    saveCameraSide("user");
    expect(loadCameraSide()).toBe("user");
    saveCameraSide("environment");
    expect(loadCameraSide()).toBe("environment");
  });
  it("nilai asing diabaikan", () => {
    localStorage.setItem("scan-camera-side", "kiri");
    expect(loadCameraSide()).toBe("environment");
  });
});

describe("sideFromLabel", () => {
  it.each([
    ["camera2 0, facing back", "environment"],
    ["Back Camera", "environment"],
    ["Rear Wide", "environment"],
    ["Kamera Belakang", "environment"],
    ["camera2 1, facing front", "user"],
    ["Front Camera", "user"],
    ["Kamera Depan", "user"],
    ["FaceTime HD Camera", null],
    ["", null],
  ])("%s -> %s", (label, side) => {
    expect(sideFromLabel(label)).toBe(side);
  });
});

describe("classifyCameras", () => {
  const dev = (deviceId: string, label: string, kind = "videoinput") => ({ kind, deviceId, label });
  it("hanya videoinput dengan deviceId; sisi dari label", () => {
    const list = classifyCameras([dev("a", "Back Camera"), dev("m", "Mic", "audioinput"), dev("", ""), dev("b", "Webcam")]);
    expect(list).toEqual([
      { deviceId: "a", label: "Back Camera", side: "environment" },
      { deviceId: "b", label: "Webcam", side: null },
    ]);
  });
  it("facingMode track aktif mengalahkan label (label iOS dilokalkan)", () => {
    const list = classifyCameras([dev("a", "Kamera Sisi Belakang"), dev("b", "Kamera FaceTime")], {
      deviceId: "b",
      facingMode: "user",
    });
    expect(list.map((c) => c.side)).toEqual(["environment", "user"]);
  });
});

describe("nextCamera", () => {
  const cam = (deviceId: string, side: CameraInfo["side"]): CameraInfo => ({ deviceId, label: deviceId, side });
  const phone = [cam("back0", "environment"), cam("back1", "environment"), cam("front", "user")];

  it("dari belakang ke depan, cadangan deviceId sisi tujuan", () => {
    expect(nextCamera(phone, { deviceId: "back0", side: "environment" })).toEqual({
      kind: "side",
      side: "user",
      deviceId: "front",
    });
  });
  it("dari depan ke belakang: perangkat belakang pertama", () => {
    expect(nextCamera(phone, { deviceId: "front", side: "user" })).toEqual({
      kind: "side",
      side: "environment",
      deviceId: "back0",
    });
  });
  it("dua webcam tanpa sisi: berputar antar perangkat", () => {
    const desk = [cam("w1", null), cam("w2", null)];
    expect(nextCamera(desk, { deviceId: "w1", side: null })).toEqual({ kind: "device", side: null, deviceId: "w2" });
    expect(nextCamera(desk, { deviceId: "w2", side: null })).toEqual({ kind: "device", side: null, deviceId: "w1" });
  });
  it("satu kamera atau kosong: tanpa tombol", () => {
    expect(nextCamera([cam("w1", null)], { deviceId: "w1", side: null })).toBeNull();
    expect(nextCamera([], { deviceId: null, side: null })).toBeNull();
  });
});

describe("cameraAttempts", () => {
  it("awal memakai facingMode ideal", () => {
    expect(cameraAttempts({ kind: "initial", side: "environment" })).toEqual([
      { video: { facingMode: { ideal: "environment" } } },
    ]);
  });
  it("ganti sisi: exact lalu cadangan deviceId", () => {
    expect(cameraAttempts({ kind: "side", side: "user", deviceId: "front" })).toEqual([
      { video: { facingMode: { exact: "user" } } },
      { video: { deviceId: { exact: "front" } } },
    ]);
    expect(cameraAttempts({ kind: "side", side: "user", deviceId: null })).toHaveLength(1);
  });
  it("perangkat tertentu: deviceId exact", () => {
    expect(cameraAttempts({ kind: "device", side: null, deviceId: "w2" })).toEqual([{ video: { deviceId: { exact: "w2" } } }]);
  });
});

describe("label tombol & pengumuman", () => {
  it("dinamis per sisi tujuan", () => {
    expect(switchCameraLabel({ kind: "side", side: "user", deviceId: null })).toBe("Ganti ke kamera depan");
    expect(switchCameraLabel({ kind: "side", side: "environment", deviceId: null })).toBe("Ganti ke kamera belakang");
    expect(switchCameraLabel({ kind: "device", side: null, deviceId: "w2" })).toBe("Ganti kamera");
    expect(cameraActiveMessage("user")).toBe("Kamera depan aktif");
    expect(cameraActiveMessage("environment")).toBe("Kamera belakang aktif");
  });
});
