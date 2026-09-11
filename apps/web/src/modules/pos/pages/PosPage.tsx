import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  DateInput,
  FormField,
  Modal,
  PageHeader,
  SearchSelect,
  SectionCard,
  Segmented,
  SelectInput,
  TextInput,
  buttonClass,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { posCheckoutSchema } from "@/modules/pos/schemas/pos-checkout.schema";
import { posService } from "@/modules/pos/services/pos.service";
import { cashChange, lineTotal } from "@/modules/pos/lib/cart";
import { buildPosOrderPayload } from "@/modules/pos/lib/checkout";
import { PosCategoryDrilldown } from "@/modules/pos/components/PosCategoryDrilldown";
import { PosCustomerPanel } from "@/modules/pos/components/PosCustomerPanel";
import { PosScannerModal } from "@/modules/pos/components/PosScannerModal";
import type {
  CartLine,
  CheckoutMode,
  CompletedSale,
  PayMethod,
  PosCategory,
  PosCreatedOrder,
  PosCustomerDetail,
  PosLocation,
  PosMemberType,
  PosProductDetail,
  PricingQuote,
} from "@/modules/pos/types";
import { payMethodLabel } from "@/modules/pos/types";

let nextKey = 1;

/** Satuan jual: salesUom dulu, lalu sisanya tanpa duplikat/kosong. */
function saleUoms(p: PosProductDetail): string[] {
  return [p.salesUom, p.baseUom, p.purchaseUom].filter(
    (u, i, arr) => (u ?? "").trim() !== "" && arr.indexOf(u) === i,
  );
}

