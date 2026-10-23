import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { formatIDR } from "@/shared/lib/format";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { posCheckoutSchema } from "@/modules/pos/schemas/pos-checkout.schema";
import { findPosPayment, posService } from "@/modules/pos/services/pos.service";
import {
  cashChange,
  clampLineDiscount,
  lineDiscount,
  lineGross,
  lineTotal,
  maxLineDiscount,
  reclampLineDiscounts,
  type MemberPrices,
} from "@/modules/pos/lib/cart";
import { formatQty, normalizeQty } from "@/modules/pos/lib/qty";
import { availableByVariant, sellableLocationIds } from "@/modules/pos/lib/stock";
import {
  CHECKOUT_STEP_LABEL,
  NO_PAY_PERMISSION_MESSAGE,
  PAY_LATER_NEEDS_CUSTOMER_MESSAGE,
  buildPosOrderPayload,
  payLaterBlockReason,
  runCheckout,
  type CheckoutApi,
} from "@/modules/pos/lib/checkout";
import { formatApiError } from "@/shared/services/http-client";
import {
  clearDraft,
  loadOrCreateDraft,
  newClientRef,
  saveDraft,
  type HeldSnapshot,
} from "@/modules/pos/lib/cart-draft";
import { DEFAULT_VAT_RATE, estimateTax, type SalesTaxType } from "@/modules/sales/lib/tax";
import { isDefiniteRejection, readApiError } from "@/modules/pos/lib/api-error";
import { deliveryItemsFor, lineKey } from "@/modules/pos/lib/holds";
import { posReadiness } from "@/modules/pos/lib/readiness";
import { scanAction, scanErrorMessage } from "@/modules/pos/lib/scan";
import { sellableVariants } from "@/modules/products/lib/variants";
import type { ScanOutcome } from "@/modules/pos/lib/scan-session";
import { useStockHolds } from "@/modules/pos/hooks/useStockHolds";
import type {
  CartLine,
  CheckoutMode,
  CompletedSale,
  PayMethod,
  PosCustomerDetail,
  PosMemberType,
  PosOrderState,
  PosProductDetail,
  PricingQuote,
  ScanVariantChoice,
} from "@/modules/pos/types";

let nextKey = 1;

/** Harga member per produk dari quote (hanya baris yang applied). */
function memberPriceMap(quote: PricingQuote | null): Map<number, number> {
  const map = new Map<number, number>();
  for (const l of quote?.lines ?? []) {
    if (l.applied) map.set(l.productId, l.memberPrice);
  }
  return map;
}

/** Satuan jual: salesUom dulu, lalu sisanya tanpa duplikat/kosong. */
function saleUoms(p: PosProductDetail): string[] {
  return [p.salesUom, p.baseUom, p.purchaseUom].filter(
    (u, i, arr) => (u ?? "").trim() !== "" && arr.indexOf(u) === i,
  );
}

export type AddResult =
  | { ok: true; name: string; qty: number }
  // "choose": produk punya > 1 varian layak jual — lembar pilih varian dibuka.
  | { ok: false; reason: "locked" | "failed" | "choose" };

export interface VariantChoiceState {
  productId: number;
  productCode: string;
  productName: string;
  variants: ScanVariantChoice[];
}

/** Stok tersedia per varian untuk pemilih varian — aturan yang sama dengan tile katalog
 * (lokasi jual, satuan jual; lib/stock). Gagal/403 = tidak diketahui (pemilih tetap tampil tanpa
 * angka stok). */
async function variantAvailability(productId: number, salesFactor?: number): Promise<Map<number, number> | null> {
  try {
    const [rows, locations] = await Promise.all([
      posService.productBalances(productId),
      posService.locations().catch(() => null),
    ]);
    return availableByVariant(rows ?? [], sellableLocationIds(locations), salesFactor);
  } catch {
    return null;
  }
}

/**
 * Logika Mode Kasir: keranjang, pelanggan + harga member, tahanan stok, draf, dan
 * checkout bertahap (idempoten lewat clientRef). Dipindahkan apa adanya dari PosPage
 * (tanpa perubahan perilaku); tampilan ada di komponen terpisah.
 */
