import { receiptSettlementSchema, toSettlementPayload } from "@/modules/goodsreceipt/schemas/goodsreceipt.schema";
import type { ReceiptSettlementPayload } from "@/modules/goodsreceipt/types";

/** Isian form pelunasan; mode "" = tanpa pelunasan. */
export interface SettlementDraft {
  mode: "" | "pay_now" | "switch_to_net";
  method: "cash" | "transfer" | "check";
  paymentDate: string;
  reference: string;
  dueDate: string;
  notes: string;
}

export function emptySettlementDraft(today: string): SettlementDraft {
  return { mode: "", method: "cash", paymentDate: today, reference: "", dueDate: "", notes: "" };
}

export type SettlementResult =
  | { ok: true; payload: ReceiptSettlementPayload | undefined }
  | { ok: false; errors: Record<string, string> };

/** Validasi draft → payload (undefined bila tanpa pelunasan). */
export function resolveSettlement(draft: SettlementDraft): SettlementResult {
  if (draft.mode === "") return { ok: true, payload: undefined };
  const parsed = receiptSettlementSchema.safeParse(draft);
  if (!parsed.success) {
    const errors: Record<string, string> = {};
    for (const issue of parsed.error.issues) {
      errors[issue.path.join(".") || "mode"] ??= issue.message;
    }
    return { ok: false, errors };
  }
  return { ok: true, payload: toSettlementPayload(parsed.data) };
}

/** Pesan server `settlement.xxx` → kunci field draft. */
export function settlementFieldErrors(fields: { field: string; message: string }[] | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  for (const f of fields ?? []) {
    if (f.field === "settlement" || f.field.startsWith("settlement.")) {
      out[f.field === "settlement" ? "mode" : f.field.slice("settlement.".length)] ??= f.message;
    }
  }
  return out;
}
