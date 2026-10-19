import { describe, expect, it } from "vitest";
import { openingPreviewSummary } from "@/modules/stock/lib/opening";

describe("openingPreviewSummary", () => {
  it("omits skipped when zero", () => {
    expect(openingPreviewSummary({ validRows: 3, skipped: 0, errors: [] })).toBe(
      "3 baris valid, 0 baris bermasalah",
    );
  });
  it("includes skipped rows", () => {
    expect(
      openingPreviewSummary({
        validRows: 2,
        skipped: 4,
        errors: [{ row: 1, column: "qty", value: "x", reason: "bad" }],
      }),
    ).toBe("2 baris valid, 4 dilewati (posisi sudah punya mutasi), 1 baris bermasalah");
  });
});
