// Setup cepat lewat REST: satu token per peran (di-cache, dimuat ulang saat 401).
import { randomUUID } from "node:crypto";
import { BASE_URL, PASSWORD } from "./env";

export type Role = "admin" | "kasir" | "gudang" | "akuntan" | "adminCb2";

export interface Session {
  access: string;
  refresh: string;
  at: number;
}

const sessions = new Map<Role, Session>();

export class ApiError extends Error {
  constructor(public status: number, message: string, public body: unknown) {
    super(`${status} ${message}`);
  }
}

export async function rawLogin(username: string, password: string) {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier: username, password }),
  });
  const json: any = await res.json();
  if (!res.ok || !json.success) throw new ApiError(res.status, json.message ?? "login gagal", json);
  return json.data;
}

/** Login (memilih cabang HQ bila perlu) dan simpan sesi per peran. */
export async function session(role: Role, fresh = false): Promise<Session> {
  const cached = sessions.get(role);
  if (cached && !fresh && Date.now() - cached.at < 8 * 60_000) return cached;
  const d = await rawLogin(role === "adminCb2" ? "admin" : role, PASSWORD);
  let access: string = d.accessToken;
  if (role === "adminCb2" || d.requiresBranchSelection || !d.branch) {
    const want = role === "adminCb2" ? "CB2" : "HQ";
    const list = d.branches?.length ? d.branches : await (await fetch(`${BASE_URL}/api/v1/branches/my-access`, { headers: { Authorization: `Bearer ${access}` } })).json().then((j: any) => j.data);
    const hq = (list ?? []).find((b: any) => b.code === want) ?? list?.[0];
    const res = await fetch(`${BASE_URL}/api/v1/auth/switch-branch`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${access}` },
      body: JSON.stringify({ branchId: hq.id }),
    });
    const j: any = await res.json();
    if (!j.success) throw new ApiError(res.status, j.message, j);
    access = j.data.accessToken;
  }
  const s = { access, refresh: d.refreshToken as string, at: Date.now() };
  sessions.set(role, s);
  return s;
}

export interface CallResult<T = any> {
  status: number;
  success: boolean;
  message: string;
  data: T;
  meta?: any;
  errors?: any;
}

/** Panggilan mentah (tanpa melempar) — dipakai untuk menegaskan penolakan. */
export async function call<T = any>(role: Role, method: string, path: string, body?: unknown): Promise<CallResult<T>> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const s = await session(role, attempt > 0);
    const res = await fetch(`${BASE_URL}/api/v1${path}`, {
      method,
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${s.access}` },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (res.status === 401 && attempt === 0) continue;
    const text = await res.text();
    let j: any = {};
    try {
      j = JSON.parse(text);
    } catch {
      /* bukan JSON */
    }
    return { status: res.status, success: !!j.success, message: j.message ?? "", data: j.data, meta: j.meta, errors: j.errors };
  }
  throw new Error("unreachable");
}

/** Panggilan yang wajib sukses; mengembalikan data. */
export async function ok<T = any>(role: Role, method: string, path: string, body?: unknown): Promise<T> {
  const r = await call<T>(role, method, path, body);
  if (r.status >= 300 || !r.success) {
    throw new ApiError(r.status, `${method} ${path}: ${r.message}`, { errors: r.errors, data: r.data });
  }
  return r.data;
}

export const uuid = () => randomUUID();

// ---- data seed (kode tetap) ------------------------------------------------

export interface Product {
  id: number;
  code: string;
  name: string;
  variantId: number;
  barcode: string;
  sellingPrice: number;
  purchasePrice: number;
}

export async function products(): Promise<Product[]> {
  const list: any[] = await ok("admin", "GET", "/products?limit=100&status=active");
  const out: Product[] = [];
  for (const p of list) {
    const d: any = await ok("admin", "GET", `/products/${p.id}`);
    const v = d.variants[0];
    out.push({
      id: p.id, code: p.code, name: p.name, variantId: v.id, barcode: v.barcode,
      sellingPrice: d.sellingPrice, purchasePrice: d.purchasePrice,
    });
  }
  return out.sort((a, b) => a.code.localeCompare(b.code));
}

export async function productByCode(code: string): Promise<Product> {
  const all = await products();
  const p = all.find((x) => x.code === code);
  if (!p) throw new Error(`produk ${code} tidak ada`);
  return p;
}

export async function partyByCode(kind: "customers" | "suppliers", code: string): Promise<any> {
  const list: any[] = await ok("admin", "GET", `/${kind}?limit=100&search=${code}`);
  const p = list.find((x) => x.code === code);
  if (!p) throw new Error(`${kind} ${code} tidak ada`);
  return p;
}

