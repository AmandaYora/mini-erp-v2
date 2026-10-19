// Penjaga induk-berstok pada lokasi (F2a-9a). Server menolak menaruh lokasi
// di bawah induk yang masih memegang stok dengan pesan berawalan
// "PARENT_HAS_STOCK:"; UI menerjemahkannya menjadi dialog konfirmasi lalu
// mengirim ulang dengan forceTransfer=true.
export const PARENT_HAS_STOCK_CODE = "PARENT_HAS_STOCK";

export function isParentHasStock(message: string | null | undefined): boolean {
  return (message ?? "").includes(PARENT_HAS_STOCK_CODE);
}

/** Buang awalan kode mesin supaya pesan server tampil apa adanya ke manusia. */
export function stripLocationCode(message: string): string {
  return message.replace(/^\s*PARENT_HAS_STOCK:\s*/, "").trim();
}

export function locationKindLabel(kind: string): string {
  return kind === "damaged" ? "Rusak" : "Normal";
}
