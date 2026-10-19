import { describe, expect, it } from "vitest";
import {
  CART_DRAFT_KEY,
  CLIENT_REF_MAX,
  clearDraft,
  emptyDraft,
  loadDraft,
  loadOrCreateDraft,
  newClientRef,
  saveDraft,
  type DraftStorage,
} from "@/modules/pos/lib/cart-draft";

function memoryStorage(): DraftStorage & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return {
    data,
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  };
}

describe("newClientRef", () => {
  it("menghasilkan UUID v4 unik, dalam batas 64 karakter", () => {
    const a = newClientRef();
    const b = newClientRef();
    expect(a).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(a).not.toBe(b);
    expect(a.length).toBeLessThanOrEqual(CLIENT_REF_MAX);
  });
});

describe("clientRef bertahan setelah reload", () => {
  it("ref yang sama dibaca kembali dari storage", () => {
    const store = memoryStorage();
    const first = emptyDraft();
    saveDraft({ ...first, started: true, tendered: "50000" }, store);
    // "reload": instance baru membaca storage yang sama
    const after = loadOrCreateDraft(store);
    expect(after.ref).toBe(first.ref);
    expect(after.started).toBe(true);
    expect(after.tendered).toBe("50000");
  });

  it("tanpa draf tersimpan, ref baru dibuat", () => {
    const store = memoryStorage();
    expect(loadDraft(store)).toBeNull();
    expect(loadOrCreateDraft(store).ref.length).toBeGreaterThan(0);
  });

  it("clearDraft membuang ref sehingga keranjang berikutnya memakai ref baru", () => {
    const store = memoryStorage();
    const first = emptyDraft();
    saveDraft(first, store);
    clearDraft(store);
    expect(loadDraft(store)).toBeNull();
    expect(loadOrCreateDraft(store).ref).not.toBe(first.ref);
  });

  it("JSON rusak / bentuk salah / ref terlalu panjang diabaikan", () => {
    const store = memoryStorage();
    store.setItem(CART_DRAFT_KEY, "{bukan json");
    expect(loadDraft(store)).toBeNull();
    store.setItem(CART_DRAFT_KEY, JSON.stringify({ ref: "x".repeat(65), lines: [], started: false }));
    expect(loadDraft(store)).toBeNull();
    store.setItem(CART_DRAFT_KEY, JSON.stringify({ lines: [] }));
    expect(loadDraft(store)).toBeNull();
  });

  it("storage yang melempar tidak merusak halaman", () => {
    const broken: DraftStorage = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
      removeItem: () => {
        throw new Error("blocked");
      },
    };
    expect(loadDraft(broken)).toBeNull();
    expect(() => saveDraft(emptyDraft(), broken)).not.toThrow();
    expect(() => clearDraft(broken)).not.toThrow();
    expect(loadDraft(null)).toBeNull();
  });
});

describe("pengaturan PPN di draf (K-03)", () => {
  it("bawaan tanpa pajak dan draf lama tanpa kunci pajak tetap terbaca", () => {
    const store = memoryStorage();
    const d = emptyDraft();
    expect(d.taxType).toBe("none");
    expect(d.taxRate).toBe(0);
    const legacy: Record<string, unknown> = { ...d };
    delete legacy.taxType;
    delete legacy.taxRate;
    store.setItem(CART_DRAFT_KEY, JSON.stringify(legacy));
    const loaded = loadDraft(store);
    expect(loaded?.taxType).toBe("none");
    expect(loaded?.taxRate).toBe(0);
  });

  it("pajak yang dipilih bertahan setelah reload; nilai rusak dinormalkan ke tanpa pajak", () => {
    const store = memoryStorage();
    saveDraft({ ...emptyDraft(), taxType: "include", taxRate: 11 }, store);
    expect(loadDraft(store)).toMatchObject({ taxType: "include", taxRate: 11 });
    store.setItem(CART_DRAFT_KEY, JSON.stringify({ ...emptyDraft(), taxType: "exclude", taxRate: 0 }));
    expect(loadDraft(store)).toMatchObject({ taxType: "none", taxRate: 0 });
    store.setItem(CART_DRAFT_KEY, JSON.stringify({ ...emptyDraft(), taxType: "bogus", taxRate: 11 }));
    expect(loadDraft(store)).toMatchObject({ taxType: "none", taxRate: 0 });
  });
});
