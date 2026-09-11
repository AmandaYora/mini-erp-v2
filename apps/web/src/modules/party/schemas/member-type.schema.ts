import { z } from "zod";

// Enum menyalin domain/pricing.go + validateMemberType (service.go).
// Kunci payload: code,name,description,basis,direction,type,value,mode,step,
// confirmed. Kunci view untuk baca: roundingMode, roundingStep.

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
  { value: "round_100", label: "Terdekat 100" },
  { value: "round_500", label: "Terdekat 500" },
  { value: "round_1000", label: "Terdekat 1000" },
  { value: "floor", label: "Ke bawah (floor)" },
  { value: "ceil", label: "Ke atas (ceil)" },
] as const;

export function optionLabel(
  options: readonly { value: string; label: string }[],
  value: string,
): string {
  return options.find((o) => o.value === value)?.label ?? value;
}

interface MemberRule {
  type: "percent" | "nominal";
  value: number;
  mode: string;
  step: number;
}

function checkMemberRule(v: MemberRule, ctx: z.RefinementCtx): void {
  if (v.type === "percent" && v.value > 100) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["value"], message: "Persen harus 0–100" });
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
  mode: z.enum(["none", "round_100", "round_500", "round_1000", "floor", "ceil"]).default("none"),
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
