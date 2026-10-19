import { z } from "zod";

// Enum menyalin domain/pricing.go + validateMemberType (service.go).
// Kunci payload: code,name,description,basis,direction,type,value,mode,step,
// confirmed. Kunci view untuk baca: roundingMode, roundingStep.
// Pembulatan: none|nearest|up|down + kelipatan (step). Nama lama
// (round_100/round_500/round_1000/floor/ceil) dilipat backend dan API hanya
// mengembalikan nama baru; foldRoundingMode menjaga data lama tetap terbaca.
// Persen: minus maksimal 100, plus (markup) boleh lebih dari 100, plafon teknis 100000.

export const NOMINAL_CONFIRM_THRESHOLD = 1_000_000;

export const basisOptions = [
  { value: "selling_price", label: "Harga jual" },
  { value: "min_selling_price", label: "Harga jual minimal" },
  { value: "purchase_price", label: "Harga beli (modal)" },
] as const;

export const directionOptions = [
  { value: "minus", label: "Diskon (minus)" },
  { value: "plus", label: "Markup (plus)" },
] as const;

export const adjustTypeOptions = [
  { value: "percent", label: "Persen (%)" },
  { value: "nominal", label: "Nominal (Rp)" },
] as const;

export const roundingModeOptions = [
  { value: "none", label: "Tanpa pembulatan" },
  { value: "nearest", label: "Terdekat" },
  { value: "up", label: "Ke atas" },
  { value: "down", label: "Ke bawah" },
] as const;

/** Kelipatan cepat untuk tombol pembulatan. */
export const ROUNDING_STEP_PRESETS = [100, 500, 1000] as const;

/** Persen maksimal teknis (backend domain.MaxPercent). */
export const MAX_PERCENT = 100_000;

const LEGACY_ROUNDING: Record<string, { mode: string; step: number }> = {
  round_100: { mode: "nearest", step: 100 },
  round_500: { mode: "nearest", step: 500 },
  round_1000: { mode: "nearest", step: 1000 },
  floor: { mode: "down", step: 100 },
  ceil: { mode: "up", step: 100 },
};

/** Lipat nama pembulatan lama ke enum baru (identitas untuk nama baru). */
export function foldRoundingMode(mode: string, step: number): { mode: string; step: number } {
  const legacy = LEGACY_ROUNDING[mode];
  if (!legacy) return { mode, step };
  return { mode: legacy.mode, step: step > 0 ? step : legacy.step };
}

/** Ringkas pembulatan untuk tampilan, mis. "Terdekat 100". */
export function roundingSummary(mode: string, step: number): string {
  const f = foldRoundingMode(mode, step);
  const label = optionLabel(roundingModeOptions, f.mode);
  return f.mode === "none" || f.step <= 0 ? label : `${label} ${f.step}`;
}

/** Batas persen menurut arah: minus <= 100, plus <= MAX_PERCENT. */
export function maxPercentFor(direction: string): number {
  return direction === "plus" ? MAX_PERCENT : 100;
}

export function optionLabel(
  options: readonly { value: string; label: string }[],
  value: string,
): string {
  return options.find((o) => o.value === value)?.label ?? value;
}

interface MemberRule {
  direction: "minus" | "plus";
  type: "percent" | "nominal";
  value: number;
  mode: string;
  step: number;
}

function checkMemberRule(v: MemberRule, ctx: z.RefinementCtx): void {
  if (v.type === "percent") {
    if (v.direction === "minus" && v.value > 100) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["value"],
        message: "Potongan persen maksimal 100",
      });
    } else if (v.value > MAX_PERCENT) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["value"],
        message: `Persen maksimal ${MAX_PERCENT.toLocaleString("id-ID")}`,
      });
    }
  }
  // Mode+kelipatan harus berpasangan (KI-73).
  if (v.mode !== "none" && v.step <= 0) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ["step"],
      message: "Kelipatan wajib diisi bila mode pembulatan dipakai",
    });
  }
  if (v.mode === "none" && v.step !== 0) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ["step"],
      message: "Kelipatan harus 0 bila tanpa pembulatan",
    });
  }
}

const memberTypeFields = {
  name: z.string().trim().min(1, "Nama wajib diisi").max(200, "Nama maksimal 200 karakter"),
  description: z.string().trim().max(500, "Deskripsi maksimal 500 karakter").default(""),
  basis: z.enum(["selling_price", "min_selling_price", "purchase_price"]).default("selling_price"),
  direction: z.enum(["minus", "plus"]).default("minus"),
  type: z.enum(["percent", "nominal"]).default("percent"),
  value: z.coerce.number().min(0, "Nilai tidak boleh negatif"),
  mode: z.enum(["none", "nearest", "up", "down"]).default("none"),
  step: z.coerce.number().min(0, "Kelipatan tidak boleh negatif").default(0),
  confirmed: z.boolean().default(false),
};

// Create: code wajib, immutable setelah terbit.
export const memberTypeCreateSchema = z
  .object({
    ...memberTypeFields,
    code: z
      .string()
      .trim()
      .min(1, "Kode wajib diisi")
      .max(50, "Kode maksimal 50 karakter")
      .transform((s) => s.toUpperCase()),
  })
  .superRefine(checkMemberRule);

// Update: tanpa code (terkunci).
export const memberTypeUpdateSchema = z.object(memberTypeFields).superRefine(checkMemberRule);

export type MemberTypeCreateValues = z.infer<typeof memberTypeCreateSchema>;
export type MemberTypeUpdateValues = z.infer<typeof memberTypeUpdateSchema>;
