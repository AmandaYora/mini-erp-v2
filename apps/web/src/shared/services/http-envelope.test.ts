import { describe, expect, it } from "vitest";
import { loadingBus } from "@/shared/services/loading-bus";
import { pickServerMessage } from "@/shared/services/http-client";

describe("loadingBus", () => {
  it("mulai tidak-loading, aktif saat begin, padam setelah end", () => {
    loadingBus.resetForTest();
    expect(loadingBus.isLoading()).toBe(false);
    loadingBus.begin();
    expect(loadingBus.isLoading()).toBe(true);
    loadingBus.begin();
    expect(loadingBus.isLoading()).toBe(true);
    loadingBus.end();
    expect(loadingBus.isLoading()).toBe(true);
    loadingBus.end();
    expect(loadingBus.isLoading()).toBe(false);
  });

  it("tidak pernah negatif bila end tanpa begin", () => {
    loadingBus.resetForTest();
    loadingBus.end();
    expect(loadingBus.isLoading()).toBe(false);
  });

  it("memberi tahu subscriber setiap perubahan", () => {
    loadingBus.resetForTest();
    let calls = 0;
    const off = loadingBus.subscribe(() => {
      calls += 1;
    });
    loadingBus.begin();
    loadingBus.end();
    off();
    loadingBus.begin();
    loadingBus.resetForTest();
    expect(calls).toBe(2);
  });
});

describe("pickServerMessage", () => {
  it("memakai pesan server bila ada", () => {
    expect(pickServerMessage("Transfer dikirim", "Fallback")).toBe("Transfer dikirim");
  });

  it("fallback bila pesan server kosong/null", () => {
    expect(pickServerMessage("", "Fallback")).toBe("Fallback");
    expect(pickServerMessage("   ", "Fallback")).toBe("Fallback");
    expect(pickServerMessage(null, "Fallback")).toBe("Fallback");
    expect(pickServerMessage(undefined, "Fallback")).toBe("Fallback");
  });
});
