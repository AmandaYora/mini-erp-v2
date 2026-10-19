import { describe, expect, it } from "vitest";
import { parseFilters } from "@/shared/hooks/use-url-filters";
import {
  EXPENSE_LIST_DEFAULTS,
  JOURNAL_LIST_DEFAULTS,
  expenseListFilterSchema,
  journalListFilterSchema,
} from "@/modules/finance/schemas/finance-list-filter.schema";

describe("filter daftar keuangan", () => {
  it("membaca filter jurnal dari URL dan mengabaikan nilai tak sah", () => {
    const f = parseFilters(
      journalListFilterSchema,
      JOURNAL_LIST_DEFAULTS,
      new URLSearchParams("status=reversed&sourceType=bukan-jenis&accountId=7&from=2026-10-01"),
    );
    expect(f.status).toBe("reversed");
    expect(f.sourceType).toBe("");
    expect(f.accountId).toBe(7);
    expect(f.from).toBe("2026-10-01");
  });

  it("filter biaya: status hanya posted atau cancelled", () => {
    const f = parseFilters(expenseListFilterSchema, EXPENSE_LIST_DEFAULTS, new URLSearchParams("status=draft&page=3"));
    expect(f.status).toBe("");
    expect(f.page).toBe(3);
  });
});