export function usePosCart() {
  const can = useAuthStore((s) => s.can);
  // Gate halaman (sales.create) sudah di route-level; langkah pembayaran
  // butuh payment.create — tanpanya order tetap bisa dibuat.
  const canPay = can("payment.create");
  // Diskon per baris butuh sales.update (F2a-12); server tetap otoritas.
  const canDiscount = can("sales.update");
  const readiness = posReadiness(can);
  const canMemberTypes = can("member_types.view") || can("member_types.manage");

  // Draf keranjang dari localStorage: clientRef (UUID per keranjang) bertahan
  // setelah reload dan dipakai sebagai kunci idempoten SO/SJ/pembayaran.
  const [initialDraft] = useState(() => {
    const d = loadOrCreateDraft();
    for (const l of d.lines) nextKey = Math.max(nextKey, l.key + 1);
    return d;
  });
  const [cartRef, setCartRef] = useState(initialDraft.ref);
  // true setelah order di server dibuat (atau mungkin dibuat): isi keranjang
  // dikunci sampai checkout selesai atau transaksi dibatalkan.
  const [started, setStarted] = useState(initialDraft.started);
  const startedLive = useRef(initialDraft.started);
  useEffect(() => {
    startedLive.current = started;
  }, [started]);
  const [heldSnap, setHeldSnap] = useState<HeldSnapshot>(initialDraft.held);
  const [resumeStep, setResumeStep] = useState<string | null>(null);

  const [addingId, setAddingId] = useState<number | null>(null);
  const [barcode, setBarcode] = useState("");
  const [scanning, setScanning] = useState(false);
  const scanQueue = useRef<string[]>([]);
  const scanBusy = useRef(false);
  const [scanNotice, setScanNotice] = useState<string | null>(null);
  // Kode produk bervarian: kasir memilih varian sebelum masuk keranjang.
  const [variantChoice, setVariantChoice] = useState<VariantChoiceState | null>(null);

  // --- keranjang ---
  const [customerId, setCustomerIdState] = useState<number | null>(initialDraft.customerId);
  // Diskon baris yang diturunkan otomatis karena harga efektif berubah (tampil menetap).
  const [discountNotice, setDiscountNotice] = useState<string | null>(null);
  const [cart, setCartState] = useState<CartLine[]>(initialDraft.lines);
  // Cermin sinkron isi keranjang: pindai beruntun bisa menambah lebih cepat dari render.
  const cartLive = useRef<CartLine[]>(initialDraft.lines);
  const setCart = useCallback((update: (rows: CartLine[]) => CartLine[]) => {
    const next = update(cartLive.current);
    cartLive.current = next;
    setCartState(next);
  }, []);

  // --- checkout ---
  const [checkoutOpen, setCheckoutOpen] = useState(false);
  const [method, setMethod] = useState<PayMethod>(initialDraft.method);
  const [mode, setMode] = useState<CheckoutMode>(initialDraft.mode);
  const [dueDate, setDueDate] = useState(initialDraft.dueDate);
  const [tendered, setTendered] = useState(initialDraft.tendered);
  const [notes, setNotes] = useState(initialDraft.notes);
  // PPN opsional (K-03): bawaan tanpa pajak; alur kasir biasa tidak berubah.
  const [taxType, setTaxTypeState] = useState<SalesTaxType>(initialDraft.taxType);
  const [taxRate, setTaxRate] = useState(initialDraft.taxRate);
  const [shipToAddressId, setShipToAddressId] = useState<number | null>(initialDraft.shipToAddressId);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [processing, setProcessing] = useState(false);
  const [completed, setCompleted] = useState<CompletedSale | null>(null);
  const [hideDiscount, setHideDiscount] = useState(false);

  // --- panel pelanggan: detail + quote harga member ---
  const [customerDetail, setCustomerDetail] = useState<PosCustomerDetail | null>(null);
  const [memberTypes, setMemberTypes] = useState<PosMemberType[]>([]);
  const [quote, setQuote] = useState<PricingQuote | null>(null);
  const [quoteStatus, setQuoteStatus] = useState<"idle" | "loading" | "error">("idle");
  const [quoteError, setQuoteError] = useState<string | null>(null);
  useEffect(() => {
    // Hanya bila diizinkan (member_types.view/manage): tanpa izin tidak perlu
    // memanggil dan menelan 403; catatan kurang-izin tampil di kepala keranjang.
    if (!canMemberTypes) return;
    posService
      .memberTypes()
      .then((list) => setMemberTypes(list ?? []))
      .catch((err) => {
        setMemberTypes([]);
        toast.warning("Tipe member tidak termuat", readApiError(err).message);
      });
  }, [canMemberTypes]);
  if (customerId === null && customerDetail !== null) {
    setCustomerDetail(null);
  }
  // Bayar Nanti (termin net) hanya untuk pelanggan terdaftar bertelepon — server menolak
  // selain itu. Detail yang belum termuat (atau milik pelanggan lain) tidak memblokir.
  const payLaterBlock = payLaterBlockReason({
    customerId,
    customerPhone: customerDetail !== null && customerDetail.id === customerId ? customerDetail.phone : undefined,
  });
  if (payLaterBlock !== null && mode === "pay_later" && !started) {
    setMode("pay_now");
  }
  useEffect(() => {
    if (customerId === null) return;
    let alive = true;
    posService
      .customerDetail(customerId)
      .then((d) => {
        if (alive) setCustomerDetail(d);
      })
      .catch(() => {
        if (alive) setCustomerDetail(null);
      });
    return () => {
      alive = false;
    };
  }, [customerId]);

  // --- tahanan stok (suggest → hold, debounce 400 ms; release saat hapus) ---
  const holdLines = useMemo(
    () => cart.map((l) => ({ productId: l.productId, variantId: l.variantId, qty: l.qty })),
    [cart],
  );
  const holds = useStockHolds({
    cartRef,
    lines: holdLines,
    mode: completed !== null ? "off" : started ? "hold" : "sync",
    initialHeld: initialDraft.started ? initialDraft.held : undefined,
    onHeldChange: setHeldSnap,
  });

  // --- persistensi draf (clientRef + isi keranjang bertahan setelah reload) ---
  useEffect(() => {
    saveDraft({
      ref: cartRef,
      lines: cart,
      customerId,
      started,
      mode,
      method,
      dueDate,
      tendered,
      notes,
      shipToAddressId,
      taxType,
      taxRate,
      held: heldSnap,
    });
  }, [cartRef, cart, customerId, started, mode, method, dueDate, tendered, notes, shipToAddressId, taxType, taxRate, heldSnap]);

  // --- quote harga member untuk isi keranjang (debounce; gagal = katalog) ---
  const cartProductIds = useMemo(
    () =>
      cart
        .map((l) => l.productId)
        .filter((id, i, arr) => arr.indexOf(id) === i)
        .sort((a, b) => a - b),
    [cart],
  );
  const cartProductKey = cartProductIds.join(",");
  if (
    (customerId === null || cartProductIds.length === 0) &&
    (quote !== null || quoteStatus !== "idle" || quoteError !== null)
  ) {
    setQuote(null);
    setQuoteStatus("idle");
    setQuoteError(null);
  }
  /** Klem ulang diskon semua baris terhadap harga efektif baru (harga member tiba,
   * berubah, atau hilang): nominal yang sah pada harga katalog bisa melampaui harga
   * member → baris negatif ditolak server. Keranjang terkunci (checkout berjalan)
   * tidak diubah — isinya harus sama dengan order di server. */
  function reclampDiscounts(prices: MemberPrices) {
    // Dibaca dari ref: quote bisa tiba setelah checkout dimulai (closure efek basi).
    if (startedLive.current) return;
    const r = reclampLineDiscounts(cartLive.current, prices);
    if (r.reduced.length === 0) return;
    setCart(() => r.lines);
    const names = r.reduced.map((l) => l.productName).join(", ");
    const text = `Diskon ${names} diturunkan agar tidak melebihi harga yang berlaku (harga member / harga jual minimum). Periksa kembali sebelum membayar.`;
    setDiscountNotice(text);
    toast.warning("Diskon disesuaikan dengan harga baru", text);
  }

  /** Ganti pelanggan. Dilepas (walk-in) = harga katalog langsung berlaku lagi. */
  function selectCustomer(id: number | null) {
    setCustomerIdState(id);
    if (id === null) reclampDiscounts(new Map());
  }

  useEffect(() => {
    if (customerId === null || cartProductIds.length === 0) {
      return;
    }
    const t = window.setTimeout(() => {
      setQuoteStatus("loading");
      setQuoteError(null);
      posService
        .pricingQuote(customerId, cartProductIds)
        .then((q) => {
          setQuote(q);
          setQuoteStatus("idle");
          reclampDiscounts(memberPriceMap(q));
        })
        .catch((err) => {
          setQuote(null);
          reclampDiscounts(new Map());
          setQuoteStatus("error");
          setQuoteError(
            `Harga member tidak dapat dimuat (${readApiError(err).message}) — memakai harga katalog.`,
          );
        });
    }, 400);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [customerId, cartProductKey]);

  /** Harga member per produk — peta, bukan pencarian linier. */
  const memberPrices = useMemo(() => memberPriceMap(quote), [quote]);
  // Galat per baris dari quote: tampil di baris itu, bukan menggagalkan seluruh quote.
  const quoteLineErrors = useMemo(() => {
    const map = new Map<number, string>();
    for (const l of quote?.lines ?? []) {
      if (l.error) map.set(l.productId, l.error);
    }
    return map;
  }, [quote]);
  const memberPriceOf = useCallback(
    (productId: number): number | null => memberPrices.get(productId) ?? null,
    [memberPrices],
  );
  const displayUnitPrice = useCallback(
    (l: CartLine): number => memberPriceOf(l.productId) ?? l.unitPrice,
    [memberPriceOf],
  );
  const memberSavings = useMemo(
    () =>
      cart.reduce(
        (sum, l) => sum + Math.max(0, l.unitPrice - displayUnitPrice(l)) * l.qty,
        0,
      ),
    [cart, displayUnitPrice],
  );
  const memberName =
    memberTypes.find((m) => m.code === (quote?.memberType ?? ""))?.name ??
    (quote?.memberType ? quote.memberType : null);

  /** Tambah produk ke keranjang (varian pindaian atau varian default). */
  const addProductById = useCallback(
    async (productId: number, variantId?: number): Promise<AddResult> => {
      if (started) {
        toast.warning(
          "Checkout sedang berjalan",
          "Selesaikan atau batalkan transaksi ini sebelum mengubah keranjang.",
        );
        return { ok: false, reason: "locked" };
      }
      setAddingId(productId);
      try {
        const d = await posService.productDetail(productId);
        if (d.variants.length === 0) {
          toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
          return { ok: false, reason: "failed" };
        }
        if (variantId !== undefined && !d.variants.some((v) => v.id === variantId)) {
          toast.danger("Gagal menambah baris", "Varian hasil pindai tidak ada di produk ini.");
          return { ok: false, reason: "failed" };
        }
        // Tanpa varian pindaian: produk dengan > 1 varian layak jual (aktif, tidak tersembunyi)
        // wajib dipilih — paritas legacy (dropdown varian di kartu POS). Varian tersembunyi
        // "Standar" tidak pernah ditawarkan selama ada varian terlihat.
        const sellable = sellableVariants(d.variants);
        if (variantId === undefined && sellable.length > 1) {
          const available = await variantAvailability(d.id, d.salesFactor);
          setVariantChoice({
            productId: d.id,
            productCode: d.code,
            productName: d.name,
            variants: sellable.map((v) => ({
              id: v.id,
              code: v.code,
              name: v.name,
              barcode: v.barcode,
              available: available?.get(v.id) ?? (available ? 0 : undefined),
            })),
          });
          return { ok: false, reason: "choose" };
        }
        const variant =
          (variantId !== undefined ? d.variants.find((v) => v.id === variantId) : undefined) ??
          sellable[0] ??
          d.variants.find((v) => v.isDefault) ??
          d.variants[0];
        const uoms = saleUoms(d);
        if (uoms.length === 0) {
          toast.danger("Gagal menambah baris", "Produk tidak punya satuan jual.");
          return { ok: false, reason: "failed" };
        }
        const uom = uoms[0];
        const found = cartLive.current.find(
          (l) => l.productId === d.id && l.variantId === variant.id && l.uom === uom,
        );
        const qty = found ? (normalizeQty(found.qty + 1) ?? 1) : 1;
        const same = (l: CartLine) => l.productId === d.id && l.variantId === variant.id && l.uom === uom;
        // Decided inside the update: two scans in a row can land before the
        // first one re-rendered, so the live ref may not show the line yet.
        setCart((rows) => {
          if (rows.some(same)) return rows.map((l) => (same(l) ? { ...l, qty: normalizeQty(l.qty + 1) ?? l.qty } : l));
          return [
            ...rows,
            {
              key: nextKey++,
              productId: d.id,
              productCode: d.code,
              productName: d.name,
              variantId: variant.id,
              variantCode: variant.code,
              variantName: variant.name,
              uom,
              qty: 1,
              unitPrice: d.sellingPrice,
              minSellingPrice: d.minSellingPrice ?? 0,
              discountPct: 0,
              discountNominal: 0,
            },
          ];
        });
        return { ok: true, name: d.name, qty };
      } catch (err) {
        toast.danger("Gagal memuat produk", readApiError(err).message);
        return { ok: false, reason: "failed" };
      } finally {
        setAddingId(null);
      }
    },
    [started, setCart],
  );

  /** Barcode → pindai lalu tambah variannya. Dipakai input ketik/wedge dan pindai kamera
   * beruntun; mengembalikan hasilnya agar pemanggil bisa menampilkan umpan balik. */
  const scanCode = useCallback(
    async (code: string): Promise<ScanOutcome> => {
      try {
        const res = await posService.scanBarcode(code);
        const action = scanAction(res, code);
        if (action.kind === "add") {
          const r = await addProductById(action.productId, action.variantId);
          if (r.ok) return { kind: "added", name: r.name, qty: r.qty };
          return r.reason === "locked"
            ? { kind: "blocked" }
            : { kind: "rejected", detail: "Barang tidak dapat ditambahkan — lihat pesan galat." };
        }
        if (action.kind === "choose") {
          setVariantChoice(action);
          return { kind: "choose", productName: action.productName };
        }
        return { kind: "rejected", detail: action.text };
      } catch (err) {
        const apiErr = readApiError(err);
        const detail = scanErrorMessage(apiErr.status, apiErr.message, code);
        return apiErr.status === 404
          ? { kind: "unknown", code, detail }
          : { kind: "rejected", detail };
      }
    },
    [addProductById],
  );

  /** Kirim barcode ketikan/wedge. Pemindai cepat bisa mengirim kode berikutnya
   * sebelum yang pertama selesai diproses: kode diantre dan diproses berurutan,
   * tidak pernah dibuang (barang yang terpindai selalu masuk keranjang). */
  async function submitBarcode() {
    const code = barcode.trim();
    if (code === "") return;
    setBarcode("");
    scanQueue.current.push(code);
    if (scanBusy.current) return;
    scanBusy.current = true;
    setScanning(true);
    setScanNotice(null);
    try {
      for (let next = scanQueue.current.shift(); next !== undefined; next = scanQueue.current.shift()) {
        const outcome = await scanCode(next);
        if (outcome.kind === "unknown" || outcome.kind === "rejected") setScanNotice(outcome.detail);
      }
    } finally {
      scanBusy.current = false;
      setScanning(false);
    }
  }

  /** Harga satuan efektif yang dijaga minimum: harga member dikecualikan
   * server, jadi klemnya 0 untuk baris ber-harga member. */
  function floorOf(l: CartLine): number {
    return memberPriceOf(l.productId) !== null ? 0 : l.minSellingPrice;
  }

  /** Setel qty (desimal > 0, 4 desimal — temuan 7); qty tidak valid diabaikan. Nominal diskon
   * dijepit ulang karena batasnya bergantung qty. */
  function setQty(key: number, requested: number) {
    if (started) return;
    const qty = normalizeQty(requested);
    if (qty === null) return;
    setCart((rows) =>
      rows.map((l) => {
        if (l.key !== key) return l;
        const c = clampLineDiscount(
          { qty, unitPrice: displayUnitPrice(l) },
          { discountPct: l.discountPct, discountNominal: l.discountNominal },
          floorOf(l),
        );
        return { ...l, qty, discountPct: c.discountPct, discountNominal: c.discountNominal };
      }),
    );
  }

  function changeQty(key: number, delta: number) {
    const line = cartLive.current.find((l) => l.key === key);
    if (line) setQty(key, line.qty + delta);
  }

  function removeLine(key: number) {
    if (started) return;
    setCart((rows) => rows.filter((l) => l.key !== key));
  }

  /** Kosongkan seluruh keranjang (tahanan stok dilepas oleh sinkronisasi). */
  function clearCart() {
    if (started) return;
    setCart(() => []);
  }

  /** Setel diskon baris (persen / nominal) dengan klem harga minimum. */
  function setLineDiscount(
    key: number,
    patch: { discountPct?: number; discountNominal?: number },
  ) {
    if (started) return;
    const line = cartLive.current.find((l) => l.key === key);
    if (!line) return;
    const c = clampLineDiscount(
      { qty: line.qty, unitPrice: displayUnitPrice(line) },
      {
        discountPct: patch.discountPct ?? line.discountPct,
        discountNominal: patch.discountNominal ?? line.discountNominal,
      },
      floorOf(line),
    );
    if (c.clamped) {
      toast.warning(
        "Diskon dibatasi harga minimum",
        `${line.productName}: potongan maksimum ${formatIDR(
          maxLineDiscount({ qty: line.qty, unitPrice: displayUnitPrice(line) }, floorOf(line)),
        )} agar tidak di bawah harga jual minimum.`,
      );
    }
    setCart((rows) =>
      rows.map((l) =>
        l.key === key
          ? { ...l, discountPct: c.discountPct, discountNominal: c.discountNominal }
          : l,
      ),
    );
  }

  /** Netto estimasi satu baris: harga tampil (member bila ada) − diskon. */
  function estimateOf(l: CartLine): number {
    return lineTotal({
      qty: l.qty,
      unitPrice: displayUnitPrice(l),
      discountPct: l.discountPct,
      discountNominal: l.discountNominal,
    });
  }

  /** Potongan maksimum baris ini (untuk teks bantuan di popover diskon). */
  function discountCeilingOf(l: CartLine): number {
    return maxLineDiscount({ qty: l.qty, unitPrice: displayUnitPrice(l) }, floorOf(l));
  }

  // Estimasi memakai harga tampil (member bila quote applied) — total final
  // tetap dari server.
  const totals = useMemo(() => {
    let subtotal = 0;
    let discount = 0;
    let itemCount = 0;
    const nets: { net: number; taxable: boolean }[] = [];
    for (const l of cart) {
      const e = {
        qty: l.qty,
        unitPrice: displayUnitPrice(l),
        discountPct: l.discountPct,
        discountNominal: l.discountNominal,
      };
      subtotal += lineGross(e);
      discount += lineDiscount(e);
      itemCount += l.qty;
      nets.push({ net: lineGross(e) - lineDiscount(e), taxable: true });
    }
    // Pajak seperti server (money.LineAmounts): exclude menambah, include memecah DPP/PPN.
    const t = estimateTax(nets, taxType, taxRate);
    return { subtotal, discount, base: t.base, tax: t.tax, total: t.total, itemCount };
  }, [cart, displayUnitPrice, taxType, taxRate]);
  const estimateTotal = totals.total;

  // Alamat kirim hanya berlaku bila milik pelanggan yang sedang dipilih.
  const effectiveShipTo =
    customerId !== null && customerDetail?.addresses.some((a) => a.id === shipToAddressId && a.status !== "archived")
      ? shipToAddressId
      : null;

  function openCheckout() {
    if (cart.length === 0 || !readiness.canCheckout) return;
    if (!started) {
      setMethod("cash");
      setMode("pay_now");
      setDueDate("");
      setTendered(String(estimateTotal));
    }
    setCheckoutError(null);
    setCompleted(null);
    setHideDiscount(false);
    setCheckoutOpen(true);
  }

  /** Adaptor API checkout (idempoten — clientRef dibawa di payload). */
  const checkoutApi: CheckoutApi = {
    createOrder: (body) => posService.createOrder(body),
    confirmOrder: (id) => posService.confirmOrder(id),
    createDelivery: (body) => posService.createDelivery(body),
    confirmDelivery: (id, settlement) => posService.confirmDelivery(id, settlement),
    createPayment: (body) => posService.createPayment(body),
    findPayment: (orderId) => findPosPayment(customerId ?? 0, orderId),
  };

  /** Pesan henti bila ada baris yang tak tertutup stok (cek SEBELUM order
   * dikonfirmasi — kalau gagal belakangan, SO terkonfirmasi tertinggal). */
  async function stockBlockMessage(): Promise<string | null> {
    const state = await holds.flush();
    for (const l of cart) {
      const st = state[lineKey(l.productId, l.variantId)];
      if (!st) continue;
      if (st.error) return `${l.productName}: ${st.error}`;
      if (st.shortfall > 0) {
        return `Stok ${l.productName} kurang ${formatQty(st.shortfall)} — kurangi qty atau hapus baris sebelum membayar.`;
      }
    }
    return null;
  }

  /** Susun ringkasan sukses dari order server + pembayaran tersimpan. */
  function buildCompleted(
    order: { id: number; number: string; grandTotal: number },
    opts: {
      payLater: boolean;
      dueDate: string | null;
      method: PayMethod;
      tenderedInput: number;
      paymentPending: boolean;
      payment: { id: number; amountTendered?: number; change?: number } | null;
    },
  ): CompletedSale {
    const { payLater, payment } = opts;
    const paid = payLater
      ? 0
      : payment && (payment.amountTendered ?? 0) > 0
        ? (payment.amountTendered as number)
        : opts.method === "cash"
          ? opts.tenderedInput
          : order.grandTotal;
    // Kembalian dibaca dari data tersimpan (server) bila ada.
    const change = payLater
      ? 0
      : payment && payment.change !== undefined && (payment.amountTendered ?? 0) > 0
        ? payment.change
        : cashChange(paid, order.grandTotal);
    return {
      orderId: order.id,
      orderNumber: order.number,
      grandTotal: order.grandTotal,
      tendered: paid,
      change,
      method: opts.method,
      payLater,
      dueDate: opts.dueDate,
      paymentId: payment?.id ?? null,
      paymentPending: opts.paymentPending,
    };
  }

  /** (1) order → (2) konfirmasi → (3) SJ draf → (4) konfirmasi SJ (+settlement
   * bayar sekarang, satu panggilan). Titik mulai SELALU dari state server
   * (nextStep): tekan lagi setelah gagal = lanjut dari langkah yang tertinggal,
   * tanpa dokumen ganda (clientRef idempoten). Bayar Nanti = termin net, tanpa
   * settlement. */
  async function handleCheckout() {
    const parsed = posCheckoutSchema.safeParse({ method, tendered, mode, dueDate, partyId: customerId ?? 0 });
    if (!parsed.success) {
      const issue = parsed.error.issues[0];
      setCheckoutError(
        issue?.path[0] === "method"
          ? "Metode pembayaran tidak valid."
          : issue?.path[0] === "partyId"
            ? PAY_LATER_NEEDS_CUSTOMER_MESSAGE
            : issue?.path[0] === "dueDate"
              ? "Jatuh tempo wajib diisi untuk Bayar Nanti."
              : "Nominal tunai tidak valid — isi angka rupiah ≥ 0.",
      );
      return;
    }
    // Order yang sudah dibuat (resume) tidak dinilai ulang — server sudah menerimanya.
    if (mode === "pay_later" && payLaterBlock !== null && !started) {
      setCheckoutError(payLaterBlock);
      return;
    }
    const values = parsed.data;
    const payLater = values.mode === "pay_later";
    if (!payLater && !canPay) {
      // K1: tanpa izin pembayaran, bayar sekarang tidak bisa diselesaikan server — tolak sebelum
      // keranjang dikunci atau dokumen apa pun dibuat.
      setCheckoutError(NO_PAY_PERMISSION_MESSAGE);
      return;
    }
    if (!payLater && values.method === "cash" && values.tendered < estimateTotal) {
      setCheckoutError(`Tunai kurang — minimal ${formatIDR(estimateTotal)} (estimasi).`);
      return;
    }
    setCheckoutError(null);
    setResumeStep(null);
    setProcessing(true);

    const partyId = customerId ?? 0;
    const items = cart.map((l) => ({
      productId: l.productId,
      variantId: l.variantId,
      uom: l.uom,
      qty: l.qty,
      ...(l.discountPct > 0 ? { discountPct: l.discountPct } : {}),
      ...(l.discountNominal > 0 ? { discountNominal: l.discountNominal } : {}),
    }));
    let orderPayload;
    try {
      orderPayload = buildPosOrderPayload({
        partyId,
        mode: values.mode,
        dueDate: values.dueDate,
        clientRef: cartRef,
        notes,
        shipToAddressId: effectiveShipTo,
        taxType,
        taxRate,
        items,
      });
    } catch (err) {
      setCheckoutError(err instanceof Error ? err.message : "Order tidak valid.");
      setProcessing(false);
      return;
    }

    let state: PosOrderState | null;
    try {
      state = await posService.orderState(cartRef);
    } catch (err) {
      setCheckoutError(
        `Status transaksi tidak dapat dibaca (${readApiError(err).message}). Coba lagi.`,
      );
      setProcessing(false);
      return;
    }

    // Order belum ada di server: periksa stok SEKARANG, sebelum mengunci
    // keranjang atau membuat dokumen apa pun (kasir masih bisa mengubah qty).
    if (state === null) {
      const blocked = await stockBlockMessage();
      if (blocked) {
        setCheckoutError(blocked);
        setProcessing(false);
        return;
      }
    }

    // Kunci isi keranjang SEBELUM panggilan tulis pertama (disimpan sinkron):
    // bila tab mati di tengah jalan, reload tak boleh mengubah isi order yang sama.
    setStarted(true);
    startedLive.current = true;
    saveDraft({
      ref: cartRef,
      lines: cart,
      customerId,
      started: true,
      mode,
      method,
      dueDate,
      tendered,
      notes,
      shipToAddressId,
      taxType,
      taxRate,
      held: heldSnap,
    });

    const outcome = await runCheckout(
      checkoutApi,
      {
        clientRef: cartRef,
        orderPayload,
        partyId,
        payNow: !payLater,
        method: values.method,
        tendered: values.tendered,
        canPay,
        deliveryItems: () =>
          cart.flatMap((l) =>
            deliveryItemsFor(l, holds.heldFor(l.productId, l.variantId), cartRef),
          ),
        beforeConfirm: stockBlockMessage,
      },
      state,
    );

    if (!outcome.ok) {
      if (outcome.blocked) {
        setCheckoutError(outcome.blocked);
        // Order sudah ada (draf) tetapi stok kurang: order terkunci; batalkan transaksi atau tambah stok.
        setProcessing(false);
        return;
      }
      // message server + pesan per-field (mis. "Validasi gagal — customer termin net
      // wajib memiliki nomor telepon"), bukan "Validasi gagal" saja.
      const reason = formatApiError(outcome.error);
      // Penolakan pasti (4xx) di langkah 1 = order tak pernah dibuat → buka kunci.
      if (outcome.step === "create_order" && !outcome.orderCreated && isDefiniteRejection(outcome.error)) {
        setStarted(false);
      }
      const where = CHECKOUT_STEP_LABEL[outcome.step];
      setCheckoutError(`Gagal ${where}: ${reason}`);
      toast.danger(
        `Gagal ${where}`,
        outcome.orderCreated || outcome.step !== "create_order"
          ? `${reason} Tekan "Lanjutkan" untuk meneruskan dari langkah ini — dokumen yang sudah dibuat tidak digandakan.`
          : `${reason} Keranjang tetap tersimpan.`,
      );
      setProcessing(false);
      return;
    }

    const sale = buildCompleted(outcome.order, {
      payLater,
      dueDate: payLater ? values.dueDate.trim() : null,
      method: values.method,
      tenderedInput: values.tendered,
      paymentPending: outcome.paymentPending,
      payment: outcome.payment,
    });
    holds.forget();
    setCompleted(sale);
    if (payLater) {
      toast.success(
        "Piutang POS tercatat",
        `Order ${sale.orderNumber} termin net jatuh tempo ${values.dueDate.trim()}.`,
      );
    } else {
      toast.success(
        sale.paymentPending ? "Order POS terkonfirmasi" : "Transaksi POS berhasil",
        sale.paymentPending
          ? `Order ${sale.orderNumber} terkonfirmasi. Pembayaran harus dicatat manual di menu Pembayaran.`
          : `Order ${sale.orderNumber} lunas ${formatIDR(sale.grandTotal)}.`,
      );
    }
    setProcessing(false);
  }

  /** Reload saat checkout tertunda: baca state server lalu lanjutkan dari
   * nextStep (kasir menekan Lanjutkan — tak ada uang bergerak otomatis). */
  useEffect(() => {
    if (!initialDraft.started) return;
    let alive = true;
    (async () => {
      try {
        const st = await posService.orderState(initialDraft.ref);
        if (!alive) return;
        if (st === null) {
          setStarted(false); // order tak pernah sampai server
          return;
        }
        if (st.nextStep === "cancelled") {
          toast.warning("Order dibatalkan", "Keranjang sebelumnya sudah dibatalkan di server.");
          clearDraft();
          setCart(() => []);
          setStarted(false);
          setCartRef(newClientRef());
          return;
        }
        if (st.nextStep === "done") {
          const payLater = st.order.paymentTerms === "net";
          let payment = null;
          try {
            payment = payLater ? null : await findPosPayment(initialDraft.customerId ?? 0, st.order.id);
          } catch {
            payment = null;
          }
          if (!alive) return;
          setCompleted(
            buildCompleted(st.order, {
              payLater,
              dueDate: null,
              method: initialDraft.method,
              tenderedInput: Number(initialDraft.tendered) || 0,
              paymentPending: !payLater && payment === null && !canPay,
              payment,
            }),
          );
          setCheckoutOpen(true);
          return;
        }
        setResumeStep(CHECKOUT_STEP_LABEL[st.nextStep as keyof typeof CHECKOUT_STEP_LABEL] ?? st.nextStep);
        setCheckoutOpen(true);
      } catch {
        /* status tak terbaca: kasir tetap bisa menekan Lanjutkan */
      }
    })();
    return () => {
      alive = false;
    };
    // Sekali saat mount — draf awal tak berubah.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /** Mulai transaksi baru: ref baru, draf dibuang. */
  function resetTransaction() {
    clearDraft();
    setCart(() => []);
    setStarted(false);
    setHeldSnap({});
    setCartRef(newClientRef());
    setCompleted(null);
    setCheckoutOpen(false);
    setTendered("");
    setNotes("");
    setShipToAddressId(null);
    setTaxTypeState("none");
    setTaxRate(0);
    setMode("pay_now");
    setDueDate("");
    setHideDiscount(false);
    setCheckoutError(null);
    setResumeStep(null);
    setDiscountNotice(null);
  }

  function newTransaction() {
    holds.forget(); // tahanan sudah dikonsumsi SJ
    resetTransaction();
  }

  /** Batalkan transaksi yang sudah dimulai: lepas tahanan, buang draf.
   * Order di server (draf/terkonfirmasi) tidak dibatalkan otomatis. */
  async function abandonTransaction() {
    setCheckoutOpen(false);
    await holds.releaseAll();
    resetTransaction();
    toast.warning(
      "Transaksi dibatalkan di kasir",
      "Order yang sempat dibuat tetap ada di server — tangani dari menu Order Jual.",
    );
  }

  const changePreview =
    mode === "pay_now" &&
    method === "cash" &&
    tendered.trim() !== "" &&
    Number.isFinite(Number(tendered))
      ? cashChange(Number(tendered), estimateTotal)
      : 0;

  return {
    // izin
    canPay,
    canDiscount,
    readiness,
    // keranjang
    cart,
    started,
    addingId,
    addProductById,
    setQty,
    changeQty,
    removeLine,
    clearCart,
    setLineDiscount,
    estimateOf,
    discountCeilingOf,
    totals,
    estimateTotal,
    holds,
    /** Pemberitahuan diskon yang diturunkan otomatis (harga efektif berubah). */
    discountNotice,
    dismissDiscountNotice: () => setDiscountNotice(null),
    // pindai
    barcode,
    setBarcode,
    scanning,
    scanNotice,
    setScanNotice,
    submitBarcode,
    scanCode,
    variantChoice,
    setVariantChoice,
    // pelanggan + harga member
    customerId,
    setCustomerId: selectCustomer,
    customerDetail,
    memberName,
    quote,
    quoteStatus,
    quoteError,
    memberSavings,
    memberPriceOf,
    displayUnitPrice,
    quoteLineErrors,
    // checkout
    checkoutOpen,
    setCheckoutOpen,
    method,
    setMethod,
    mode,
    setMode,
    /** Alasan Bayar Nanti tidak tersedia (null = tersedia). */
    payLaterBlock,
    dueDate,
    setDueDate,
    tendered,
    setTendered,
    notes,
    setNotes,
    taxType,
    taxRate,
    /** Kena pajak? (false = tanpa pajak; true = exclude dengan tarif bawaan bila belum ada). */
    setTaxed: (on: boolean) => {
      setTaxTypeState(on ? "exclude" : "none");
      setTaxRate((r) => (on ? (r > 0 ? r : DEFAULT_VAT_RATE) : 0));
    },
    setTaxIncluded: (included: boolean) => setTaxTypeState(included ? "include" : "exclude"),
    setTaxRate,
    shipToAddressId: effectiveShipTo,
    setShipToAddressId,
    checkoutError,
    processing,
    completed,
    hideDiscount,
    setHideDiscount,
    resumeStep,
    changePreview,
    openCheckout,
    handleCheckout,
    newTransaction,
    abandonTransaction,
  };
}

export type PosCartApi = ReturnType<typeof usePosCart>;
