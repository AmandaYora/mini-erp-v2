import { test, expect } from "@playwright/test";
import {
  call, ok, createSO, confirmSO, createDelivery, confirmDelivery, createPO, confirmPO, receiveBody, pay,
  locationByCode, stockQty, today, drainPosting, type Product,
} from "../helpers/api";

// T-04 (padanan legacy 11-volume-realistic-ui-50-po-100-sales + 20/21/22 integritas relasi pelanggan).
//
// Membuat >=50 PO dan >=100 penjualan lewat API (4 jalur paralel; satu jalur = satu produk, jadi tak ada
// rebutan baris stok), lalu memeriksa daftar (total, paginasi tanpa duplikat), filter status bayar,
// laporan order, saldo piutang/hutang dari BUKU, stok, dan batas waktu. Semua data memakai produk,
// pemasok dan pelanggan KHUSUS spec ini ("Volume ...") dan asersi berbasis selisih, sehingga spec lain
// yang membaca data seed tidak terganggu.
const PO_COUNT = 50;
const SO_COUNT = 100;
const LANES = 4;
const CUSTOMERS = 12;
const LIST_BUDGET_MS = 3_000; // daftar/laporan pada ~150 dokumen harus responsif
const LOAD_BUDGET_MS = 180_000; // pembuatan massal 150 dokumen lengkap

/** Jalankan jobs per jalur: antar-jalur paralel, di dalam jalur berurutan. */
async function lanes<T>(items: T[], laneOf: (item: T, idx: number) => number, fn: (item: T, idx: number) => Promise<void>) {
  const buckets: { item: T; idx: number }[][] = Array.from({ length: LANES }, () => []);
  items.forEach((item, idx) => buckets[laneOf(item, idx) % LANES].push({ item, idx }));
  await Promise.all(
    buckets.map(async (b) => {
      for (const { item, idx } of b) await fn(item, idx);
    }),
  );
}

async function timed<T>(label: string, budgetMs: number, fn: () => Promise<T>): Promise<T> {
  const t0 = Date.now();
  const out = await fn();
  const took = Date.now() - t0;
  expect(took, `${label} ${took}ms melebihi ${budgetMs}ms`).toBeLessThan(budgetMs);
  return out;
}

const poLines = (i: number) => 20 + (i % 5) * 5; // qty per PO: 20..40
const soQty = (j: number) => 1 + (j % 3); // 1..3

