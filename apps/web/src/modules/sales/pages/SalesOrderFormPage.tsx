import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Check, ChevronRight, ScanLine, UserPlus } from "lucide-react";
import {
  AsyncSearchSelect,
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
  SearchSelect,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { AsyncSearchOption, SearchSelectOption } from "@/shared/components/ui";
import { LineItemsEditor, Money, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber, statusLabel, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useDraftAutosave, useDraftInitial } from "@/shared/hooks/use-form-draft";
import { clearDraft, draftKey } from "@/shared/lib/draft-storage";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesOrderSchema } from "@/modules/sales/schemas/sales-order.schema";
import {
  productOptionService,
  salesService,
} from "@/modules/sales/services/sales.service";
import type { PaymentTerms, SalesOrderPayload } from "@/modules/sales/types";
import { pricingSourceLabel } from "@/modules/sales/types";
import { belowMinPrice, estimateLine } from "@/modules/sales/lib/estimate";
import {
  DEFAULT_VAT_RATE,
  estimateTax,
  normalizeTaxType,
  type SalesTaxType,
} from "@/modules/sales/lib/tax";
import { splitItemErrors } from "@/modules/sales/lib/server-errors";
import type { ItemErrors } from "@/modules/sales/lib/server-errors";
import { parseManualPrice, toLinePayload } from "@/modules/sales/lib/payload";
import { customerService } from "@/modules/party/services/party.service";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";
import { QuickCustomerSheet } from "@/modules/party/components/QuickCustomerSheet";
import type { Party } from "@/modules/party/types";
import type { Address } from "@/modules/party/types";
import { productsService } from "@/modules/products/services/products.service";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import type { ScanOutcome } from "@/modules/pos/lib/scan-session";
import type { Product } from "@/modules/products/types";

/** Satu baris editor. uoms/variants difoto dari detail produk saat baris
 * ditambah (dipilih ulang per baris), catalogPrice = harga katalog/server
 * untuk estimasi klien — final selalu dihitung ulang server. */
interface LineRow {
  key: number;
  productId: number;
  productLabel: string;
  variantId: number;
  uom: string;
  qty: number;
  discountPct: number;
  discountNominal: number;
  catalogPrice: number;
  /** Harga jual minimum produk per satuan (0/kosong = tanpa minimum); bantuan tampilan saja. */
  minPrice?: number;
  /** Input harga manual (string agar kosong = harga server). */
  manualPrice: string;
  discountReason: string;
  note: string;
  taxable: boolean;
  /** Sumber harga tersimpan (hanya saat edit): catalog | member_rule | manual. */
  pricingSource: string;
  uoms: string[];
  variants: { id: number; code: string; name: string }[];
}

/** Alamat kirim sekali pakai: disalin ke order, tidak masuk buku alamat kecuali dicentang. */
interface ShipAdHoc {
  on: boolean;
  label: string;
  recipient: string;
  phone: string;
  address: string;
  saveToBook: boolean;
}

const EMPTY_AD_HOC: ShipAdHoc = { on: false, label: "", recipient: "", phone: "", address: "", saveToBook: false };

let nextKey = 1;

/** Isian form tambah yang dipulihkan setelah sesi habis / tab tertutup. */
interface SalesFormDraft {
  partyId: number | null;
  pickedCustomer: SearchSelectOption | null;
  orderDate: string;
  paymentTerms: PaymentTerms;
  dueDate: string;
  notes: string;
  shipToAddressId: number | null;
  /** Alamat sekali pakai (SL-01); opsional agar draf lama tetap terbaca. */
  shipAdHoc?: ShipAdHoc;
  assignedUserId?: number | null;
  taxInvoiceNumber: string;
  taxInvoiceDate: string;
  /** Opsional: draf lama (sebelum kontrol PPN) tidak membawanya. */
  taxType?: SalesTaxType;
  taxRate?: number;
  lines: LineRow[];
}

