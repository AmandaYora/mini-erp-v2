import { z } from "zod";

// Koreksi harga modal (apps/api/internal/modules/finance/presentation/correction_handlers.go):
// - commit hitung ulang HPP & revaluasi: reason wajib, maks 500 karakter (dipangkas).
// - revaluasi: newUnitCost rupiah bulat ≥ 0 per satuan dasar (revaluation.go: validateRevaluation).
// - tanggal opsional YYYY-MM-DD; masa depan/periode tertutup ditolak server (ADR-0021).

export const correctionReasonSchema = z.object({
  reason: z
    .string()
    .trim()
    .min(1, "Alasan wajib diisi")
    .max(500, "Alasan maksimal 500 karakter"),
});

export type CorrectionReasonValues = z.infer<typeof correctionReasonSchema>;

export const revaluationCostSchema = z.object({
  newUnitCost: z.coerce
    .number({ invalid_type_error: "Harga modal harus angka" })
    .int("Harga modal harus rupiah bulat")
    .nonnegative("Harga modal tidak boleh negatif"),
});

export type RevaluationCostValues = z.infer<typeof revaluationCostSchema>;
