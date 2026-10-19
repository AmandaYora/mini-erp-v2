/**
 * Kelas dasar kontrol isian (input/select/textarea/trigger combobox).
 * Tinggi 36px, border `input` (kontras ≥ 3:1), cincin fokus token `ring`, status galat via aria-invalid.
 */
export const INPUT =
  "h-9 w-full rounded-md border border-input bg-card px-3 text-base text-foreground shadow-[var(--shadow-card)] transition-[border-color,box-shadow] duration-150 placeholder:text-muted-foreground/70 hover:border-muted-foreground/60 focus:border-ring focus:ring-[3px] focus:ring-ring/25 aria-invalid:border-destructive aria-invalid:ring-destructive/20 disabled:cursor-not-allowed disabled:bg-muted disabled:text-muted-foreground";
