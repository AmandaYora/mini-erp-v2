// Galat daftar item dari server: SEMUA kegagalan harga/qty memakai field
// "items" dengan awalan "baris N: ..." (sales/application priceLines). Pemetaan
// biasa `mapped[field] ??= message` hanya menyimpan yang pertama — di sini
// seluruh pesan dipertahankan dan dipetakan ke baris.

export interface FieldErrorLike {
  field: string;
  message: string;
}

export interface ItemErrors {
  /** Pesan tanpa nomor baris (mis. "wajib diisi"). */
  general: string[];
  /** Nomor baris 0-based -> pesan (pesan banyak digabung). */
  byRow: Record<number, string>;
}

const ROW_RE = /^baris\s+(\d+):\s*(.*)$/i;

export function splitItemErrors(errors: FieldErrorLike[] | undefined): ItemErrors {
  const out: ItemErrors = { general: [], byRow: {} };
  for (const e of errors ?? []) {
    if (e.field !== "items") continue;
    const m = ROW_RE.exec(e.message.trim());
    if (m === null) {
      out.general.push(e.message);
      continue;
    }
    const idx = Number(m[1]) - 1;
    const text = m[2];
    out.byRow[idx] = out.byRow[idx] ? `${out.byRow[idx]}; ${text}` : text;
  }
  return out;
}
