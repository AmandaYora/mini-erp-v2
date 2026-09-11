import { useCallback, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ActionRow,
  Button,
  DateInput,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import type { ApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseOrderSchema } from "@/modules/purchasing/schemas/purchase-order.schema";
import {
  purchaseOrderService,
  purchasingLookupService,
} from "@/modules/purchasing/services/purchase-order.service";
import { calcLineEstimate } from "@/modules/purchasing/lib/estimate";
import type {
  PurchaseOrder,
  PurchasingProductDetail,
} from "@/modules/purchasing/types";

interface LineRow {
  key: number;
  productId: number | null;
  variantId: number | null;
  uom: string;
  qty: number;
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
}

const EMPTY_ROW: Omit<LineRow, "key"> = {
  productId: null,
  variantId: null,
  uom: "",
  qty: 1,
  unitPrice: 0,
  discountPct: 0,
  discountNominal: 0,
};

function flattenIssues(input: unknown): Record<string, string> {
  const parsed = purchaseOrderSchema.safeParse(input);
  if (parsed.success) return {};
  const out: Record<string, string> = {};
  for (const issue of parsed.error.issues) {
    const key = issue.path.join(".");
    out[key || "_"] ??= issue.message;
  }
  return out;
}

/** Field error backend → kunci error form (best-effort, selebihnya Notice). */
function backendErrors(err: ApiError): { record: Record<string, string>; general: string[] } {
  const record: Record<string, string> = {};
  const general: string[] = [];
  for (const e of err.errors ?? []) {
    if (!e.field) {
      general.push(e.message);
      continue;
    }
    const f = e.field;
    if (
      f === "partyId" ||
      f === "orderDate" ||
      f === "dueDate" ||
      f === "paymentTerms" ||
      f === "taxType" ||
      f === "taxRate" ||
      f === "notes" ||
      f === "supplierInvoiceNumber" ||
      f === "supplierInvoiceDate" ||
      f === "items"
    ) {
      record[f === "items" ? "lines" : f] ??= e.message;
    } else {
      general.push(`${f}: ${e.message}`);
    }
  }
  return { record, general };
}

function toNumberInput(value: string): number {
  if (value.trim() === "") return 0;
  const n = Number(value);
  return Number.isNaN(n) ? 0 : n;
}

export default function PurchaseOrderFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const keySeq = useRef(1);
  const detailsRef = useRef<Record<number, PurchasingProductDetail>>({});
  const inflightRef = useRef(new Map<number, Promise<PurchasingProductDetail | undefined>>());

  const isEdit = id !== undefined;
  const orderId = isEdit ? Number(id) : NaN;
  const allowed = isEdit ? can("purchasing.update") : can("purchasing.create");

  const [partyId, setPartyId] = useState<number | null>(null);
  const [orderDate, setOrderDate] = useState("");
  const [paymentTerms, setPaymentTerms] = useState<"net" | "cod">("net");
  const [dueDate, setDueDate] = useState("");
  const [taxType, setTaxType] = useState<"none" | "include" | "exclude">("none");
  const [taxRate, setTaxRate] = useState(0);
  const [notes, setNotes] = useState("");
  const [supplierInvoiceNumber, setSupplierInvoiceNumber] = useState("");
  const [supplierInvoiceDate, setSupplierInvoiceDate] = useState("");
  const [lines, setLines] = useState<LineRow[]>([{ ...EMPTY_ROW, key: 0 }]);

  const [supplierOptions, setSupplierOptions] = useState<SearchSelectOption[]>([]);
  const [loadedPartyId, setLoadedPartyId] = useState<number | null>(null);
  const [details, setDetails] = useState<Record<number, PurchasingProductDetail>>({});
  const [detailLoading, setDetailLoading] = useState<Record<number, boolean>>({});

  const [loaded, setLoaded] = useState<PurchaseOrder | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverMessages, setServerMessages] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);

  const readOnly = isEdit && loaded !== null && loaded.status !== "draft";

  // Kunci mode form: 0 = tambah, -1 = id tak valid, >0 = id order edit.
  const validId = !isEdit || (Number.isInteger(orderId) && orderId > 0);
  const formKey = !isEdit ? 0 : !validId ? -1 : orderId;

  // --- lookups ---

  const { data: lookupData } = useAsyncData(
    () =>
      Promise.all([
        purchasingLookupService.supplierOptions().then(
          (res): SearchSelectOption[] =>
            res.items.map((s) => ({ value: s.id, label: `${s.name} (${s.code})` })),
          (): SearchSelectOption[] => [],
        ),
        purchasingLookupService.productOptions().then(
          (opts): SearchSelectOption[] =>
            opts.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
          (): SearchSelectOption[] => [],
        ),
      ]).then(([suppliers, products]) => ({ suppliers, products })),
    [],
  );
  const baseSuppliers = lookupData?.suppliers ?? [];
  const productOptions = lookupData?.products ?? [];
  const suppliersReady = lookupData !== null;

  const [appliedLookup, setAppliedLookup] = useState(false);
  if (lookupData && !appliedLookup) {
    setAppliedLookup(true);
    setSupplierOptions(lookupData.suppliers);
  }

  // Prefill edit bisa menunjuk supplier arsip/di luar 50 opsi — tambahkan manual.
  const { data: extraSupplier } = useAsyncData(
    () => {
      if (loadedPartyId === null || !suppliersReady) return Promise.resolve(null);
      if (baseSuppliers.some((o) => o.value === loadedPartyId)) return Promise.resolve(null);
      return purchasingLookupService.supplierById(loadedPartyId).then(
        (s): SearchSelectOption => ({ value: s.id, label: `${s.name} (${s.code})` }),
        (): SearchSelectOption | null => null,
      );
    },
    [loadedPartyId, suppliersReady],
  );
  const [appliedExtraId, setAppliedExtraId] = useState<string | number | null>(null);
  if (
    extraSupplier &&
    appliedExtraId !== extraSupplier.value &&
    !supplierOptions.some((o) => o.value === extraSupplier.value)
  ) {
    setAppliedExtraId(extraSupplier.value);
    setSupplierOptions((cur) =>
      cur.some((o) => o.value === extraSupplier.value) ? cur : [extraSupplier, ...cur],
    );
  }

  const ensureDetail = useCallback((productId: number) => {
    const cached = detailsRef.current[productId];
    if (cached) return Promise.resolve(cached);
    const inflight = inflightRef.current.get(productId);
    if (inflight) return inflight;
    setDetailLoading((prev) => (prev[productId] ? prev : { ...prev, [productId]: true }));
    const p: Promise<PurchasingProductDetail | undefined> = purchasingLookupService
      .productDetail(productId)
      .then((d) => {
        detailsRef.current[productId] = d;
        setDetails((prev) => (prev[productId] ? prev : { ...prev, [productId]: d }));
        return d;
      })
      .catch((): undefined => undefined)
      .finally(() => {
        inflightRef.current.delete(productId);
        setDetailLoading((prev) => {
          if (!(productId in prev)) return prev;
          const next = { ...prev };
          delete next[productId];
          return next;
        });
      });
    inflightRef.current.set(productId, p);
    return p;
  }, []);

  // --- prefill edit ---

  const { data: editBundle, loading: editLoading, error: editFetchError } = useAsyncData(
    () => {
      if (!isEdit || !validId) return Promise.resolve(null);
      const key = formKey;
      return purchaseOrderService.get(orderId).then(async (po) => {
        // Dropdown varian/satuan per baris dilengkapi dari detail produk.
        const ids = Array.from(new Set(po.items.map((it) => it.productId)));
        const fetched = await Promise.all(
          ids.map((pid) =>
            purchasingLookupService.productDetail(pid).then(
              (d) => d,
              (): undefined => undefined,
            ),
          ),
        );
        const detailMap: Record<number, PurchasingProductDetail> = {};
        ids.forEach((pid, i) => {
          const d = fetched[i];
          if (d) {
            detailMap[pid] = d;
            detailsRef.current[pid] = d;
          }
        });
        return {
          key,
          loaded: po,
          partyId: po.partyId,
          orderDate: po.orderDate ? po.orderDate.slice(0, 10) : "",
          paymentTerms: (po.paymentTerms === "cod" ? "cod" : "net") as "net" | "cod",
          dueDate: po.dueDate ? po.dueDate.slice(0, 10) : "",
          taxType: (
            po.taxType === "include" || po.taxType === "exclude" ? po.taxType : "none"
          ) as "none" | "include" | "exclude",
          taxRate: po.taxRate ?? 0,
          notes: po.notes ?? "",
          supplierInvoiceNumber: po.supplierInvoiceNumber ?? "",
          supplierInvoiceDate: po.supplierInvoiceDate ?? "",
          lines: po.items.map((it) => ({
            key: keySeq.current++,
            productId: it.productId,
            variantId: it.variantId,
            uom: it.uom,
            qty: it.qty,
            unitPrice: it.unitPrice,
            discountPct: it.discountPct,
            discountNominal: it.discountNominal,
          })),
          detailMap,
        };
      });
    },
    [formKey],
  );
  const loading = isEdit ? editLoading : false;
  const loadError = !validId ? "ID order beli tidak valid." : editFetchError;

  // Reset form saat ganti mode (tambah/edit/id lain).
  const [appliedKey, setAppliedKey] = useState<number | null>(null);
  const [prevKey, setPrevKey] = useState(formKey);
  if (formKey !== prevKey) {
    setPrevKey(formKey);
    setAppliedKey(null);
    setPartyId(null);
    setOrderDate("");
    setPaymentTerms("net");
    setDueDate("");
    setTaxType("none");
    setTaxRate(0);
    setNotes("");
    setSupplierInvoiceNumber("");
    setSupplierInvoiceDate("");
    setLines([{ ...EMPTY_ROW, key: 0 }]);
    setLoaded(null);
    setLoadedPartyId(null);
    setErrors({});
    setServerMessages([]);
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editBundle && editBundle.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setLoaded(editBundle.loaded);
    setLoadedPartyId(editBundle.partyId);
    setPartyId(editBundle.partyId);
    setOrderDate(editBundle.orderDate);
    setPaymentTerms(editBundle.paymentTerms);
    setDueDate(editBundle.dueDate);
    setTaxType(editBundle.taxType);
    setTaxRate(editBundle.taxRate);
    setNotes(editBundle.notes);
    setSupplierInvoiceNumber(editBundle.supplierInvoiceNumber);
    setSupplierInvoiceDate(editBundle.supplierInvoiceDate);
    setLines(editBundle.lines);
    setDetails((prev) => ({ ...prev, ...editBundle.detailMap }));
  }

  // --- line editor ---

  function updateLine(key: number, patch: Partial<LineRow>) {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)));
  }

  async function handleProductChange(key: number, productId: number | null) {
    if (productId === null) {
      updateLine(key, { productId: null, variantId: null, uom: "", unitPrice: 0 });
      return;
    }
    updateLine(key, { productId, variantId: null });
    const d = await ensureDetail(productId);
    if (!d) return;
    const fallbackVariant = d.variants.find((v) => v.isDefault) ?? d.variants[0];
    const uoms = [d.purchaseUom, d.baseUom, d.salesUom].filter(
      (u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i,
    );
    setLines((prev) =>
      prev.map((l) =>
        l.key === key && l.productId === productId
          ? {
              ...l,
              variantId: fallbackVariant ? fallbackVariant.id : null,
              uom: l.uom.trim() !== "" ? l.uom : (uoms[0] ?? ""),
              unitPrice: l.unitPrice === 0 ? d.purchasePrice : l.unitPrice,
            }
          : l,
      ),
    );
  }

  function addRow() {
    setLines((prev) => [...prev, { ...EMPTY_ROW, key: keySeq.current++ }]);
  }

  function removeRow(key: number) {
    setLines((prev) => prev.filter((l) => l.key !== key));
  }

  // --- estimasi (replika domain.CalcLine, BUKAN total resmi) ---

  const estimate = useMemo(() => {
    let subtotal = 0;
    let tax = 0;
    let grand = 0;
    const perRow = new Map<number, number>();
    for (const l of lines) {
      const e = calcLineEstimate(
        l.qty,
        l.unitPrice,
        l.discountPct,
        l.discountNominal,
        taxType,
        taxRate,
      );
      perRow.set(l.key, e.total);
      subtotal += e.net;
      tax += e.tax;
      grand += e.total;
    }
    return { subtotal, tax, grand, perRow };
  }, [lines, taxType, taxRate]);

  // --- submit ---

  function formValues() {
    return {
      partyId: partyId ?? 0,
      orderDate,
      paymentTerms,
      dueDate,
      taxType,
      taxRate,
      notes,
      lines: lines.map((l) => ({
        productId: l.productId ?? 0,
        variantId: l.variantId ?? 0,
        uom: l.uom,
        qty: l.qty,
        unitPrice: l.unitPrice,
        discountPct: l.discountPct,
        discountNominal: l.discountNominal,
      })),
    };
  }

  async function handleSubmit() {
    const fieldErrors = flattenIssues(formValues());
    // Faktur supplier opsional — bila diisi tanggal harus YYYY-MM-DD
    // (backend: resolveSupplierInvoice).
    if (
      supplierInvoiceDate.trim() !== "" &&
      !/^\d{4}-\d{2}-\d{2}$/.test(supplierInvoiceDate.trim())
    ) {
      fieldErrors.supplierInvoiceDate ??= "Format tanggal YYYY-MM-DD";
    }
    setServerMessages([]);
    if (Object.keys(fieldErrors).length > 0) {
      setErrors(fieldErrors);
      toast.warning("Periksa kembali isian form");
      return;
    }
    setErrors({});
    setSubmitting(true);
    try {
      const body = {
        partyId: (partyId as number | null) ?? 0,
        orderDate: orderDate.trim() === "" ? undefined : orderDate.trim(),
        paymentTerms,
        dueDate: dueDate.trim() === "" ? undefined : dueDate.trim(),
        taxType,
        taxRate,
        notes: notes.trim() === "" ? undefined : notes.trim(),
        supplierInvoiceNumber:
          supplierInvoiceNumber.trim() === ""
            ? undefined
            : supplierInvoiceNumber.trim(),
        supplierInvoiceDate:
          supplierInvoiceDate.trim() === "" ? undefined : supplierInvoiceDate.trim(),
        items: lines.map((l) => ({
          productId: (l.productId as number | null) ?? 0,
          variantId: (l.variantId as number | null) ?? 0,
          uom: l.uom.trim(),
          qty: l.qty,
          unitPrice: l.unitPrice,
          discountPct: l.discountPct,
          discountNominal: l.discountNominal,
        })),
      };
      if (isEdit) {
        const res = await purchaseOrderService.update(orderId, body);
        toast.fromServer(res.message, "Purchase order disimpan", res.data.number);
        void navigate(`${ROUTE_PATHS.purchaseOrders}/${res.data.id}`);
      } else {
        const res = await purchaseOrderService.create(body);
        toast.fromServer(res.message, "Purchase order dibuat", res.data.number);
        void navigate(`${ROUTE_PATHS.purchaseOrders}/${res.data.id}`);
      }
    } catch (err) {
      const apiErr = toApiError(err);
      const mapped = backendErrors(apiErr);
      setErrors(mapped.record);
      setServerMessages(
        mapped.general.length > 0 ? mapped.general : [apiErr.message],
      );
      toast.danger("Gagal menyimpan order beli", apiErr.message);
    } finally {
      setSubmitting(false);
    }
  }

  // --- render ---

  if (!allowed) {
    return (
      <div>
        <PageHeader
          eyebrow="Pembelian"
          title={isEdit ? "Ubah Order Beli" : "Order Beli Baru"}
        />
        <Notice tone="danger" title="Akses ditolak">
          Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "membuat"} order beli.
        </Notice>
        <div className="mt-4">
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            Kembali ke Daftar
          </Button>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Ubah Order Beli" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (loadError) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Ubah Order Beli" />
        <Notice tone="danger" title={loadError} />
        <div className="mt-4">
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            Kembali ke Daftar
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title={isEdit ? `Ubah Order ${loaded?.number ?? ""}` : "Order Beli Baru"}
        description={
          isEdit
            ? "Hanya order berstatus draf yang bisa diubah."
            : "Harga & pajak final dihitung server saat simpan."
        }
      />

      {readOnly && (
        <div className="mb-4">
          <Notice tone="warning" title="Order tidak bisa diubah">
            Hanya order berstatus draf yang bisa diubah. Detail bersifat baca-saja.
          </Notice>
        </div>
      )}

      {serverMessages.length > 0 && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal menyimpan">
            <ul className="list-disc pl-5">
              {serverMessages.map((m, i) => (
                <li key={i}>{m}</li>
              ))}
            </ul>
          </Notice>
        </div>
      )}

      <div className="space-y-4">
        <SectionCard title="Header" description="Supplier dan ketentuan order.">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="md:col-span-1">
              <FormField label="Supplier" required errorText={errors.partyId}>
                <SearchSelect
                  options={supplierOptions}
                  value={partyId}
                  onChange={(v) => setPartyId(typeof v === "number" ? v : null)}
                  placeholder={
                    supplierOptions.length === 0 ? "Tidak ada supplier aktif" : "Pilih supplier…"
                  }
                  allowClear
                  disabled={readOnly}
                />
              </FormField>
            </div>
            <FormField label="Tanggal Order" errorText={errors.orderDate}>
              <DateInput
                value={orderDate}
                onChange={(e) => setOrderDate(e.target.value)}
                disabled={readOnly}
              />
            </FormField>
            <FormField label="Termin" required errorText={errors.paymentTerms}>
              <SelectInput
                value={paymentTerms}
                onChange={(e) => {
                  const v = e.target.value === "cod" ? "cod" : "net";
                  setPaymentTerms(v);
                  if (v === "cod") setDueDate("");
                }}
                disabled={readOnly}
              >
                <option value="net">Net (tempo)</option>
                <option value="cod">COD (tunai)</option>
              </SelectInput>
            </FormField>
            <FormField
              label="Jatuh Tempo"
              required={paymentTerms === "net"}
              errorText={errors.dueDate}
            >
              <DateInput
                value={dueDate}
                onChange={(e) => setDueDate(e.target.value)}
                disabled={readOnly}
              />
            </FormField>
            <FormField label="Pajak" required errorText={errors.taxType}>
              <SelectInput
                value={taxType}
                onChange={(e) => {
                  const v =
                    e.target.value === "include" || e.target.value === "exclude"
                      ? e.target.value
                      : "none";
                  setTaxType(v);
                  if (v === "none") setTaxRate(0);
                }}
                disabled={readOnly}
              >
                <option value="none">Tanpa pajak</option>
                <option value="exclude">Exclude (tambah)</option>
                <option value="include">Include (termasuk)</option>
              </SelectInput>
            </FormField>
            <FormField label="Tarif Pajak (%)" errorText={errors.taxRate}>
              <TextInput
                type="number"
                min={0}
                max={100}
                value={taxRate === 0 ? "" : taxRate}
                placeholder={taxType === "none" ? "0" : "cth. 11"}
                onChange={(e) => setTaxRate(toNumberInput(e.target.value))}
                disabled={readOnly || taxType === "none"}
              />
            </FormField>
            <FormField label="No. Faktur Supplier" errorText={errors.supplierInvoiceNumber}>
              <TextInput
                value={supplierInvoiceNumber}
                placeholder="Nomor faktur supplier (opsional)…"
                onChange={(e) => setSupplierInvoiceNumber(e.target.value)}
                disabled={readOnly}
              />
            </FormField>
            <FormField label="Tgl. Faktur Supplier" errorText={errors.supplierInvoiceDate}>
              <DateInput
                value={supplierInvoiceDate}
                onChange={(e) => setSupplierInvoiceDate(e.target.value)}
                disabled={readOnly}
              />
            </FormField>
            <div className="md:col-span-3">
              <FormField label="Catatan" errorText={errors.notes}>
                <TextArea
                  rows={2}
                  value={notes}
                  placeholder="Catatan untuk supplier (opsional)…"
                  onChange={(e) => setNotes(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title="Baris Barang"
          description="Harga per baris mengikuti penawaran supplier — server menghitung ulang total resmi saat simpan."
          actions={
            readOnly ? undefined : (
              <Button variant="secondary" size="sm" onClick={addRow}>
                Tambah Baris
              </Button>
            )
          }
        >
          {errors.lines && (
            <div className="mb-4">
              <Notice tone="danger" title={errors.lines} />
            </div>
          )}
          <div className="space-y-4">
            {lines.map((row, idx) => {
              const d = row.productId !== null ? details[row.productId] : undefined;
              const loadingDetail =
                row.productId !== null && detailLoading[row.productId] === true && !d;
              const uoms = d
                ? [d.purchaseUom, d.baseUom, d.salesUom].filter(
                    (u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i,
                  )
                : [];
              const variants = (d?.variants ?? []).filter((v) => v.status === "active");
              const err = (field: string) => errors[`lines.${idx}.${field}`];
              return (
                <div key={row.key} className="rounded-lg border border-hairline p-4">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <p className="text-sm font-semibold text-ink">Baris #{idx + 1}</p>
                    <div className="flex items-center gap-3">
                      <span className="text-muted text-sm">
                        Estimasi:{" "}
                        <span className="font-semibold text-ink">
                          {formatIDR(estimate.perRow.get(row.key) ?? 0)}
                        </span>
                      </span>
                      {!readOnly && lines.length > 1 && (
                        <Button variant="ghost" size="sm" onClick={() => removeRow(row.key)}>
                          Hapus
                        </Button>
                      )}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3 md:grid-cols-6">
                    <div className="col-span-2">
                      <FormField label="Produk" required errorText={err("productId")}>
                        <SearchSelect
                          options={productOptions}
                          value={row.productId}
                          onChange={(v) =>
                            void handleProductChange(
                              row.key,
                              typeof v === "number" ? v : null,
                            )
                          }
                          placeholder="Pilih produk…"
                          allowClear
                          disabled={readOnly}
                        />
                      </FormField>
                    </div>
                    <div className="col-span-2 md:col-span-1">
                      <FormField label="Varian" required errorText={err("variantId")}>
                        <SelectInput
                          value={row.variantId ?? ""}
                          onChange={(e) =>
                            updateLine(row.key, {
                              variantId:
                                e.target.value === "" ? null : Number(e.target.value),
                            })
                          }
                          disabled={readOnly || row.productId === null || loadingDetail}
                        >
                          <option value="">
                            {loadingDetail
                              ? "Memuat…"
                              : row.productId === null
                                ? "Pilih produk dulu"
                                : variants.length === 0
                                  ? "Tidak ada varian aktif"
                                  : "Pilih varian…"}
                          </option>
                          {variants.map((v) => (
                            <option key={v.id} value={v.id}>
                              {v.code} — {v.name}
                            </option>
                          ))}
                        </SelectInput>
                      </FormField>
                    </div>
                    <FormField label="Satuan" required errorText={err("uom")}>
                      {d ? (
                        <SelectInput
                          value={row.uom}
                          onChange={(e) => updateLine(row.key, { uom: e.target.value })}
                          disabled={readOnly}
                        >
                          <option value="">Pilih…</option>
                          {uoms.map((u) => (
                            <option key={u} value={u}>
                              {u}
                            </option>
                          ))}
                        </SelectInput>
                      ) : (
                        <TextInput
                          value={row.uom}
                          placeholder="cth. pcs"
                          onChange={(e) => updateLine(row.key, { uom: e.target.value })}
                          disabled={readOnly}
                        />
                      )}
                    </FormField>
                    <FormField label="Qty" required errorText={err("qty")}>
                      <TextInput
                        type="number"
                        min={0}
                        value={row.qty === 0 ? "" : row.qty}
                        placeholder="0"
                        onChange={(e) =>
                          updateLine(row.key, { qty: toNumberInput(e.target.value) })
                        }
                        disabled={readOnly}
                      />
                    </FormField>
                    <FormField label="Harga Satuan" required errorText={err("unitPrice")}>
                      <TextInput
                        type="number"
                        min={0}
                        value={row.unitPrice === 0 ? "" : row.unitPrice}
                        placeholder={d ? String(d.purchasePrice) : "0"}
                        onChange={(e) =>
                          updateLine(row.key, { unitPrice: toNumberInput(e.target.value) })
                        }
                        disabled={readOnly}
                      />
                    </FormField>
                    <FormField label="Diskon %" errorText={err("discountPct")}>
                      <TextInput
                        type="number"
                        min={0}
                        max={100}
                        value={row.discountPct === 0 ? "" : row.discountPct}
                        placeholder="0"
                        onChange={(e) =>
                          updateLine(row.key, { discountPct: toNumberInput(e.target.value) })
                        }
                        disabled={readOnly}
                      />
                    </FormField>
                    <FormField label="Diskon Rp" errorText={err("discountNominal")}>
                      <TextInput
                        type="number"
                        min={0}
                        value={row.discountNominal === 0 ? "" : row.discountNominal}
                        placeholder="0"
                        onChange={(e) =>
                          updateLine(row.key, {
                            discountNominal: toNumberInput(e.target.value),
                          })
                        }
                        disabled={readOnly}
                      />
                    </FormField>
                  </div>
                </div>
              );
            })}
          </div>
        </SectionCard>

        <SectionCard title="Estimasi Total" description="Hitungan sementara di browser — total resmi dari server setelah simpan.">
          <dl className="max-w-md space-y-2 text-sm">
            <div className="flex justify-between">
              <dt className="text-muted">Subtotal (net)</dt>
              <dd className="font-medium text-ink">{formatIDR(estimate.subtotal)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-muted">Pajak</dt>
              <dd className="font-medium text-ink">{formatIDR(estimate.tax)}</dd>
            </div>
            <div className="flex justify-between border-t border-hairline pt-2 text-base">
              <dt className="font-semibold text-ink">Estimasi Grand Total</dt>
              <dd className="font-bold text-ink">{formatIDR(estimate.grand)}</dd>
            </div>
          </dl>
        </SectionCard>

        <ActionRow>
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            {readOnly ? "Kembali" : "Batal"}
          </Button>
          {!readOnly && (
            <Button onClick={() => void handleSubmit()} disabled={submitting}>
              {submitting ? "Menyimpan…" : isEdit ? "Simpan Perubahan" : "Buat Order"}
            </Button>
          )}
        </ActionRow>
      </div>
    </div>
  );
}
