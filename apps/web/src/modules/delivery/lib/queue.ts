// Label umur SJ untuk tab Menunggu Kembali (J3). Murni: unit-test tanpa DOM.

/** "Hari ini" untuk 0/negatif, "{n} hari" selebihnya. */
export function queueAgeLabel(days: number): string {
  if (!Number.isFinite(days) || days <= 0) return "Hari ini";
  return `${Math.trunc(days)} hari`;
}

/** Nada badge umur: merah bila > 7 hari, kuning bila > 0, netral hari ini. */
export function queueAgeTone(days: number): "danger" | "warning" | "neutral" {
  if (!Number.isFinite(days) || days <= 0) return "neutral";
  if (days > 7) return "danger";
  return "warning";
}
