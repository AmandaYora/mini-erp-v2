import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DRAFT_MAX_AGE_MS,
  clearAllDrafts,
  clearDraft,
  draftKey,
  loadDraft,
  saveDraft,
} from "@/shared/lib/draft-storage";

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe("draft-storage", () => {
  it("kunci per pengguna; tanpa pengguna = tanpa draf", () => {
    expect(draftKey(7, "so-new")).toBe("mini-erp.draft:7:so-new");
    expect(draftKey(null, "so-new")).toBeNull();
  });

  it("simpan lalu muat", () => {
    const k = draftKey(1, "f")!;
    expect(saveDraft(k, { a: 1 }, 1000)).toBe(true);
    expect(loadDraft<{ a: number }>(k, 2000)).toEqual({ a: 1 });
  });

  it("draf basi dibuang", () => {
    const k = draftKey(1, "f")!;
    saveDraft(k, { a: 1 }, 0);
    expect(loadDraft(k, DRAFT_MAX_AGE_MS + 1)).toBeNull();
    expect(localStorage.getItem(k)).toBeNull();
  });

  it("isi rusak tidak melempar", () => {
    const k = draftKey(1, "f")!;
    localStorage.setItem(k, "{bukan json");
    expect(loadDraft(k)).toBeNull();
    localStorage.setItem(k, JSON.stringify({ v: 9 }));
    expect(loadDraft(k)).toBeNull();
  });

  it("penyimpanan diblokir: semua aman", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(saveDraft("mini-erp.draft:1:f", 1)).toBe(false);
    expect(loadDraft("mini-erp.draft:1:f")).toBeNull();
    expect(() => clearDraft("x")).not.toThrow();
  });

  it("clearAllDrafts hanya menghapus kunci draf", () => {
    saveDraft(draftKey(1, "a")!, 1);
    saveDraft(draftKey(2, "b")!, 2);
    localStorage.setItem("mini-erp.access-token", "t");
    clearAllDrafts();
    expect(localStorage.getItem("mini-erp.access-token")).toBe("t");
    expect(loadDraft(draftKey(1, "a")!)).toBeNull();
    expect(loadDraft(draftKey(2, "b")!)).toBeNull();
  });
});
