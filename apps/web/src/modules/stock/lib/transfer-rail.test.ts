import { describe, expect, it } from "vitest";
import { transferRail } from "@/modules/stock/lib/transfer-rail";

describe("transferRail", () => {
  it("memetakan status ke langkah aktif", () => {
    expect(transferRail("draft")).toMatchObject({ current: "draft", stopped: false });
    expect(transferRail("dispatched")).toMatchObject({ current: "dispatched", stopped: false });
    expect(transferRail("received")).toMatchObject({ current: "received", stopped: false });
  });
  it("transfer batal berhenti di draf", () => {
    expect(transferRail("cancelled")).toMatchObject({ current: "draft", stopped: true });
  });
  it("status tak dikenal jatuh ke draf", () => {
    expect(transferRail(undefined).current).toBe("draft");
  });
  it("selalu tiga langkah berurutan", () => {
    expect(transferRail("draft").steps.map((s) => s.key)).toEqual(["draft", "dispatched", "received"]);
  });
});
