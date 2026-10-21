import { describe, expect, it } from "vitest";
import { failedGates, gateLabel, offsetSummary, restateState, revaluedValue } from "@/modules/finance/lib/corrections";
import { correctionReasonSchema, revaluationCostSchema } from "@/modules/finance/schemas/corrections.schema";
import type { RestateReport } from "@/modules/finance/types";

function report(over: Partial<RestateReport>): RestateReport {
  return {
    commit: false,
    branchId: 1,
    positions: 3,
    rowsChanged: 1,
    products: [],
    skipped: [],
    delta: -6000,
    gates: [
      { key: "trial_balance", ok: true, detail: "" },
      { key: "pending_documents", ok: true, detail: "" },
    ],
    applied: false,
    date: "2026-10-21",
    cutoverMovementId: 0,
    ...over,
  };
}

describe("corrections lib", () => {
  it("status dry run: kosong, terhalang gerbang, atau siap", () => {
    expect(restateState(report({ rowsChanged: 0 }))).toBe("nothing");
    expect(restateState(report({}))).toBe("ready");
    const blocked = report({ gates: [{ key: "pending_documents", ok: false, detail: "2 dokumen" }] });
    expect(restateState(blocked)).toBe("blocked");
    expect(failedGates(blocked.gates).map((g) => g.key)).toEqual(["pending_documents"]);
    expect(failedGates(null)).toEqual([]);
  });

  it("label gerbang dan akun lawan berbahasa Indonesia", () => {
    expect(gateLabel("inventory_recon")).toBe("Persediaan buku = ledger biaya");
    expect(gateLabel("baru")).toBe("baru");
    expect(offsetSummary({ cogs: -6000, writeoff: 0, adjustment: 1000 }, String)).toBe("Selisih stok 1000, HPP -6000");
    expect(offsetSummary(null, String)).toBe("-");
  });

  it("nilai revaluasi = qty × harga, rupiah bulat", () => {
    expect(revaluedValue(10, 9000)).toBe(90000);
    expect(revaluedValue(2.5, 333)).toBe(833);
  });

  it("alasan wajib dan dipangkas; harga modal rupiah bulat ≥ 0", () => {
    expect(correctionReasonSchema.safeParse({ reason: "   " }).success).toBe(false);
    expect(correctionReasonSchema.parse({ reason: "  salah ketik " }).reason).toBe("salah ketik");
    expect(correctionReasonSchema.safeParse({ reason: "x".repeat(501) }).success).toBe(false);
    expect(revaluationCostSchema.parse({ newUnitCost: "9000" }).newUnitCost).toBe(9000);
    expect(revaluationCostSchema.safeParse({ newUnitCost: "-1" }).success).toBe(false);
    expect(revaluationCostSchema.safeParse({ newUnitCost: "12.5" }).success).toBe(false);
  });
});
