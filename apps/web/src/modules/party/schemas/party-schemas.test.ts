import { describe, expect, it } from "vitest";
import {
  MAX_PERCENT,
  foldRoundingMode,
  maxPercentFor,
  memberTypeCreateSchema,
  roundingModeOptions,
  roundingSummary,
} from "@/modules/party/schemas/member-type.schema";
import { partyFormSchema } from "@/modules/party/schemas/party.schema";

const member = {
  code: "gold",
  name: "Gold",
  basis: "selling_price",
  direction: "minus",
  type: "percent",
  value: 10,
  mode: "none",
  step: 0,
};

function issues(input: unknown): Record<string, string> {
  const r = memberTypeCreateSchema.safeParse(input);
  if (r.success) return {};
  const out: Record<string, string> = {};
  for (const i of r.error.issues) out[i.path.join(".")] ??= i.message;
  return out;
}

describe("pembulatan tipe member", () => {
  it("enum baru none|nearest|up|down", () => {
    expect(roundingModeOptions.map((o) => o.value)).toEqual(["none", "nearest", "up", "down"]);
    for (const mode of ["nearest", "up", "down"]) {
      expect(issues({ ...member, mode, step: 100 })).toEqual({});
    }
  });

  it("nama lama tidak lagi diterima dikirim, tetapi dilipat untuk tampilan", () => {
    expect(issues({ ...member, mode: "round_100", step: 100 }).mode).toBeDefined();
    expect(foldRoundingMode("round_500", 0)).toEqual({ mode: "nearest", step: 500 });
    expect(foldRoundingMode("round_1000", 250)).toEqual({ mode: "nearest", step: 250 });
    expect(foldRoundingMode("floor", 0)).toEqual({ mode: "down", step: 100 });
    expect(foldRoundingMode("ceil", 50)).toEqual({ mode: "up", step: 50 });
    expect(foldRoundingMode("nearest", 100)).toEqual({ mode: "nearest", step: 100 });
  });

  it("mode dan kelipatan harus berpasangan", () => {
    expect(issues({ ...member, mode: "nearest", step: 0 }).step).toMatch(/wajib/);
    expect(issues({ ...member, mode: "none", step: 100 }).step).toMatch(/0/);
  });

  it("ringkasan tampilan", () => {
    expect(roundingSummary("nearest", 250)).toBe("Terdekat 250");
    expect(roundingSummary("round_100", 0)).toBe("Terdekat 100");
    expect(roundingSummary("none", 0)).toBe("Tanpa pembulatan");
  });
});

describe("persen tipe member", () => {
  it("minus maksimal 100", () => {
    expect(issues({ ...member, value: 100 })).toEqual({});
    expect(issues({ ...member, value: 100.5 }).value).toMatch(/maksimal 100/);
  });

  it("plus (markup) boleh melebihi 100 sampai plafon teknis", () => {
    expect(issues({ ...member, direction: "plus", value: 250 })).toEqual({});
    expect(issues({ ...member, direction: "plus", value: MAX_PERCENT })).toEqual({});
    expect(issues({ ...member, direction: "plus", value: MAX_PERCENT + 1 }).value).toMatch(/maksimal/);
  });

  it("batas input mengikuti arah", () => {
    expect(maxPercentFor("minus")).toBe(100);
    expect(maxPercentFor("plus")).toBe(MAX_PERCENT);
  });

  it("nominal tidak dibatasi persen", () => {
    expect(issues({ ...member, type: "nominal", value: 500000 })).toEqual({});
  });
});

describe("partyFormSchema: Informasi Tambahan", () => {
  const party = { name: "Toko A" };

  it("tanpa atribut = daftar kosong", () => {
    const r = partyFormSchema.safeParse(party);
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.attrs).toEqual([]);
  });

  it("maksimal 20 kunci", () => {
    const rows = Array.from({ length: 21 }, (_, i) => ({ key: `k${i}`, value: "v" }));
    expect(partyFormSchema.safeParse({ ...party, attrs: rows }).success).toBe(false);
    expect(partyFormSchema.safeParse({ ...party, attrs: rows.slice(0, 20) }).success).toBe(true);
  });

  it("kunci 1-60, isi 1-255, tanpa ganda", () => {
    expect(partyFormSchema.safeParse({ ...party, attrs: [{ key: "a".repeat(61), value: "x" }] }).success).toBe(false);
    expect(partyFormSchema.safeParse({ ...party, attrs: [{ key: "a", value: "x".repeat(256) }] }).success).toBe(false);
    expect(partyFormSchema.safeParse({ ...party, attrs: [{ key: "a", value: "" }] }).success).toBe(false);
    expect(
      partyFormSchema.safeParse({
        ...party,
        attrs: [
          { key: "NPWP", value: "1" },
          { key: "NPWP", value: "2" },
        ],
      }).success,
    ).toBe(false);
    expect(partyFormSchema.safeParse({ ...party, attrs: [{ key: "NPWP", value: "01.234" }] }).success).toBe(true);
  });
});