export default function PosPage() {
  const can = useAuthStore((s) => s.can);
  // Gate halaman (sales.create) sudah di route-level; langkah pembayaran
  // butuh payment.create — tanpanya order tetap bisa dibuat.
  const canPay = can("payment.create");

  // --- panel produk ---
  const [productQuery, setProductQuery] = useState("");
  const [productOptions, setProductOptions] = useState<
    { id: number; code: string; name: string; sellingPrice: number }[]
  >([]);
  const [searching, setSearching] = useState(false);
  const [addingId, setAddingId] = useState<number | null>(null);
  const [barcode, setBarcode] = useState("");
  const [scanning, setScanning] = useState(false);
  const [scanNotice, setScanNotice] = useState<string | null>(null);
  const [scannerOpen, setScannerOpen] = useState(false);

  // --- katalog per kategori (drilldown; null = semua) ---
  const [categories, setCategories] = useState<PosCategory[]>([]);
  const [activeCategoryId, setActiveCategoryId] = useState<number | null>(null);
  useEffect(() => {
    posService
      .categories()
      .then((list) => setCategories(list ?? []))
      .catch(() => setCategories([]));
  }, []);

  // --- keranjang ---
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [customerQuery, setCustomerQuery] = useState("");
  const [customerOptions, setCustomerOptions] = useState<SearchSelectOption[]>([]);
  const [cart, setCart] = useState<CartLine[]>([]);

  // --- lokasi asal barang (satu lokasi untuk seluruh struk POS) ---
  const [locations, setLocations] = useState<PosLocation[]>([]);
  const [locationId, setLocationId] = useState<number | null>(null);
  useEffect(() => {
    posService
      .locations()
      .then((list) => {
        const active = (list ?? []).filter((l) => l.status === "active");
        setLocations(active);
        setLocationId(
          active.find((l) => l.code === "GDG")?.id ?? active[0]?.id ?? null,
        );
      })
      .catch(() => setLocations([]));
  }, []);

  // --- checkout ---
  const [checkoutOpen, setCheckoutOpen] = useState(false);
  const [method, setMethod] = useState<PayMethod>("cash");
  const [mode, setMode] = useState<CheckoutMode>("pay_now");
  const [dueDate, setDueDate] = useState("");
  const [tendered, setTendered] = useState("");
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
    posService
      .memberTypes()
      .then((list) => setMemberTypes(list ?? []))
      .catch(() => setMemberTypes([]));
  }, []);
  if (customerId === null && customerDetail !== null) {
    setCustomerDetail(null);
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

  // --- quote harga member untuk isi keranjang (debounce; gagal = katalog) ---
  const cartProductIds = cart
    .map((l) => l.productId)
    .filter((id, i, arr) => arr.indexOf(id) === i)
    .sort((a, b) => a - b);
  const cartProductKey = cartProductIds.join(",");
  if (
    (customerId === null || cartProductIds.length === 0) &&
    (quote !== null || quoteStatus !== "idle" || quoteError !== null)
  ) {
    setQuote(null);
    setQuoteStatus("idle");
    setQuoteError(null);
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
        })
        .catch((err) => {
          setQuote(null);
          setQuoteStatus("error");
          setQuoteError(
            `Harga member tidak dapat dimuat (${toApiError(err).message}) — memakai harga katalog.`,
          );
        });
    }, 400);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [customerId, cartProductKey]);

  /** Harga tampil per baris: member bila quote applied, sonst katalog. */
  const memberPriceOf = (productId: number): number | null => {
    const line = quote?.lines.find((l) => l.productId === productId);
    return line && line.applied ? line.memberPrice : null;
  };
  const displayUnitPrice = (l: CartLine): number =>
    memberPriceOf(l.productId) ?? l.unitPrice;
  const memberSavings = cart.reduce(
    (sum, l) => sum + Math.max(0, l.unitPrice - displayUnitPrice(l)) * l.qty,
    0,
  );
  const memberName =
    memberTypes.find((m) => m.code === (quote?.memberType ?? ""))?.name ??
    (quote?.memberType ? quote.memberType : null);

  // --- pencarian produk (debounce; kategori aktif memfilter katalog) ---
  useEffect(() => {
    const t = window.setTimeout(() => {
      setSearching(true);
      const req =
        activeCategoryId === null
          ? posService.searchProducts(productQuery)
          : posService
              .products({
                search: productQuery,
                categoryId: activeCategoryId,
                limit: 20,
              })
              .then((res) => res.items);
      req
        .then((list) => setProductOptions(list))
        .catch((err) => {
          toast.danger("Gagal mencari produk", toApiError(err).message);
          setProductOptions([]);
        })
        .finally(() => setSearching(false));
    }, 350);
    return () => window.clearTimeout(t);
  }, [productQuery, activeCategoryId]);

  // --- pencarian customer aktif (debounce, walk-in = kosong) ---
  useEffect(() => {
    const t = window.setTimeout(() => {
      posService
        .searchCustomers(customerQuery)
        .then((res) =>
          setCustomerOptions((prev) => {
            const fetched: SearchSelectOption[] = res.items.map((c) => ({
              value: c.id,
              label: `${c.code} — ${c.name}`,
            }));
            const keep = prev.filter(
              (o) => o.value === customerId && !fetched.some((f) => f.value === o.value),
            );
            return [...keep, ...fetched];
          }),
        )
        .catch(() => setCustomerOptions([]));
    }, 350);
    return () => window.clearTimeout(t);
  }, [customerQuery, customerId]);

  /** Tambah produk ke keranjang (varian pindaian atau varian default). */
  async function addProductById(productId: number, variantId?: number) {
    setAddingId(productId);
    try {
      const d = await posService.productDetail(productId);
      if (d.variants.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
        return;
      }
      const variant =
        (variantId !== undefined
          ? d.variants.find((v) => v.id === variantId)
          : undefined) ??
        d.variants.find((v) => v.isDefault) ??
        d.variants[0];
      if (variantId !== undefined && !d.variants.some((v) => v.id === variantId)) {
        toast.danger("Gagal menambah baris", "Varian hasil pindai tidak ada di produk ini.");
        return;
      }
      const uoms = saleUoms(d);
      if (uoms.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya satuan jual.");
        return;
      }
      const uom = uoms[0];
      const found = cart.find(
        (l) => l.productId === d.id && l.variantId === variant.id && l.uom === uom,
      );
      if (found) {
        setCart((rows) =>
          rows.map((l) => (l.key === found.key ? { ...l, qty: l.qty + 1 } : l)),
        );
      } else {
        setCart((rows) => [
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
          },
        ]);
      }
    } catch (err) {
      toast.danger("Gagal memuat produk", toApiError(err).message);
    } finally {
      setAddingId(null);
    }
  }

  /** Barcode → pindai lalu tambah variannya, atau Notice bila asing.
   * Dipakai form ketik/wedge maupun hasil pindaian kamera. */
  async function scanCode(code: string) {
    if (code === "" || scanning) return;
    setScanning(true);
    setScanNotice(null);
    try {
      const res = await posService.scanBarcode(code);
      await addProductById(res.product.id, res.variant.id);
      setBarcode("");
    } catch (err) {
      const apiErr = toApiError(err);
      setScanNotice(
        apiErr.status === 404
          ? `Barcode "${code}" tidak dikenal di katalog.`
          : apiErr.message,
      );
    } finally {
      setScanning(false);
    }
  }

  /** Barcode + Enter → pindai lewat scanCode. */
  async function handleScan(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    await scanCode(barcode.trim());
  }

  function changeQty(key: number, delta: number) {
    setCart((rows) =>
      rows.map((l) =>
        l.key === key ? { ...l, qty: Math.max(1, l.qty + delta) } : l,
      ),
    );
  }

  function removeLine(key: number) {
    setCart((rows) => rows.filter((l) => l.key !== key));
  }

  // Estimasi memakai harga tampil (member bila quote applied) — total final
  // tetap dari server.
  const estimateTotal = cart.reduce(
    (sum, l) => sum + lineTotal({ qty: l.qty, unitPrice: displayUnitPrice(l) }),
    0,
  );

  function openCheckout() {
    if (cart.length === 0) return;
    setMethod("cash");
    setMode("pay_now");
    setDueDate("");
    setTendered(String(estimateTotal));
    setCheckoutError(null);
    setCompleted(null);
    setHideDiscount(false);
    setCheckoutOpen(true);
  }

  /** 1) buat order → 2) konfirmasi → 3) SJ + konfirmasi (stok keluar) →
   * 4) catat pembayaran lunas (bayar sekarang + boleh) — Bayar Nanti
   * berhenti di langkah 3: order termin net tanpa langkah pembayaran.
   * Gagal di langkah mana pun: toast danger + diam (keranjang utuh). */
  async function handleCheckout() {
    const parsed = posCheckoutSchema.safeParse({ method, tendered, mode, dueDate });
    if (!parsed.success) {
      const issue = parsed.error.issues[0];
      setCheckoutError(
        issue?.path[0] === "method"
          ? "Metode pembayaran tidak valid."
          : issue?.path[0] === "dueDate"
            ? "Jatuh tempo wajib diisi untuk Bayar Nanti."
            : "Nominal tunai tidak valid — isi angka rupiah ≥ 0.",
      );
      return;
    }
    const values = parsed.data;
    const payLater = values.mode === "pay_later";
    if (!payLater && values.method === "cash" && values.tendered < estimateTotal) {
      setCheckoutError(`Tunai kurang — minimal ${formatIDR(estimateTotal)} (estimasi).`);
      return;
    }
    setCheckoutError(null);
    setProcessing(true);

    const partyId = customerId ?? 0;
    const items = cart.map((l) => ({
      productId: l.productId,
      variantId: l.variantId,
      uom: l.uom,
      qty: l.qty,
    }));

    // Langkah 1 — order channel pos (server yang memberi harga; Bayar Nanti =
    // termin net + jatuh tempo via pembangun murni yang sudah diuji).
    let orderPayload;
    try {
      orderPayload = buildPosOrderPayload({
        partyId,
        mode: values.mode,
        dueDate: values.dueDate,
        items,
      });
    } catch (err) {
      setCheckoutError(err instanceof Error ? err.message : "Order tidak valid.");
      setProcessing(false);
      return;
    }
    let order;
    try {
      order = await posService.createOrder(orderPayload);
    } catch (err) {
      toast.danger(
        "Gagal membuat order jual",
        `${toApiError(err).message} Keranjang tetap tersimpan.`,
      );
      setProcessing(false);
      return;
    }

    // Langkah 2 — konfirmasi (tanpa body).
    let confirmed: PosCreatedOrder;
    try {
      confirmed = await posService.confirmOrder(order.id);
    } catch (err) {
      toast.danger(
        "Order dibuat tetapi gagal dikonfirmasi",
        `${toApiError(err).message} Order ${order.number} masih draf — lanjutkan dari menu Order Jual. Keranjang tetap tersimpan.`,
      );
      setProcessing(false);
      return;
    }
    const grandTotal = confirmed.grandTotal;

    // Langkah 3 — surat jalan + konfirmasi: stok keluar di sini. Gagal
    // (mis. stok kurang) → berhenti: order sudah terkonfirmasi, kirim susulan
    // dari menu Pengiriman.
    if (locationId === null) {
      setCheckoutError("Pilih lokasi asal barang terlebih dahulu.");
      setProcessing(false);
      return;
    }
    try {
      const note = await posService.createDelivery({
        salesOrderId: confirmed.id,
        items: cart.map((l) => ({
          productId: l.productId,
          variantId: l.variantId,
          locationId,
          uom: l.uom,
          qty: l.qty,
        })),
      });
      await posService.confirmDelivery(note.id);
    } catch (err) {
      toast.danger(
        "Order terkonfirmasi tetapi stok gagal keluar",
        `${toApiError(err).message} Order ${confirmed.number} sudah terkonfirmasi — buat surat jalan susulan dari menu Pengiriman. Keranjang tetap tersimpan.`,
      );
      setProcessing(false);
      return;
    }

    // Langkah 4 — pembayaran lunas 1 order sebesar grandTotal SERVER.
    // Dilewati untuk Bayar Nanti (piutang): tidak ada pembayaran dicatat.
    let paymentId: number | null = null;
    const paymentPending = payLater ? false : !canPay;
    if (!payLater && canPay) {
      try {
        const payment = await posService.createPayment({
          partyId,
          method: values.method,
          amount: grandTotal,
          allocations: [{ orderType: "sales", orderId: confirmed.id, amount: grandTotal }],
        });
        paymentId = payment.id;
      } catch (err) {
        toast.danger(
          "Order terkonfirmasi tetapi pembayaran gagal dicatat",
          `${toApiError(err).message} Order ${confirmed.number} sudah terkonfirmasi — catat pembayaran manual di menu Pembayaran. Keranjang tetap tersimpan.`,
        );
        const paid = values.method === "cash" ? values.tendered : grandTotal;
        setCompleted({
          orderId: confirmed.id,
          orderNumber: confirmed.number,
          grandTotal,
          tendered: paid,
          change: cashChange(paid, grandTotal),
          method: values.method,
          payLater: false,
          dueDate: null,
          paymentId: null,
          paymentPending: true,
        });
        setProcessing(false);
        return;
      }
    }

    const paid = payLater ? 0 : values.method === "cash" ? values.tendered : grandTotal;
    setCompleted({
      orderId: confirmed.id,
      orderNumber: confirmed.number,
      grandTotal,
      tendered: paid,
      change: payLater ? 0 : cashChange(paid, grandTotal),
      method: values.method,
      payLater,
      dueDate: payLater ? values.dueDate.trim() : null,
      paymentId,
      paymentPending,
    });
    if (payLater) {
      toast.success(
        "Piutang POS tercatat",
        `Order ${confirmed.number} termin net jatuh tempo ${values.dueDate.trim()}.`,
      );
    } else {
      toast.success(
        paymentPending ? "Order POS terkonfirmasi" : "Transaksi POS berhasil",
        paymentPending
          ? `Order ${confirmed.number} terkonfirmasi. Pembayaran harus dicatat manual di menu Pembayaran.`
          : `Order ${confirmed.number} lunas ${formatIDR(grandTotal)}.`,
      );
    }
    setProcessing(false);
  }

  function newTransaction() {
    setCart([]);
    setCompleted(null);
    setCheckoutOpen(false);
    setTendered("");
    setMode("pay_now");
    setDueDate("");
    setHideDiscount(false);
    setCheckoutError(null);
  }

  const changePreview =
    mode === "pay_now" &&
    method === "cash" &&
    tendered.trim() !== "" &&
    Number.isFinite(Number(tendered))
      ? cashChange(Number(tendered), estimateTotal)
      : 0;

  return (
    <div>
      <PageHeader
        eyebrow="Kasir"
        title="POS"
        description="Kasir cepat: cari atau pindai produk, isi keranjang, lalu bayar tunai lunas atau catat sebagai piutang (Bayar Nanti). Harga final dihitung ulang server."
      />

      {!canPay && (
        <div className="mb-4">
          <Notice tone="warning" title="Tanpa izin pencatatan pembayaran">
            Akun ini tidak memiliki izin pembayaran — order tetap bisa dibuat dan
            dikonfirmasi, tetapi pembayaran harus dicatat manual di{" "}
            <Link
              to={ROUTE_PATHS.payments}
              className="font-semibold text-brand hover:underline"
            >
              menu Pembayaran
            </Link>
            .
          </Notice>
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* --- kiri: produk --- */}
        <SectionCard
          title="Produk"
          description="Cari produk atau pindai barcode untuk menambah ke keranjang."
        >
          <div className="space-y-4">
            <form onSubmit={handleScan}>
              <FormField
                label="Barcode"
                helperText="Ketik, tempel dari wedge, atau pindai via kamera."
                htmlFor="pos-barcode"
              >
                <div className="flex gap-2">
                  <TextInput
                    id="pos-barcode"
                    value={barcode}
                    onChange={(e) => setBarcode(e.target.value)}
                    placeholder="Pindai / ketik barcode lalu Enter…"
                    autoComplete="off"
                  />
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setScannerOpen(true)}
                    aria-label="Scan dengan kamera"
                  >
                    Kamera
                  </Button>
                </div>
              </FormField>
            </form>
            {scanNotice && <Notice tone="warning" title={scanNotice} />}

            <PosCategoryDrilldown
              categories={categories}
              value={activeCategoryId}
              onChange={setActiveCategoryId}
            />

            <FormField label="Cari Produk" htmlFor="pos-search">
              <TextInput
                id="pos-search"
                value={productQuery}
                onChange={(e) => setProductQuery(e.target.value)}
                placeholder="Kode / nama produk…"
                autoComplete="off"
              />
            </FormField>

            {searching ? (
              <p className="text-muted py-4 text-center text-sm">Mencari…</p>
            ) : productOptions.length === 0 ? (
              <EmptyState
                title="Tidak ada hasil"
                description="Coba kata kunci lain atau pindai barcode."
              />
            ) : (
              <ul className="max-h-[420px] divide-y divide-hairline overflow-auto rounded-md border border-hairline">
                {productOptions.map((p) => (
                  <li
                    key={p.id}
                    className="flex items-center justify-between gap-3 px-4 py-2.5"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-ink">
                        {p.code} — {p.name}
                      </p>
                      <p className="text-muted text-xs">{formatIDR(p.sellingPrice)}</p>
                    </div>
                    <Button
                      size="sm"
                      disabled={addingId === p.id}
                      onClick={() => addProductById(p.id)}
                      aria-label={`Tambah ${p.name}`}
                    >
                      {addingId === p.id ? "…" : "+"}
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </SectionCard>

        {/* --- kanan: keranjang --- */}
        <SectionCard
          title="Keranjang"
          description="Estimasi memakai harga katalog — total final dari server."
        >
          <div className="space-y-4">
            <div>
              <FormField
                label="Customer"
                helperText="Kosongkan untuk walk-in (tanpa customer)."
              >
                <TextInput
                  value={customerQuery}
                  onChange={(e) => setCustomerQuery(e.target.value)}
                  placeholder="Ketik untuk mencari customer…"
                  className="mb-2"
                  autoComplete="off"
                />
              </FormField>
              <SearchSelect
                options={customerOptions}
                value={customerId}
                onChange={(v) => setCustomerId(typeof v === "number" ? v : null)}
                placeholder="Walk-in (tanpa customer)"
                allowClear
              />
              <div className="mt-2">
                <PosCustomerPanel
                  customer={customerDetail}
                  memberName={memberName}
                  quote={quote}
                  quoteStatus={quoteStatus}
                  quoteError={quoteError}
                  savings={memberSavings}
                />
              </div>
            </div>

            <div>
              <FormField
                label="Lokasi Asal"
                helperText="Stok keluar dari lokasi ini saat bayar."
                htmlFor="pos-location"
              >
                <SelectInput
                  id="pos-location"
                  value={locationId ?? ""}
                  onChange={(e) =>
                    setLocationId(e.target.value === "" ? null : Number(e.target.value))
                  }
                >
                  <option value="">— Pilih lokasi —</option>
                  {locations.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.code} — {l.name}
                    </option>
                  ))}
                </SelectInput>
              </FormField>
            </div>

            {cart.length === 0 ? (
              <EmptyState
                title="Keranjang kosong"
                description="Tambahkan produk dari panel kiri."
              />
            ) : (
              <ul className="divide-y divide-hairline rounded-md border border-hairline">
                {cart.map((l) => (
                  <li key={l.key} className="px-4 py-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium text-ink">
                          {l.productName}
                        </p>
                        <p className="text-muted text-xs">
                          {l.variantCode} — {l.variantName} · {l.uom} ·{" "}
                          {formatIDR(l.unitPrice)}
                        </p>
                        {memberPriceOf(l.productId) !== null && (
                          <p className="text-xs font-semibold text-ok">
                            Harga member: {formatIDR(displayUnitPrice(l))}
                          </p>
                        )}
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => removeLine(l.key)}
                        aria-label={`Hapus ${l.productName}`}
                      >
                        Hapus
                      </Button>
                    </div>
                    <div className="mt-2 flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => changeQty(l.key, -1)}
                          disabled={l.qty <= 1}
                          aria-label="Kurangi qty"
                        >
                          −
                        </Button>
                        <span className="min-w-10 text-center text-sm font-semibold text-ink">
                          {formatNumber(l.qty)}
                        </span>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => changeQty(l.key, 1)}
                          aria-label="Tambah qty"
                        >
                          +
                        </Button>
                      </div>
                      <p className="text-sm font-semibold text-ink">
                        {formatIDR(lineTotal({ qty: l.qty, unitPrice: displayUnitPrice(l) }))}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}

            <div className="flex items-center justify-between border-t border-hairline pt-3">
              <p className="text-muted text-sm">
                Estimasi total ({cart.reduce((n, l) => n + l.qty, 0)} item)
              </p>
              <p className="text-lg font-bold text-ink">{formatIDR(estimateTotal)}</p>
            </div>

            <Button
              className="w-full"
              disabled={cart.length === 0}
              onClick={openCheckout}
            >
              Bayar · {formatIDR(estimateTotal)}
            </Button>
          </div>
        </SectionCard>
      </div>

      {/* --- checkout --- */}
      <Modal
        open={checkoutOpen}
        title={completed ? "Transaksi Berhasil" : mode === "pay_later" ? "Bayar Nanti" : "Pembayaran"}
        description={
          completed
            ? `Order ${completed.orderNumber} selesai diproses.`
            : mode === "pay_later"
              ? `Piutang termin net ${formatIDR(estimateTotal)} (estimasi) — tanpa pembayaran sekarang.`
              : `Estimasi ${formatIDR(estimateTotal)} — total final dihitung server.`
        }
        size="sm"
        onClose={() => {
          if (!processing) setCheckoutOpen(false);
        }}
        actions={
          completed ? undefined : (
            <>
              <Button
                variant="secondary"
                disabled={processing}
                onClick={() => setCheckoutOpen(false)}
              >
                Batal
              </Button>
              <Button disabled={processing} onClick={handleCheckout}>
                {processing
                  ? "Memproses…"
                  : mode === "pay_later"
                    ? "Buat Piutang"
                    : "Proses Bayar"}
              </Button>
            </>
          )
        }
      >
        {completed ? (
          <div className="space-y-4">
            <dl className="space-y-2 rounded-md border border-hairline p-4 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted">No. Order</dt>
                <dd className="font-semibold text-ink">{completed.orderNumber}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted">Metode</dt>
                <dd className="font-semibold text-ink">
                  {completed.payLater
                    ? payMethodLabel("pay_later")
                    : payMethodLabel(completed.method)}
                </dd>
              </div>
              {completed.payLater && completed.dueDate && (
                <>
                  <div className="flex justify-between">
                    <dt className="text-muted">Termin</dt>
                    <dd className="font-semibold text-ink">Net</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-muted">Jatuh Tempo</dt>
                    <dd className="font-semibold text-ink">{completed.dueDate}</dd>
                  </div>
                </>
              )}
              <div className="flex justify-between">
                <dt className="text-muted">Total (server)</dt>
                <dd className="font-semibold text-ink">
                  {formatIDR(completed.grandTotal)}
                </dd>
              </div>
              {!completed.payLater && (
                <>
                  <div className="flex justify-between">
                    <dt className="text-muted">Diterima</dt>
                    <dd className="font-semibold text-ink">
                      {formatIDR(completed.tendered)}
                    </dd>
                  </div>
                  <div className="flex justify-between border-t border-hairline pt-2">
                    <dt className="text-muted">Kembalian</dt>
                    <dd className="text-base font-bold text-ink">
                      {formatIDR(completed.change)}
                    </dd>
                  </div>
                </>
              )}
            </dl>
            {completed.payLater && (
              <Notice tone="info" title="Piutang tercatat">
                Tagih order {completed.orderNumber} sebelum {completed.dueDate} — tanpa
                pembayaran yang dicatat sekarang.
              </Notice>
            )}
            <div className="rounded-md border border-hairline p-4">
              <div className="flex items-center justify-between gap-3">
                <p className="text-sm font-bold text-ink">Struk</p>
                <label className="text-muted flex cursor-pointer items-center gap-2 text-xs">
                  <input
                    type="checkbox"
                    checked={hideDiscount}
                    onChange={(e) => setHideDiscount(e.target.checked)}
                  />
                  Sembunyikan diskon (harga penuh)
                </label>
              </div>
              <ul className="mt-2 divide-y divide-hairline text-sm">
                {cart.map((l) => {
                  const unit = hideDiscount ? l.unitPrice : displayUnitPrice(l);
                  return (
                    <li key={l.key} className="flex justify-between gap-3 py-1.5">
                      <span className="text-muted min-w-0 truncate">
                        {l.productName} × {l.qty}
                      </span>
                      <span className="font-semibold text-ink">
                        {formatIDR(lineTotal({ qty: l.qty, unitPrice: unit }))}
                      </span>
                    </li>
                  );
                })}
              </ul>
              {!hideDiscount && memberSavings > 0 && (
                <p className="mt-2 text-right text-xs font-semibold text-ok">
                  Termasuk hemat member {formatIDR(memberSavings)}
                </p>
              )}
            </div>
            {completed.change < 0 && !completed.payLater && (
              <Notice tone="warning" title="Total server melebihi tunai">
                Selisih {formatIDR(-completed.change)} harus ditagih manual ke
                pembeli.
              </Notice>
            )}
            {completed.paymentPending && (
              <Notice tone="warning" title="Pembayaran belum tercatat">
                Catat pembayaran lunas order {completed.orderNumber} manual di{" "}
                <Link
                  to={ROUTE_PATHS.payments}
                  className="font-semibold text-brand hover:underline"
                >
                  menu Pembayaran
                </Link>
                .
              </Notice>
            )}
            <div className="flex justify-end gap-3">
              <a
                href={
                  completed.paymentId != null
                    ? `/print/pos/${completed.orderId}?paymentId=${completed.paymentId}`
                    : `/print/pos/${completed.orderId}`
                }
                target="_blank"
                rel="noreferrer"
                className={buttonClass("secondary", "md")}
              >
                Cetak Struk
              </a>
              <Link
                to={`${ROUTE_PATHS.salesOrders}/${completed.orderId}`}
                className={buttonClass("secondary", "md")}
              >
                Lihat Order
              </Link>
              <Button onClick={newTransaction}>Transaksi Baru</Button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <Segmented
              options={[
                { value: "pay_now", label: "Bayar Sekarang" },
                { value: "pay_later", label: "Bayar Nanti" },
              ]}
              value={mode}
              onChange={(v) => setMode(v as CheckoutMode)}
            />
            {mode === "pay_later" ? (
              <>
                <FormField
                  label="Jatuh Tempo"
                  required
                  htmlFor="pos-due-date"
                  helperText="Order dicatat sebagai piutang termin net."
                >
                  <DateInput
                    id="pos-due-date"
                    value={dueDate}
                    min={new Date().toISOString().split("T")[0]}
                    onChange={(e) => setDueDate(e.target.value)}
                    disabled={processing}
                  />
                </FormField>
                {customerId === null && (
                  <Notice
                    tone="warning"
                    title="Piutang tanpa customer — penagihan harus dilakukan manual"
                  />
                )}
              </>
            ) : (
              <>
                <FormField label="Metode" required htmlFor="pos-method">
                  <SelectInput
                    id="pos-method"
                    value={method}
                    onChange={(e) => {
                      const m = e.target.value as PayMethod;
                      setMethod(m);
                      if (m === "transfer") setTendered(String(estimateTotal));
                    }}
                    disabled={processing}
                  >
                    <option value="cash">Tunai</option>
                    <option value="transfer">Transfer</option>
                  </SelectInput>
                </FormField>
                {method === "cash" ? (
                  <FormField label="Tunai Diterima" required htmlFor="pos-tendered">
                    <TextInput
                      id="pos-tendered"
                      type="number"
                      min={0}
                      step={1}
                      inputMode="numeric"
                      value={tendered}
                      onChange={(e) => setTendered(e.target.value)}
                      placeholder="Nominal tunai…"
                      disabled={processing}
                    />
                  </FormField>
                ) : (
                  <Notice tone="info" title="Transfer pas">
                    Nominal transfer mengikuti total server ({formatIDR(estimateTotal)}{" "}
                    estimasi) — tanpa kembalian.
                  </Notice>
                )}
                {method === "cash" && (
                  <div className="flex justify-between rounded-md border border-hairline p-3 text-sm">
                    <span className="text-muted">Kembalian (estimasi)</span>
                    <span className="font-bold text-ink">{formatIDR(changePreview)}</span>
                  </div>
                )}
              </>
            )}
            {checkoutError && <Notice tone="danger" title={checkoutError} />}
          </div>
        )}
      </Modal>

      <PosScannerModal
        open={scannerOpen}
        onClose={() => setScannerOpen(false)}
        onScan={(code) => {
          setScannerOpen(false);
          void scanCode(code);
        }}
      />
    </div>
  );
}
