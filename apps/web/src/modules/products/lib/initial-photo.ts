import { toApiError } from "@/shared/services/http-client";

// Foto awal saat membuat produk (PR-07): produk disimpan DULU, baru foto
// diunggah sebagai foto utama. Gagal unggah tidak boleh membatalkan produk
// yang sudah tersimpan — hasilnya dilaporkan agar form bisa memberi tahu
// pengguna dan tetap membuka halaman detail (tempat mengunggah ulang).

export type InitialPhotoResult = { ok: true } | { ok: false; message: string };

export async function uploadInitialPhoto(
  upload: (file: File, opts: { isPrimary: boolean; sortOrder: number }) => Promise<unknown>,
  file: File,
): Promise<InitialPhotoResult> {
  try {
    await upload(file, { isPrimary: true, sortOrder: 0 });
    return { ok: true };
  } catch (err) {
    return { ok: false, message: toApiError(err).message };
  }
}
