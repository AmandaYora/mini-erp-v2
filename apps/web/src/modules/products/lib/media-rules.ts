// Batas unggah foto produk — cerminan media/application/service.go (MaxFileBytes
// 5 MB) dan product/presentation/media.go (Accept jpeg|png|webp; .pdf dan
// gif/bmp sudah dibuang). Server tetap memeriksa isi berkas; ini hanya
// pagar awal supaya pengguna tak menunggu unggahan yang pasti ditolak.

export const MEDIA_MAX_BYTES = 5 * 1024 * 1024;
export const MEDIA_ACCEPT_MIME = ["image/jpeg", "image/png", "image/webp"] as const;
export const MEDIA_ACCEPT_EXT = [".jpg", ".jpeg", ".png", ".webp"] as const;
/** Nilai atribut `accept` untuk input berkas. */
export const MEDIA_ACCEPT_ATTR = [...MEDIA_ACCEPT_MIME, ...MEDIA_ACCEPT_EXT].join(",");

export interface FileLike {
  name: string;
  size: number;
  type: string;
}

/** Pesan galat bila berkas ditolak, atau null bila boleh diunggah. */
export function validatePhotoFile(file: FileLike): string | null {
  const name = file.name.toLowerCase();
  const extOk = MEDIA_ACCEPT_EXT.some((e) => name.endsWith(e));
  const mimeOk = file.type === "" || (MEDIA_ACCEPT_MIME as readonly string[]).includes(file.type);
  if (!extOk || !mimeOk) {
    return "Foto harus berformat JPEG, PNG, atau WebP (PDF dan format lain tidak diterima).";
  }
  if (file.size <= 0) return "Berkas kosong.";
  if (file.size > MEDIA_MAX_BYTES) return "Ukuran foto maksimal 5 MB.";
  return null;
}

/** "1600 × 1200 px" atau null bila server tak mengirim dimensi (baris lama). */
export function formatDimensions(width?: number, height?: number): string | null {
  if (!width || !height || width <= 0 || height <= 0) return null;
  return `${width} × ${height} px`;
}
