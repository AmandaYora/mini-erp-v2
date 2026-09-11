import { describe, expect, it } from "vitest";
import {
  cameraFallbackMessage,
  cameraUnavailableReason,
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
