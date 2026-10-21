import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import {
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
  SearchSelect,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { LineItemsEditor, Money, Page, StickyFooter, type LineColumn } from "@/shared/components/patterns";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import type { ApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useDraftAutosave, useDraftInitial } from "@/shared/hooks/use-form-draft";
import { clearDraft, draftKey } from "@/shared/lib/draft-storage";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
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
  PurchasePaymentTerms,
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
  discountReason: string;
  note: string;
  taxable: boolean;
  /** Tarif pajak baris (PO-03); null = tarif order. */
  taxRate: number | null;
}

const EMPTY_ROW: Omit<LineRow, "key"> = {
  productId: null,
  variantId: null,
  uom: "",
  qty: 1,
  unitPrice: 0,
  discountPct: 0,
  discountNominal: 0,
  discountReason: "",
  note: "",
  taxable: true,
  taxRate: null,
};

/** Galat sel per baris (SearchSelect/Select tidak punya prop error sendiri). */
function CellError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="mt-1 text-xs font-medium text-destructive">
      {message}
    </p>
  );
}

/** Alasan diskon + catatan baris: jarang dipakai, jadi dilipat di bawah sel produk. */
function LineDetail({
  row,
  readOnly,
  errors,
  onChange,
}: {
  row: LineRow;
  readOnly: boolean;
  errors: { discountReason?: string; note?: string };
  onChange: (patch: Partial<LineRow>) => void;
}) {
  const filled = row.discountReason.trim() !== "" || row.note.trim() !== "";
  const [open, setOpen] = useState<boolean | null>(null);
  const isOpen = open ?? (filled || Boolean(errors.discountReason || errors.note));
  return (
    <Collapsible open={isOpen} onOpenChange={setOpen} className="mt-1.5">
      <CollapsibleTrigger className="group inline-flex cursor-pointer items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground">
        <ChevronRight size={12} aria-hidden className="transition-transform group-data-[state=open]:rotate-90" />
        Alasan diskon &amp; catatan
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-1.5 flex flex-col gap-1.5">
        <TextInput
          aria-label="Alasan diskon"
          value={row.discountReason}
          maxLength={255}
          placeholder="Alasan diskon (opsional)"
          error={errors.discountReason}
          onChange={(e) => onChange({ discountReason: e.target.value })}
          disabled={readOnly}
        />
        <TextInput
          aria-label="Catatan baris"
          value={row.note}
          maxLength={255}
          placeholder="Catatan baris (opsional)"
          error={errors.note}
          onChange={(e) => onChange({ note: e.target.value })}
          disabled={readOnly}
        />
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Isian form tambah yang dipulihkan setelah sesi habis / tab tertutup. */
interface PurchaseFormDraft {
  partyId: number | null;
  orderDate: string;
  paymentTerms: PurchasePaymentTerms;
  dueDate: string;
  paymentTermDays: number;
  taxType: "none" | "include" | "exclude";
  taxRate: number;
  notes: string;
  supplierInvoiceNumber: string;
  supplierInvoiceDate: string;
  taxInvoiceNumber?: string;
  taxInvoiceDate?: string;
  assignedUserId?: number | null;
  lines: LineRow[];
}

function isPurchaseDraftEmpty(d: PurchaseFormDraft): boolean {
  return d.partyId === null && d.notes.trim() === "" && d.lines.every((l) => l.productId === null);
}

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
      f === "paymentTermDays" ||
      f === "taxType" ||
      f === "taxRate" ||
      f === "notes" ||
      f === "supplierInvoiceNumber" ||
      f === "supplierInvoiceDate" ||
      f === "taxInvoiceNumber" ||
      f === "taxInvoiceDate" ||
      f === "assignedUserId" ||
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
  const detailsRef = useRef<Record<number, PurchasingProductDetail>>({});
  const inflightRef = useRef(new Map<number, Promise<PurchasingProductDetail | undefined>>());

  const isEdit = id !== undefined;
  const orderId = isEdit ? Number(id) : NaN;
  const allowed = isEdit ? can("purchasing.update") : can("purchasing.create");

  // Draf hanya untuk form tambah (mode ubah memuat dari server).
  const userId = useAuthStore((s) => s.user?.id);
  const formDraftKey = isEdit ? null : draftKey(userId, "purchase-order-new");
  const draft = useDraftInitial<PurchaseFormDraft>(formDraftKey);
  const [draftRestored, setDraftRestored] = useState(draft !== null);
  // Kunci baris baru harus melewati kunci baris yang dipulihkan.
  const keySeq = useRef(Math.max(1, ...(draft?.lines ?? []).map((l) => l.key + 1)));

  const [partyId, setPartyId] = useState<number | null>(draft?.partyId ?? null);
  const [orderDate, setOrderDate] = useState(draft?.orderDate ?? "");
  const [paymentTerms, setPaymentTerms] = useState<PurchasePaymentTerms>(draft?.paymentTerms ?? "net");
  const [dueDate, setDueDate] = useState(draft?.dueDate ?? "");
  const [paymentTermDays, setPaymentTermDays] = useState(draft?.paymentTermDays ?? 0);
  const [taxType, setTaxType] = useState<"none" | "include" | "exclude">(draft?.taxType ?? "none");
  const [taxRate, setTaxRate] = useState(draft?.taxRate ?? 0);
  const [notes, setNotes] = useState(draft?.notes ?? "");
  const [supplierInvoiceNumber, setSupplierInvoiceNumber] = useState(draft?.supplierInvoiceNumber ?? "");
  const [supplierInvoiceDate, setSupplierInvoiceDate] = useState(draft?.supplierInvoiceDate ?? "");
  const [taxInvoiceNumber, setTaxInvoiceNumber] = useState(draft?.taxInvoiceNumber ?? "");
  const [taxInvoiceDate, setTaxInvoiceDate] = useState(draft?.taxInvoiceDate ?? "");
  const [assignedUserId, setAssignedUserId] = useState<number | null>(draft?.assignedUserId ?? null);
  const [lines, setLines] = useState<LineRow[]>(
    draft && draft.lines.length > 0 ? draft.lines : [{ ...EMPTY_ROW, key: 0 }],
  );

  const [details, setDetails] = useState<Record<number, PurchasingProductDetail>>({});
  const [detailLoading, setDetailLoading] = useState<Record<number, boolean>>({});

  const [loaded, setLoaded] = useState<PurchaseOrder | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverMessages, setServerMessages] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  // null = otomatis (terbuka bila ada isi/galat); true/false = pilihan pengguna.
  const [detailOpenState, setDetailOpenState] = useState<boolean | null>(null);

  const readOnly = isEdit && loaded !== null && loaded.status !== "draft";
  usePageCrumb(isEdit ? loaded?.number : undefined);

  const draftSnapshot = useMemo<PurchaseFormDraft>(
    () => ({
      partyId,
      orderDate,
      paymentTerms,
      dueDate,
      paymentTermDays,
      taxType,
      taxRate,
      notes,
      supplierInvoiceNumber,
      supplierInvoiceDate,
      taxInvoiceNumber,
      taxInvoiceDate,
      assignedUserId,
      lines,
    }),
    [partyId, orderDate, paymentTerms, dueDate, paymentTermDays, taxType, taxRate, notes, supplierInvoiceNumber, supplierInvoiceDate, taxInvoiceNumber, taxInvoiceDate, assignedUserId, lines],
  );
  useDraftAutosave(formDraftKey, draftSnapshot, isPurchaseDraftEmpty);

  function discardDraft() {
    if (formDraftKey !== null) clearDraft(formDraftKey);
    setDraftRestored(false);
    setPartyId(null);
    setOrderDate("");
    setPaymentTerms("net");
    setDueDate("");
    setPaymentTermDays(0);
    setTaxType("none");
    setTaxRate(0);
    setNotes("");
    setSupplierInvoiceNumber("");
    setSupplierInvoiceDate("");
    setTaxInvoiceNumber("");
    setTaxInvoiceDate("");
    setAssignedUserId(null);
    setLines([{ ...EMPTY_ROW, key: 0 }]);
  }

  // Kunci mode form: 0 = tambah, -1 = id tak valid, >0 = id order edit.
  const validId = !isEdit || (Number.isInteger(orderId) && orderId > 0);
  const formKey = !isEdit ? 0 : !validId ? -1 : orderId;

  // --- lookups ---

  // Supplier dicari server-side (PartySearchSelect); hanya opsi produk yang
  // dimuat di muka.
  const { data: lookupData } = useAsyncData(
    () =>
      purchasingLookupService.productOptions().then(
        (opts): SearchSelectOption[] =>
          opts.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
        (): SearchSelectOption[] => [],
      ),
    [],
  );
  const productOptions = lookupData ?? [];

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

  // Draf dipulihkan: muat detail produk tiap baris agar pilihan satuan/varian terisi.
  useEffect(() => {
    const ids = new Set<number>();
    for (const l of draft?.lines ?? []) if (l.productId !== null) ids.add(l.productId);
    if (ids.size === 0) return;
    // Ditunda satu tick: ensureDetail mengubah state, jangan sinkron di effect.
    void Promise.resolve().then(() => {
      for (const pid of ids) void ensureDetail(pid);
    });
    // Hanya sekali saat mount untuk draf awal.
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
          paymentTerms: (po.paymentTerms === "cod" || po.paymentTerms === "prepaid"
            ? po.paymentTerms
            : "net") as PurchasePaymentTerms,
          dueDate: po.dueDate ? po.dueDate.slice(0, 10) : "",
          paymentTermDays: po.paymentTermDays ?? 0,
          taxType: (
            po.taxType === "include" || po.taxType === "exclude" ? po.taxType : "none"
          ) as "none" | "include" | "exclude",
          taxRate: po.taxRate ?? 0,
          notes: po.notes ?? "",
          supplierInvoiceNumber: po.supplierInvoiceNumber ?? "",
          supplierInvoiceDate: po.supplierInvoiceDate ?? "",
          taxInvoiceNumber: po.taxInvoiceNumber ?? "",
          taxInvoiceDate: po.taxInvoiceDate ? po.taxInvoiceDate.slice(0, 10) : "",
          assignedUserId: po.assignedUserId > 0 ? po.assignedUserId : null,
          lines: po.items.map((it) => ({
            key: keySeq.current++,
            productId: it.productId,
            variantId: it.variantId,
            uom: it.uom,
            qty: it.qty,
            unitPrice: it.unitPrice,
            discountPct: it.discountPct,
            discountNominal: it.discountNominal,
            discountReason: it.discountReason ?? "",
            note: it.note ?? "",
            taxable: it.taxable !== false,
            // Tarif baris hanya ditampilkan bila beda dari tarif order.
            taxRate: it.taxable !== false && it.taxRate > 0 && it.taxRate !== po.taxRate ? it.taxRate : null,
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
    setPaymentTermDays(0);
    setTaxType("none");
    setTaxRate(0);
    setNotes("");
    setSupplierInvoiceNumber("");
    setSupplierInvoiceDate("");
    setTaxInvoiceNumber("");
    setTaxInvoiceDate("");
    setAssignedUserId(null);
    setLines([{ ...EMPTY_ROW, key: 0 }]);
    setLoaded(null);
    setErrors({});
    setServerMessages([]);
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editBundle && editBundle.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setLoaded(editBundle.loaded);
    setPartyId(editBundle.partyId);
    setOrderDate(editBundle.orderDate);
    setPaymentTerms(editBundle.paymentTerms);
    setDueDate(editBundle.dueDate);
    setPaymentTermDays(editBundle.paymentTermDays);
    setTaxType(editBundle.taxType);
    setTaxRate(editBundle.taxRate);
    setNotes(editBundle.notes);
    setSupplierInvoiceNumber(editBundle.supplierInvoiceNumber);
    setSupplierInvoiceDate(editBundle.supplierInvoiceDate);
    setTaxInvoiceNumber(editBundle.taxInvoiceNumber);
    setTaxInvoiceDate(editBundle.taxInvoiceDate);
    setAssignedUserId(editBundle.assignedUserId);
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
        l.taxRate ?? taxRate,
        l.taxable,
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
      paymentTermDays,
      supplierInvoiceDate,
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
        discountReason: l.discountReason,
        note: l.note,
        taxable: l.taxable,
        ...(l.taxRate !== null && taxType !== "none" ? { taxRate: l.taxRate } : {}),
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
        paymentTermDays: paymentTermDays > 0 ? paymentTermDays : undefined,
        taxType,
        taxRate,
        notes: notes.trim() === "" ? undefined : notes.trim(),
        supplierInvoiceNumber:
          supplierInvoiceNumber.trim() === ""
            ? undefined
            : supplierInvoiceNumber.trim(),
        supplierInvoiceDate:
          supplierInvoiceDate.trim() === "" ? undefined : supplierInvoiceDate.trim(),
        taxInvoiceNumber: taxInvoiceNumber.trim() === "" ? undefined : taxInvoiceNumber.trim(),
        taxInvoiceDate: taxInvoiceDate.trim() === "" ? undefined : taxInvoiceDate.trim(),
        // 0 saat diedit = hapus penanggung jawab.
        assignedUserId: assignedUserId ?? 0,
        items: lines.map((l) => ({
          productId: (l.productId as number | null) ?? 0,
          variantId: (l.variantId as number | null) ?? 0,
          uom: l.uom.trim(),
          qty: l.qty,
          unitPrice: l.unitPrice,
          discountPct: l.discountPct,
          discountNominal: l.discountNominal,
          ...(l.discountReason.trim() !== "" ? { discountReason: l.discountReason.trim() } : {}),
          ...(l.note.trim() !== "" ? { note: l.note.trim() } : {}),
          taxable: l.taxable,
          ...(l.taxRate !== null && taxType !== "none" ? { taxRate: l.taxRate } : {}),
        })),
      };
      if (isEdit) {
        const res = await purchaseOrderService.update(orderId, body);
        toast.fromServer(res.message, "Purchase order disimpan", res.data.number);
        void navigate(`${ROUTE_PATHS.purchaseOrders}/${res.data.id}`);
      } else {
        const res = await purchaseOrderService.create(body);
        toast.fromServer(res.message, "Purchase order dibuat", res.data.number);
        if (formDraftKey !== null) clearDraft(formDraftKey);
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
      <Page>
        <PageHeader title={isEdit ? "Ubah order beli" : "Order beli baru"} />
        <Notice tone="danger" title="Akses ditolak">
          Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "membuat"} order beli.
        </Notice>
        <div>
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            Kembali ke daftar
          </Button>
        </div>
      </Page>
    );
  }

  if (loading) {
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  }

  if (loadError) {
    return (
      <Page>
        <PageHeader title="Ubah order beli" />
        <ErrorState message={loadError} />
        <div>
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            Kembali ke daftar
          </Button>
        </div>
      </Page>
    );
  }

  const err = (field: string) => errors[field];
  const errorCount = Object.keys(errors).length;
  const detailHasValue =
    paymentTermDays > 0 ||
    supplierInvoiceNumber.trim() !== "" ||
    supplierInvoiceDate.trim() !== "" ||
    taxInvoiceNumber.trim() !== "" ||
    taxInvoiceDate.trim() !== "" ||
    assignedUserId !== null ||
    notes.trim() !== "";
  const detailHasError = Boolean(
    errors.paymentTermDays ||
      errors.supplierInvoiceNumber ||
      errors.supplierInvoiceDate ||
      errors.taxInvoiceNumber ||
      errors.taxInvoiceDate ||
      errors.assignedUserId ||
      errors.notes,
  );
  const detailOpen = detailOpenState ?? (detailHasValue || detailHasError);
  const showDraftIndicator = !isEdit && !isPurchaseDraftEmpty(draftSnapshot);

  const columns: LineColumn<LineRow>[] = [
    {
      id: "product",
      header: "Produk",
      width: "18rem",
      render: (row, idx) => {
        const d = row.productId !== null ? details[row.productId] : undefined;
        const loadingDetail = row.productId !== null && detailLoading[row.productId] === true && !d;
        const variants = (d?.variants ?? []).filter((v) => v.status === "active");
        const showVariant =
          row.productId !== null && (loadingDetail || variants.length !== 1 || Boolean(err(`lines.${idx}.variantId`)));
        return (
          <div className="min-w-0">
            <SearchSelect
              aria-label={`Produk baris ${idx + 1}`}
              options={productOptions}
              value={row.productId}
              onChange={(v) => void handleProductChange(row.key, typeof v === "number" ? v : null)}
              placeholder="Cari produk…"
              allowClear
              disabled={readOnly}
            />
            <CellError message={err(`lines.${idx}.productId`)} />
            {showVariant && (
              <div className="mt-1.5">
                <SelectInput
                  aria-label={`Varian baris ${idx + 1}`}
                  value={row.variantId ?? ""}
                  onChange={(e) =>
                    updateLine(row.key, { variantId: e.target.value === "" ? null : Number(e.target.value) })
                  }
                  disabled={readOnly || loadingDetail}
                >
                  <option value="">
                    {loadingDetail ? "Mengambil varian…" : variants.length === 0 ? "Tidak ada varian aktif" : "Pilih varian…"}
                  </option>
                  {variants.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.code} — {v.name}
                    </option>
                  ))}
                </SelectInput>
                <CellError message={err(`lines.${idx}.variantId`)} />
              </div>
            )}
            <LineDetail
              row={row}
              readOnly={readOnly}
              errors={{
                discountReason: err(`lines.${idx}.discountReason`),
                note: err(`lines.${idx}.note`),
              }}
              onChange={(patch) => updateLine(row.key, patch)}
            />
          </div>
        );
      },
    },
    {
      id: "uom",
      header: "Satuan",
      width: "7rem",
      render: (row, idx) => {
        const d = row.productId !== null ? details[row.productId] : undefined;
        const uoms = d
          ? [d.purchaseUom, d.baseUom, d.salesUom].filter((u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i)
          : [];
        return d ? (
          <>
            <SelectInput
              aria-label={`Satuan baris ${idx + 1}`}
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
            <CellError message={err(`lines.${idx}.uom`)} />
          </>
        ) : (
          <TextInput
            aria-label={`Satuan baris ${idx + 1}`}
            value={row.uom}
            placeholder="cth. pcs"
            error={err(`lines.${idx}.uom`)}
            onChange={(e) => updateLine(row.key, { uom: e.target.value })}
            disabled={readOnly}
          />
        );
      },
    },
    {
      id: "qty",
      header: "Qty",
      width: "6rem",
      align: "right",
      render: (row, idx) => (
        <TextInput
          type="number"
          aria-label={`Qty baris ${idx + 1}`}
          min={0}
          className="text-right"
          value={row.qty === 0 ? "" : row.qty}
          placeholder="0"
          error={err(`lines.${idx}.qty`)}
          onChange={(e) => updateLine(row.key, { qty: toNumberInput(e.target.value) })}
          disabled={readOnly}
        />
      ),
    },
    {
      id: "price",
      header: "Harga satuan",
      width: "9rem",
      align: "right",
      render: (row, idx) => {
        const d = row.productId !== null ? details[row.productId] : undefined;
        return (
          <TextInput
            type="number"
            aria-label={`Harga satuan baris ${idx + 1}`}
            min={0}
            className="text-right"
            value={row.unitPrice === 0 ? "" : row.unitPrice}
            placeholder={d ? String(d.purchasePrice) : "0"}
            error={err(`lines.${idx}.unitPrice`)}
            onChange={(e) => updateLine(row.key, { unitPrice: toNumberInput(e.target.value) })}
            disabled={readOnly}
          />
        );
      },
    },
    {
      id: "discount",
      header: "Diskon % / Rp",
      width: "9rem",
      align: "right",
      render: (row, idx) => (
        <div className="flex flex-col gap-1.5">
          <TextInput
            type="number"
            aria-label={`Diskon persen baris ${idx + 1}`}
            min={0}
            max={100}
            className="text-right"
            value={row.discountPct === 0 ? "" : row.discountPct}
            placeholder="0 %"
            error={err(`lines.${idx}.discountPct`)}
            onChange={(e) => updateLine(row.key, { discountPct: toNumberInput(e.target.value) })}
            disabled={readOnly}
          />
          <TextInput
            type="number"
            aria-label={`Diskon rupiah baris ${idx + 1}`}
            min={0}
            className="text-right"
            value={row.discountNominal === 0 ? "" : row.discountNominal}
            placeholder="Rp 0"
            error={err(`lines.${idx}.discountNominal`)}
            onChange={(e) => updateLine(row.key, { discountNominal: toNumberInput(e.target.value) })}
            disabled={readOnly}
          />
        </div>
      ),
    },
    {
      id: "taxable",
      header: "Kena pajak",
      width: "5.5rem",
      align: "center",
      render: (row, idx) => (
        <span className="inline-flex h-9 items-center">
          <Checkbox
            aria-label={`Kena pajak baris ${idx + 1}`}
            checked={row.taxable}
            onCheckedChange={(v) => updateLine(row.key, { taxable: v === true })}
            disabled={readOnly}
          />
        </span>
      ),
    },
    {
      id: "taxRate",
      header: "Tarif %",
      width: "5.5rem",
      align: "right",
      render: (row, idx) => (
        <TextInput
          type="number"
          aria-label={`Tarif pajak baris ${idx + 1}`}
          min={0}
          max={100}
          step="any"
          className="text-right"
          value={row.taxRate === null ? "" : row.taxRate}
          placeholder={taxType === "none" ? "-" : String(taxRate)}
          error={err(`lines.${idx}.taxRate`)}
          onChange={(e) =>
            updateLine(row.key, { taxRate: e.target.value.trim() === "" ? null : toNumberInput(e.target.value) })
          }
          disabled={readOnly || taxType === "none" || !row.taxable}
        />
      ),
    },
    {
      id: "total",
      header: "Estimasi total",
      width: "9rem",
      align: "right",
      render: (row) => (
        <span className="inline-flex h-9 items-center font-medium">
          <Money value={estimate.perRow.get(row.key) ?? 0} />
        </span>
      ),
    },
  ];

  return (
    <Page>
      <PageHeader
        title={isEdit ? `Ubah order ${loaded?.number ?? ""}`.trim() : "Order beli baru"}
        description={
          isEdit ? "Hanya order berstatus draf yang bisa diubah." : "Harga dan pajak final dihitung server saat simpan."
        }
        actions={
          showDraftIndicator ? (
            <span className="inline-flex items-center gap-2 text-xs text-muted-foreground">
              {draftRestored ? "Draf dipulihkan dan tersimpan otomatis" : "Draf tersimpan otomatis"}
              <Button variant="ghost" size="sm" onClick={discardDraft}>
                Buang draf
              </Button>
            </span>
          ) : undefined
        }
      />

      {readOnly && (
        <Notice tone="warning" title="Order tidak bisa diubah">
          Hanya order berstatus draf yang bisa diubah. Detail bersifat baca-saja.
        </Notice>
      )}

      {serverMessages.length > 0 && (
        <Notice tone="danger" title="Gagal menyimpan">
          <ul className="list-disc pl-5">
            {serverMessages.map((m, i) => (
              <li key={i}>{m}</li>
            ))}
          </ul>
        </Notice>
      )}

      <SectionCard title="Dokumen" description="Pemasok dan ketentuan order.">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <FormField label="Pemasok" required errorText={err("partyId")}>
            <PartySearchSelect
              partyKind="supplier"
              value={partyId}
              onChange={(id) => setPartyId(id)}
              placeholder="Cari pemasok (nama, kode, telepon)…"
              disabled={readOnly}
            />
          </FormField>
          <FormField label="Tanggal order" errorText={err("orderDate")}>
            <DateInput value={orderDate} onChange={(e) => setOrderDate(e.target.value)} disabled={readOnly} />
          </FormField>
          <FormField
            label="Termin"
            required
            errorText={err("paymentTerms")}
            helperText={paymentTerms === "prepaid" ? "Penerimaan barang baru bisa dicatat setelah PO dibayar lunas." : undefined}
          >
            <SelectInput
              value={paymentTerms}
              onChange={(e) => {
                const v: PurchasePaymentTerms =
                  e.target.value === "cod" || e.target.value === "prepaid" ? e.target.value : "net";
                setPaymentTerms(v);
                if (v !== "net") {
                  setDueDate("");
                  setPaymentTermDays(0);
                }
              }}
              disabled={readOnly}
            >
              <option value="net">Net (tempo)</option>
              <option value="cod">COD (tunai)</option>
              <option value="prepaid">Prabayar (bayar dulu)</option>
            </SelectInput>
          </FormField>
          <FormField label="Jatuh tempo" required={paymentTerms === "net"} errorText={err("dueDate")}>
            <DateInput
              value={dueDate}
              onChange={(e) => setDueDate(e.target.value)}
              disabled={readOnly || paymentTerms !== "net"}
            />
          </FormField>
          <FormField label="Pajak" required errorText={err("taxType")}>
            <SelectInput
              value={taxType}
              onChange={(e) => {
                const v =
                  e.target.value === "include" || e.target.value === "exclude" ? e.target.value : "none";
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
          <FormField label="Tarif pajak (%)" errorText={err("taxRate")}>
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
        </div>

        <Collapsible open={detailOpen} onOpenChange={setDetailOpenState} className="mt-4 border-t border-border pt-3">
          <CollapsibleTrigger className="group inline-flex cursor-pointer items-center gap-1 text-sm font-medium text-muted-foreground hover:text-foreground">
            <ChevronRight size={14} aria-hidden className="transition-transform group-data-[state=open]:rotate-90" />
            Detail lain (masa tempo, faktur pemasok, catatan)
          </CollapsibleTrigger>
          <CollapsibleContent className="mt-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {paymentTerms === "net" && (
                <FormField
                  label="Masa tempo (hari)"
                  errorText={err("paymentTermDays")}
                  helperText="Opsional: dengan tanggal faktur pemasok, jatuh tempo dihitung server bila kosong."
                >
                  <TextInput
                    type="number"
                    min={0}
                    max={3650}
                    step={1}
                    value={paymentTermDays === 0 ? "" : paymentTermDays}
                    placeholder="0"
                    onChange={(e) => setPaymentTermDays(toNumberInput(e.target.value))}
                    disabled={readOnly}
                  />
                </FormField>
              )}
              <FormField label="No. faktur pemasok" errorText={err("supplierInvoiceNumber")}>
                <TextInput
                  value={supplierInvoiceNumber}
                  placeholder="Opsional"
                  onChange={(e) => setSupplierInvoiceNumber(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
              <FormField label="Tanggal faktur pemasok" errorText={err("supplierInvoiceDate")}>
                <DateInput
                  value={supplierInvoiceDate}
                  onChange={(e) => setSupplierInvoiceDate(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
              <FormField label="No. faktur pajak" errorText={err("taxInvoiceNumber")}>
                <TextInput
                  value={taxInvoiceNumber}
                  placeholder="Opsional"
                  onChange={(e) => setTaxInvoiceNumber(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
              <FormField label="Tanggal faktur pajak" errorText={err("taxInvoiceDate")}>
                <DateInput
                  value={taxInvoiceDate}
                  onChange={(e) => setTaxInvoiceDate(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
              {can("users.view") ? (
                <FormField label="Penanggung jawab" errorText={err("assignedUserId")}>
                  <UserSearchSelect
                    value={assignedUserId}
                    onChange={(uid) => setAssignedUserId(uid)}
                    placeholder="Tanpa penanggung jawab"
                    disabled={readOnly}
                  />
                </FormField>
              ) : null}
              <div className="sm:col-span-2 lg:col-span-4">
                <FormField label="Catatan" errorText={err("notes")}>
                  <TextArea
                    rows={2}
                    value={notes}
                    placeholder="Catatan untuk pemasok (opsional)"
                    onChange={(e) => setNotes(e.target.value)}
                    disabled={readOnly}
                  />
                </FormField>
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>
      </SectionCard>

      <SectionCard
        title="Barang"
        description="Harga per baris mengikuti penawaran pemasok. Server menghitung ulang total resmi saat simpan."
      >
        {err("lines") && <Notice tone="danger" title={err("lines")} className="mb-4" />}
        <LineItemsEditor<LineRow>
          lines={lines}
          getKey={(l) => l.key}
          columns={columns}
          readOnly={readOnly}
          addLabel="Tambah baris"
          onAdd={addRow}
          onRemove={(i) => {
            const row = lines[i];
            if (!row) return;
            if (lines.length > 1) removeRow(row.key);
            else updateLine(row.key, { ...EMPTY_ROW });
          }}
        />
      </SectionCard>

      <StickyFooter
        summary={
          <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <span>{lines.filter((l) => l.productId !== null).length} barang</span>
            <span>
              Subtotal <Money value={estimate.subtotal} className="font-medium text-foreground" />
            </span>
            <span>
              Pajak <Money value={estimate.tax} className="font-medium text-foreground" />
            </span>
            <span>
              Estimasi total <Money value={estimate.grand} className="text-base font-semibold text-foreground" />
            </span>
            {errorCount > 0 && (
              <button
                type="button"
                className="cursor-pointer font-medium text-destructive hover:underline"
                onClick={() =>
                  document.querySelector('[role="alert"]')?.scrollIntoView({ block: "center", behavior: "smooth" })
                }
              >
                {errorCount} isian perlu diperbaiki
              </button>
            )}
          </span>
        }
      >
        <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
          {readOnly ? "Kembali" : "Batal"}
        </Button>
        {!readOnly && (
          <Button onClick={() => void handleSubmit()} loading={submitting}>
            {isEdit ? "Simpan perubahan" : "Buat order"}
          </Button>
        )}
      </StickyFooter>
    </Page>
  );
}
