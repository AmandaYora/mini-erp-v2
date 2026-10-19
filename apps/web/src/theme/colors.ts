// Cermin hex dari token DS v2 (src/theme/theme.css, OKLCH) — hanya untuk tempat yang
// butuh string warna nyata di luar CSS: kanvas, cetak, dan pustaka yang tidak membaca
// var(--token). Recharts boleh memakai "var(--chart-1)" langsung. UI biasa memakai kelas
// semantik (bg-card, text-muted-foreground, ...) — JANGAN hardcode hex di komponen.
// Nilai dihitung dari OKLCH di PLAN Lampiran C; ubah keduanya bersamaan.
export const colors = {
  ink: "#121824",
  heading: "#121824",
  muted: "#5c6470",
  hairline: "#e0e3e8",
  lineStrong: "#8c939c",
  surface: "#ffffff",
  surfaceSubtle: "#f9fafc",
  surfaceSunken: "#f1f4f7",
  brand: "#1e3a5f",
  brandHover: "#162e4e",
  brandSoft: "#e8f1fc",
  accent: "#9d580c",
  accentSoft: "#fff2d2",
  ok: "#0b7643",
  okSoft: "#e0f9e8",
  warn: "#9d580c",
  warnSoft: "#fff2d2",
  bad: "#be2323",
  badSoft: "#ffece9",
  sidebar: "#101e33",
  sidebarFg: "#b4bfce",
  sidebarStrong: "#f2f5f9",
  chart1: "#355e95",
  chart2: "#1aa598",
  chart3: "#e1a035",
  chart4: "#a059b1",
  chart5: "#8693a5",
} as const;
