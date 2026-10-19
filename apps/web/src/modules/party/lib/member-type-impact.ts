// Teks dialog arsip tipe member (D3 / PR-09): arsip tetap diizinkan, tetapi
// pelanggan aktif yang memakai tipe itu turun ke harga standar (KI-72) —
// jumlahnya ditampilkan SEBELUM pengguna mengonfirmasi.

export interface ArchiveImpactState {
  loading: boolean;
  error: string | null;
  /** Jumlah pelanggan aktif pada tipe; null selama belum dimuat. */
  activeCustomers: number | null;
}

export function archiveImpactSentence(state: ArchiveImpactState): string {
  if (state.loading) return "Memeriksa pelanggan yang terdampak…";
  if (state.error || state.activeCustomers === null) {
    return "Jumlah pelanggan terdampak tidak dapat dimuat — pelanggan aktif pada tipe ini akan turun ke harga standar.";
  }
  if (state.activeCustomers === 0) return "Tidak ada pelanggan aktif yang memakai tipe ini.";
  return `${state.activeCustomers} pelanggan aktif akan turun ke harga standar.`;
}

export function archiveConfirmMessage(code: string, state: ArchiveImpactState): string {
  return `Tipe ${code} akan diarsipkan. Riwayat tetap menunjuk ke tipe ini, tetapi customer baru tidak dapat memakainya. ${archiveImpactSentence(state)}`;
}
