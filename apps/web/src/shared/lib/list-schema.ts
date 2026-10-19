import { z } from "zod";
import { fromCsv } from "./list-query";

// Potongan skema Zod yang dipakai ulang oleh skema filter daftar tiap modul.

/** Tanggal YYYY-MM-DD atau kosong. */
export const dateFilter = z.string().regex(/^(\d{4}-\d{2}-\d{2})?$/, "Format tanggal YYYY-MM-DD");

/** Teks bebas pendek (kolom cari). */
export const searchFilter = z.string().max(100);

/** Id pihak/dokumen: 0 = tidak difilter. */
export const idFilter = z.number().int().nonnegative();

/** Nomor halaman >= 1. */
export const pageFilter = z.number().int().min(1);

/** CSV yang setiap itemnya harus salah satu dari `allowed` (kosong = semua). */
export function csvOf(allowed: readonly string[]) {
  return z.string().refine(
    (v) => fromCsv(v).every((item) => allowed.includes(item)),
    "Nilai tidak dikenal",
  );
}

/** Enum yang menerima "" (tidak difilter) selain nilai sahnya. */
export function enumOrEmpty<T extends readonly [string, ...string[]]>(values: T) {
  return z.enum(["", ...values] as [string, ...string[]]);
}
