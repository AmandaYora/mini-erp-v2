import { useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import {
  AsyncSearchSelect,
  Badge,
  Button,
  Checkbox,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  DateInput,
  DetailSkeleton,
  ErrorState,
  FormField,
  PageHeader,
  Popover,
  PopoverContent,
  PopoverTrigger,
  SectionCard,
  SelectInput,
  Spinner,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { AsyncSearchOption } from "@/shared/components/ui";
import { DescriptionList, DocNumber, LineItemsEditor, Money, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { productOptionService, salesService } from "@/modules/sales/services/sales.service";
import type {
  FromDeliveriesLinePayload,
  FromDeliveriesPaymentTerms,
  FromDeliveriesPayload,
  FromDeliveriesPreview,
  FromDeliveriesPreviewItem,
  FromDeliveriesPreviewPayload,
  FromDeliveriesSettlement,
  SalesOrderLinePayload,
} from "@/modules/sales/types";
import { pricingSourceLabel } from "@/modules/sales/types";
import { DEFAULT_VAT_RATE, type SalesTaxType } from "@/modules/sales/lib/tax";
import { splitItemErrors } from "@/modules/sales/lib/server-errors";
import type { ItemErrors } from "@/modules/sales/lib/server-errors";
import { parseManualPrice } from "@/modules/sales/lib/payload";
import { clearClientRef, clientRefFor } from "@/modules/sales/lib/from-deliveries";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";
import { productsService } from "@/modules/products/services/products.service";

// Mode "order dari surat jalan" (ADR-0022 §4, PLAN W-06). Beda dengan form biasa:
// pelanggan, alamat kirim, dan baris stok datang dari SJ (qty terkunci, tidak bisa
// ditambah/dihapus); klien hanya mengirim harga/diskon/pajak/termin dan baris jasa
// non-stok. Total selalu dari pratinjau server (debounce) agar sama dengan yang
// disimpan. Simpan = satu langkah idempoten lewat clientRef di sessionStorage.

/** Jeda pratinjau ulang setelah isian berubah (ms). */
const PREVIEW_DEBOUNCE_MS = 400;

/** Isian klien untuk satu baris SJ (kunci produk/varian/satuan; qty milik server). */
interface StockLine {
  key: string;
  productId: number;
  variantId: number;
  uom: string;
  productName: string;
  productCode: string;
  variantName: string;
  qty: number;
  manualPrice: string;
  discountPct: number;
  discountNominal: number;
  discountReason: string;
  note: string;
  taxable: boolean;
}

/** Baris jasa / non-stok tambahan (K6): qty milik klien. */
interface ExtraLine {
  key: number;
  productId: number;
  variantId: number;
  productLabel: string;
  uom: string;
  uoms: string[];
  variants: { id: number; name: string; code: string }[];
  qty: number;
  manualPrice: string;
  discountPct: number;
  discountNominal: number;
  discountReason: string;
  note: string;
  taxable: boolean;
}

type SettleMode = "" | "pay_now" | "switch_to_net";

let nextExtraKey = 1;

function lineKeyOf(it: { productId: number; variantId: number; uom: string }): string {
  return `${it.productId}/${it.variantId}/${it.uom}`;
}

function toStockLine(it: FromDeliveriesPreviewItem): StockLine {
  return {
    key: lineKeyOf(it),
    productId: it.productId,
    variantId: it.variantId,
    uom: it.uom,
    productName: it.productName,
    productCode: it.productCode,
    variantName: it.variantName,
    qty: it.qty,
    manualPrice: "",
    discountPct: 0,
    discountNominal: 0,
    discountReason: "",
    note: "",
    taxable: true,
  };
}

/** Baris SJ hanya dikirim bila ada isian klien; sisanya diberi harga server (member/katalog). */
function stockLineHasInput(l: StockLine): boolean {
  return (
    l.manualPrice.trim() !== "" ||
    l.discountPct !== 0 ||
    l.discountNominal !== 0 ||
    l.discountReason.trim() !== "" ||
    l.note.trim() !== "" ||
    !l.taxable
  );
}

function toStockPayload(l: StockLine): FromDeliveriesLinePayload {
  const out: FromDeliveriesLinePayload = {
    productId: l.productId,
    variantId: l.variantId,
    uom: l.uom,
    discountPct: l.discountPct,
    discountNominal: l.discountNominal,
    taxable: l.taxable,
  };
  const mp = parseManualPrice(l.manualPrice);
  if (mp !== null && !Number.isNaN(mp)) out.unitPrice = mp;
  if (l.discountReason.trim() !== "") out.discountReason = l.discountReason.trim();
  if (l.note.trim() !== "") out.note = l.note.trim();
  return out;
}

function toExtraPayload(l: ExtraLine): SalesOrderLinePayload {
  const out: SalesOrderLinePayload = {
    productId: l.productId,
    variantId: l.variantId,
    uom: l.uom,
    qty: l.qty,
    discountPct: l.discountPct,
    discountNominal: l.discountNominal,
    taxable: l.taxable,
  };
  const mp = parseManualPrice(l.manualPrice);
  if (mp !== null && !Number.isNaN(mp)) out.unitPrice = mp;
  if (l.discountReason.trim() !== "") out.discountReason = l.discountReason.trim();
  if (l.note.trim() !== "") out.note = l.note.trim();
  return out;
}

export interface FromDeliveriesFormProps {
  /** Id SJ Langsung (1–20, satu pelanggan) dari `?fromDeliveries=`. */
  deliveryIds: number[];
}

export function FromDeliveriesForm({ deliveryIds }: FromDeliveriesFormProps) {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const allowed = can("sales.create");
  const canPayNow = can("payment.create");
  const canSwitchNet = can("sales.approve");

  // --- header ---
  const [orderDate, setOrderDate] = useState(todayWIB());
  const [paymentTerms, setPaymentTerms] = useState<FromDeliveriesPaymentTerms>("cod");
  const [dueDate, setDueDate] = useState("");
  const [taxType, setTaxType] = useState<SalesTaxType>("none");
  const [taxRate, setTaxRate] = useState(0);
  const [notes, setNotes] = useState("");
  const [assignedUserId, setAssignedUserId] = useState<number | null>(null);
  const [taxInvoiceNumber, setTaxInvoiceNumber] = useState("");
  const [taxInvoiceDate, setTaxInvoiceDate] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);

  // --- pelunasan order tunai (cermin konfirmasi SJ COD) ---
  const [settleMode, setSettleMode] = useState<SettleMode>("");
  const [payMethod, setPayMethod] = useState<"cash" | "transfer" | "check">("cash");
  const [payDate, setPayDate] = useState(todayWIB());
  const [payRef, setPayRef] = useState("");
  const [payTendered, setPayTendered] = useState("");
  const [settleDue, setSettleDue] = useState("");

  // --- baris ---
  const [stockLines, setStockLines] = useState<StockLine[]>([]);
  const [extras, setExtras] = useState<ExtraLine[]>([]);
  const [addingLine, setAddingLine] = useState(false);

  // --- pratinjau server ---
  const [preview, setPreview] = useState<FromDeliveriesPreview | null>(null);
  const [initialized, setInitialized] = useState(false);
  // Baris SJ diisi dari pratinjau PERTAMA saja; ref agar effect pratinjau tidak bergantung padanya.
  const initializedRef = useRef(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [retryNonce, setRetryNonce] = useState(0);
  const [itemErrors, setItemErrors] = useState<ItemErrors>({ general: [], byRow: {} });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const previewBody = useMemo<FromDeliveriesPreviewPayload>(() => {
    const lines = stockLines.filter(stockLineHasInput).map(toStockPayload);
    const extraItems = extras.filter((l) => l.qty > 0).map(toExtraPayload);
    return {
      deliveryIds,
      orderDate,
      paymentTerms,
      dueDate: paymentTerms === "net" ? dueDate.trim() : "",
      taxType,
      taxRate,
      notes,
      assignedUserId: assignedUserId ?? 0,
      taxInvoiceNumber: taxInvoiceNumber.trim(),
      taxInvoiceDate: taxInvoiceDate.trim(),
      ...(lines.length > 0 ? { lines } : {}),
      ...(extraItems.length > 0 ? { extraItems } : {}),
    };
  }, [deliveryIds, orderDate, paymentTerms, dueDate, taxType, taxRate, notes, assignedUserId, taxInvoiceNumber, taxInvoiceDate, stockLines, extras]);
  const previewKey = JSON.stringify(previewBody);

  // Pratinjau ulang (debounce) tiap isian yang memengaruhi harga/total berubah.
  // Respons yang datang terlambat (sudah ada permintaan lebih baru) diabaikan.
  useEffect(() => {
    let alive = true;
    const body = JSON.parse(previewKey) as FromDeliveriesPreviewPayload;
    const timer = window.setTimeout(
      () => {
        setPreviewLoading(true);
        salesService
          .previewFromDeliveries(body)
          .then(
            (pv) => {
              if (!alive) return;
              setPreview(pv);
              setPreviewError(null);
              setItemErrors({ general: [], byRow: {} });
              if (!initializedRef.current) {
                initializedRef.current = true;
                setStockLines(pv.items.map(toStockLine));
                setInitialized(true);
              }
            },
            (err: unknown) => {
              if (!alive) return;
              const apiErr = toApiError(err);
              setPreviewError(apiErr.message);
              setItemErrors(splitItemErrors(apiErr.errors));
            },
          )
          .finally(() => {
            if (alive) setPreviewLoading(false);
          });
      },
      initializedRef.current ? PREVIEW_DEBOUNCE_MS : 0,
    );
    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, [previewKey, retryNonce]);

  const serverItems = useMemo(() => {
    const m = new Map<string, FromDeliveriesPreviewItem>();
    for (const it of preview?.items ?? []) m.set(lineKeyOf(it), it);
    return m;
  }, [preview]);

  function updateStock(key: string, patch: Partial<StockLine>) {
    setStockLines((rows) => rows.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }
  function updateExtra(key: number, patch: Partial<ExtraLine>) {
    setExtras((rows) => rows.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  // Hanya produk non-stok (jasa) yang boleh ditambah (K6); opsi berstok disaring.
  const loadServiceOptions = (search: string): Promise<AsyncSearchOption[]> =>
    productOptionService.search(search).then(
      (list) =>
        list
          .filter((p) => p.tracked !== true)
          .map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
      (): AsyncSearchOption[] => [],
    );

  async function addExtra(productId: number) {
    setAddingLine(true);
    try {
      const d = await productsService.detail(productId);
      if (d.tracked) {
        toast.warning("Baris stok tidak bisa ditambah", "Barang berstok harus lewat surat jalan; hanya jasa/non-stok yang boleh ditambah di sini.");
        return;
      }
      const variants = d.variants.map((v) => ({ id: v.id, code: v.code, name: v.name }));
      if (variants.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
        return;
      }
      const uoms = [d.salesUom, d.purchaseUom, d.baseUom].filter((u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i);
      const defaultVariant = d.variants.find((v) => v.isDefault) ?? d.variants[0];
      const uom = d.salesUom || uoms[0] || "";
      if (extras.some((r) => r.productId === d.id && r.variantId === defaultVariant.id && r.uom === uom)) {
        toast.warning("Baris sudah ada", "Ubah qty pada baris yang sudah ada.");
        return;
      }
      setExtras((rows) => [
        ...rows,
        {
          key: nextExtraKey++,
          productId: d.id,
          variantId: defaultVariant.id,
          productLabel: `${d.code} — ${d.name}`,
          uom,
          uoms: uoms.length > 0 ? uoms : [uom],
          variants,
          qty: 1,
          manualPrice: "",
          discountPct: 0,
          discountNominal: 0,
          discountReason: "",
          note: "",
          taxable: true,
        },
      ]);
    } catch (err) {
      toast.danger("Gagal memuat produk", toApiError(err).message);
    } finally {
      setAddingLine(false);
    }
  }

  function onTaxedChange(v: boolean) {
    if (v) {
      setTaxType("exclude");
      setTaxRate((r) => (r > 0 ? r : DEFAULT_VAT_RATE));
      // PPN terutang saat penyerahan barang: sarankan tanggal faktur pajak = tanggal SJ (ADR-0022 §6).
      if (taxInvoiceDate.trim() === "" && preview?.deliveryDate) setTaxInvoiceDate(preview.deliveryDate);
    } else {
      setTaxType("none");
      setTaxRate(0);
    }
  }

  function validate(): Record<string, string> {
    const e: Record<string, string> = {};
    if (orderDate.trim() === "") e.orderDate = "Tanggal order wajib diisi";
    if (paymentTerms === "net" && dueDate.trim() === "") e.dueDate = "Jatuh tempo wajib diisi untuk termin tempo";
    if (paymentTerms === "cod") {
      if (settleMode === "") {
        e.settlement =
          canPayNow || canSwitchNet
            ? "Pilih cara pelunasan: order tunai diselesaikan saat disimpan"
            : "Order tunai butuh izin pembayaran atau persetujuan kredit — pilih termin tempo";
      } else if (settleMode === "switch_to_net" && settleDue.trim() === "") {
        e["settlement.dueDate"] = "Jatuh tempo wajib diisi";
      } else if (settleMode === "pay_now" && payMethod !== "cash" && payRef.trim() === "") {
        e["settlement.reference"] = payMethod === "check" ? "Nomor cek wajib diisi" : "Referensi transfer wajib diisi";
      }
    }
    if (taxInvoiceDate.trim() !== "" && !/^\d{4}-\d{2}-\d{2}$/.test(taxInvoiceDate.trim())) {
      e.taxInvoiceDate = "Format tanggal YYYY-MM-DD";
    }
    extras.forEach((l, i) => {
      if (!(l.qty > 0)) e[`extraItems.${i}.qty`] = "Qty harus lebih dari 0";
    });
    return e;
  }

  function settlementPayload(): FromDeliveriesSettlement | undefined {
    if (paymentTerms !== "cod" || settleMode === "") return undefined;
    if (settleMode === "switch_to_net") return { mode: "switch_to_net", dueDate: settleDue.trim() };
    const tendered = Number(payTendered);
    return {
      mode: "pay_now",
      method: payMethod,
      paymentDate: payDate.trim() || undefined,
      reference: payRef.trim() || undefined,
      amountTendered: payMethod === "cash" && Number.isFinite(tendered) && tendered > 0 ? Math.round(tendered) : undefined,
    };
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const clientErrors = validate();
    if (Object.keys(clientErrors).length > 0) {
      setErrors(clientErrors);
      if (clientErrors.taxInvoiceDate) setDetailOpen(true);
      return;
    }
    setErrors({});
    setSubmitError(null);
    setItemErrors({ general: [], byRow: {} });
    // clientRef yang sama dipakai ulang bila percobaan sebelumnya gagal di tengah:
    // server melanjutkan orkestrasi (finish_from_deliveries), bukan membuat order kedua.
    const clientRef = clientRefFor(deliveryIds);
    const body: FromDeliveriesPayload = { ...previewBody, settlement: settlementPayload(), clientRef };
    setSubmitting(true);
    try {
      const res = await salesService.createFromDeliveries(body);
      clearClientRef(deliveryIds);
      toast.fromServer(res.message, "Order dibuat dari surat jalan dan selesai", res.data.number);
      const warnings = res.data.warnings ?? [];
      if (warnings.length > 0) toast.warning("Perlu diperhatikan", warnings.join(" · "));
      navigate(`${ROUTE_PATHS.salesOrders}/${res.data.id}`, { state: { fromDeliveries: true } });
    } catch (err) {
      const apiErr = toApiError(err);
      toast.danger("Gagal membuat order dari surat jalan", apiErr.message);
      setSubmitError(apiErr.message);
      if (apiErr.errors) {
        const mapped: Record<string, string> = {};
        for (const fe of apiErr.errors) mapped[fe.field] ??= fe.message;
        setErrors(mapped);
        setItemErrors(splitItemErrors(apiErr.errors));
      }
    } finally {
      setSubmitting(false);
    }
  }

  const title = "Order dari Surat Jalan";

  if (!initialized && previewError) {
    return (
      <Page>
        <PageHeader title={title} />
        <ErrorState title="Gagal memuat surat jalan" message={previewError} onRetry={() => setRetryNonce((n) => n + 1)} />
      </Page>
    );
  }
  if (!initialized || preview === null) return <DetailSkeleton />;

  const shipTo = [preview.shipToRecipient, preview.shipToPhone, preview.shipToAddress].filter((t) => t && t.trim() !== "").join(" · ");
  const errorCount = Object.keys(errors).length + Object.keys(itemErrors.byRow).length + itemErrors.general.length;
  const lineCount = stockLines.length + extras.length;

  /** Kolom isian harga/diskon/rincian yang sama untuk baris SJ dan baris jasa. */
  function priceCell(
    label: string,
    manualPrice: string,
    server: FromDeliveriesPreviewItem | undefined,
    rowIndex: number,
    onChange: (v: string) => void,
  ) {
    return (
      <div>
        <TextInput
          type="number"
          min={0}
          step={1}
          value={manualPrice}
          placeholder={server ? String(server.unitPrice) : ""}
          onChange={(e) => onChange(e.target.value)}
          error={itemErrors.byRow[rowIndex]}
          aria-label={`Harga manual ${label}`}
          className="text-right"
        />
        <p className="mt-1 text-xs text-muted-foreground">
          {server
            ? manualPrice === ""
              ? `${pricingSourceLabel(server.pricingSource)} ${formatIDR(server.unitPrice)}`
              : `Manual · ${server.basePrice > 0 ? `dasar ${formatIDR(server.basePrice)}` : "harga server menyesuaikan"}`
            : "Menunggu harga server"}
        </p>
      </div>
    );
  }

  function discountCell(
    label: string,
    pct: number,
    nominal: number,
    onChange: (patch: { discountPct?: number; discountNominal?: number }) => void,
  ) {
    return (
      <div className="flex items-start gap-1.5">
        <TextInput
          type="number"
          min={0}
          max={100}
          step="any"
          value={String(pct)}
          onChange={(e) => onChange({ discountPct: Number(e.target.value) })}
          aria-label={`Diskon persen ${label}`}
          className="w-16 text-right"
        />
        <TextInput
          type="number"
          min={0}
          step={1}
          value={String(nominal)}
          onChange={(e) => onChange({ discountNominal: Number(e.target.value) })}
          aria-label={`Diskon nominal ${label}`}
          className="w-24 text-right"
        />
      </div>
    );
  }

  function detailCell(
    label: string,
    row: { discountReason: string; note: string; taxable: boolean },
    onChange: (patch: { discountReason?: string; note?: string; taxable?: boolean }) => void,
  ) {
    const summary = [row.discountReason ? "alasan diskon" : "", row.note ? "catatan" : "", row.taxable ? "" : "tanpa pajak"].filter(Boolean);
    return (
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="sm" aria-label={`Rincian baris ${label}`}>
            Ubah
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="flex w-72 flex-col gap-3 p-4">
          <FormField label="Alasan diskon">
            <TextInput
              value={row.discountReason}
              maxLength={255}
              onChange={(e) => onChange({ discountReason: e.target.value })}
              aria-label={`Alasan diskon ${label}`}
            />
          </FormField>
          <FormField label="Catatan baris">
            <TextInput value={row.note} maxLength={255} onChange={(e) => onChange({ note: e.target.value })} aria-label={`Catatan baris ${label}`} />
          </FormField>
          <label className="flex cursor-pointer items-center gap-2 text-sm">
            <Checkbox checked={row.taxable} onCheckedChange={(v) => onChange({ taxable: v === true })} aria-label={`Kena pajak ${label}`} />
            Kena pajak
          </label>
        </PopoverContent>
        {summary.length > 0 && <p className="mt-1 text-xs text-muted-foreground">{summary.join(", ")}</p>}
      </Popover>
    );
  }

  return (
    <Page>
      <PageHeader
        title={title}
        meta={<Badge tone="info">Dari SJ</Badge>}
        description="Barang sudah dikirim lewat surat jalan langsung. Qty mengikuti surat jalan; Anda hanya menetapkan harga, diskon, pajak, dan termin. Order langsung selesai saat disimpan."
      />

      {!allowed && (
        <Notice tone="warning" title="Akses terbatas">
          Anda tidak memiliki izin untuk menambah order jual.
        </Notice>
      )}
      {submitError && (
        <Notice tone="danger" title="Gagal membuat order dari surat jalan">
          {submitError}
        </Notice>
      )}
      {previewError && (
        <Notice tone="danger" title="Pratinjau ditolak server">
          <p>{previewError}</p>
          <div className="mt-2">
            <Button variant="secondary" size="sm" onClick={() => setRetryNonce((n) => n + 1)} disabled={previewLoading}>
              Coba lagi
            </Button>
          </div>
        </Notice>
      )}
      {preview.warnings.length > 0 && (
        <Notice tone="warning" title="Perlu diperhatikan">
          <ul className="list-disc pl-5" data-testid="preview-warnings">
            {preview.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </Notice>
      )}

      <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-6">
        <SectionCard title="Pelanggan & surat jalan" description="Terkunci dari surat jalan — tidak bisa diubah di sini.">
          <DescriptionList
            columns={2}
            items={[
              { label: "Pelanggan", value: <span data-testid="locked-customer">{preview.partyName || `Customer #${preview.partyId}`}</span> },
              { label: "Tanggal SJ terakhir", value: formatDate(preview.deliveryDate) },
              {
                label: "Surat jalan",
                value: (
                  <span className="flex flex-wrap gap-2">
                    {preview.deliveries.map((d) => (
                      <Link key={d.id} to={`${ROUTE_PATHS.deliveries}/${d.id}`} className="text-primary hover:underline">
                        <DocNumber value={d.number} />
                        <span className="ml-1 text-xs text-muted-foreground">{formatDate(d.deliveryDate)}</span>
                      </Link>
                    ))}
                  </span>
                ),
                wide: true,
              },
              {
                label: preview.shipToLabel ? `Alamat kirim (${preview.shipToLabel})` : "Alamat kirim",
                value: shipTo,
                wide: true,
              },
            ]}
          />
        </SectionCard>

        <SectionCard title="Dokumen">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <FormField label="Tanggal order" required htmlFor="f-date" errorText={errors.orderDate}>
              <DateInput id="f-date" value={orderDate} onChange={(e) => setOrderDate(e.target.value)} />
            </FormField>
            <FormField label="Termin" required htmlFor="f-terms" errorText={errors.paymentTerms}>
              <SelectInput
                id="f-terms"
                value={paymentTerms}
                onChange={(e) => setPaymentTerms(e.target.value as FromDeliveriesPaymentTerms)}
              >
                <option value="cod">Tunai (dilunasi saat simpan)</option>
                <option value="net">Tempo (net)</option>
              </SelectInput>
            </FormField>
            {paymentTerms === "net" && (
              <FormField label="Jatuh tempo" required htmlFor="f-due" errorText={errors.dueDate}>
                <DateInput id="f-due" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
              </FormField>
            )}
            <div className="sm:col-span-2 lg:col-span-4">
              <div className="flex flex-wrap items-end gap-x-6 gap-y-3" data-testid="tax-controls">
                <label className="flex cursor-pointer items-center gap-2 text-sm font-medium text-foreground">
                  <Checkbox id="f-taxed" checked={taxType !== "none"} onCheckedChange={(v) => onTaxedChange(v === true)} />
                  Kena pajak (PPN)
                </label>
                {taxType !== "none" && (
                  <>
                    <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                      <Checkbox
                        id="f-tax-include"
                        checked={taxType === "include"}
                        onCheckedChange={(v) => setTaxType(v === true ? "include" : "exclude")}
                      />
                      Harga sudah termasuk pajak
                    </label>
                    <FormField label="Tarif (%)" htmlFor="f-tax-rate" errorText={errors.taxRate}>
                      <TextInput
                        id="f-tax-rate"
                        inputMode="decimal"
                        className="w-24"
                        value={String(taxRate)}
                        onChange={(e) => {
                          const n = Number(e.target.value.replace(",", "."));
                          setTaxRate(Number.isFinite(n) ? Math.min(100, Math.max(0, n)) : 0);
                        }}
                      />
                    </FormField>
                  </>
                )}
              </div>
              {taxType !== "none" && (
                <p className="mt-2 text-xs text-muted-foreground">
                  PPN terutang saat barang diserahkan: tanggal faktur pajak disarankan {formatDate(preview.deliveryDate)} (tanggal SJ).
                </p>
              )}
            </div>
          </div>

          {paymentTerms === "cod" && (
            <div className="mt-4 rounded-md border border-border p-4" data-testid="settlement">
              <p className="text-sm font-semibold text-foreground">Pelunasan order tunai</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Barang sudah diterima pelanggan, jadi order tunai diselesaikan saat disimpan: catat pembayarannya sekarang
                atau ubah menjadi tempo.
              </p>
              <div className="mt-3 grid grid-cols-1 gap-4 md:grid-cols-2">
                <FormField label="Cara pelunasan" required htmlFor="f-settle" errorText={errors.settlement ?? errors["settlement.mode"]}>
                  <SelectInput id="f-settle" value={settleMode} onChange={(e) => setSettleMode(e.target.value as SettleMode)}>
                    <option value="">Pilih…</option>
                    <option value="pay_now" disabled={!canPayNow}>
                      Bayar sekarang{canPayNow ? "" : " (butuh izin pembayaran)"}
                    </option>
                    <option value="switch_to_net" disabled={!canSwitchNet}>
                      Jadikan tempo (net){canSwitchNet ? "" : " (butuh izin persetujuan kredit)"}
                    </option>
                  </SelectInput>
                </FormField>
                {settleMode === "switch_to_net" && (
                  <FormField label="Jatuh tempo" required htmlFor="f-settle-due" errorText={errors["settlement.dueDate"]}>
                    <DateInput id="f-settle-due" value={settleDue} onChange={(e) => setSettleDue(e.target.value)} />
                  </FormField>
                )}
                {settleMode === "pay_now" && (
                  <>
                    <FormField label="Metode" htmlFor="f-pay-method" errorText={errors["settlement.method"]}>
                      <SelectInput
                        id="f-pay-method"
                        value={payMethod}
                        onChange={(e) => setPayMethod(e.target.value as "cash" | "transfer" | "check")}
                      >
                        <option value="cash">Tunai</option>
                        <option value="transfer">Transfer</option>
                        <option value="check">Cek</option>
                      </SelectInput>
                    </FormField>
                    <FormField label="Tanggal bayar" htmlFor="f-pay-date" errorText={errors["settlement.paymentDate"]}>
                      <DateInput id="f-pay-date" value={payDate} onChange={(e) => setPayDate(e.target.value)} />
                    </FormField>
                    {payMethod !== "cash" && (
                      <FormField
                        label={payMethod === "check" ? "Nomor cek" : "Referensi transfer"}
                        required
                        htmlFor="f-pay-ref"
                        errorText={errors["settlement.reference"]}
                      >
                        <TextInput id="f-pay-ref" value={payRef} onChange={(e) => setPayRef(e.target.value)} />
                      </FormField>
                    )}
                    {payMethod === "cash" && (
                      <FormField
                        label="Uang diterima (Rp)"
                        htmlFor="f-pay-tendered"
                        helperText="Opsional, untuk catatan kembalian."
                        errorText={errors["settlement.amountTendered"]}
                      >
                        <TextInput
                          id="f-pay-tendered"
                          type="number"
                          min={0}
                          step={1}
                          value={payTendered}
                          onChange={(e) => setPayTendered(e.target.value)}
                        />
                      </FormField>
                    )}
                  </>
                )}
              </div>
            </div>
          )}

          <Collapsible open={detailOpen} onOpenChange={setDetailOpen} className="mt-4 border-t border-border pt-3">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="-ml-2">
                <ChevronRight aria-hidden className={detailOpen ? "rotate-90 transition-transform" : "transition-transform"} />
                Detail lain
                <span className="font-normal text-muted-foreground">(faktur pajak, penanggung jawab, catatan)</span>
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-3">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <div className="sm:col-span-1 lg:col-span-2">
                  <FormField label="No. faktur pajak" htmlFor="f-tax-no" errorText={errors.taxInvoiceNumber}>
                    <TextInput
                      id="f-tax-no"
                      value={taxInvoiceNumber}
                      onChange={(e) => setTaxInvoiceNumber(e.target.value)}
                      placeholder="Nomor faktur pajak (opsional)"
                    />
                  </FormField>
                </div>
                <FormField label="Tgl. faktur pajak" htmlFor="f-tax-date" errorText={errors.taxInvoiceDate}>
                  <DateInput id="f-tax-date" value={taxInvoiceDate} onChange={(e) => setTaxInvoiceDate(e.target.value)} />
                </FormField>
                {can("users.view") && (
                  <div className="sm:col-span-2">
                    <FormField label="Penanggung jawab" errorText={errors.assignedUserId}>
                      <UserSearchSelect value={assignedUserId} onChange={(uid) => setAssignedUserId(uid)} placeholder="Tanpa penanggung jawab" />
                    </FormField>
                  </div>
                )}
                <div className="sm:col-span-2 lg:col-span-4">
                  <FormField label="Catatan" htmlFor="f-notes" errorText={errors.notes}>
                    <TextArea id="f-notes" value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Catatan order (opsional)" rows={2} />
                  </FormField>
                </div>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </SectionCard>

        <SectionCard
          title={`Barang dari surat jalan (${stockLines.length})`}
          description="Qty terkunci mengikuti surat jalan. Harga kosong = harga server (member/katalog); isi untuk harga manual (tidak boleh di bawah harga minimum produk)."
        >
          {(itemErrors.general.length > 0 || errors.items) && (
            <Notice tone="danger" title="Barang ditolak server" className="mb-4">
              <ul className="list-disc pl-5">
                {itemErrors.general.map((m) => (
                  <li key={m}>{m}</li>
                ))}
                {errors.items && <li>{errors.items}</li>}
              </ul>
            </Notice>
          )}
          <LineItemsEditor<StockLine>
            lines={stockLines}
            getKey={(r) => r.key}
            emptyText="Surat jalan tidak punya baris barang."
            columns={[
              {
                id: "product",
                header: "Produk",
                render: (r) => (
                  <div className="min-w-44">
                    <p className="font-medium">{r.productName}</p>
                    <p className="text-xs text-muted-foreground">
                      {r.productCode}
                      {r.variantName ? ` · ${r.variantName}` : ""}
                    </p>
                  </div>
                ),
              },
              { id: "uom", header: "Satuan", width: "6rem", render: (r) => r.uom },
              {
                id: "qty",
                header: "Qty (SJ)",
                width: "7rem",
                align: "right",
                render: (r) => <Qty value={r.qty} className="inline-block py-2" />,
              },
              {
                id: "price",
                header: "Harga (Rp)",
                width: "11rem",
                align: "right",
                render: (r, i) => priceCell(r.productName, r.manualPrice, serverItems.get(r.key), i, (v) => updateStock(r.key, { manualPrice: v })),
              },
              {
                id: "discount",
                header: "Diskon",
                width: "10rem",
                align: "right",
                render: (r) => discountCell(r.productName, r.discountPct, r.discountNominal, (p) => updateStock(r.key, p)),
              },
              {
                id: "detail",
                header: "Rincian",
                width: "7rem",
                render: (r) => detailCell(r.productName, r, (p) => updateStock(r.key, p)),
              },
              {
                id: "total",
                header: "Total",
                width: "8rem",
                align: "right",
                render: (r) => <Money value={serverItems.get(r.key)?.lineTotal ?? 0} className="inline-block py-2 font-medium" />,
              },
            ]}
          />
        </SectionCard>

        <SectionCard
          title={`Jasa / non-stok tambahan (${extras.length})`}
          description="Opsional: ongkos kirim, jasa pasang, dan produk non-stok lain. Barang berstok tidak bisa ditambah — buat surat jalan baru."
        >
          <div className="flex flex-col gap-4">
            <FormField label="Tambah jasa / non-stok" htmlFor="f-extra-product">
              <AsyncSearchSelect
                id="f-extra-product"
                value={null}
                loadOptions={loadServiceOptions}
                onChange={(v) => {
                  if (typeof v === "number") void addExtra(v);
                }}
                disabled={addingLine}
                placeholder={addingLine ? "Menambah baris…" : "Cari produk jasa (kode atau nama)…"}
                searchPlaceholder="Kode / nama produk…"
              />
            </FormField>
            <LineItemsEditor<ExtraLine>
              lines={extras}
              getKey={(r) => r.key}
              onRemove={(i) => setExtras((rows) => rows.filter((_, idx) => idx !== i))}
              emptyText="Belum ada baris jasa."
              columns={[
                {
                  id: "product",
                  header: "Produk",
                  render: (r) => (
                    <div className="min-w-44">
                      <p className="font-medium">{r.productLabel}</p>
                      {r.variants.length <= 1 ? (
                        <p className="text-xs text-muted-foreground">{r.variants.find((v) => v.id === r.variantId)?.name ?? ""}</p>
                      ) : (
                        <div className="mt-1">
                          <SelectInput
                            value={String(r.variantId)}
                            onChange={(e) => updateExtra(r.key, { variantId: Number(e.target.value) })}
                            aria-label={`Varian ${r.productLabel}`}
                          >
                            {r.variants.map((v) => (
                              <option key={v.id} value={String(v.id)}>
                                {v.code} — {v.name}
                              </option>
                            ))}
                          </SelectInput>
                        </div>
                      )}
                    </div>
                  ),
                },
                {
                  id: "uom",
                  header: "Satuan",
                  width: "7rem",
                  render: (r) => (
                    <SelectInput value={r.uom} onChange={(e) => updateExtra(r.key, { uom: e.target.value })} aria-label={`Satuan ${r.productLabel}`}>
                      {r.uoms.map((u) => (
                        <option key={u} value={u}>
                          {u}
                        </option>
                      ))}
                    </SelectInput>
                  ),
                },
                {
                  id: "qty",
                  header: "Qty",
                  width: "6rem",
                  align: "right",
                  render: (r, i) => (
                    <TextInput
                      type="number"
                      min={0}
                      step="any"
                      value={String(r.qty)}
                      onChange={(e) => updateExtra(r.key, { qty: Number(e.target.value) })}
                      error={errors[`extraItems.${i}.qty`]}
                      aria-label={`Qty ${r.productLabel}`}
                      className="text-right"
                    />
                  ),
                },
                {
                  id: "price",
                  header: "Harga (Rp)",
                  width: "11rem",
                  align: "right",
                  render: (r, i) =>
                    priceCell(r.productLabel, r.manualPrice, serverItems.get(lineKeyOf(r)), stockLines.length + i, (v) =>
                      updateExtra(r.key, { manualPrice: v }),
                    ),
                },
                {
                  id: "discount",
                  header: "Diskon",
                  width: "10rem",
                  align: "right",
                  render: (r) => discountCell(r.productLabel, r.discountPct, r.discountNominal, (p) => updateExtra(r.key, p)),
                },
                {
                  id: "detail",
                  header: "Rincian",
                  width: "7rem",
                  render: (r) => detailCell(r.productLabel, r, (p) => updateExtra(r.key, p)),
                },
                {
                  id: "total",
                  header: "Total",
                  width: "8rem",
                  align: "right",
                  render: (r) => <Money value={serverItems.get(lineKeyOf(r))?.lineTotal ?? 0} className="inline-block py-2 font-medium" />,
                },
              ]}
            />
          </div>
        </SectionCard>

        <StickyFooter
          summary={
            <span className="flex flex-wrap items-center gap-x-4 gap-y-1" data-testid="preview-summary">
              <span>
                <span className="tabular">{lineCount}</span> baris
              </span>
              {preview.discountTotal > 0 && (
                <span className="text-muted-foreground">
                  Diskon <Money value={preview.discountTotal} />
                </span>
              )}
              {taxType !== "none" && (
                <span data-testid="tax-summary" className="text-muted-foreground">
                  PPN {formatNumber(taxRate)}% <Money value={preview.taxTotal} />
                </span>
              )}
              <span>
                Total <Money value={preview.grandTotal} className="font-display text-lg font-semibold text-foreground" />
              </span>
              {previewLoading && (
                <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground" role="status" aria-live="polite">
                  <Spinner className="size-3.5" aria-label="Menghitung ulang" /> menghitung ulang
                </span>
              )}
              {errorCount > 0 && (
                <span role="alert" className="font-medium text-destructive">
                  {errorCount} isian perlu diperbaiki
                </span>
              )}
            </span>
          }
        >
          <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesOrders)}>
            Batal
          </Button>
          <Button type="submit" loading={submitting} disabled={!allowed || previewLoading || previewError !== null}>
            Simpan &amp; selesaikan
          </Button>
        </StickyFooter>
      </form>
    </Page>
  );
}
