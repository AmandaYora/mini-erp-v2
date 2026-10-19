import { z } from "zod";
import type { SettingDefinition } from "@/modules/company/types";

/** Kunci yang punya form sendiri (kop dokumen) dan tidak masuk form operasional. */
export const SETTINGS_WITH_OWN_FORM = ["document_profile"];

/** Definisi yang diedit lewat kontrol bertipe di form operasional. */
export function editableDefinitions(defs: SettingDefinition[]): SettingDefinition[] {
  return defs.filter((d) => !SETTINGS_WITH_OWN_FORM.includes(d.key) && d.kind !== "json");
}

/** Nilai awal form: nilai tersimpan bila ada, jika tidak nilai bawaan definisi. */
export function initialSettingValues(
  defs: SettingDefinition[],
  stored: Record<string, string>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const d of defs) out[d.key] = stored[d.key] ?? d.default;
  return out;
}

function fieldSchema(def: SettingDefinition): z.ZodType<string> {
  switch (def.kind) {
    case "boolean":
      return z.enum(["true", "false"], { message: "Harus true atau false" });
    case "enum": {
      const allowed = def.enum ?? [];
      return z.string().refine((v) => allowed.includes(v), {
        message: `Harus salah satu dari: ${allowed.join(", ")}`,
      });
    }
    case "number":
      return z
        .string()
        .trim()
        .refine((v) => v !== "" && Number.isFinite(Number(v.replace(",", "."))), {
          message: "Harus berupa angka",
        })
        .refine((v) => Number(v.replace(",", ".")) >= 0, {
          message: "Harus angka tidak negatif",
        })
        .transform((v) => v.replace(",", "."));
    default:
      return z.string();
  }
}

/** Skema Zod dinamis dari definisi server; tipe keluaran: peta kunci → string. */
export function buildOperationalSettingsSchema(defs: SettingDefinition[]) {
  const shape: Record<string, z.ZodType<string>> = {};
  for (const d of editableDefinitions(defs)) shape[d.key] = fieldSchema(d);
  return z.object(shape);
}

export type OperationalSettingsValues = z.infer<ReturnType<typeof buildOperationalSettingsSchema>>;

export type SettingsParseResult =
  | { ok: true; payload: Record<string, string> }
  | { ok: false; errors: Record<string, string> };

/** Validasi nilai form dan hasilkan payload merge PUT /company/settings. */
export function parseOperationalSettings(
  defs: SettingDefinition[],
  values: Record<string, string>,
): SettingsParseResult {
  const res = buildOperationalSettingsSchema(defs).safeParse(values);
  if (res.success) return { ok: true, payload: res.data };
  const errors: Record<string, string> = {};
  for (const issue of res.error.issues) errors[String(issue.path[0] ?? "")] ??= issue.message;
  return { ok: false, errors };
}

/** Pesan galat server `settings.<kunci>` → peta kunci → pesan. */
export function serverSettingErrors(
  errors: { field?: string; message: string }[] | undefined,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const e of errors ?? []) {
    if (!e.field) continue;
    out[e.field.replace(/^settings\./, "")] ??= e.message;
  }
  return out;
}
