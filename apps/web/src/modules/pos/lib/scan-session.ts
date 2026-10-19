// Pindai kamera beruntun (logika murni, diuji di scan-session.test.ts).
//
// Kamera membaca kode di setiap bingkai, jadi satu barcode yang ditahan di depan
// lensa akan terbaca puluhan kali. `gateScan` meredam bacaan berulang: kode yang
// sama dalam SCAN_DEBOUNCE_MS dari bacaan TERAKHIR (bukan dari yang diterima)
// diabaikan, sehingga barang yang masih terlihat tidak menambah lagi. Kode lain
// di antaranya mereset penjaga, jadi A-B-A tetap tiga barang.

/** Jendela peredam kode yang sama (±1,5 detik). */
export const SCAN_DEBOUNCE_MS = 1500;

export interface ScanGate {
  /** Kode terakhir yang terlihat kamera. */
  code: string | null;
  /** Waktu (ms) bacaan terakhir itu. */
  at: number;
}

export const EMPTY_SCAN_GATE: ScanGate = { code: null, at: 0 };

export interface GateResult {
  accept: boolean;
  gate: ScanGate;
}

/** Putuskan apakah bacaan `code` pada waktu `now` diterima; kembalikan penjaga baru. */
export function gateScan(
  gate: ScanGate,
  code: string,
  now: number,
  windowMs: number = SCAN_DEBOUNCE_MS,
): GateResult {
  const clean = code.trim();
  if (clean === "") return { accept: false, gate };
  if (gate.code === clean && Math.abs(now - gate.at) < windowMs) {
    // Masih kode yang sama di bidikan: perpanjang jendela, jangan terima.
    return { accept: false, gate: { code: clean, at: now } };
  }
  return { accept: true, gate: { code: clean, at: now } };
}

/** Hasil satu pindaian yang dilaporkan ke dialog kamera. */
export type ScanOutcome =
  | { kind: "added"; name: string; qty: number }
  | { kind: "choose"; productName: string }
  | { kind: "unknown"; code: string; detail: string }
  | { kind: "rejected"; detail: string }
  | { kind: "blocked" };

export type ScanChipTone = "success" | "warning" | "danger";

export interface ScanChip {
  id: number;
  tone: ScanChipTone;
  text: string;
}

/** Potong kode panjang untuk chip ("8992761…"). */
export function shortCode(code: string, max = 10): string {
  return code.length > max ? `${code.slice(0, max)}…` : code;
}

/** Teks + nada chip untuk satu hasil pindai. */
export function chipFor(outcome: ScanOutcome): { tone: ScanChipTone; text: string } {
  switch (outcome.kind) {
    case "added":
      return { tone: "success", text: `＋ ${outcome.name} (${outcome.qty})` };
    case "choose":
      return { tone: "warning", text: `Pilih varian: ${outcome.productName}` };
    case "unknown":
      return { tone: "danger", text: `Kode ${shortCode(outcome.code)} tidak ditemukan` };
    case "blocked":
      return { tone: "danger", text: "Keranjang terkunci — checkout sedang berjalan" };
    case "rejected":
    default:
      return { tone: "danger", text: outcome.detail };
  }
}

/** Tambah chip terbaru di depan; simpan paling banyak `max`. */
export function pushChip(list: ScanChip[], chip: ScanChip, max = 6): ScanChip[] {
  return [chip, ...list].slice(0, max);
}
