import { beforeEach, describe, expect, it, vi } from "vitest";

const so = vi.fn();
const po = vi.fn();
const pay = vi.fn();
const sj = vi.fn();
const rtr = vi.fn();
const rtb = vi.fn();
vi.mock("@/modules/sales/services/sales.service", () => ({ salesService: { list: (...a: unknown[]) => so(...a) } }));
vi.mock("@/modules/purchasing/services/purchase-order.service", () => ({
  purchaseOrderService: { list: (...a: unknown[]) => po(...a) },
}));
vi.mock("@/modules/payment/services/payment.service", () => ({ paymentService: { list: (...a: unknown[]) => pay(...a) } }));
vi.mock("@/modules/delivery/services/delivery.service", () => ({ deliveryService: { list: (...a: unknown[]) => sj(...a) } }));
vi.mock("@/modules/salesreturn/services/sales-return.service", () => ({
  salesReturnService: { list: (...a: unknown[]) => rtr(...a) },
}));
vi.mock("@/modules/purchasereturn/services/purchase-return.service", () => ({
  purchaseReturnService: { list: (...a: unknown[]) => rtb(...a) },
}));

import { documentSourcesFor, looksLikeDocumentNumber, searchDocuments } from "./document-search";

const page = <T,>(items: T[]) => Promise.resolve({ items, meta: { page: 1, limit: 8, total: items.length, totalPages: 1 } });
const all = () => true;

beforeEach(() => {
  for (const fn of [so, po, pay, sj, rtr, rtb]) {
    fn.mockReset();
    fn.mockImplementation(() => page([]));
  }
});

describe("looksLikeDocumentNumber", () => {
  it("butuh ≥ 3 karakter dan angka atau awalan dikenal", () => {
    expect(looksLikeDocumentNumber("SJ")).toBe(false);
    expect(looksLikeDocumentNumber("stok")).toBe(false);
    expect(looksLikeDocumentNumber("sj-")).toBe(true);
    expect(looksLikeDocumentNumber("00012")).toBe(true);
  });
});

describe("documentSourcesFor", () => {
  it("awalan menentukan sumber; ORD- tanpa /PJ atau /PB mencari kedua order", () => {
    expect(documentSourcesFor("ORD-HQ/PJ/2026", all)).toEqual({ ids: ["so"], owned: true });
    expect(documentSourcesFor("ord-hq/pb/2026", all)).toEqual({ ids: ["po"], owned: true });
    expect(documentSourcesFor("ORD-HQ", all)).toEqual({ ids: ["so", "po"], owned: true });
    expect(documentSourcesFor("PAY-HQ", all).ids).toEqual(["pay"]);
    expect(documentSourcesFor("SJ-HQ", all).ids).toEqual(["sj"]);
    expect(documentSourcesFor("RTR-HQ", all).ids).toEqual(["rtr"]);
    expect(documentSourcesFor("RTB-HQ", all).ids).toEqual(["rtb"]);
    expect(documentSourcesFor("00012", all)).toEqual({ ids: ["so", "po", "pay", "sj", "rtr", "rtb"], owned: false });
  });

  it("hanya sumber yang diizinkan peran", () => {
    const can = (p: string) => p === "delivery.view";
    expect(documentSourcesFor("00012", can).ids).toEqual(["sj"]);
    expect(documentSourcesFor("PAY-HQ", can)).toEqual({ ids: ["sj"], owned: false });
  });
});

describe("searchDocuments", () => {
  it("memanggil service pemilik awalan dengan search + limit + signal, memetakan ke rute detail", async () => {
    sj.mockImplementation(() => page([{ id: 5, number: "SJ-HQ/2026/00012", orderNumber: "ORD-HQ/PJ/2026/10/00003", status: "confirmed" }]));
    const ctrl = new AbortController();
    const hits = await searchDocuments(" SJ-HQ/2026/00012 ", all, ctrl.signal);
    expect(sj).toHaveBeenCalledWith({ search: "SJ-HQ/2026/00012", page: 1, limit: 8 }, ctrl.signal);
    expect(so).not.toHaveBeenCalled();
    expect(hits).toEqual([
      {
        key: "sj-5",
        kind: "Surat jalan",
        number: "SJ-HQ/2026/00012",
        detail: "ORD-HQ/PJ/2026/10/00003 · Terkonfirmasi",
        path: "/deliveries/5",
      },
    ]);
  });

  it("tanpa awalan: semua sumber (limit 3), sumber gagal dilewati, nomor persis di atas", async () => {
    so.mockImplementation(() => page([{ id: 1, number: "ORD-HQ/PJ/2026/10/000123", partyName: "", status: "draft" }]));
    pay.mockImplementation(() => Promise.reject(new Error("403")));
    rtb.mockImplementation(() => page([{ id: 2, number: "000123", orderNumber: "", status: "completed" }]));
    const hits = await searchDocuments("000123", all, new AbortController().signal);
    expect(po).toHaveBeenCalledWith({ search: "000123", page: 1, limit: 3 }, expect.any(AbortSignal));
    expect(hits.map((h) => h.key)).toEqual(["rtb-2", "so-1"]);
    expect(hits[1].detail).toBe("Tunai · Draf");
  });

  it("teks yang bukan nomor tidak memanggil jaringan", async () => {
    expect(await searchDocuments("kopi", all, new AbortController().signal)).toEqual([]);
    expect(so).not.toHaveBeenCalled();
  });
});
