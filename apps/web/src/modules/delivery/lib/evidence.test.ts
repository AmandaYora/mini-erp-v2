import { describe, expect, it } from "vitest";
import { evidenceChecklist, evidenceRequired, missingEvidence } from "@/modules/delivery/lib/evidence";

const full = {
  driverName: "Budi",
  warehouseStaffName: "Sari",
  recipientName: "Ani",
  signatureStatus: "signed",
  signedDocumentCount: 1,
};

describe("evidence surat jalan", () => {
  it("POS dan SJ pengganti dikecualikan", () => {
    expect(evidenceRequired("pos", "")).toBe(false);
    expect(evidenceRequired("regular", "replacement")).toBe(false);
    expect(evidenceRequired("regular", "")).toBe(true);
    expect(evidenceRequired(undefined, undefined)).toBe(true);
  });

  it("lengkap = tidak ada yang kurang", () => {
    expect(missingEvidence(evidenceChecklist(full))).toEqual([]);
  });

  it("melaporkan tiap kekurangan", () => {
    const miss = missingEvidence(
      evidenceChecklist({ ...full, driverName: " ", warehouseStaffName: "", signatureStatus: "", signedDocumentCount: 0 }),
    ).map((c) => c.key);
    expect(miss).toEqual(["driverName", "warehouseStaffName", "recipientName", "signedDocument"]);
  });

  it("nama penerima tanpa status tanda tangan belum cukup", () => {
    expect(missingEvidence(evidenceChecklist({ ...full, signatureStatus: "" })).map((c) => c.key)).toEqual([
      "recipientName",
    ]);
  });
});
