import { describe, expect, it } from "vitest";
import {
  editableDefinitions,
  initialSettingValues,
  parseOperationalSettings,
  serverSettingErrors,
} from "@/modules/company/schemas/settings.schema";
import type { SettingDefinition } from "@/modules/company/types";

const defs: SettingDefinition[] = [
  { key: "stock_adjust_always_approve", kind: "boolean", default: "true", label: "a" },
  { key: "stock_adjust_approval_mode", kind: "enum", default: "simple", enum: ["simple", "strict"], label: "b" },
  { key: "stock_adjust_approval_threshold", kind: "number", default: "100", label: "c" },
  { key: "delivery_require_evidence", kind: "boolean", default: "true", label: "d" },
  { key: "document_profile", kind: "json", default: "", label: "e" },
];

describe("settings.schema", () => {
  it("document_profile tidak masuk form operasional", () => {
    expect(editableDefinitions(defs).map((d) => d.key)).not.toContain("document_profile");
  });

  it("nilai awal: tersimpan menang atas bawaan", () => {
    const v = initialSettingValues(editableDefinitions(defs), { stock_adjust_approval_mode: "strict" });
    expect(v.stock_adjust_approval_mode).toBe("strict");
    expect(v.stock_adjust_approval_threshold).toBe("100");
  });

  it("payload valid dinormalkan", () => {
    const res = parseOperationalSettings(defs, {
      stock_adjust_always_approve: "false",
      stock_adjust_approval_mode: "strict",
      stock_adjust_approval_threshold: "12,5",
      delivery_require_evidence: "true",
    });
    expect(res).toEqual({
      ok: true,
      payload: {
        stock_adjust_always_approve: "false",
        stock_adjust_approval_mode: "strict",
        stock_adjust_approval_threshold: "12.5",
        delivery_require_evidence: "true",
      },
    });
  });

  it("menolak angka negatif, enum tak dikenal, boolean salah", () => {
    const res = parseOperationalSettings(defs, {
      stock_adjust_always_approve: "ya",
      stock_adjust_approval_mode: "lenient",
      stock_adjust_approval_threshold: "-1",
      delivery_require_evidence: "true",
    });
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(Object.keys(res.errors).sort()).toEqual([
        "stock_adjust_always_approve",
        "stock_adjust_approval_mode",
        "stock_adjust_approval_threshold",
      ]);
    }
  });

  it("galat server settings.<kunci> dipetakan ke kunci", () => {
    expect(
      serverSettingErrors([{ field: "settings.stock_adjust_approval_threshold", message: "harus angka tidak negatif" }]),
    ).toEqual({ stock_adjust_approval_threshold: "harus angka tidak negatif" });
  });
});
