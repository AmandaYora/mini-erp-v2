import { describe, expect, it } from "vitest";
import {
  EMPTY_SCAN_GATE,
  SCAN_DEBOUNCE_MS,
  chipFor,
  gateScan,
  pushChip,
  shortCode,
  type ScanChip,
  type ScanGate,
} from "@/modules/pos/lib/scan-session";

function feed(events: [string, number][]): boolean[] {
  let gate: ScanGate = EMPTY_SCAN_GATE;
  return events.map(([code, t]) => {
    const r = gateScan(gate, code, t);
    gate = r.gate;
    return r.accept;
  });
}

describe("gateScan (debounce pindai beruntun)", () => {
  it("bacaan pertama diterima", () => {
    expect(feed([["899", 0]])).toEqual([true]);
  });

  it("kode sama dalam 1,5 detik diabaikan", () => {
    expect(feed([["899", 0], ["899", 100], ["899", 1499]])).toEqual([true, false, false]);
  });

  it("barang yang terus terlihat tidak menambah lagi (jendela dihitung dari bacaan terakhir)", () => {
    const reads: [string, number][] = [];
    for (let t = 0; t <= 6000; t += 200) reads.push(["899", t]);
    expect(feed(reads).filter(Boolean)).toHaveLength(1);
  });

  it("kode sama diterima lagi setelah menghilang ≥ 1,5 detik", () => {
    expect(feed([["899", 0], ["899", SCAN_DEBOUNCE_MS], ["899", SCAN_DEBOUNCE_MS + 10]])).toEqual([true, true, false]);
  });

  it("kode berbeda langsung diterima dan mereset penjaga (A-B-A = 3 barang)", () => {
    expect(feed([["A", 0], ["B", 100], ["A", 200]])).toEqual([true, true, true]);
  });

  it("kode kosong / spasi ditolak tanpa mengubah penjaga", () => {
    const g: ScanGate = { code: "A", at: 50 };
    const r = gateScan(g, "   ", 60);
    expect(r.accept).toBe(false);
    expect(r.gate).toBe(g);
  });

  it("spasi di sekitar kode dipangkas sebelum dibandingkan", () => {
    expect(feed([["A", 0], [" A ", 10]])).toEqual([true, false]);
  });

  it("jendela bisa diatur", () => {
    const first = gateScan(EMPTY_SCAN_GATE, "A", 0, 500);
    expect(gateScan(first.gate, "A", 600, 500).accept).toBe(true);
  });
});

describe("chip pindai", () => {
  it("barang masuk: ＋ nama (jumlah)", () => {
    expect(chipFor({ kind: "added", name: "Semen 50kg", qty: 21 })).toEqual({
      tone: "success",
      text: "＋ Semen 50kg (21)",
    });
  });

  it("kode tak dikenal: merah, kode dipotong", () => {
    expect(chipFor({ kind: "unknown", code: "8997654321098", detail: "tak dikenal" })).toEqual({
      tone: "danger",
      text: "Kode 8997654321… tidak ditemukan",
    });
  });

  it("varian: kuning; ditolak: pesan apa adanya; terkunci: merah", () => {
    expect(chipFor({ kind: "choose", productName: "Kaos" }).tone).toBe("warning");
    expect(chipFor({ kind: "rejected", detail: "Produk X tidak aktif" }).text).toBe("Produk X tidak aktif");
    expect(chipFor({ kind: "blocked" }).tone).toBe("danger");
  });

  it("shortCode tidak memotong kode pendek", () => {
    expect(shortCode("12345")).toBe("12345");
  });

  it("pushChip: terbaru di depan, dibatasi", () => {
    let list: ScanChip[] = [];
    for (let i = 1; i <= 8; i++) list = pushChip(list, { id: i, tone: "success", text: `c${i}` }, 6);
    expect(list.map((c) => c.id)).toEqual([8, 7, 6, 5, 4, 3]);
  });
});
