import { describe, expect, it } from "vitest";
import {
  cameraFallbackMessage,
  cameraUnavailableReason,
  pickRearCamera,
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

describe("pickRearCamera", () => {
  const cam = (deviceId: string, label: string) => ({ deviceId, label });
  it("memilih label kamera belakang", () => {
    expect(pickRearCamera([cam("a", "Back Camera"), cam("b", "Front Camera")])?.deviceId).toBe("a");
  });
  it("tanpa petunjuk label: yang terakhir", () => {
    expect(pickRearCamera([cam("a", ""), cam("b", "")])?.deviceId).toBe("b");
  });
  it("daftar kosong: undefined", () => {
    expect(pickRearCamera([])).toBeUndefined();
  });
});
