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

// --- Ganti kamera depan/belakang -------------------------------------------
// Label & deviceId baru terisi SETELAH izin kamera diberikan, jadi kamera dibuka
// dulu dengan facingMode lalu daftar perangkat dibaca ulang. Sisi tiap perangkat:
// facingMode track aktif dulu (label iOS dilokalkan), lalu label, selain itu
// tidak diketahui (umumnya webcam desktop).

export type CameraSide = "environment" | "user";

export interface CameraInfo {
  deviceId: string;
  label: string;
  side: CameraSide | null;
}

/** Permintaan buka kamera: awal (sisi pilihan, longgar) atau hasil tombol ganti. */
export type CameraRequest =
  | { kind: "initial"; side: CameraSide }
  // Ganti sisi: facingMode exact; deviceId = cadangan bila exact ditolak.
  | { kind: "side"; side: CameraSide; deviceId: string | null }
  // Berputar antar perangkat yang sisinya tak diketahui (mis. dua webcam desktop).
  | { kind: "device"; side: CameraSide | null; deviceId: string };

export type CameraTarget = Exclude<CameraRequest, { kind: "initial" }>;

const SIDE_KEY = "scan-camera-side";

/** Sisi kamera tersimpan per perangkat; bawaan kamera belakang. */
export function loadCameraSide(): CameraSide {
  try {
    if (localStorage.getItem(SIDE_KEY) === "user") return "user";
  } catch {
    /* penyimpanan tak tersedia: ikut bawaan */
  }
  return "environment";
}

export function saveCameraSide(side: CameraSide): void {
  try {
    localStorage.setItem(SIDE_KEY, side);
  } catch {
    /* abaikan */
  }
}

export function sideFromFacingMode(facingMode: string | undefined): CameraSide | null {
  return facingMode === "environment" || facingMode === "user" ? facingMode : null;
}

export function sideFromLabel(label: string): CameraSide | null {
  if (/back|rear|environment|belakang/i.test(label)) return "environment";
  if (/front|user|depan/i.test(label)) return "user";
  return null;
}

/** Daftar videoinput (hasil enumerateDevices setelah izin) beserta sisinya.
 * `active` = settings track yang sedang terbuka: facingMode-nya mengalahkan label. */
export function classifyCameras(
  devices: readonly { kind: string; deviceId: string; label: string }[],
  active?: { deviceId?: string; facingMode?: string },
): CameraInfo[] {
  return devices
    .filter((d) => d.kind === "videoinput" && d.deviceId !== "")
    .map((d) => {
      const facing = active?.deviceId === d.deviceId ? sideFromFacingMode(active.facingMode) : null;
      return { deviceId: d.deviceId, label: d.label, side: facing ?? sideFromLabel(d.label) };
    });
}

/** Tujuan tombol ganti kamera, atau null bila tombol tidak perlu dirender:
 * ada kamera di dua sisi -> ganti sisi; >=2 perangkat tanpa dua sisi -> perangkat berikutnya. */
export function nextCamera(
  cameras: readonly CameraInfo[],
  active: { deviceId: string | null; side: CameraSide | null },
): CameraTarget | null {
  const hasBack = cameras.some((c) => c.side === "environment");
  const hasFront = cameras.some((c) => c.side === "user");
  if (hasBack && hasFront) {
    const side: CameraSide = active.side === "user" ? "environment" : "user";
    const fallback = cameras.find((c) => c.side === side && c.deviceId !== active.deviceId);
    return { kind: "side", side, deviceId: fallback?.deviceId ?? null };
  }
  if (cameras.length >= 2) {
    const i = cameras.findIndex((c) => c.deviceId === active.deviceId);
    const next = cameras[(i + 1) % cameras.length];
    return { kind: "device", side: next.side, deviceId: next.deviceId };
  }
  return null;
}

/** Constraint yang dicoba berurutan; berikutnya dipakai bila sebelumnya Overconstrained. */
export function cameraAttempts(request: CameraRequest): MediaStreamConstraints[] {
  if (request.kind === "initial") return [{ video: { facingMode: { ideal: request.side } } }];
  if (request.kind === "device") return [{ video: { deviceId: { exact: request.deviceId } } }];
  const attempts: MediaStreamConstraints[] = [{ video: { facingMode: { exact: request.side } } }];
  if (request.deviceId) attempts.push({ video: { deviceId: { exact: request.deviceId } } });
  return attempts;
}

/** aria-label tombol ganti kamera. */
export function switchCameraLabel(target: CameraTarget): string {
  if (target.side === "user") return "Ganti ke kamera depan";
  if (target.side === "environment") return "Ganti ke kamera belakang";
  return "Ganti kamera";
}

/** Pengumuman pembaca layar setelah kamera berganti. */
export function cameraActiveMessage(side: CameraSide | null): string {
  if (side === "user") return "Kamera depan aktif";
  if (side === "environment") return "Kamera belakang aktif";
  return "Kamera lain aktif";
}