test.describe.serial("E13 volume 50 PO + 100 penjualan dan integritas relasi pelanggan", () => {
  let prods: Product[] = [];
  let supplierIds: number[] = [];
  let customerIds: number[] = [];
  let hq: any;

  const poTotals: number[] = [];
  const poPaid: number[] = [];
  const poIds: number[] = [];
  const soTotals: number[] = [];
  const soPaid: number[] = [];
  const soIds: number[] = [];
  let base = { po: 0, so: 0, reportSo: { total: 0, grand: 0 }, reportPo: { total: 0, grand: 0 } };
  let loadMs = 0;

  const listTotal = async (path: string) => ((await call("admin", "GET", `${path}?limit=1`)).meta?.total ?? 0) as number;
  const report = async (type: "sales" | "purchasing") => {
    const r = (await ok<any>("admin", "GET", `/reporting/orders?type=${type}&from=${today(-1)}&to=${today(1)}&limit=1`)) as any;
    return { total: r.total as number, grand: r.grandTotal as number };
  };

  test("persiapan: produk, pemasok, pelanggan khusus spec + dasar hitung", async () => {
    hq = await locationByCode("HQ-GA");
    for (let k = 0; k < LANES; k++) {
      const created = await ok<any>("admin", "POST", "/products", {
        code: `VOL-P${k + 1}`, name: `Volume Produk ${k + 1}`, baseUom: "pcs", purchaseUom: "pcs", salesUom: "pcs",
        purchaseFactor: 1, salesFactor: 1, purchasePrice: 6000 + k * 500, sellingPrice: 10_000 + k * 1000,
      });
      const d = await ok<any>("admin", "GET", `/products/${created.id}`);
      prods.push({
        id: d.id, code: d.code, name: d.name, variantId: d.variants[0].id, barcode: d.variants[0].barcode ?? "",
        sellingPrice: d.sellingPrice, purchasePrice: d.purchasePrice,
      });
      supplierIds.push((await ok<any>("admin", "POST", "/suppliers", { code: `VOL-S${k + 1}`, name: `Volume Pemasok ${k + 1}`, phone: `08120000${k}` })).id);
    }
    for (let c = 0; c < CUSTOMERS; c++) {
      customerIds.push(
        (await ok<any>("admin", "POST", "/customers", { code: `VOL-C${String(c + 1).padStart(2, "0")}`, name: `Volume Pelanggan ${String(c + 1).padStart(2, "0")}`, phone: `0813000${String(c).padStart(2, "0")}` })).id,
      );
    }
    // jurnalkan sisa dokumen spec sebelumnya agar selisih buku hanya berasal dari spec ini
    await drainPosting();
    base = { po: await listTotal("/purchase-orders"), so: await listTotal("/sales-orders"), reportSo: await report("sales"), reportPo: await report("purchasing") };
  });

  test(`buat ${PO_COUNT} PO (konfirmasi, penerimaan penuh, 40% dibayar lunas) lewat 4 jalur paralel`, async () => {
    test.setTimeout(LOAD_BUDGET_MS + 60_000);
    const t0 = Date.now();
    const jobs = Array.from({ length: PO_COUNT }, (_, i) => i);
    poTotals.length = poPaid.length = poIds.length = PO_COUNT;
    await lanes(jobs, (i) => i, async (i) => {
      const lane = i % LANES;
      const p = prods[lane];
      const qty = poLines(i);
      const po = await createPO("admin", supplierIds[lane], [{ p, qty }]);
      expect(po.grandTotal).toBe(qty * p.purchasePrice);
      await confirmPO("admin", po.id);
      await ok("gudang", "POST", "/goods-receipts", receiveBody(po.id, hq.id, [{ p, qty }]));
      poIds[i] = po.id;
      poTotals[i] = po.grandTotal;
      poPaid[i] = 0;
      if (i % 5 < 2) {
        await pay("akuntan", supplierIds[lane], po.grandTotal, [{ orderType: "purchase", orderId: po.id, amount: po.grandTotal }], "transfer");
        poPaid[i] = po.grandTotal;
      }
    });
    loadMs += Date.now() - t0;
  });

  test(`buat ${SO_COUNT} penjualan (konfirmasi, kirim, bayar penuh/sebagian/belum) lewat 4 jalur paralel`, async () => {
    test.setTimeout(LOAD_BUDGET_MS + 60_000);
    const t0 = Date.now();
    const jobs = Array.from({ length: SO_COUNT }, (_, j) => j);
    soTotals.length = soPaid.length = soIds.length = SO_COUNT;
    await lanes(jobs, (j) => j, async (j) => {
      const lane = j % LANES;
      const p = prods[lane];
      const qty = soQty(j);
      const cust = customerIds[j % CUSTOMERS];
      const so = await createSO("admin", cust, [{ p, qty }], { paymentTerms: "net", dueDate: today(14) });
      expect(so.grandTotal).toBe(qty * p.sellingPrice);
      await confirmSO("admin", so.id);
      await confirmDelivery("gudang", (await createDelivery("gudang", so.id, [{ p, qty }])).id);
      soIds[j] = so.id;
      soTotals[j] = so.grandTotal;
      soPaid[j] = 0;
      if (j % 3 !== 2) {
        const amt = j % 3 === 0 ? so.grandTotal : Math.floor(so.grandTotal / 2);
        await pay("akuntan", cust, amt, [{ orderType: "sales", orderId: so.id, amount: amt }], "transfer");
        soPaid[j] = amt;
      }
    });
    loadMs += Date.now() - t0;
    expect(loadMs, `total pembuatan massal ${loadMs}ms`).toBeLessThan(LOAD_BUDGET_MS);
  });

  test("daftar: total naik tepat 50/100, paginasi tanpa duplikat/lompat, jumlah nilai cocok, responsif", async () => {
    expect(await listTotal("/purchase-orders")).toBe(base.po + PO_COUNT);
    expect(await listTotal("/sales-orders")).toBe(base.so + SO_COUNT);

    for (const [path, expectedIds, expectedSum, partyPrefix] of [
      ["/purchase-orders", poIds, poTotals, "Volume Pemasok"],
      ["/sales-orders", soIds, soTotals, "Volume Pelanggan"],
    ] as const) {
      const seen: any[] = [];
      let page = 1;
      for (;;) {
        const r = await timed(`${path} halaman ${page}`, LIST_BUDGET_MS, () => call("admin", "GET", `${path}?search=Volume&limit=25&page=${page}`));
        expect(r.status).toBe(200);
        seen.push(...r.data);
        expect(r.meta.limit).toBe(25);
        if (page >= r.meta.totalPages) {
          expect(r.meta.total).toBe(expectedIds.length);
          break;
        }
        page++;
        expect(page).toBeLessThan(20);
      }
      expect(seen).toHaveLength(expectedIds.length);
      const ids = seen.map((o) => o.id as number);
      expect(new Set(ids).size, "tanpa duplikat antar halaman").toBe(ids.length);
      expect([...ids].sort((a, b) => a - b)).toEqual([...expectedIds].sort((a, b) => a - b));
      expect(ids, "urut id turun").toEqual([...ids].sort((a, b) => b - a));
      expect(seen.every((o) => String(o.partyName).startsWith(partyPrefix))).toBe(true);
      expect(seen.reduce((a, o) => a + o.grandTotal, 0)).toBe(expectedSum.reduce((a, b) => a + b, 0));
    }
    // halaman terbesar (limit 100) tetap cepat
    await timed("SO limit=100", LIST_BUDGET_MS, () => call("admin", "GET", "/sales-orders?limit=100"));
  });

  test("filter status bayar pada daftar penjualan sesuai data (lunas/sebagian/belum)", async () => {
    const want = { paid: 0, partial: 0, unpaid: 0 };
    soTotals.forEach((t, j) => {
      if (soPaid[j] === 0) want.unpaid++;
      else if (soPaid[j] >= t) want.paid++;
      else want.partial++;
    });
    for (const status of ["paid", "partial", "unpaid"] as const) {
      const r = await timed(`SO paymentStatus=${status}`, LIST_BUDGET_MS, () => call("admin", "GET", `/sales-orders?search=Volume&paymentStatus=${status}&limit=100`));
      expect(r.status, r.message).toBe(200);
      expect(r.meta.total, status).toBe(want[status]);
    }
  });

  test("laporan order dan dasbor: selisih sama dengan data yang dibuat, dalam batas waktu", async () => {
    const so = await timed("laporan order jual", LIST_BUDGET_MS, () => report("sales"));
    const po = await timed("laporan order beli", LIST_BUDGET_MS, () => report("purchasing"));
    expect(so.total - base.reportSo.total).toBe(SO_COUNT);
    expect(so.grand - base.reportSo.grand).toBe(soTotals.reduce((a, b) => a + b, 0));
    expect(po.total - base.reportPo.total).toBe(PO_COUNT);
    expect(po.grand - base.reportPo.grand).toBe(poTotals.reduce((a, b) => a + b, 0));
    await timed("dasbor", LIST_BUDGET_MS, () => ok("admin", "GET", "/dashboard/summary"));
    await timed("laporan stok", LIST_BUDGET_MS, () => ok("admin", "GET", "/reporting/stock?limit=100"));
  });

  test("stok tiap produk volume = diterima - terjual", async () => {
    for (let k = 0; k < LANES; k++) {
      const received = poIds.reduce((a, _, i) => a + (i % LANES === k ? poLines(i) : 0), 0);
      const sold = soIds.reduce((a, _, j) => a + (j % LANES === k ? soQty(j) : 0), 0);
      expect(await stockQty(prods[k].id), prods[k].code).toBe(received - sold);
    }
  });

  test("buku: posting menjurnal seimbang; piutang/hutang per pihak = saldo dokumen", async () => {
    test.setTimeout(240_000);
    // N-52: SATU close-day memposting per 50 dokumen sampai habis dan melaporkan sisa (0 di sini)
    const closed = await timed("posting close-day (satu panggilan)", 120_000, () =>
      ok<any>("akuntan", "POST", "/finance/posting/close-day", { date: today() }));
    expect(closed.posted, "dokumen diposting dalam satu panggilan").toBeGreaterThanOrEqual(PO_COUNT + SO_COUNT);
    expect(closed.failed).toBe(0);
    expect(closed.remaining, "antrean tersisa setelah tutup harian").toBe(0);
    expect(closed.results.length).toBe(closed.posted);
    const tb = await ok<any>("akuntan", "GET", `/finance/reports/trial-balance?asOf=${today()}`);
    expect(tb.balanced).toBe(true);

    const recv = await timed("laporan piutang", LIST_BUDGET_MS, () => ok<any>("akuntan", "GET", `/finance/reports/receivables?asOf=${today()}`));
    const pay_ = await timed("laporan hutang", LIST_BUDGET_MS, () => ok<any>("akuntan", "GET", `/finance/reports/payables?asOf=${today()}`));
    const byCust = new Map<number, number>((recv.parties as any[]).map((r) => [r.partyId, r.balance]));
    const bySupp = new Map<number, number>((pay_.parties as any[]).map((r) => [r.partyId, r.balance]));

    const diffs: string[] = [];
    for (let c = 0; c < CUSTOMERS; c++) {
      let owed = 0;
      soTotals.forEach((t, j) => {
        if (j % CUSTOMERS === c) owed += t - soPaid[j];
      });
      const book = byCust.get(customerIds[c]) ?? 0;
      const bal = await ok<any>("akuntan", "GET", `/payments/party-balances?partyId=${customerIds[c]}`);
      if (book !== owed || bal.outstanding !== owed) diffs.push(`pelanggan ${c + 1}: dokumen=${owed} saldo-pihak=${bal.outstanding} buku=${book}`);
    }
    for (let k = 0; k < LANES; k++) {
      let owed = 0;
      poTotals.forEach((t, i) => {
        if (i % LANES === k) owed += t - poPaid[i];
      });
      const book = bySupp.get(supplierIds[k]) ?? 0;
      if (book !== owed) diffs.push(`pemasok ${k + 1}: dokumen=${owed} buku=${book}`);
    }
    expect(diffs, "piutang/hutang buku = saldo dokumen per pihak").toEqual([]);
  });

  // ---- integritas relasional pelanggan (padanan legacy 22-A/B/C, 20/21) -----------------------------

  test("rename pelanggan: order tetap tertaut by-id, nama terbaca live, tanpa duplikat", async () => {
    const p = prods[0];
    const oldName = "Relasi Nama Awal";
    const newName = "Relasi Nama Baru";
    const c = await ok<any>("admin", "POST", "/customers", { name: oldName, phone: "081200010001" });
    const so = await createSO("admin", c.id, [{ p, qty: 1 }], { paymentTerms: "net", dueDate: today(14) });

    await ok("admin", "PUT", `/customers/${c.id}`, { name: newName, phone: "081200010001" });

    const detail = await ok<any>("admin", "GET", `/sales-orders/${so.id}`);
    expect(detail.partyId).toBe(c.id);
    expect(detail.partyName).toBe(newName);
    const byNew = await ok<any[]>("admin", "GET", `/sales-orders?search=${encodeURIComponent(newName)}&limit=50`);
    expect(byNew.some((o) => o.id === so.id)).toBe(true);
    const byOld = await ok<any[]>("admin", "GET", `/sales-orders?search=${encodeURIComponent(oldName)}&limit=50`);
    expect(byOld.some((o) => o.id === so.id)).toBe(false);
    const named = (list: any[], n: string) => list.filter((x) => x.name === n).length;
    expect(named(await ok<any[]>("admin", "GET", `/customers?search=${encodeURIComponent(newName)}&limit=50`), newName)).toBe(1);
    expect(named(await ok<any[]>("admin", "GET", `/customers?search=${encodeURIComponent(oldName)}&limit=50`), oldName)).toBe(0);
  });

  test("tipe member: arsip diizinkan (KI-72 dibuang), dampaknya terbaca lewat customer-count; harga turun ke standar", async () => {
    const code = "VOLMBR";
    const m = await ok<any>("admin", "POST", "/member-types", { code, name: "Volume Member Grosir", value: 5 });
    const c = await ok<any>("admin", "POST", "/customers", { name: "Relasi Pelanggan Member", phone: "081200010002", memberTypeId: m.id });
    expect((await ok<any>("admin", "GET", `/customers/${c.id}`)).member?.id ?? m.id).toBe(m.id);
    expect((await ok<any>("admin", "GET", `/member-types/${m.id}/customer-count`)).activeCustomers).toBe(1);

    await ok("admin", "POST", `/member-types/${m.id}/archive`);
    // pelanggan tidak kehilangan relasi; tipe arsip tetap terlihat di customer-count agar UI bisa memperingatkan
    expect((await ok<any>("admin", "GET", `/member-types/${m.id}/customer-count`)).activeCustomers).toBe(1);
    // harga: tipe arsip = harga standar + penanda (bukan diskon diam-diam)
    const q = await ok<any>("admin", "POST", "/pricing/quote", { customerId: c.id, productIds: [prods[0].id] });
    expect(JSON.stringify(q)).toContain('"memberInactive":true');
    expect(JSON.stringify(q)).toContain(String(prods[0].sellingPrice));
    expect(JSON.stringify(q)).not.toContain(String(Math.round(prods[0].sellingPrice * 0.95)));
    await ok("admin", "POST", `/member-types/${m.id}/restore`);
  });

  test("arsip pelanggan berpiutang: hilang dari daftar aktif, saldo & piutang buku tetap terlihat, order BARU ditolak", async () => {
    const p = prods[1];
    const c = await ok<any>("admin", "POST", "/customers", { name: "Relasi Pelanggan Arsip", phone: "081200010003" });
    const so = await createSO("admin", c.id, [{ p, qty: 3 }], { paymentTerms: "net", dueDate: today(14) });
    await confirmSO("admin", so.id);
    await confirmDelivery("gudang", (await createDelivery("gudang", so.id, [{ p, qty: 3 }])).id);
    await drainPosting();
    const owed = 3 * p.sellingPrice;
    expect((await ok<any>("admin", "GET", `/payments/party-balances?partyId=${c.id}`)).outstanding).toBe(owed);

    await ok("admin", "POST", `/customers/${c.id}/archive`);

    const active = await ok<any[]>("admin", "GET", `/customers?search=${encodeURIComponent("Relasi Pelanggan Arsip")}&limit=50`);
    expect(active.some((x) => x.id === c.id && x.status !== "archived")).toBe(false);
    const archived = await ok<any[]>("admin", "GET", `/customers?search=${encodeURIComponent("Relasi Pelanggan Arsip")}&status=archived&limit=50`);
    expect(archived.some((x) => x.id === c.id)).toBe(true);

    // saldo tidak menggantung: dokumen dan buku tetap melaporkan piutang pihak terarsip
    expect((await ok<any>("admin", "GET", `/payments/party-balances?partyId=${c.id}`)).outstanding).toBe(owed);
    const recv = await ok<any>("akuntan", "GET", `/finance/reports/receivables?asOf=${today()}`);
    expect((recv.parties as any[]).find((r) => r.partyId === c.id)?.balance).toBe(owed);
    const ledger = await call("admin", "GET", `/payments/party-ledger?partyId=${c.id}`);
    expect(ledger.status).toBe(200);

    // tidak boleh ada bisnis baru untuk pihak terarsip
    const again = await call("admin", "POST", "/sales-orders", {
      partyId: c.id, channel: "regular", paymentTerms: "cod", taxType: "none",
      items: [{ productId: p.id, variantId: p.variantId, uom: "pcs", qty: 1 }],
    });
    expect(again.status).toBeGreaterThanOrEqual(400);
    // Pembayaran piutang pihak terarsip: lihat E07 test.fixme (K2, README).
  });
});
