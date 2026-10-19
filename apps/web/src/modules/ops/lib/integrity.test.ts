import { describe, expect, it } from "vitest";
import {
  groupByDocType,
  issueActionHint,
  issueDocLabel,
  issueDocPath,
} from "@/modules/ops/lib/integrity";

describe("ops integrity helpers", () => {
  it("builds document paths", () => {
    expect(issueDocPath("delivery", 3)).toBe("/deliveries/3");
    expect(issueDocPath("goods_receipt", 4)).toBe("/goods-receipts/4");
    expect(issueDocPath("sales_return", 5)).toBe("/sales-returns/5");
    expect(issueDocPath("purchase_return", 6)).toBe("/purchase-returns/6");
    expect(issueDocPath("unknown", 1)).toBeNull();
  });

  it("labels and hints", () => {
    expect(issueDocLabel("goods_receipt")).toBe("Penerimaan Barang");
    expect(issueActionHint("pending_receipt")).toMatch(/Posting ulang/);
    expect(issueActionHint("zzz")).toMatch(/Buka dokumen/);
  });

  it("groups by doc type", () => {
    const g = groupByDocType([
      { kind: "a", docType: "delivery", docId: 1, number: "D1", detail: "" },
      { kind: "b", docType: "sales_return", docId: 2, number: "R1", detail: "" },
      { kind: "a", docType: "delivery", docId: 3, number: "D2", detail: "" },
    ]);
    expect(g.get("delivery")).toHaveLength(2);
    expect([...g.keys()]).toEqual(["delivery", "sales_return"]);
  });
});
