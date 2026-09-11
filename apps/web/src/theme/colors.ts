// Cermin JS dari token DS v1 (src/theme/theme.css) — hanya untuk tempat yang
// butuh string warna nyata: recharts, kanvas, cetak. UI biasa pakai class
// Tailwind (bg-brand, text-muted, ...) — JANGAN hardcode hex di komponen.
export const colors = {
  ink: "#0f172a",
  heading: "#334155",
  muted: "#64748b",
  hairline: "#cbd5e1",
  surface: "#ffffff",
  surfaceSubtle: "#f8fafc",
  surfaceSunken: "#f1f5f9",
  brand: "#1e3a5f",
  brandHover: "#16314f",
  accent: "#c2603a",
  accentSoft: "#fceae0",
  ok: "#15803d",
  okSoft: "#dcfce7",
  warn: "#b45309",
  warnSoft: "#fef3c7",
  bad: "#b91c1c",
  badSoft: "#fee2e2",
  sidebar: "#0f172a",
  sidebarFg: "#94a3b8",
  sidebarStrong: "#f8fafc",
} as const;
