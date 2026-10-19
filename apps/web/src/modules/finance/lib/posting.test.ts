import { describe, expect, it } from "vitest";
import {
  buildBatch,
  buildCatchup,
  closeDayCandidates,
  isMissingCostError,
  missingCostProduct,
  queueKey,
  sanitizeSelection,
  selectableQueue,
} from "@/modules/finance/lib/posting";
import {
  MAPPING_KEYS,
  mappingKeyLabel,
  missingMappingKeys,
} from "@/modules/finance/types";
import type { Account, PostingSource } from "@/modules/finance/types";

const q = (id: number, closed = false): PostingSource => ({
  docType: "delivery",
  docId: id,
  number: `D-${id}`,
  date: "2026-09-01",
  periodClosed: closed,
});

describe("posting queue helpers", () => {
  it("excludes period-closed docs from selection and batch", () => {
    const queue = [q(1), q(2, true), q(3)];
    expect(selectableQueue(queue).map((x) => x.docId)).toEqual([1, 3]);
    const sel = new Set(queue.map(queueKey));
    expect(buildBatch(queue, sel).map((b) => b.docId)).toEqual([1, 3]);
  });

  it("caps batch at 50 and carries the action", () => {
    const queue = Array.from({ length: 80 }, (_, i) => q(i + 1));
    const sel = new Set(queue.map(queueKey));
    const batch = buildBatch(queue, sel, "reverse");
    expect(batch).toHaveLength(50);
    expect(batch[0].action).toBe("reverse");
  });

  it("detects the missing-cost conflict", () => {
    const msg = "Harga modal produk Kaos Polos belum tersedia";
    expect(isMissingCostError(msg)).toBe(true);
    expect(missingCostProduct(msg)).toBe("Kaos Polos");
    expect(isMissingCostError("Periode sudah ditutup")).toBe(false);
    expect(
      missingCostProduct("Harga modal produk pada retur belum tersedia"),
    ).toBeNull();
  });
});

describe("mapping keys", () => {
  it("has 18 keys, all labelled", () => {
    expect(MAPPING_KEYS).toHaveLength(18);
    for (const k of MAPPING_KEYS) expect(mappingKeyLabel(k)).not.toBe(k);
  });

  it("reports unmapped or archived keys", () => {
    const acc: Account = {
      id: 1,
      code: "1000",
      name: "Kas",
      type: "aset",
      isCash: true,
      status: "active",
    };
    const missing = missingMappingKeys({
      cash: acc,
      bank: { ...acc, status: "archived" },
    });
    expect(missing).not.toContain("cash");
    expect(missing).toContain("bank");
    expect(missing).toHaveLength(17);
  });
});

describe("catch-up item", () => {
  it("builds a catchup action only for period-closed documents", () => {
    expect(buildCatchup(q(1))).toBeNull();
    expect(buildCatchup(q(2, true))).toEqual({
      docType: "delivery",
      docId: 2,
      action: "catchup",
    });
  });
});

describe("pilihan tabel antrean", () => {
  it("membuang kunci periode tertutup dan kunci asing", () => {
    const queue = [q(1), q(2, true), q(3)];
    const out = sanitizeSelection(queue, new Set([queueKey(queue[0]), queueKey(queue[1]), "delivery:99"]));
    expect([...out]).toEqual(["delivery:1"]);
  });

  it("dipotong pada batas batch backend", () => {
    const queue = Array.from({ length: 80 }, (_, i) => q(i + 1));
    const out = sanitizeSelection(queue, new Set(queue.map(queueKey)));
    expect(out.size).toBe(50);
  });

  it("tutup harian hanya memuat dokumen bertanggal sampai batas dan bukan periode tertutup", () => {
    const queue = [
      { ...q(1), date: "2026-09-01T00:00:00Z" },
      { ...q(2), date: "2026-09-05" },
      { ...q(3, true), date: "2026-09-01" },
    ];
    expect(closeDayCandidates(queue, "2026-09-01").map((r) => r.docId)).toEqual([1]);
    expect(closeDayCandidates(queue, "2026-09-30").map((r) => r.docId)).toEqual([1, 2]);
    expect(closeDayCandidates(queue, "")).toEqual([]);
  });
});
