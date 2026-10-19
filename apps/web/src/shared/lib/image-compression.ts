// Kompresi gambar di klien sebelum unggah (OP-03; padanan legacy
// `lib/image-compression.ts`). Mengecilkan sisi terpanjang dan menurunkan
// kualitas JPEG/WebP agar foto kamera ponsel (3–8 MB) tidak membebani jaringan
// toko. Prinsip:
//   - FORMAT TIDAK DIUBAH: jpeg tetap jpeg, webp tetap webp, png tetap png
//     (PNG hanya diperkecil dimensinya; canvas tak punya parameter kualitas PNG).
//   - GAGAL = BERKAS ASLI: tipe tak didukung, canvas/decoder tak ada (mis. jsdom),
//     galat apa pun, atau hasil yang tidak lebih kecil → berkas asli dikembalikan.
//   - Berkas kecil yang dimensinya sudah di dalam batas tidak disentuh.

export interface CompressImageOptions {
  /** Sisi terpanjang maksimum (px). Default 1600. */
  maxDimension?: number;
  /** Kualitas 0–1 untuk jpeg/webp. Default 0,84. */
  quality?: number;
  /** Berkas ≤ ambang ini DAN dimensinya sudah muat → tidak diproses. Default 350 KiB. */
  smallFileThresholdBytes?: number;
}

const SUPPORTED = new Set(["image/jpeg", "image/png", "image/webp"]);

/** Hitung dimensi baru yang muat dalam `maxDimension` tanpa membesarkan. null bila dimensi tak valid. */
export function containWithin(
  width: number,
  height: number,
  maxDimension: number,
): { width: number; height: number } | null {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  const scale = Math.min(1, maxDimension / Math.max(width, height));
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}

function normalizeType(type: string): string {
  const t = type.toLowerCase();
  return t === "image/jpg" ? "image/jpeg" : t;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob((b) => resolve(b), type, quality));
}

export async function compressImageForUpload(
  file: File,
  options: CompressImageOptions = {},
): Promise<File> {
  const type = normalizeType(file.type);
  if (!SUPPORTED.has(type)) return file;
  // Tanpa decoder/canvas (jsdom, peramban lawas) → langsung berkas asli.
  if (typeof createImageBitmap !== "function" || typeof document === "undefined") return file;

  const maxDimension = options.maxDimension ?? 1600;
  const quality = options.quality ?? 0.84;
  const smallFileThreshold = options.smallFileThresholdBytes ?? 350 * 1024;

  let bitmap: ImageBitmap | null = null;
  try {
    bitmap = await createImageBitmap(file);
    const target = containWithin(bitmap.width, bitmap.height, maxDimension);
    if (!target) return file;
    const unchangedSize = target.width === bitmap.width && target.height === bitmap.height;
    if (unchangedSize && file.size <= smallFileThreshold) return file;

    const canvas = document.createElement("canvas");
    canvas.width = target.width;
    canvas.height = target.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return file;
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(bitmap, 0, 0, target.width, target.height);

    const blob = await canvasToBlob(canvas, type, quality);
    // Peramban yang tak mendukung tipe keluaran jatuh ke PNG → jangan ganti format.
    if (!blob || blob.type !== type || blob.size >= file.size) return file;
    return new File([blob], file.name, { type, lastModified: file.lastModified });
  } catch {
    return file;
  } finally {
    try {
      bitmap?.close();
    } catch {
      /* abaikan */
    }
  }
}
