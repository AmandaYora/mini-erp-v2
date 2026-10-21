import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  MAX_VIEWS,
  deleteView,
  normalizeViewQuery,
  onHiddenColumnsChange,
  readHiddenColumns,
  readSavedViews,
  saveView,
  writeHiddenColumns,
} from "./list-views";
import { setPrefsOwner } from "@/shared/stores/ui-prefs.store";

beforeEach(() => {
  localStorage.clear();
  setPrefsOwner(1);
});
afterEach(() => setPrefsOwner(null));

describe("normalizeViewQuery", () => {
  it("membuang page dan nilai kosong, mengurutkan kunci", () => {
    expect(normalizeViewQuery("?status=draft&page=3&search=&dir=asc")).toBe("dir=asc&status=draft");
    expect(normalizeViewQuery("")).toBe("");
  });
});

describe("tampilan tersimpan", () => {
  it("menyimpan, menimpa nama yang sama, lalu menghapus", () => {
    const first = saveView("so", { name: "Draf", query: "?status=draft&page=2", hiddenColumns: ["Kanal"] });
    expect(first.ok).toBe(true);
    expect(readSavedViews("so")).toEqual([
      expect.objectContaining({ name: "Draf", query: "status=draft", hiddenColumns: ["Kanal"] }),
    ]);

    const again = saveView("so", { name: "draf", query: "status=completed", hiddenColumns: null });
    expect(again.ok).toBe(true);
    const views = readSavedViews("so");
    expect(views).toHaveLength(1);
    expect(views[0]).toMatchObject({ name: "draf", query: "status=completed", hiddenColumns: null });

    expect(deleteView("so", views[0].id)).toEqual([]);
  });

  it("menolak nama kosong dan batas jumlah per daftar", () => {
    expect(saveView("so", { name: "  ", query: "", hiddenColumns: null })).toEqual({
      ok: false,
      message: "Nama tampilan wajib diisi.",
    });
    for (let i = 0; i < MAX_VIEWS; i++) saveView("so", { name: `V${i}`, query: "", hiddenColumns: null });
    const res = saveView("so", { name: "Satu lagi", query: "", hiddenColumns: null });
    expect(res.ok).toBe(false);
  });

  it("terpisah per pengguna dan per daftar", () => {
    saveView("so", { name: "Milik 1", query: "", hiddenColumns: null });
    setPrefsOwner(2);
    expect(readSavedViews("so")).toEqual([]);
    setPrefsOwner(1);
    expect(readSavedViews("po")).toEqual([]);
    expect(readSavedViews("so")).toHaveLength(1);
  });

  it("data rusak di penyimpanan dibaca sebagai kosong", () => {
    localStorage.setItem("mini-erp.views.so@1", "{bukan json");
    expect(readSavedViews("so")).toEqual([]);
    localStorage.setItem("mini-erp.views.so@1", JSON.stringify([{ id: 1 }, { id: "a", name: "OK", query: "", hiddenColumns: null }]));
    expect(readSavedViews("so")).toEqual([{ id: "a", name: "OK", query: "", hiddenColumns: null }]);
  });
});

describe("kolom tersembunyi", () => {
  it("menulis, membaca, mereset, dan memberi tahu pelanggan kunci yang sama saja", () => {
    let soCalls = 0;
    let poCalls = 0;
    const offSo = onHiddenColumnsChange("so", () => soCalls++);
    const offPo = onHiddenColumnsChange("po", () => poCalls++);

    expect(readHiddenColumns("so")).toBeNull();
    writeHiddenColumns("so", ["Kanal"]);
    expect(readHiddenColumns("so")).toEqual(["Kanal"]);
    writeHiddenColumns("so", null);
    expect(readHiddenColumns("so")).toBeNull();
    expect(soCalls).toBe(2);
    expect(poCalls).toBe(0);

    offSo();
    offPo();
  });
});
