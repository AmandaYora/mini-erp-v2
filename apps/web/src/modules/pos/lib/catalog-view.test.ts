import { describe, expect, it } from "vitest";
import { readCatalogView } from "@/modules/pos/lib/catalog-view";

describe("readCatalogView", () => {
  it("bawaan foto besar; hanya 'compact' yang dikenali; storage galat = bawaan", () => {
    expect(readCatalogView({ getItem: () => null })).toBe("photo");
    expect(readCatalogView({ getItem: () => "compact" })).toBe("compact");
    expect(readCatalogView({ getItem: () => "lain" })).toBe("photo");
    expect(
      readCatalogView({
        getItem: () => {
          throw new Error("blocked");
        },
      }),
    ).toBe("photo");
    expect(readCatalogView(null)).toBe("photo");
  });
});
