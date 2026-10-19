import { describe, expect, it } from "vitest";
import { expenseRailCurrent, expenseRelated, journalRailCurrent, journalRelated } from "@/modules/finance/lib/document-rails";

describe("rel dokumen keuangan", () => {
  it("jurnal dibalik berpindah ke langkah Dibalik dan menautkan pembaliknya", () => {
    expect(journalRailCurrent({ status: "posted" })).toBe("posted");
    expect(journalRailCurrent({ status: "reversed" })).toBe("reversed");
    expect(journalRelated({ reversedBy: 0 })).toEqual([]);
    expect(journalRelated({ reversedBy: 9 })[0].to).toBe("/finance/journals/9");
  });

  it("biaya terposting berada di Dijurnalkan dan menautkan jurnalnya", () => {
    expect(expenseRailCurrent({ status: "posted", journalEntryId: 4 })).toBe("posted");
    expect(expenseRailCurrent({ status: "cancelled", journalEntryId: 4 })).toBe("posted");
    expect(expenseRelated({ journalEntryId: 4 })[0].to).toBe("/finance/journals/4");
    expect(expenseRelated({ journalEntryId: 0 })).toEqual([]);
  });
});
