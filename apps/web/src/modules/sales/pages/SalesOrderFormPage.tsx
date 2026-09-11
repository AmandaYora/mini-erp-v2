import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ActionRow,
  Button,
  DataTable,
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
import { formatIDR, formatNumber, statusLabel, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesOrderSchema } from "@/modules/sales/schemas/sales-order.schema";
import {
  productOptionService,
  salesService,
} from "@/modules/sales/services/sales.service";
import type { SalesOrderPayload } from "@/modules/sales/types";
import { estimateLine } from "@/modules/sales/lib/estimate";
import { customerService } from "@/modules/party/services/party.service";
import type { Address } from "@/modules/party/types";
import { productsService } from "@/modules/products/services/products.service";
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
  uoms: string[];
  variants: { id: number; code: string; name: string }[];
}

let nextKey = 1;

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

  const [partyId, setPartyId] = useState<number | null>(null);
  const [customerQuery, setCustomerQuery] = useState("");
  const [customerQueryDebounced, setCustomerQueryDebounced] = useState("");
  const [pickedCustomer, setPickedCustomer] = useState<SearchSelectOption | null>(null);
  const [orderDate, setOrderDate] = useState(todayWIB());
  const [paymentTerms, setPaymentTerms] = useState<"cod" | "net">("cod");
  const [dueDate, setDueDate] = useState("");
  const [notes, setNotes] = useState("");
  // Pajak tidak diedit di wave ini — dipertahankan dari detail saat edit
  // agar simpan ulang tidak menghapus pajak order lama.
  const [taxType, setTaxType] = useState("none");
  const [taxRate, setTaxRate] = useState(0);
  // Alamat kirim (buku alamat customer) + nomor/tanggal faktur pajak.
  // shipToAddressId null = tanpa alamat kirim (dikirim 0 ke backend).
  const [shipToAddressId, setShipToAddressId] = useState<number | null>(null);
  const [taxInvoiceNumber, setTaxInvoiceNumber] = useState("");
  const [taxInvoiceDate, setTaxInvoiceDate] = useState("");

  const [lines, setLines] = useState<LineRow[]>([]);
  const [productQuery, setProductQuery] = useState("");
  const [productQueryDebounced, setProductQueryDebounced] = useState("");
  const [pickedProductId, setPickedProductId] = useState<number | null>(null);
  const [addingLine, setAddingLine] = useState(false);

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loadedStatus, setLoadedStatus] = useState<string>("draft");
  const [submitting, setSubmitting] = useState(false);

  const readOnly = isEdit && loadedStatus !== "draft";

  // Kunci mode form: 0 = tambah, -1 = id tak valid, >0 = id order edit.
  const formKey = !isEdit ? 0 : invalidId ? -1 : orderId;

  // --- opsi customer (debounce): hanya customer aktif ---
  // Debounce di sisi query; fetch via hook agar tanpa effect-setState.
  useEffect(() => {
    const t = window.setTimeout(() => setCustomerQueryDebounced(customerQuery), 350);
    return () => window.clearTimeout(t);
  }, [customerQuery]);

  const { data: customerLookup } = useAsyncData(
    () =>
      customerService
        .list({ search: customerQueryDebounced, status: "active", page: 1, limit: 20 })
        .then(
          (res) => res.items.map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` })),
          (): { value: number; label: string }[] => [],
        ),
    [customerQueryDebounced],
  );

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

  // --- opsi produk (debounce): /products/search-options?q= ---
  useEffect(() => {
    const t = window.setTimeout(() => setProductQueryDebounced(productQuery), 350);
    return () => window.clearTimeout(t);
  }, [productQuery]);

  const { data: productLookup } = useAsyncData(
    () =>
      productOptionService.search(productQueryDebounced).then(
        (list) => list.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
        (): { value: number; label: string }[] => [],
      ),
    [productQueryDebounced],
  );
  const productOptions = productLookup ?? [];

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
        if (o.partyId > 0) {
          try {
            const c = await customerService.get(o.partyId);
            customerOption = { value: c.id, label: `${c.code} — ${c.name}` };
          } catch {
            customerOption = { value: o.partyId, label: o.partyName || `Customer #${o.partyId}` };
          }
        }
        return {
          key,
          loadedStatus: o.status,
          partyId: o.partyId > 0 ? o.partyId : null,
          orderDate: o.orderDate || todayWIB(),
          paymentTerms: (o.paymentTerms === "net" ? "net" : "cod") as "cod" | "net",
          dueDate: o.dueDate || "",
          notes: o.notes || "",
          taxType: o.taxType || "none",
          taxRate: o.taxRate || 0,
          shipToAddressId: o.shipToAddressId > 0 ? o.shipToAddressId : null,
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
              uoms,
              variants,
            };
          }),
          customerOption,
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
    setTaxInvoiceNumber("");
    setTaxInvoiceDate("");
    setLines([]);
    setLoadedStatus("draft");
    setErrors({});
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editBundle && editBundle.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setLoadedStatus(editBundle.loadedStatus);
    setPartyId(editBundle.partyId);
    setOrderDate(editBundle.orderDate);
    setPaymentTerms(editBundle.paymentTerms);
    setDueDate(editBundle.dueDate);
    setNotes(editBundle.notes);
    setTaxType(editBundle.taxType);
    setTaxRate(editBundle.taxRate);
    setShipToAddressId(editBundle.shipToAddressId);
    setTaxInvoiceNumber(editBundle.taxInvoiceNumber);
    setTaxInvoiceDate(editBundle.taxInvoiceDate);
    setLines(editBundle.lines);
  }

  // Jangan buang opsi terpilih (mis. prefill edit / pilihan user) yang tak cocok query.
  const editCustomerOption =
    editBundle && editBundle.key === formKey ? editBundle.customerOption : null;
  const selectedCustomer = pickedCustomer ?? editCustomerOption;
  const customerOptions = useMemo(() => {
    const fetched = customerLookup ?? [];
    if (
      selectedCustomer &&
      !fetched.some((o) => o.value === selectedCustomer.value)
    ) {
      return [selectedCustomer, ...fetched];
    }
    return fetched;
  }, [customerLookup, selectedCustomer]);

  function updateLine(key: number, patch: Partial<LineRow>) {
    setLines((rows) => rows.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function removeLine(key: number) {
    setLines((rows) => rows.filter((r) => r.key !== key));
  }

  async function addLine() {
    if (pickedProductId === null) {
      toast.warning("Pilih produk dulu", "Cari produk lalu tekan Tambah baris.");
      return;
    }
    setAddingLine(true);
    try {
      const d = await productsService.detail(pickedProductId);
      const variants = d.variants.map((v) => ({ id: v.id, code: v.code, name: v.name }));
      if (variants.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
        return;
      }
      const uoms = uniqueUoms(d);
      if (uoms.length === 0) {
        toast.danger("Gagal menambah baris", "Produk tidak punya satuan jual.");
        return;
      }
      const defaultVariant = d.variants.find((v) => v.isDefault) ?? d.variants[0];
      const uom = d.salesUom || uoms[0];
      const dupe = lines.some(
        (r) => r.productId === d.id && r.variantId === defaultVariant.id && r.uom === uom,
      );
      if (dupe) {
        toast.warning("Baris sudah ada", "Ubah qty pada baris yang sudah ada.");
        return;
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
          uoms,
          variants,
        },
      ]);
      setPickedProductId(null);
    } catch (err) {
      toast.danger("Gagal memuat produk", toApiError(err).message);
    } finally {
      setAddingLine(false);
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
    if (!parsed.success || Object.keys(extraErrors).length > 0) {
      const mapped: Record<string, string> = { ...extraErrors };
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const key = issue.path.join(".");
          mapped[key || "_"] ??= issue.message;
        }
      }
      setErrors(mapped);
      if (mapped.items) toast.warning("Belum bisa disimpan", mapped.items);
      return;
    }
    setErrors({});
    const v = parsed.data;
    const body: SalesOrderPayload = {
      partyId: v.partyId ?? 0,
      channel: "regular",
      orderDate: v.orderDate,
      dueDate: v.paymentTerms === "net" ? v.dueDate.trim() : "",
      paymentTerms: v.paymentTerms,
      taxType,
      taxRate,
      notes: v.notes,
      shipToAddressId: shipToAddressId ?? 0,
      taxInvoiceNumber:
        taxInvoiceNumber.trim() === "" ? undefined : taxInvoiceNumber.trim(),
      taxInvoiceDate:
        taxInvoiceDate.trim() === "" ? undefined : taxInvoiceDate.trim(),
      items: v.items.map((it) => ({
        productId: it.productId,
        variantId: it.variantId,
        uom: it.uom.trim(),
        qty: it.qty,
        discountPct: it.discountPct,
        discountNominal: it.discountNominal,
      })),
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
      navigate(`${ROUTE_PATHS.salesOrders}/${res.data.id}`);
    } catch (err) {
      const apiErr = toApiError(err);
      toast.danger(isEdit ? "Gagal menyimpan order" : "Gagal membuat order", apiErr.message);
      if (apiErr.errors) {
        const mapped: Record<string, string> = {};
        for (const fe of apiErr.errors) mapped[fe.field] ??= fe.message;
        setErrors((prev) => ({ ...prev, ...mapped }));
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return <p className="text-muted py-10 text-center text-sm">Memuat…</p>;
  }

  if (loadError) {
    return (
      <div className="space-y-4">
        <PageHeader title={isEdit ? "Ubah Order Jual" : "Tambah Order Jual"} />
        <Notice tone="danger" title="Gagal memuat order">
          {loadError}
        </Notice>
        <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesOrders)}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  const estimateTotal = lines.reduce((sum, r) => sum + estimateLine(r), 0);

  return (
    <div>
      <PageHeader
        eyebrow="Order Jual"
        title={isEdit ? "Ubah Order Jual" : "Tambah Order Jual"}
        description={
          isEdit
            ? "Hanya order draf yang bisa diubah. Harga dihitung ulang server saat simpan."
            : "Channel reguler. Harga dihitung server (member/katalog) saat simpan."
        }
      />

      {!allowed && (
        <div className="mb-4">
          <Notice tone="warning" title="Akses terbatas">
            Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "menambah"} order jual.
          </Notice>
        </div>
      )}

      {readOnly && (
        <div className="mb-4">
          <Notice tone="warning" title={`Order berstatus ${statusLabel(loadedStatus)} — hanya baca`}>
            Hanya order draf yang bisa diubah.{" "}
            <Link
              to={`${ROUTE_PATHS.salesOrders}/${orderId}`}
              className="font-semibold text-brand hover:underline"
            >
              Lihat detail
            </Link>
          </Notice>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
          <SectionCard title="Pembeli & Termin">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <FormField
                  label="Customer"
                  helperText="Kosongkan untuk tunai walk-in (tanpa customer)."
                  errorText={errors.partyId}
                >
                  <TextInput
                    value={customerQuery}
                    onChange={(e) => setCustomerQuery(e.target.value)}
                    placeholder="Ketik untuk mencari customer…"
                    disabled={readOnly}
                    className="mb-2"
                  />
                </FormField>
                <SearchSelect
                  options={customerOptions}
                  value={partyId}
                  onChange={(v) => {
                    const num = typeof v === "number" ? v : null;
                    setPartyId(num);
                    setPickedCustomer(
                      num === null
                        ? null
                        : (customerOptions.find((o) => o.value === num) ?? {
                            value: num,
                            label: `Customer #${num}`,
                          }),
                    );
                    // Alamat milik customer lama tak berlaku untuk customer baru.
                    setShipToAddressId(null);
                  }}
                  placeholder="Pilih customer…"
                  allowClear
                  disabled={readOnly}
                />
              </div>
              {partyId !== null && (
                <div className="sm:col-span-2">
                  <FormField
                    label="Alamat Kirim"
                    helperText={
                      addrLoading
                        ? "Memuat buku alamat…"
                        : "Kosongkan untuk tanpa alamat kirim."
                    }
                    errorText={errors.shipToAddressId}
                  >
                    <SearchSelect
                      options={shipAddresses.map((a) => ({
                        value: a.id,
                        label: `${a.label || "Alamat"} — ${a.recipient || "-"}${a.isPrimary ? " (Utama)" : ""}`,
                      }))}
                      value={shipToAddressId}
                      onChange={(v) =>
                        setShipToAddressId(typeof v === "number" ? v : null)
                      }
                      placeholder="Tanpa alamat kirim"
                      allowClear
                      disabled={readOnly || addrLoading}
                    />
                  </FormField>
                  {shipToAddressId !== null &&
                    (() => {
                      const picked = shipAddresses.find(
                        (a) => a.id === shipToAddressId,
                      );
                      if (!picked) return null;
                      const contact = [picked.recipient, picked.phone]
                        .filter((s) => s && s.trim() !== "")
                        .join(" · ");
                      return (
                        <p className="text-muted mt-1 text-[0.82rem]">
                          {picked.text}
                          {contact !== "" && <> · {contact}</>}
                        </p>
                      );
                    })()}
                </div>
              )}
              <FormField
                label="No. Faktur Pajak"
                htmlFor="f-tax-no"
                errorText={errors.taxInvoiceNumber}
              >
                <TextInput
                  id="f-tax-no"
                  value={taxInvoiceNumber}
                  onChange={(e) => setTaxInvoiceNumber(e.target.value)}
                  placeholder="Nomor faktur pajak (opsional)"
                  disabled={readOnly}
                />
              </FormField>
              <FormField
                label="Tgl. Faktur Pajak"
                htmlFor="f-tax-date"
                errorText={errors.taxInvoiceDate}
              >
                <DateInput
                  id="f-tax-date"
                  value={taxInvoiceDate}
                  onChange={(e) => setTaxInvoiceDate(e.target.value)}
                  disabled={readOnly}
                />
              </FormField>
              <FormField label="Tanggal Order" required htmlFor="f-date" errorText={errors.orderDate}>
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
                  onChange={(e) => setPaymentTerms(e.target.value as "cod" | "net")}
                  disabled={readOnly}
                >
                  <option value="cod">COD / Tunai</option>
                  <option value="net">Net / Tempo</option>
                </SelectInput>
              </FormField>
              {paymentTerms === "net" && (
                <FormField
                  label="Jatuh Tempo"
                  required
                  htmlFor="f-due"
                  errorText={errors.dueDate}
                >
                  <DateInput
                    id="f-due"
                    value={dueDate}
                    onChange={(e) => setDueDate(e.target.value)}
                    disabled={readOnly}
                  />
                </FormField>
              )}
              <div className="sm:col-span-2">
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
          </SectionCard>

          <SectionCard
            title="Item"
            description="Harga final dihitung server (member/katalog). Angka di bawah hanya estimasi."
          >
            {!readOnly && (
              <div className="mb-4 flex flex-wrap items-end gap-3">
                <div className="min-w-52 flex-1">
                  <FormField label="Cari Produk">
                    <TextInput
                      value={productQuery}
                      onChange={(e) => setProductQuery(e.target.value)}
                      placeholder="Kode / nama produk…"
                    />
                  </FormField>
                  <div className="mt-2">
                    <SearchSelect
                      options={productOptions}
                      value={pickedProductId}
                      onChange={(v) => setPickedProductId(typeof v === "number" ? v : null)}
                      placeholder="Pilih produk…"
                      allowClear
                    />
                  </div>
                </div>
                <Button onClick={addLine} disabled={addingLine || pickedProductId === null}>
                  {addingLine ? "Menambah…" : "Tambah baris"}
                </Button>
              </div>
            )}

            {errors.items && (
              <div className="mb-4">
                <Notice tone="danger" title={errors.items} />
              </div>
            )}

            <DataTable<LineRow>
              rows={lines}
              rowKey={(r) => r.key}
              emptyTitle="Belum ada item"
              emptyDescription="Cari produk di atas lalu tekan Tambah baris."
              columns={[
                {
                  header: "Produk",
                  render: (r) => (
                    <div>
                      <p className="font-medium">{r.productLabel}</p>
                      <p className="text-muted text-xs">
                        Estimasi {formatIDR(estimateLine(r))}
                      </p>
                    </div>
                  ),
                },
                {
                  header: "Varian",
                  render: (r) =>
                    readOnly ? (
                      r.variants.find((v) => v.id === r.variantId)?.name ?? "-"
                    ) : (
                      <SelectInput
                        value={String(r.variantId)}
                        onChange={(e) => updateLine(r.key, { variantId: Number(e.target.value) })}
                        aria-label="Varian"
                      >
                        {r.variants.map((v) => (
                          <option key={v.id} value={String(v.id)}>
                            {v.code} — {v.name}
                          </option>
                        ))}
                      </SelectInput>
                    ),
                },
                {
                  header: "Satuan",
                  render: (r) =>
                    readOnly ? (
                      r.uom
                    ) : (
                      <SelectInput
                        value={r.uom}
                        onChange={(e) => updateLine(r.key, { uom: e.target.value })}
                        aria-label="Satuan"
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
                  header: "Qty",
                  align: "right",
                  render: (r) =>
                    readOnly ? (
                      formatNumber(r.qty)
                    ) : (
                      <TextInput
                        type="number"
                        min={0}
                        step="any"
                        value={String(r.qty)}
                        onChange={(e) => updateLine(r.key, { qty: Number(e.target.value) })}
                        error={errors[`items.${lines.indexOf(r)}.qty`]}
                        aria-label="Qty"
                      />
                    ),
                },
                {
                  header: "Diskon %",
                  align: "right",
                  render: (r) =>
                    readOnly ? (
                      formatNumber(r.discountPct)
                    ) : (
                      <TextInput
                        type="number"
                        min={0}
                        max={100}
                        step="any"
                        value={String(r.discountPct)}
                        onChange={(e) =>
                          updateLine(r.key, { discountPct: Number(e.target.value) })
                        }
                        aria-label="Diskon persen"
                      />
                    ),
                },
                {
                  header: "Diskon Rp",
                  align: "right",
                  render: (r) =>
                    readOnly ? (
                      formatIDR(r.discountNominal)
                    ) : (
                      <TextInput
                        type="number"
                        min={0}
                        step={1}
                        value={String(r.discountNominal)}
                        onChange={(e) =>
                          updateLine(r.key, { discountNominal: Number(e.target.value) })
                        }
                        aria-label="Diskon nominal"
                      />
                    ),
                },
                ...(readOnly
                  ? []
                  : [
                      {
                        header: "Aksi",
                        align: "right" as const,
                        render: (r: LineRow) => (
                          <Button variant="ghost" size="sm" onClick={() => removeLine(r.key)}>
                            Hapus
                          </Button>
                        ),
                      },
                    ]),
              ]}
            />

            {lines.length > 0 && (
              <p className="text-muted mt-3 text-right text-sm">
                Estimasi total: <span className="font-semibold text-ink">{formatIDR(estimateTotal)}</span>
              </p>
            )}
          </SectionCard>
        </div>

        <ActionRow>
          <Button
            variant="secondary"
            onClick={() =>
              navigate(isEdit ? `${ROUTE_PATHS.salesOrders}/${orderId}` : ROUTE_PATHS.salesOrders)
            }
          >
            {readOnly ? "Kembali" : "Batal"}
          </Button>
          {!readOnly && (
            <Button type="submit" disabled={submitting || !allowed}>
              {submitting ? "Menyimpan…" : isEdit ? "Simpan" : "Buat Order"}
            </Button>
          )}
        </ActionRow>
      </form>
    </div>
  );
}
