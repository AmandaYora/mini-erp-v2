// Alasan kamera tidak tersedia — murni (tanpa IO) agar bisa diuji.
// Pola diport dari sistem lama (qr-scanner-modal.tsx:
// getQrCameraUnavailableReason): kamera butuh secure context (HTTPS /
// localhost) + MediaDevices.getUserMedia. Bila salah satu tak ada, halaman
// memakai input ketik/wedge sebagai fallback, bukan gagal total.

export type CameraUnavailableReason =
  | "insecure_context"
  | "no_camera";

export interface CameraRuntime {
  isSecureContext?: boolean;
  navigator?: {
    mediaDevices?: {
      getUserMedia?: unknown;
    };
  };
}

/** null = kamera bisa dipakai; selain itu kode alasan untuk pesan fallback. */
export function cameraUnavailableReason(
  runtime: CameraRuntime = globalThis as unknown as CameraRuntime,
): CameraUnavailableReason | null {
  if (runtime.isSecureContext === false) {
    return "insecure_context";
  }
  if (typeof runtime.navigator?.mediaDevices?.getUserMedia !== "function") {
    return "no_camera";
  }
  return null;
}

/** Pesan fallback Indonesia untuk tiap alasan. */
export function cameraFallbackMessage(reason: CameraUnavailableReason): string {
  if (reason === "insecure_context") {
    return "Kamera perlu HTTPS — gunakan ketik manual / scanner USB di bawah.";
  }
  return "Kamera tidak tersedia — gunakan ketik manual / scanner USB di bawah.";
}

/** Pilih kamera belakang: label berbau "belakang" bila ada, selain itu yang terakhir
 * (perilaku lama: tablet/ponsel biasanya mendaftar kamera belakang di urutan akhir). */
export function pickRearCamera<T extends { deviceId: string; label: string }>(devices: T[]): T | undefined {
  const byLabel = devices.find((d) => /back|rear|belakang|environment/i.test(d.label));
  return byLabel ?? devices[devices.length - 1] ?? devices[0];
}