function isSalesDraftEmpty(d: SalesFormDraft): boolean {
  return d.lines.length === 0 && d.partyId === null && d.notes.trim() === "";
}

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `so-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

/** Estimasi satu baris editor (harga manual bila valid, selain itu katalog). */
function rowEstimate(r: LineRow): number {
  const mp = parseManualPrice(r.manualPrice);
  return estimateLine({
    qty: r.qty,
    catalogPrice: r.catalogPrice,
    manualPrice: mp === null || Number.isNaN(mp) ? null : mp,
    discountPct: r.discountPct,
    discountNominal: r.discountNominal,
  });
}

function uniqueUoms(p: Product): string[] {
  return [p.salesUom, p.purchaseUom, p.baseUom].filter(
    (u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i,
  );
}

export default function SalesOrderFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const isEdit = id !== undefined;
  const orderId = isEdit ? Number(id) : NaN;
  const invalidId = isEdit && (!Number.isInteger(orderId) || orderId <= 0);

  const allowed = isEdit ? can("sales.update") : can("sales.create");

  // Draf hanya untuk form tambah (mode ubah memuat dari server).
  const userId = useAuthStore((s) => s.user?.id);
  const formDraftKey = isEdit ? null : draftKey(userId, "sales-order-new");
  const draft = useDraftInitial<SalesFormDraft>(formDraftKey);
  const [draftRestored, setDraftRestored] = useState(draft !== null);

  const [partyId, setPartyId] = useState<number | null>(draft?.partyId ?? null);
  // Pelanggan terpilih (hasil pencarian server) — dipakai untuk telepon (termin net).
  const [pickedParty, setPickedParty] = useState<Party | null>(null);
  const [pickedCustomer, setPickedCustomer] = useState<SearchSelectOption | null>(draft?.pickedCustomer ?? null);
  const [orderDate, setOrderDate] = useState(draft?.orderDate || todayWIB());
  const [paymentTerms, setPaymentTerms] = useState<PaymentTerms>(draft?.paymentTerms ?? "cod");
  // Satu UUID per percobaan simpan baru: klik ganda/retry = replay, bukan order ganda.
  const [clientRef, setClientRef] = useState(newClientRef);
  const [dueDate, setDueDate] = useState(draft?.dueDate ?? "");
  const [notes, setNotes] = useState(draft?.notes ?? "");
  // PPN per header (F-01): kena pajak? harga sudah termasuk pajak? tarif. Edit memuat
  // nilai order; order lama tanpa pajak tetap "none". Per baris tetap bisa dikecualikan.
  const [taxType, setTaxType] = useState<SalesTaxType>(normalizeTaxType(draft?.taxType));
  const [taxRate, setTaxRate] = useState(draft?.taxType && draft.taxType !== "none" ? (draft.taxRate ?? 0) : 0);
  // Alamat kirim (buku alamat customer) + nomor/tanggal faktur pajak.
  // shipToAddressId null = tanpa alamat kirim (dikirim 0 ke backend).
  const [shipToAddressId, setShipToAddressId] = useState<number | null>(draft?.shipToAddressId ?? null);
  const [shipAdHoc, setShipAdHoc] = useState<ShipAdHoc>(draft?.shipAdHoc ?? EMPTY_AD_HOC);
  const [assignedUserId, setAssignedUserId] = useState<number | null>(draft?.assignedUserId ?? null);
  const [quickCustomerOpen, setQuickCustomerOpen] = useState(false);
  const [taxInvoiceNumber, setTaxInvoiceNumber] = useState(draft?.taxInvoiceNumber ?? "");
  const [taxInvoiceDate, setTaxInvoiceDate] = useState(draft?.taxInvoiceDate ?? "");

  const [lines, setLines] = useState<LineRow[]>(() => {
    const restored = draft?.lines ?? [];
    // Kunci baris baru harus melewati kunci baris yang dipulihkan.
    for (const r of restored) nextKey = Math.max(nextKey, r.key + 1);
    return restored;
  });
  const [addingLine, setAddingLine] = useState(false);
  const [scanOpen, setScanOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  // Pesan API asli untuk galat simpan (tampil sebagai Notice di atas form).
  const [submitError, setSubmitError] = useState<string | null>(null);

  const [errors, setErrors] = useState<Record<string, string>>({});
  // Galat item dari server: semua pesan "baris N: ..." (harga minimum, dll.), per baris.
  const [itemErrors, setItemErrors] = useState<ItemErrors>({ general: [], byRow: {} });
  const [loadedStatus, setLoadedStatus] = useState<string>("draft");
  // Mode ubah dari server (GET detail editMode) dan kanal asli order: order
  // terkonfirmasi tanpa dokumen turunan boleh dikoreksi lewat form lengkap.
  const [loadedEditMode, setLoadedEditMode] = useState<string>("full");
  const [loadedChannel, setLoadedChannel] = useState<string>("regular");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const correcting = isEdit && loadedStatus === "confirmed" && loadedEditMode === "full";
  const readOnly = isEdit && loadedStatus !== "draft" && !correcting;

  const draftSnapshot = useMemo<SalesFormDraft>(
    () => ({
      partyId,
      pickedCustomer,
      orderDate,
      paymentTerms,
      dueDate,
      notes,
      shipToAddressId,
      shipAdHoc,
      assignedUserId,
      taxInvoiceNumber,
      taxInvoiceDate,
      taxType,
      taxRate,
      lines,
    }),
    [partyId, pickedCustomer, orderDate, paymentTerms, dueDate, notes, shipToAddressId, shipAdHoc, assignedUserId, taxInvoiceNumber, taxInvoiceDate, taxType, taxRate, lines],
  );
  useDraftAutosave(formDraftKey, draftSnapshot, isSalesDraftEmpty);

  function discardDraft() {
    if (formDraftKey !== null) clearDraft(formDraftKey);
    setDraftRestored(false);
    setPartyId(null);
    setPickedCustomer(null);
    setOrderDate(todayWIB());
    setPaymentTerms("cod");
    setDueDate("");
    setNotes("");
    setShipToAddressId(null);
    setShipAdHoc(EMPTY_AD_HOC);
    setAssignedUserId(null);
    setTaxInvoiceNumber("");
    setTaxInvoiceDate("");
    setTaxType("none");
    setTaxRate(0);
    setLines([]);
  }

  // Kunci mode form: 0 = tambah, -1 = id tak valid, >0 = id order edit.
  const formKey = !isEdit ? 0 : invalidId ? -1 : orderId;

  // --- buku alamat customer: dimuat ulang tiap ganti customer ---
  // addressView backend = {id, label, recipient, phone, text, isPrimary,
  // sortOrder, status} (party/presentation/handler.go: addressView).
  const { data: addrRows, loading: addrLoadingRaw } = useAsyncData(
    () =>
      partyId === null
        ? Promise.resolve([] as Address[])
        : customerService.listAddresses(partyId).then(
            (rows) => rows,
            (): Address[] => [],
          ),
    [partyId],
  );
  const shipAddresses = partyId === null ? [] : (addrRows ?? []);
  const addrLoading = partyId === null ? false : addrLoadingRaw;

  // --- opsi produk: pencarian server-side /products/search-options?q= ---
  const loadProductOptions = (search: string): Promise<AsyncSearchOption[]> =>
    productOptionService.search(search).then(
      (list) => list.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
      (): AsyncSearchOption[] => [],
    );

  // --- prefill edit ---
  const { data: editBundle, loading: editLoading, error: editFetchError } = useAsyncData(
    () => {
      if (!isEdit || invalidId) return Promise.resolve(null);
      const key = formKey;
      return (async () => {
        const o = await salesService.get(orderId);
        // Ambil detail produk per baris untuk opsi varian/satuan.
        const uniqueIds = [...new Set(o.items.map((it) => it.productId))];
        const details = new Map<number, Product | null>();
        await Promise.all(
          uniqueIds.map(async (pid) => {
            try {
              details.set(pid, await productsService.detail(pid));
            } catch {
              details.set(pid, null);
            }
          }),
        );
        // Tanam opsi customer terpilih agar SearchSelect menampilkan nama.
        let customerOption: SearchSelectOption | null = null;
        let customerPhone = "";
        if (o.partyId > 0) {
          try {
            const c = await customerService.get(o.partyId);
            customerOption = { value: c.id, label: `${c.code} — ${c.name}` };
            customerPhone = c.phone ?? "";
          } catch {
            customerOption = { value: o.partyId, label: o.partyName || `Customer #${o.partyId}` };
          }
        }
        return {
          key,
          loadedStatus: o.status,
          loadedEditMode: o.editMode ?? (o.status === "draft" ? "full" : "none"),
          loadedChannel: o.channel || "regular",
          partyId: o.partyId > 0 ? o.partyId : null,
          orderDate: o.orderDate || todayWIB(),
          paymentTerms: (o.paymentTerms === "net" || o.paymentTerms === "prepaid"
            ? o.paymentTerms
            : "cod") as PaymentTerms,
          dueDate: o.dueDate || "",
          notes: o.notes || "",
          taxType: o.taxType || "none",
          taxRate: o.taxRate || 0,
          shipToAddressId: o.shipToAddressId > 0 ? o.shipToAddressId : null,
          // Alamat tersimpan di order tanpa id buku alamat = alamat sekali pakai.
          assignedUserId: (o.assignedUserId ?? 0) > 0 ? (o.assignedUserId ?? null) : null,
          shipAdHoc:
            o.shipToAddressId <= 0 && (o.shipToAddress ?? "").trim() !== ""
              ? {
                  on: true,
                  label: o.shipToLabel || "",
                  recipient: o.shipToRecipient || "",
                  phone: o.shipToPhone || "",
                  address: o.shipToAddress,
                  saveToBook: false,
                }
              : EMPTY_AD_HOC,
          taxInvoiceNumber: o.taxInvoiceNumber || "",
          taxInvoiceDate: o.taxInvoiceDate || "",
          lines: o.items.map((it) => {
            const d = details.get(it.productId);
            const variants =
              d && d.variants.length > 0
                ? d.variants.map((v) => ({ id: v.id, code: v.code, name: v.name }))
                : [{ id: it.variantId, code: "-", name: "Varian tersimpan" }];
            const uoms = d && uniqueUoms(d).length > 0 ? uniqueUoms(d) : [it.uom];
            return {
              key: nextKey++,
              productId: it.productId,
              productLabel: `${it.productCode} — ${it.productName}`,
              variantId: it.variantId,
              uom: it.uom,
              qty: it.qty,
              discountPct: it.discountPct,
              discountNominal: it.discountNominal,
              catalogPrice: it.unitPrice,
              minPrice: d?.minSellingPrice ?? 0,
              // Harga manual dipertahankan saat edit; harga member/katalog
              // dikosongkan agar server menghitung ulang.
              manualPrice: it.pricingSource === "manual" ? String(it.unitPrice) : "",
              discountReason: it.discountReason || "",
              note: it.note || "",
              taxable: it.taxable,
              pricingSource: it.pricingSource,
              uoms,
              variants,
            };
          }),
          customerOption,
          customerPhone,
        };
      })();
    },
    [formKey],
  );
  const loading = isEdit ? editLoading : false;
  const loadError = invalidId ? "ID order tidak valid." : editFetchError;

  // Reset form saat ganti mode (tambah/edit/id lain).
  const [appliedKey, setAppliedKey] = useState<number | null>(null);
  const [prevKey, setPrevKey] = useState(formKey);
  if (formKey !== prevKey) {
    setPrevKey(formKey);
    setAppliedKey(null);
    setPartyId(null);
    setPickedCustomer(null);
    setOrderDate(todayWIB());
    setPaymentTerms("cod");
    setDueDate("");
    setNotes("");
    setTaxType("none");
    setTaxRate(0);
    setShipToAddressId(null);
    setShipAdHoc(EMPTY_AD_HOC);
    setAssignedUserId(null);
    setTaxInvoiceNumber("");
    setTaxInvoiceDate("");
    setLines([]);
    setLoadedStatus("draft");
    setLoadedEditMode("full");
    setLoadedChannel("regular");
    setReason("");
    setErrors({});
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editBundle && editBundle.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setLoadedStatus(editBundle.loadedStatus);
    setLoadedEditMode(editBundle.loadedEditMode);
    setLoadedChannel(editBundle.loadedChannel);
    setPartyId(editBundle.partyId);
    setOrderDate(editBundle.orderDate);
    setPaymentTerms(editBundle.paymentTerms);
    setDueDate(editBundle.dueDate);
    setNotes(editBundle.notes);
    setTaxType(normalizeTaxType(editBundle.taxType));
    setTaxRate(editBundle.taxRate);
    setShipToAddressId(editBundle.shipToAddressId);
    setShipAdHoc(editBundle.shipAdHoc);
    setAssignedUserId(editBundle.assignedUserId);
    setTaxInvoiceNumber(editBundle.taxInvoiceNumber);
    setTaxInvoiceDate(editBundle.taxInvoiceDate);
    setLines(editBundle.lines);
  }

  // Telepon customer terpilih (termin net wajib customer ber-telepon).
  const selectedPhone =
    partyId === null
      ? null
      : pickedParty && pickedParty.id === partyId
        ? (pickedParty.phone ?? "")
        : editBundle && editBundle.key === formKey
          ? editBundle.customerPhone
          : null;
  const netNeedsPhone =
    paymentTerms === "net" && partyId !== null && selectedPhone !== null && selectedPhone.trim() === "";

  function updateLine(key: number, patch: Partial<LineRow>) {
    setLines((rows) => rows.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function removeLine(key: number) {
    setLines((rows) => rows.filter((r) => r.key !== key));
  }

  /** Tambah baris produk; variantId (hasil pindai) menggantikan varian bawaan. */
  async function addLine(productId: number, variantId?: number): Promise<{ ok: boolean; qty?: number; name?: string }> {
    setAddingLine(true);
    try {
      const d = await productsService.detail(productId);
      const variants = d.variants.map((v) => ({ id: v.id, code: v.code, name: v.name }));
      if (variants.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
        return { ok: false };
      }
      const uoms = uniqueUoms(d);
      if (uoms.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya satuan jual.");
        return { ok: false };
      }
      const defaultVariant =
        (variantId !== undefined ? d.variants.find((v) => v.id === variantId) : undefined) ??
        d.variants.find((v) => v.isDefault) ??
        d.variants[0];
      const uom = d.salesUom || uoms[0];
      const dupe = lines.some(
        (r) => r.productId === d.id && r.variantId === defaultVariant.id && r.uom === uom,
      );
      if (dupe) {
        if (variantId !== undefined) {
          // Pindai ulang barang yang sama = tambah satu qty pada baris yang ada.
          const existing = lines.find((r) => r.productId === d.id && r.variantId === defaultVariant.id && r.uom === uom);
          if (existing) updateLine(existing.key, { qty: existing.qty + 1 });
          return { ok: true, qty: (existing?.qty ?? 0) + 1, name: `${d.code} — ${d.name}` };
        }
        toast.warning("Baris sudah ada", "Ubah qty pada baris yang sudah ada.");
        return { ok: false };
      }
      setLines((rows) => [
        ...rows,
        {
          key: nextKey++,
          productId: d.id,
          productLabel: `${d.code} — ${d.name}`,
          variantId: defaultVariant.id,
          uom,
          qty: 1,
          discountPct: 0,
          discountNominal: 0,
          catalogPrice: d.sellingPrice,
          minPrice: d.minSellingPrice,
          manualPrice: "",
          discountReason: "",
          note: "",
          taxable: true,
          pricingSource: "",
          uoms,
          variants,
        },
      ]);
      return { ok: true, qty: 1, name: `${d.code} — ${d.name}` };
    } catch (err) {
      toast.danger("Gagal memuat produk", toApiError(err).message);
      return { ok: false };
    } finally {
      setAddingLine(false);
    }
  }

  /** Pindai QR / barcode di form SO (SL-10): kode dikenal -> baris produk (atau qty + 1). */
  async function handleScan(code: string): Promise<ScanOutcome> {
    try {
      const hit = await productsService.scanLookup(code);
      if (hit.status !== "active") {
        return { kind: "rejected", detail: `${hit.code} — ${hit.name} tidak aktif dan tidak bisa dijual.` };
      }
      if (hit.variantRequired || hit.variantId === undefined) {
        return {
          kind: "rejected",
          detail: `${hit.code} — ${hit.name} punya beberapa varian: pilih lewat pencarian produk.`,
        };
      }
      const res = await addLine(hit.id, hit.variantId);
      return res.ok
        ? { kind: "added", name: res.name ?? hit.name, qty: res.qty ?? 1 }
        : { kind: "rejected", detail: `${hit.code} gagal ditambahkan ke order.` };
    } catch (err) {
      const apiErr = toApiError(err);
      return apiErr.status === 404
        ? { kind: "unknown", code, detail: `Kode "${code}" tidak ditemukan di katalog produk.` }
        : { kind: "rejected", detail: apiErr.message };
    }
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const values = {
      partyId,
      orderDate,
      paymentTerms,
      dueDate,
      notes,
      items: lines.map((r) => ({
        productId: r.productId,
        variantId: r.variantId,
        uom: r.uom,
        qty: r.qty,
        discountPct: r.discountPct,
        discountNominal: r.discountNominal,
        unitPrice: parseManualPrice(r.manualPrice),
        discountReason: r.discountReason,
        note: r.note,
        taxable: r.taxable,
      })),
    };
    const parsed = salesOrderSchema.safeParse(values);
    const extraErrors: Record<string, string> = {};
    // Tanggal faktur pajak opsional — bila diisi harus YYYY-MM-DD
    // (backend memvalidasi via resolveTaxInvoice).
    if (
      taxInvoiceDate.trim() !== "" &&
      !/^\d{4}-\d{2}-\d{2}$/.test(taxInvoiceDate.trim())
    ) {
      extraErrors.taxInvoiceDate = "Format tanggal YYYY-MM-DD";
    }
    const adHocActive = shipAdHoc.on || partyId === null;
    const adHocFilled =
      adHocActive &&
      [shipAdHoc.label, shipAdHoc.recipient, shipAdHoc.phone, shipAdHoc.address].some((t) => t.trim() !== "");
    if (adHocFilled && shipAdHoc.address.trim() === "") {
      extraErrors.shipToAddress = "Alamat wajib diisi bila memakai alamat sekali pakai";
    }
    if (netNeedsPhone) {
      extraErrors.paymentTerms = "Customer termin net wajib memiliki nomor telepon";
    }
    if (!parsed.success || Object.keys(extraErrors).length > 0) {
      const mapped: Record<string, string> = { ...extraErrors };
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const key = issue.path.join(".");
          mapped[key || "_"] ??= issue.message;
        }
      }
      setErrors(mapped);
      if (mapped.shipToAddress) setDetailOpen(true);
      if (mapped.items) toast.warning("Belum bisa disimpan", mapped.items);
      return;
    }
    setErrors({});
    setSubmitError(null);
    setItemErrors({ general: [], byRow: {} });
    const v = parsed.data;
    const body: SalesOrderPayload = {
      partyId: v.partyId ?? 0,
      channel: isEdit ? loadedChannel : "regular",
      orderDate: v.orderDate,
      dueDate: v.paymentTerms === "net" ? v.dueDate.trim() : "",
      paymentTerms: v.paymentTerms,
      taxType,
      taxRate,
      notes: v.notes,
      assignedUserId: assignedUserId ?? 0,
      shipToAddressId: adHocActive ? 0 : (shipToAddressId ?? 0),
      ...(adHocFilled
        ? {
            shipToLabel: shipAdHoc.label.trim(),
            shipToRecipient: shipAdHoc.recipient.trim(),
            shipToPhone: shipAdHoc.phone.trim(),
            shipToAddress: shipAdHoc.address.trim(),
          }
        : {}),
      taxInvoiceNumber:
        taxInvoiceNumber.trim() === "" ? undefined : taxInvoiceNumber.trim(),
      taxInvoiceDate:
        taxInvoiceDate.trim() === "" ? undefined : taxInvoiceDate.trim(),
      items: v.items.map(toLinePayload),
      ...(isEdit ? {} : { clientRef }),
      ...(correcting && reason.trim() !== "" ? { reason: reason.trim() } : {}),
    };
    setSubmitting(true);
    try {
      const res = isEdit
        ? await salesService.update(orderId, body)
        : await salesService.create(body);
      toast.fromServer(
        res.message,
        isEdit ? "Order disimpan" : "Order dibuat",
        "Harga final dihitung ulang server.",
      );
      setClientRef(newClientRef());
      if (formDraftKey !== null) clearDraft(formDraftKey);
      // Opsional: alamat sekali pakai ikut disimpan ke buku alamat pelanggan.
      // Order sudah tersimpan — kegagalan di sini hanya peringatan.
      if (adHocFilled && shipAdHoc.saveToBook && partyId !== null && can("customers.update")) {
        try {
          await customerService.createAddress(partyId, {
            label: shipAdHoc.label.trim() || undefined,
            recipient: shipAdHoc.recipient.trim() || undefined,
            phone: shipAdHoc.phone.trim() || undefined,
            text: shipAdHoc.address.trim(),
          });
        } catch (addrErr) {
          toast.warning("Alamat belum masuk buku alamat", toApiError(addrErr).message);
        }
      }
      navigate(`${ROUTE_PATHS.salesOrders}/${res.data.id}`);
    } catch (err) {
      const apiErr = toApiError(err);
      toast.danger(isEdit ? "Gagal menyimpan order" : "Gagal membuat order", apiErr.message);
      setSubmitError(apiErr.message);
      if (apiErr.errors) {
        const mapped: Record<string, string> = {};
        for (const fe of apiErr.errors) mapped[fe.field] ??= fe.message;
        setErrors((prev) => ({ ...prev, ...mapped }));
        setItemErrors(splitItemErrors(apiErr.errors));
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) return <DetailSkeleton />;

  if (loadError) {
    return (
      <Page>
        <PageHeader title={isEdit ? "Ubah Order Jual" : "Tambah Order Jual"} />
        <ErrorState title="Gagal memuat order" message={loadError} />
      </Page>
    );
  }

  const taxTotals = estimateTax(
    lines.map((r) => ({ net: rowEstimate(r), taxable: r.taxable })),
    taxType,
    taxRate,
  );
  const estimateTotal = taxTotals.total;
  const errorCount = Object.keys(errors).length + Object.keys(itemErrors.byRow).length + itemErrors.general.length;
  const showDraftIndicator = !isEdit && !isSalesDraftEmpty(draftSnapshot);

  return (
    <Page>
      <PageHeader
        title={isEdit ? "Ubah Order Jual" : "Tambah Order Jual"}
        description={
          isEdit
            ? "Order draf, atau order terkonfirmasi yang belum punya surat jalan, pembayaran, atau retur. Harga dihitung ulang server saat simpan."
            : "Kanal reguler. Harga dihitung server (member/katalog) saat simpan."
        }
        actions={
          showDraftIndicator ? (
            <span className="inline-flex items-center gap-2 text-xs text-muted-foreground" role="status">
              <Check size={14} aria-hidden className="text-success" />
              {draftRestored ? "Draf dipulihkan dan tersimpan otomatis" : "Draf tersimpan otomatis"}
              {draftRestored && (
                <Button variant="ghost" size="sm" onClick={discardDraft} className="h-auto px-1.5 py-0.5">
                  Buang draf
                </Button>
              )}
            </span>
          ) : undefined
        }
      />

      {!allowed && (
        <Notice tone="warning" title="Akses terbatas">
          Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "menambah"} order jual.
        </Notice>
      )}

      {readOnly && (
        <Notice tone="warning" title={`Order berstatus ${statusLabel(loadedStatus)} — hanya baca`}>
          {loadedStatus === "confirmed"
            ? "Order sudah punya surat jalan, pembayaran, atau retur: ubah info order (jatuh tempo, catatan, alamat, faktur pajak) lewat halaman detail."
            : "Order yang sudah selesai atau dibatalkan tidak bisa diubah."}{" "}
          <Link to={`${ROUTE_PATHS.salesOrders}/${orderId}`} className="font-semibold text-primary hover:underline">
            Lihat detail
          </Link>
        </Notice>
      )}

      {submitError && (
        <Notice tone="danger" title={isEdit ? "Gagal menyimpan order" : "Gagal membuat order"}>
          {submitError}
        </Notice>
      )}

      <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-6">
        {correcting && (
          <Notice tone="info" title="Mengoreksi order terkonfirmasi">
            <p>
              Belum ada surat jalan, pembayaran, atau retur, jadi seluruh isi order masih bisa diperbaiki. Perubahan
              dan alasannya dicatat di jejak audit.
            </p>
            <div className="mt-3 max-w-xl">
              <FormField label="Alasan koreksi" htmlFor="f-correction-reason" errorText={errors.reason}>
                <TextInput
                  id="f-correction-reason"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  maxLength={255}
                  placeholder="Opsional, mis. salah ketik qty"
                />
              </FormField>
            </div>
          </Notice>
        )}
        <SectionCard title="Dokumen">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="sm:col-span-2">
              <FormField
                label="Pelanggan"
                helperText="Kosongkan untuk tunai walk-in (tanpa pelanggan)."
                errorText={errors.partyId}
              >
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                <PartySearchSelect
                  partyKind="customer"
                  value={partyId}
                  selectedParty={pickedParty}
                  onChange={(id, party) => {
                    setPartyId(id);
                    setPickedParty(party ?? null);
                    setPickedCustomer(
                      id === null
                        ? null
                        : { value: id, label: party ? `${party.code} — ${party.name}` : `Customer #${id}` },
                    );
                    // Alamat milik pelanggan lama tak berlaku untuk pelanggan baru.
                    setShipToAddressId(null);
                  }}
                  placeholder="Cari pelanggan (nama, kode, telepon)…"
                  disabled={readOnly}
                />
                  </div>
                  {!readOnly && can("customers.create") && (
                    <Button variant="secondary" onClick={() => setQuickCustomerOpen(true)}>
                      <UserPlus aria-hidden />
                      Pelanggan baru
                    </Button>
                  )}
                </div>
              </FormField>
            </div>
            <FormField label="Tanggal order" required htmlFor="f-date" errorText={errors.orderDate}>
              <DateInput
                id="f-date"
                value={orderDate}
                onChange={(e) => setOrderDate(e.target.value)}
                disabled={readOnly}
              />
            </FormField>
            <FormField label="Termin" required htmlFor="f-terms" errorText={errors.paymentTerms}>
              <SelectInput
                id="f-terms"
                value={paymentTerms}
                onChange={(e) => setPaymentTerms(e.target.value as PaymentTerms)}
                disabled={readOnly}
              >
                <option value="cod">COD / Tunai</option>
                <option value="net">Net / Tempo</option>
                <option value="prepaid">Prabayar (bayar dulu)</option>
              </SelectInput>
            </FormField>
            {paymentTerms === "net" && (
              <FormField label="Jatuh tempo" required htmlFor="f-due" errorText={errors.dueDate}>
                <DateInput
                  id="f-due"
                  value={dueDate}
                  onChange={(e) => setDueDate(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
            )}
            <div className="sm:col-span-2 lg:col-span-4">
              <div className="flex flex-wrap items-end gap-x-6 gap-y-3" data-testid="tax-controls">
                <label className="flex cursor-pointer items-center gap-2 text-sm font-medium text-foreground">
                  <Checkbox
                    id="f-taxed"
                    checked={taxType !== "none"}
                    disabled={readOnly}
                    onCheckedChange={(v) => {
                      if (v === true) {
                        setTaxType("exclude");
                        setTaxRate((r) => (r > 0 ? r : DEFAULT_VAT_RATE));
                      } else {
                        setTaxType("none");
                        setTaxRate(0);
                      }
                    }}
                  />
                  Kena pajak (PPN)
                </label>
                {taxType !== "none" && (
                  <>
                    <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                      <Checkbox
                        id="f-tax-include"
                        checked={taxType === "include"}
                        disabled={readOnly}
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
                        disabled={readOnly}
                      />
                    </FormField>
                  </>
                )}
              </div>
              {paymentTerms === "net" && (
                <p className="text-xs text-muted-foreground">
                  Termin net hanya untuk pelanggan terdaftar yang memiliki nomor telepon.
                </p>
              )}
              {paymentTerms === "prepaid" && (
                <p className="text-xs text-muted-foreground">
                  Surat jalan baru bisa dibuat/dikonfirmasi setelah order dibayar lunas.
                </p>
              )}
              {netNeedsPhone && (
                <p className="text-xs font-medium text-destructive">
                  Pelanggan ini belum punya nomor telepon — lengkapi di data pelanggan.
                </p>
              )}
            </div>
          </div>

          <Collapsible open={detailOpen} onOpenChange={setDetailOpen} className="mt-4 border-t border-border pt-3">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="-ml-2">
                <ChevronRight
                  aria-hidden
                  className={detailOpen ? "rotate-90 transition-transform" : "transition-transform"}
                />
                Detail lain
                <span className="font-normal text-muted-foreground">(alamat kirim, faktur pajak, catatan)</span>
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-3">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                {partyId !== null && !shipAdHoc.on && (
                  <div className="sm:col-span-2 lg:col-span-4">
                    <FormField
                      label="Alamat kirim"
                      helperText={addrLoading ? "Memuat buku alamat…" : "Kosongkan untuk tanpa alamat kirim."}
                      errorText={errors.shipToAddressId}
                    >
                      <SearchSelect
                        options={shipAddresses.map((a) => ({
                          value: a.id,
                          label: `${a.label || "Alamat"} — ${a.recipient || "-"}${a.isPrimary ? " (Utama)" : ""}`,
                        }))}
                        value={shipToAddressId}
                        onChange={(v) => setShipToAddressId(typeof v === "number" ? v : null)}
                        placeholder="Tanpa alamat kirim"
                        allowClear
                        disabled={readOnly || addrLoading}
                      />
                    </FormField>
                    {shipToAddressId !== null &&
                      (() => {
                        const picked = shipAddresses.find((a) => a.id === shipToAddressId);
                        if (!picked) return null;
                        const contact = [picked.recipient, picked.phone]
                          .filter((s) => s && s.trim() !== "")
                          .join(" · ");
                        return (
                          <p className="mt-1 text-xs text-muted-foreground">
                            {picked.text}
                            {contact !== "" && <> · {contact}</>}
                          </p>
                        );
                      })()}
                  </div>
                )}
                {partyId !== null && !readOnly && (
                  <div className="sm:col-span-2 lg:col-span-4">
                    <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                      <Checkbox
                        id="f-ship-adhoc"
                        checked={shipAdHoc.on}
                        onCheckedChange={(v) => {
                          setShipAdHoc((a) => ({ ...a, on: v === true }));
                          if (v === true) setShipToAddressId(null);
                        }}
                      />
                      Pakai alamat sekali pakai (tidak dari buku alamat)
                    </label>
                  </div>
                )}
                {(partyId === null || shipAdHoc.on) && (
                  <div className="grid grid-cols-1 gap-4 sm:col-span-2 sm:grid-cols-2 lg:col-span-4" data-testid="ship-adhoc">
                    <FormField label="Label" htmlFor="f-ship-label" errorText={errors.shipToLabel}>
                      <TextInput
                        id="f-ship-label"
                        value={shipAdHoc.label}
                        maxLength={120}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, label: e.target.value }))}
                        placeholder="mis. Proyek, Toko cabang"
                        disabled={readOnly}
                      />
                    </FormField>
                    <FormField label="Penerima" htmlFor="f-ship-recipient" errorText={errors.shipToRecipient}>
                      <TextInput
                        id="f-ship-recipient"
                        value={shipAdHoc.recipient}
                        maxLength={150}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, recipient: e.target.value }))}
                        disabled={readOnly}
                      />
                    </FormField>
                    <FormField label="Telepon penerima" htmlFor="f-ship-phone" errorText={errors.shipToPhone}>
                      <TextInput
                        id="f-ship-phone"
                        value={shipAdHoc.phone}
                        maxLength={30}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, phone: e.target.value }))}
                        disabled={readOnly}
                      />
                    </FormField>
                    <div className="sm:col-span-2">
                      <FormField
                        label="Alamat kirim"
                        htmlFor="f-ship-address"
                        helperText={
                          partyId === null
                            ? "Kosongkan untuk tanpa alamat kirim (ambil sendiri)."
                            : "Disalin ke order ini saja."
                        }
                        errorText={errors.shipToAddress}
                      >
                        <TextArea
                          id="f-ship-address"
                          rows={2}
                          value={shipAdHoc.address}
                          maxLength={1000}
                          onChange={(e) => setShipAdHoc((a) => ({ ...a, address: e.target.value }))}
                          placeholder="Alamat lengkap"
                          disabled={readOnly}
                        />
                      </FormField>
                    </div>
                    {partyId !== null && !readOnly && can("customers.update") && (
                      <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground sm:col-span-2">
                        <Checkbox
                          id="f-ship-save"
                          checked={shipAdHoc.saveToBook}
                          onCheckedChange={(v) => setShipAdHoc((a) => ({ ...a, saveToBook: v === true }))}
                        />
                        Simpan juga ke buku alamat pelanggan
                      </label>
                    )}
                  </div>
                )}
                <div className="sm:col-span-1 lg:col-span-2">
                  <FormField label="No. faktur pajak" htmlFor="f-tax-no" errorText={errors.taxInvoiceNumber}>
                    <TextInput
                      id="f-tax-no"
                      value={taxInvoiceNumber}
                      onChange={(e) => setTaxInvoiceNumber(e.target.value)}
                      placeholder="Nomor faktur pajak (opsional)"
                      disabled={readOnly}
                    />
                  </FormField>
                </div>
                <FormField label="Tgl. faktur pajak" htmlFor="f-tax-date" errorText={errors.taxInvoiceDate}>
                  <DateInput
                    id="f-tax-date"
                    value={taxInvoiceDate}
                    onChange={(e) => setTaxInvoiceDate(e.target.value)}
                    disabled={readOnly}
                  />
                </FormField>
                {can("users.view") && (
                  <div className="sm:col-span-2">
                    <FormField label="Penanggung jawab" errorText={errors.assignedUserId}>
                      <UserSearchSelect
                        value={assignedUserId}
                        onChange={(uid) => setAssignedUserId(uid)}
                        placeholder="Tanpa penanggung jawab"
                        disabled={readOnly}
                      />
                    </FormField>
                  </div>
                )}
                <div className="sm:col-span-2 lg:col-span-4">
                  <FormField label="Catatan" htmlFor="f-notes" errorText={errors.notes}>
                    <TextArea
                      id="f-notes"
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder="Catatan order (opsional)"
                      rows={2}
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
          description="Harga kosong = server memberi harga (member/katalog). Isi harga untuk harga manual (tidak boleh di bawah harga minimum produk). Angka total hanya estimasi."
        >
          <div className="flex flex-col gap-4">
            {!readOnly && (
              <FormField label="Tambah produk" htmlFor="f-product">
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <AsyncSearchSelect
                      id="f-product"
                      value={null}
                      loadOptions={loadProductOptions}
                      onChange={(v) => {
                        if (typeof v === "number") void addLine(v);
                      }}
                      disabled={addingLine}
                      placeholder={addingLine ? "Menambah baris…" : "Cari produk (kode atau nama)…"}
                      searchPlaceholder="Kode / nama produk…"
                    />
                  </div>
                  {can("products.view") && (
                    <Button variant="secondary" onClick={() => setScanOpen(true)} disabled={addingLine}>
                      <ScanLine aria-hidden />
                      Pindai
                    </Button>
                  )}
                </div>
              </FormField>
            )}

            {Object.keys(itemErrors.byRow).length > 0 || itemErrors.general.length > 0 ? (
              <Notice tone="danger" title="Barang ditolak server">
                <ul className="list-disc pl-5">
                  {itemErrors.general.map((m) => (
                    <li key={m}>{m}</li>
                  ))}
                  {Object.entries(itemErrors.byRow).map(([idx, m]) => (
                    <li key={idx}>
                      Baris {Number(idx) + 1}: {m}
                    </li>
                  ))}
                </ul>
              </Notice>
            ) : (
              errors.items && <Notice tone="danger" title={errors.items} />
            )}

            <LineItemsEditor<LineRow>
              lines={lines}
              getKey={(r) => r.key}
              readOnly={readOnly}
              onRemove={readOnly ? undefined : (i) => removeLine(lines[i].key)}
              emptyText="Belum ada barang. Cari produk di atas untuk menambahkan baris."
              columns={[
                {
                  id: "product",
                  header: "Produk",
                  render: (r) => (
                    <div className="min-w-44">
                      <p className="font-medium">{r.productLabel}</p>
                      {readOnly || r.variants.length <= 1 ? (
                        <p className="text-xs text-muted-foreground">
                          {r.variants.find((v) => v.id === r.variantId)?.name ?? ""}
                        </p>
                      ) : (
                        <div className="mt-1">
                          <SelectInput
                            value={String(r.variantId)}
                            onChange={(e) => updateLine(r.key, { variantId: Number(e.target.value) })}
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
                  render: (r) =>
                    readOnly ? (
                      r.uom
                    ) : (
                      <SelectInput
                        value={r.uom}
                        onChange={(e) => updateLine(r.key, { uom: e.target.value })}
                        aria-label={`Satuan ${r.productLabel}`}
                      >
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
                  render: (r, i) =>
                    readOnly ? (
                      <Qty value={r.qty} />
                    ) : (
                      <TextInput
                        type="number"
                        min={0}
                        step="any"
                        value={String(r.qty)}
                        onChange={(e) => updateLine(r.key, { qty: Number(e.target.value) })}
                        error={errors[`items.${i}.qty`]}
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
                  render: (r, i) => {
                    if (readOnly) {
                      return r.manualPrice !== "" ? <Money value={Number(r.manualPrice)} /> : pricingSourceLabel(r.pricingSource);
                    }
                    const below =
                      (r.minPrice ?? 0) > 0 &&
                      belowMinPrice({
                        qty: r.qty,
                        catalogPrice: r.catalogPrice,
                        manualPrice: parseManualPrice(r.manualPrice),
                        discountPct: r.discountPct,
                        discountNominal: r.discountNominal,
                        minPrice: r.minPrice,
                        memberPrice: r.pricingSource === "member_rule" && r.manualPrice === "",
                      });
                    return (
                      <div>
                        <TextInput
                          type="number"
                          min={0}
                          step={1}
                          value={r.manualPrice}
                          placeholder={String(r.catalogPrice)}
                          onChange={(e) => updateLine(r.key, { manualPrice: e.target.value })}
                          error={itemErrors.byRow[i]}
                          aria-label={`Harga manual ${r.productLabel}`}
                          className="text-right"
                        />
                        {/* Info harga = teks bantu di bawah sel harga (PLAN §8 sales). */}
                        <p className="mt-1 text-xs text-muted-foreground">
                          {r.manualPrice === ""
                            ? r.pricingSource === "member_rule"
                              ? "Harga member (aturan)"
                              : `Katalog ${formatIDR(r.catalogPrice)}`
                            : `Manual · katalog ${formatIDR(r.catalogPrice)}`}
                        </p>
                        {(r.minPrice ?? 0) > 0 && (
                          <p className={below ? "mt-0.5 text-xs font-semibold text-destructive" : "mt-0.5 text-xs text-muted-foreground"}>
                            Minimum {formatIDR(r.minPrice ?? 0)}/satuan
                            {below ? " — harga bersih di bawah minimum, server akan menolak" : ""}
                          </p>
                        )}
                      </div>
                    );
                  },
                },
                {
                  id: "discount",
                  header: "Diskon",
                  width: "10rem",
                  align: "right",
                  render: (r) =>
                    readOnly ? (
                      <span className="tabular">
                        {formatNumber(r.discountPct)}% · {formatIDR(r.discountNominal)}
                      </span>
                    ) : (
                      <div className="flex items-start gap-1.5">
                        <TextInput
                          type="number"
                          min={0}
                          max={100}
                          step="any"
                          value={String(r.discountPct)}
                          onChange={(e) => updateLine(r.key, { discountPct: Number(e.target.value) })}
                          aria-label={`Diskon persen ${r.productLabel}`}
                          className="w-16 text-right"
                        />
                        <TextInput
                          type="number"
                          min={0}
                          step={1}
                          value={String(r.discountNominal)}
                          onChange={(e) => updateLine(r.key, { discountNominal: Number(e.target.value) })}
                          aria-label={`Diskon nominal ${r.productLabel}`}
                          className="w-24 text-right"
                        />
                      </div>
                    ),
                },
                {
                  id: "detail",
                  header: "Rincian",
                  width: "7rem",
                  render: (r) => {
                    const summary = [
                      r.discountReason ? "alasan diskon" : "",
                      r.note ? "catatan" : "",
                      r.taxable ? "" : "tanpa pajak",
                    ].filter(Boolean);
                    if (readOnly) {
                      return (
                        <div className="text-xs">
                          {r.discountReason && <p>Alasan diskon: {r.discountReason}</p>}
                          {r.note && <p>Catatan: {r.note}</p>}
                          {!r.taxable && <p>Tanpa pajak</p>}
                        </div>
                      );
                    }
                    return (
                      <Popover>
                        <PopoverTrigger asChild>
                          <Button variant="ghost" size="sm" aria-label={`Rincian baris ${r.productLabel}`}>
                            Ubah
                          </Button>
                        </PopoverTrigger>
                        <PopoverContent align="end" className="flex w-72 flex-col gap-3 p-4">
                          <FormField label="Alasan diskon">
                            <TextInput
                              value={r.discountReason}
                              maxLength={255}
                              onChange={(e) => updateLine(r.key, { discountReason: e.target.value })}
                              placeholder="Alasan diskon"
                              aria-label={`Alasan diskon ${r.productLabel}`}
                            />
                          </FormField>
                          <FormField label="Catatan baris">
                            <TextInput
                              value={r.note}
                              maxLength={255}
                              onChange={(e) => updateLine(r.key, { note: e.target.value })}
                              placeholder="Catatan baris"
                              aria-label={`Catatan baris ${r.productLabel}`}
                            />
                          </FormField>
                          <label className="flex cursor-pointer items-center gap-2 text-sm">
                            <Checkbox
                              checked={r.taxable}
                              onCheckedChange={(v) => updateLine(r.key, { taxable: v === true })}
                              aria-label={`Kena pajak ${r.productLabel}`}
                            />
                            Kena pajak
                          </label>
                        </PopoverContent>
                        {summary.length > 0 && <p className="mt-1 text-xs text-muted-foreground">{summary.join(", ")}</p>}
                      </Popover>
                    );
                  },
                },
                {
                  id: "total",
                  header: "Estimasi",
                  width: "8rem",
                  align: "right",
                  render: (r) => <Money value={rowEstimate(r)} className="inline-block py-2 font-medium" />,
                },
              ]}
            />
          </div>
        </SectionCard>

        <StickyFooter
          summary={
            <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span>
                <span className="tabular">{lines.length}</span> barang
              </span>
              {taxType !== "none" && (
                <span data-testid="tax-summary" className="text-muted-foreground">
                  DPP <Money value={taxTotals.base} /> · PPN {taxRate}% <Money value={taxTotals.tax} />
                </span>
              )}
              <span>
                Estimasi total <Money value={estimateTotal} className="font-display text-lg font-semibold text-foreground" />
              </span>
              {errorCount > 0 && (
                <span role="alert" className="font-medium text-destructive">
                  {errorCount} isian perlu diperbaiki
                </span>
              )}
            </span>
          }
        >
          <Button
            variant="secondary"
            onClick={() => navigate(isEdit ? `${ROUTE_PATHS.salesOrders}/${orderId}` : ROUTE_PATHS.salesOrders)}
          >
            {readOnly ? "Kembali" : "Batal"}
          </Button>
          {!readOnly && (
            <Button type="submit" loading={submitting} disabled={!allowed}>
              {isEdit ? "Simpan" : "Simpan order"}
            </Button>
          )}
        </StickyFooter>
      </form>
      <ScanDialog open={scanOpen} paused={false} onClose={() => setScanOpen(false)} onScan={handleScan} />
      <QuickCustomerSheet
        open={quickCustomerOpen}
        requirePhone={paymentTerms === "net"}
        onClose={() => setQuickCustomerOpen(false)}
        onCreated={(party) => {
          setQuickCustomerOpen(false);
          setPartyId(party.id);
          setPickedParty(party);
          setPickedCustomer({ value: party.id, label: `${party.code} — ${party.name}` });
          setShipToAddressId(null);
        }}
      />
    </Page>
  );
}