export async function locations(): Promise<any[]> {
  return ok("admin", "GET", "/stock/locations");
}

export async function locationByCode(code: string): Promise<any> {
  const l = (await locations()).find((x) => x.code === code);
  if (!l) throw new Error(`lokasi ${code} tidak ada`);
  return l;
}

export async function branches(): Promise<any[]> {
  return ok("admin", "GET", "/branches?limit=50");
}

// ---- alur dokumen ------------------------------------------------------------

export function today(offsetDays = 0): string {
  const d = new Date(Date.now() + offsetDays * 86_400_000);
  return new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Jakarta" }).format(d);
}

export async function createSO(role: Role, partyId: number, lines: { p: Product; qty: number; unitPrice?: number }[], opts: Record<string, unknown> = {}) {
  return ok<any>(role, "POST", "/sales-orders", {
    partyId, channel: "regular", paymentTerms: "cod", taxType: "none", clientRef: uuid(),
    items: lines.map((l) => ({ productId: l.p.id, variantId: l.p.variantId, uom: "pcs", qty: l.qty, unitPrice: l.unitPrice })),
    ...opts,
  });
}

export async function confirmSO(role: Role, id: number) {
  return ok<any>(role, "POST", `/sales-orders/${id}/confirm`);
}

export async function createDelivery(role: Role, soId: number, lines: { p: Product; qty: number }[]) {
  return ok<any>(role, "POST", "/deliveries", {
    salesOrderId: soId, clientRef: uuid(),
    items: lines.map((l) => ({ productId: l.p.id, variantId: l.p.variantId, uom: "pcs", qty: l.qty })),
  });
}

export async function confirmDelivery(role: Role, id: number, body: unknown = {}) {
  return ok<any>(role, "POST", `/deliveries/${id}/confirm`, body);
}

export async function pay(role: Role, partyId: number, amount: number, allocations?: { orderType: string; orderId: number; amount: number }[], method = "cash") {
  return ok<any>(role, "POST", "/payments", { partyId, amount, method, clientRef: uuid(), allocations, ...(method === "cash" ? { amountTendered: amount } : {}) });
}

export async function stockQty(productId: number, locationId?: number): Promise<number> {
  const rows: any[] = await ok("admin", "GET", `/stock/balances/${productId}`);
  return rows.filter((r) => locationId === undefined || r.locationId === locationId).reduce((a, r) => a + (r.qty ?? r.onHand ?? 0), 0);
}

// ---- pembelian -----------------------------------------------------------------

export async function createPO(role: Role, supplierId: number, lines: { p: Product; qty: number; unitPrice?: number }[], opts: Record<string, unknown> = {}) {
  return ok<any>(role, "POST", "/purchase-orders", {
    partyId: supplierId, paymentTerms: "net", dueDate: today(30), taxType: "none",
    items: lines.map((l) => ({
      productId: l.p.id, variantId: l.p.variantId, uom: "pcs", qty: l.qty,
      unitPrice: l.unitPrice ?? l.p.purchasePrice, discountPct: 0, discountNominal: 0,
    })),
    ...opts,
  });
}

export async function confirmPO(role: Role, id: number) {
  return ok<any>(role, "POST", `/purchase-orders/${id}/confirm`);
}

export function receiveBody(poId: number, locationId: number, lines: { p: Product; qty: number }[], extra: Record<string, unknown> = {}) {
  return {
    purchaseOrderId: poId, clientRef: uuid(),
    items: lines.map((l) => ({ productId: l.p.id, variantId: l.p.variantId, locationId, uom: "pcs", qty: l.qty })),
    ...extra,
  };
}

/**
 * Tutup harian kini memposting per 50 dokumen sampai antrean hari itu habis (maks. 500 per panggilan)
 * dan melaporkan `remaining` (N-52). Ulangi hanya bila masih tersisa dan ada kemajuan; mengembalikan
 * jumlah dokumen yang diposting.
 */
export async function drainPosting(day = today()): Promise<number> {
  let total = 0;
  for (let i = 0; i < 10; i++) {
    const res = await ok<{ results: any[]; posted: number; failed: number; remaining: number }>("akuntan", "POST", "/finance/posting/close-day", { date: day });
    total += res.posted;
    if (res.remaining === 0 || res.posted === 0) return total; // habis, atau sisa hanya yang gagal
  }
  throw new Error("antrean posting tak kunjung habis setelah 10 panggilan");
}
