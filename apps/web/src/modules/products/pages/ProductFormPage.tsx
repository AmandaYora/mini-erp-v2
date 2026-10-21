import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Plus } from "lucide-react";
import {
  Button,
  DetailSkeleton,
  FormField,
  PageHeader,
  RadioGroup,
  RadioGroupItem,
  SearchSelect,
  SelectInput,
  Switch,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { KeyValueEditor } from "@/shared/components/ui/key-value-editor";
import { FileDropzone, FormSection, Page, StickyFooter } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { PRODUCT_ATTR_LIMITS, attrsToRows } from "@/shared/lib/key-value";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toast } from "@/shared/stores/toast.store";
import {
  BARCODE_MAX,
  NAME_MAX,
  PRODUCT_CODE_MAX,
  SUMMARY_MAX,
  UOM_MAX,
  VARIANT_CODE_MAX,
  productSchema,
} from "@/modules/products/schemas/product.schema";
import type {
  ProductFormValues,
  ProductVariantFormValues,
} from "@/modules/products/schemas/product.schema";
import {
  PRODUCT_TYPE_OPTIONS,
  applyTypeRules,
  asProductType,
  buildProductPayload,
  canTrack,
  emptyVariantRow,
  guardTitle,
  mapProductServerErrors,
  normalizeDefault,
  variantRowsFromProduct,
  visibleVariants,
} from "@/modules/products/lib/product-form";
import type { MappedProductErrors } from "@/modules/products/lib/product-form";
import { uploadInitialPhoto } from "@/modules/products/lib/initial-photo";
import { MEDIA_ACCEPT_ATTR, validatePhotoFile } from "@/modules/products/lib/media-rules";
import { productsService } from "@/modules/products/services/products.service";
import type { ProductCategory, ProductType } from "@/modules/products/types";

const EMPTY: ProductFormValues = {
  code: "",
  name: "",
  type: "barang",
  status: "active",
  categoryId: null,
  tracked: true,
  baseUom: "",
  purchaseUom: "",
  salesUom: "",
  purchaseFactor: 1,
  salesFactor: 1,
  purchasePrice: 0,
  sellingPrice: 0,
  minSellingPrice: 0,
  minStock: 0,
  summary: "",
  attrs: [],
  hasVariants: false,
  variants: [],
};

const ATTR_SUGGESTIONS = ["Merek", "Warna", "Ukuran", "Bahan", "Asal", "Garansi"] as const;

/** Seksi form (urutan = urutan navigasi kiri). */
const SECTIONS = [
  { id: "sec-identitas", label: "Identitas" },
  { id: "sec-satuan", label: "Satuan" },
  { id: "sec-harga", label: "Harga" },
  { id: "sec-varian", label: "Varian" },
  { id: "sec-info", label: "Info tambahan" },
] as const;

function flattenErrors(input: unknown): Record<string, string> {
  const parsed = productSchema.safeParse(input);
  if (parsed.success) return {};
  const out: Record<string, string> = {};
  for (const issue of parsed.error.issues) {
    const key = issue.path.join(".");
    out[key || "_"] ??= issue.message;
  }
  return out;
}

/** Navigasi seksi yang menempel di kiri (layar lebar); menyorot seksi yang sedang terlihat. */
function SectionNav() {
  const [activeId, setActiveId] = useState<string>(SECTIONS[0].id);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) setActiveId(visible[0].target.id);
      },
      { rootMargin: "-80px 0px -60% 0px" },
    );
    for (const s of SECTIONS) {
      const el = document.getElementById(s.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, []);

  function jump(id: string) {
    setActiveId(id);
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <nav aria-label="Seksi form" className="sticky top-20 hidden self-start xl:block">
      <ul className="flex flex-col gap-0.5 border-l border-border">
        {SECTIONS.map((s) => (
          <li key={s.id}>
            <button
              type="button"
              onClick={() => jump(s.id)}
              aria-current={activeId === s.id ? "true" : undefined}
              className={cn(
                "-ml-px w-full border-l-2 px-3 py-1.5 text-left text-sm transition-colors",
                activeId === s.id
                  ? "border-primary font-medium text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}
            >
              {s.label}
            </button>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export default function ProductFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const isEdit = id !== undefined;
  const productId = isEdit ? Number(id) : NaN;

  const [form, setForm] = useState<ProductFormValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [banner, setBanner] = useState<MappedProductErrors["banner"]>(null);
  const [submitting, setSubmitting] = useState(false);
  // Apakah produk yang dimuat sudah punya varian pilihan (menentukan `[]` vs tanpa kunci).
  const [hadVisible, setHadVisible] = useState(false);
  const [productStatus, setProductStatus] = useState<string>("");
  // Foto awal (hanya form tambah): diunggah setelah produk tersimpan.
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoError, setPhotoError] = useState<string | null>(null);

  const allowed = isEdit ? can("products.update") : can("products.create");

  // Kunci mode form: 0 = tambah, -1 = id tak valid, >0 = id order edit.
  const validId = !isEdit || (Number.isInteger(productId) && productId > 0);
  const formKey = !isEdit ? 0 : !validId ? -1 : productId;

  const { data: categoryData } = useAsyncData(
    () =>
      productsService.listCategories().then(
        (cats) => cats,
        () => [] as ProductCategory[],
      ),
    [],
  );
  const categories = categoryData ?? [];

  const {
    data: editData,
    loading: editLoading,
    error: editFetchError,
    reload: reloadEdit,
  } = useAsyncData(() => {
    if (!isEdit || !validId) return Promise.resolve(null);
    const key = formKey;
    return productsService.detail(productId).then((p) => {
      const rows = variantRowsFromProduct(p);
      return {
        key,
        status: p.status,
        hadVisible: visibleVariants(p.variants).length > 0,
        form: {
          code: p.code,
          name: p.name,
          type: asProductType(p.type),
          status: p.status === "inactive" ? "inactive" : "active",
          categoryId: p.categoryId,
          tracked: p.tracked,
          baseUom: p.baseUom,
          purchaseUom: p.purchaseUom,
          salesUom: p.salesUom,
          purchaseFactor: p.purchaseFactor || 1,
          salesFactor: p.salesFactor || 1,
          purchasePrice: p.purchasePrice,
          sellingPrice: p.sellingPrice,
          minSellingPrice: p.minSellingPrice,
          minStock: p.minStock,
          summary: p.summary ?? "",
          attrs: attrsToRows(p.attributes),
          hasVariants: rows.length > 0,
          variants: rows,
        } satisfies ProductFormValues,
      };
    });
  }, [formKey]);
  const loading = isEdit ? editLoading : false;
  const loadError = !validId ? "ID produk tidak valid." : editFetchError;

  // Reset form saat ganti mode (tambah/edit/id lain).
  const [appliedKey, setAppliedKey] = useState<number | null>(null);
  const [prevKey, setPrevKey] = useState(formKey);
  if (formKey !== prevKey) {
    setPrevKey(formKey);
    setAppliedKey(null);
    setForm(EMPTY);
    setErrors({});
    setBanner(null);
    setProductStatus("");
    setHadVisible(false);
    setPhoto(null);
    setPhotoError(null);
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editData && editData.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setForm(editData.form);
    setProductStatus(editData.status);
    setHadVisible(editData.hadVisible);
  }

  function set<K extends keyof ProductFormValues>(key: K, value: ProductFormValues[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function changeType(next: ProductType) {
    setForm((f) => applyTypeRules({ ...f, type: next }));
  }

  function setVariants(rows: ProductVariantFormValues[]) {
    setForm((f) => ({ ...f, variants: rows }));
  }

  function patchVariant(index: number, patch: Partial<ProductVariantFormValues>) {
    setForm((f) => {
      let rows = f.variants.map((x, j) => (j === index ? { ...x, ...patch } : x));
      // Tanda "utama" eksklusif: memilih satu melepas yang lain.
      if (patch.isDefault) rows = rows.map((x, j) => (j === index ? x : { ...x, isDefault: false }));
      return { ...f, variants: rows };
    });
  }

  function toggleHasVariants(on: boolean) {
    setForm((f) => ({
      ...f,
      hasVariants: on,
      variants: on && f.variants.length === 0 ? [emptyVariantRow(0)] : f.variants,
    }));
  }

  function addVariant() {
    setForm((f) => ({ ...f, variants: [...f.variants, emptyVariantRow(f.variants.length)] }));
  }

  /** Varian tersimpan tidak dibuang (bisa punya riwayat) — hanya baris baru. */
  function removeNewVariant(index: number) {
    setForm((f) => ({ ...f, variants: f.variants.filter((_, i) => i !== index) }));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBanner(null);
    // Baris varian yang tersisa saat toggle dimatikan tidak divalidasi/dikirim.
    const normalized = applyTypeRules({
      ...form,
      variants: form.hasVariants ? normalizeDefault(form.variants) : [],
    });
    const fieldErrors = flattenErrors(normalized);
    setErrors(fieldErrors);
    if (Object.keys(fieldErrors).length > 0) {
      toast.warning("Periksa isian", "Ada isian yang belum valid.");
      return;
    }
    const parsed = productSchema.safeParse(normalized);
    if (!parsed.success) return;

    const body = buildProductPayload(parsed.data, hadVisible);

    setSubmitting(true);
    try {
      if (isEdit) {
        const updated = await productsService.update(productId, body);
        toast.fromServer(updated.message, "Produk disimpan");
        navigate(`${ROUTE_PATHS.products}/${updated.data.id}`);
      } else {
        const created = await productsService.create(body);
        toast.fromServer(created.message, "Produk dibuat");
        if (photo) {
          const res = await uploadInitialPhoto(
            (file, opts) => productsService.uploadMedia(created.data.id, file, opts),
            photo,
          );
          if (!res.ok) {
            toast.warning(
              "Produk dibuat, foto belum terunggah",
              `${res.message} Unggah ulang dari halaman detail produk.`,
            );
          }
        }
        navigate(`${ROUTE_PATHS.products}/${created.data.id}`);
      }
    } catch (err) {
      const apiErr = toApiError(err);
      const mapped = mapProductServerErrors(apiErr.message, apiErr.errors, normalized.variants);
      setErrors((prev) => ({ ...prev, ...mapped.fields }));
      setBanner(mapped.banner);
      toast.danger(isEdit ? "Gagal menyimpan produk" : "Gagal membuat produk", apiErr.message);
    } finally {
      setSubmitting(false);
    }
  }

  const title = isEdit ? "Ubah produk" : "Tambah produk";

  if (loading) return <DetailSkeleton />;

  if (loadError) {
    return (
      <Page>
        <PageHeader title={title} />
        <ErrorState title="Gagal memuat produk" message={loadError} onRetry={validId ? reloadEdit : undefined} />
      </Page>
    );
  }

  const archived = productStatus === "archived";
  const trackable = canTrack(form.type);
  const nonGoods = !trackable;
  const errorCount = Object.keys(errors).length;
  const defaultIndex = form.variants.findIndex((v) => v.isDefault && v.status === "active");

  return (
    <Page>
      <PageHeader
        title={title}
        description={
          isEdit
            ? "Ubah data katalog. Satuan, faktor, dan tipe terkunci bila sudah ada riwayat stok."
            : "Varian bawaan dibuat otomatis bila produk tidak punya varian."
        }
      />

      {!allowed && (
        <Notice tone="warning" title="Akses terbatas">
          Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "menambah"} produk.
        </Notice>
      )}

      {archived && (
        <Notice tone="muted" title="Produk diarsipkan">
          Produk yang sudah diarsipkan tidak dapat diubah.
        </Notice>
      )}

      {banner && (
        <Notice tone="warning" title={guardTitle(banner.kind)}>
          {banner.message}
        </Notice>
      )}

      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-1 gap-x-10 xl:grid-cols-[10rem_minmax(0,1fr)]">
          <SectionNav />
          <div className="min-w-0">
            <FormSection id="sec-identitas" title="Identitas" description="Kode, nama, tipe, dan kategori produk.">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField
                  label="Kode"
                  required
                  htmlFor="f-code"
                  errorText={errors.code}
                  helperText={`Maksimal ${PRODUCT_CODE_MAX} karakter. Kode tidak boleh sama dengan kode atau barcode varian produk lain.`}
                >
                  <TextInput
                    id="f-code"
                    value={form.code}
                    maxLength={PRODUCT_CODE_MAX}
                    onChange={(e) => set("code", e.target.value)}
                    placeholder="BRG-001"
                  />
                </FormField>
                <FormField label="Nama" required htmlFor="f-name" errorText={errors.name}>
                  <TextInput
                    id="f-name"
                    value={form.name}
                    maxLength={NAME_MAX}
                    onChange={(e) => set("name", e.target.value)}
                    placeholder="Nama produk"
                  />
                </FormField>
                <FormField
                  label="Tipe"
                  required
                  htmlFor="f-type"
                  errorText={errors.type}
                  helperText={
                    isEdit ? "Terkunci bila sudah ada riwayat stok." : "Hanya barang yang dapat dipantau stoknya."
                  }
                >
                  <SelectInput
                    id="f-type"
                    value={form.type}
                    onChange={(e) => changeType(e.target.value as ProductType)}
                  >
                    {PRODUCT_TYPE_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
                <FormField
                  label="Status"
                  htmlFor="f-status"
                  errorText={errors.status}
                  helperText="Produk nonaktif tidak bisa dipakai transaksi baru tetapi tetap tampil di daftar."
                >
                  <SelectInput
                    id="f-status"
                    value={form.status}
                    onChange={(e) => set("status", e.target.value as "active" | "inactive")}
                  >
                    <option value="active">Aktif</option>
                    <option value="inactive">Nonaktif</option>
                  </SelectInput>
                </FormField>
                <FormField label="Kategori" errorText={errors.categoryId}>
                  <SearchSelect
                    options={categories.map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }))}
                    value={form.categoryId}
                    onChange={(v) => set("categoryId", typeof v === "number" ? v : null)}
                    placeholder="Tanpa kategori"
                    allowClear
                  />
                </FormField>
                <FormField
                  label="Terpantau"
                  htmlFor="f-tracked"
                  errorText={errors.tracked}
                  helperText={
                    nonGoods
                      ? "Hanya tipe barang yang dipantau stoknya."
                      : isEdit
                        ? "Pantau stok tidak dapat dimatikan bila produk sudah punya riwayat atau saldo stok."
                        : undefined
                  }
                >
                  <SelectInput
                    id="f-tracked"
                    value={form.tracked ? "ya" : "tidak"}
                    disabled={nonGoods}
                    onChange={(e) => set("tracked", e.target.value === "ya")}
                  >
                    <option value="ya">Ya — stok dicatat</option>
                    <option value="tidak">Tidak</option>
                  </SelectInput>
                </FormField>
                <FormField label="Stok minimum" htmlFor="f-minstock" errorText={errors.minStock}>
                  <TextInput
                    id="f-minstock"
                    type="number"
                    min={0}
                    step="any"
                    value={String(form.minStock)}
                    onChange={(e) => set("minStock", Number(e.target.value))}
                  />
                </FormField>
                <div className="sm:col-span-2">
                  <FormField label="Ringkasan" htmlFor="f-summary" errorText={errors.summary}>
                    <TextArea
                      id="f-summary"
                      rows={3}
                      value={form.summary}
                      maxLength={SUMMARY_MAX}
                      onChange={(e) => set("summary", e.target.value)}
                      placeholder="Deskripsi singkat produk (opsional)"
                    />
                  </FormField>
                </div>
                {!isEdit && (
                  <div className="sm:col-span-2">
                    <FormField
                      label="Foto utama"
                      errorText={photoError ?? undefined}
                      helperText="Opsional. JPEG, PNG, atau WebP maksimal 5 MB — diunggah setelah produk tersimpan."
                    >
                      <FileDropzone
                        accept={MEDIA_ACCEPT_ATTR}
                        disabled={!allowed || submitting}
                        label="Seret foto ke sini atau pilih"
                        fileName={photo?.name ?? null}
                        onFile={(file) => {
                          const problem = validatePhotoFile(file);
                          setPhotoError(problem);
                          setPhoto(problem ? null : file);
                        }}
                      />
                    </FormField>
                    {photo && (
                      <Button variant="ghost" size="sm" onClick={() => setPhoto(null)}>
                        Hapus foto
                      </Button>
                    )}
                  </div>
                )}
              </div>
            </FormSection>

            <FormSection
              id="sec-satuan"
              title="Satuan"
              description={
                nonGoods
                  ? "Tipe ini memakai satu satuan: faktor 1 dan satuan beli/jual sama dengan satuan dasar."
                  : "Faktor konversi ke satuan dasar. Terkunci bila sudah ada riwayat stok."
              }
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <FormField label="Satuan dasar" required htmlFor="f-buom" errorText={errors.baseUom}>
                  <TextInput
                    id="f-buom"
                    value={form.baseUom}
                    maxLength={UOM_MAX}
                    onChange={(e) => {
                      const v = e.target.value;
                      setForm((f) => applyTypeRules({ ...f, baseUom: v }));
                    }}
                    placeholder="pcs"
                  />
                </FormField>
                <FormField label="Satuan beli" required htmlFor="f-puom" errorText={errors.purchaseUom}>
                  <TextInput
                    id="f-puom"
                    value={nonGoods ? form.baseUom : form.purchaseUom}
                    maxLength={UOM_MAX}
                    disabled={nonGoods}
                    onChange={(e) => set("purchaseUom", e.target.value)}
                    placeholder="dus"
                  />
                </FormField>
                <FormField label="Satuan jual" required htmlFor="f-suom" errorText={errors.salesUom}>
                  <TextInput
                    id="f-suom"
                    value={nonGoods ? form.baseUom : form.salesUom}
                    maxLength={UOM_MAX}
                    disabled={nonGoods}
                    onChange={(e) => set("salesUom", e.target.value)}
                    placeholder="pcs"
                  />
                </FormField>
                <FormField label="Faktor beli" required htmlFor="f-pf" errorText={errors.purchaseFactor}>
                  <TextInput
                    id="f-pf"
                    type="number"
                    min={0}
                    step="any"
                    disabled={nonGoods}
                    value={String(nonGoods ? 1 : form.purchaseFactor)}
                    onChange={(e) => set("purchaseFactor", Number(e.target.value))}
                  />
                </FormField>
                <FormField label="Faktor jual" required htmlFor="f-sf" errorText={errors.salesFactor}>
                  <TextInput
                    id="f-sf"
                    type="number"
                    min={0}
                    step="any"
                    disabled={nonGoods}
                    value={String(nonGoods ? 1 : form.salesFactor)}
                    onChange={(e) => set("salesFactor", Number(e.target.value))}
                  />
                </FormField>
              </div>
            </FormSection>

            <FormSection id="sec-harga" title="Harga" description="Rupiah penuh, tanpa desimal.">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <FormField
                  label="Harga beli"
                  htmlFor="f-pp"
                  errorText={errors.purchasePrice}
                  helperText={form.tracked ? "Wajib untuk produk terpantau." : undefined}
                >
                  <TextInput
                    id="f-pp"
                    type="number"
                    min={0}
                    step={1}
                    value={String(form.purchasePrice)}
                    onChange={(e) => set("purchasePrice", Number(e.target.value))}
                  />
                </FormField>
                <FormField label="Harga jual" htmlFor="f-sp" errorText={errors.sellingPrice}>
                  <TextInput
                    id="f-sp"
                    type="number"
                    min={0}
                    step={1}
                    value={String(form.sellingPrice)}
                    onChange={(e) => set("sellingPrice", Number(e.target.value))}
                  />
                </FormField>
                <FormField label="Harga jual minimal" htmlFor="f-msp" errorText={errors.minSellingPrice}>
                  <TextInput
                    id="f-msp"
                    type="number"
                    min={0}
                    step={1}
                    value={String(form.minSellingPrice)}
                    onChange={(e) => set("minSellingPrice", Number(e.target.value))}
                  />
                </FormField>
              </div>
            </FormSection>

            <FormSection
              id="sec-varian"
              title="Varian"
              description="Aktifkan bila produk dijual dalam beberapa varian. Tanpa varian, varian bawaan dipakai di belakang layar."
            >
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Switch id="f-has-variants" checked={form.hasVariants} onCheckedChange={toggleHasVariants} />
                  <label htmlFor="f-has-variants" className="text-sm font-medium text-foreground">
                    Produk punya varian
                  </label>
                </div>
                {form.hasVariants && (
                  <Button variant="secondary" size="sm" onClick={addVariant}>
                    <Plus aria-hidden /> Tambah varian
                  </Button>
                )}
              </div>
              {errors.variants && (
                <p role="alert" className="mb-3 text-xs font-medium text-destructive">
                  {errors.variants}
                </p>
              )}
              {!form.hasVariants ? (
                <p className="text-sm text-muted-foreground">
                  {isEdit && hadVisible
                    ? "Varian pilihan akan dikembalikan ke varian bawaan. Ditolak server bila ada varian yang masih punya stok atau riwayat."
                    : "Varian bawaan dibuat otomatis dengan kode produk."}
                </p>
              ) : (
                <RadioGroup
                  aria-label="Varian utama"
                  value={defaultIndex >= 0 ? String(defaultIndex) : ""}
                  onValueChange={(v) => patchVariant(Number(v), { isDefault: true })}
                  className="gap-4"
                >
                  {form.variants.map((vv, i) => {
                    const persisted = vv.id !== undefined;
                    return (
                      <div key={vv.id ?? `new-${i}`} className="rounded-lg border border-border p-4">
                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                          <FormField label="Kode" required errorText={errors[`variants.${i}.code`]}>
                            <TextInput
                              aria-label={`Kode varian ${i + 1}`}
                              value={vv.code}
                              maxLength={VARIANT_CODE_MAX}
                              onChange={(e) => patchVariant(i, { code: e.target.value })}
                              placeholder="BRG-001-A"
                            />
                          </FormField>
                          <FormField label="Nama" required errorText={errors[`variants.${i}.name`]}>
                            <TextInput
                              aria-label={`Nama varian ${i + 1}`}
                              value={vv.name}
                              maxLength={NAME_MAX}
                              onChange={(e) => patchVariant(i, { name: e.target.value })}
                              placeholder="Varian A"
                            />
                          </FormField>
                          <FormField label="Barcode" errorText={errors[`variants.${i}.barcode`]}>
                            <TextInput
                              aria-label={`Barcode varian ${i + 1}`}
                              value={vv.barcode}
                              maxLength={BARCODE_MAX}
                              onChange={(e) => patchVariant(i, { barcode: e.target.value })}
                              placeholder="Opsional"
                            />
                          </FormField>
                          <FormField label="Status">
                            <SelectInput
                              aria-label={`Status varian ${i + 1}`}
                              value={vv.status}
                              onChange={(e) => patchVariant(i, { status: e.target.value as "active" | "inactive" })}
                            >
                              <option value="active">Aktif</option>
                              <option value="inactive">Nonaktif</option>
                            </SelectInput>
                          </FormField>
                        </div>
                        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
                          <div className="flex items-center gap-2">
                            <RadioGroupItem
                              id={`variant-default-${i}`}
                              value={String(i)}
                              disabled={vv.status !== "active"}
                            />
                            <label htmlFor={`variant-default-${i}`} className="text-sm text-foreground">
                              Varian utama
                            </label>
                          </div>
                          {persisted ? (
                            <span className="text-xs text-muted-foreground">
                              Varian tersimpan tidak dihapus — nonaktifkan bila tak dipakai.
                            </span>
                          ) : (
                            <Button variant="ghost" size="sm" onClick={() => removeNewVariant(i)}>
                              Hapus
                            </Button>
                          )}
                        </div>
                        <details className="mt-3">
                          <summary className="cursor-pointer text-sm text-muted-foreground">
                            Info tambahan varian ({vv.attrs.length})
                          </summary>
                          <div className="mt-2">
                            <KeyValueEditor
                              idPrefix={`variant-attrs-${i}`}
                              rows={vv.attrs}
                              onChange={(rows) => patchVariant(i, { attrs: rows })}
                              limits={PRODUCT_ATTR_LIMITS}
                              suggestions={["Warna", "Ukuran", "Rasa"]}
                              serverError={errors[`variants.${i}.attrs`]}
                            />
                          </div>
                        </details>
                      </div>
                    );
                  })}
                  {form.variants.length === 0 && (
                    <Button variant="secondary" size="sm" className="w-fit" onClick={() => setVariants([emptyVariantRow(0)])}>
                      <Plus aria-hidden /> Tambah varian pertama
                    </Button>
                  )}
                </RadioGroup>
              )}
            </FormSection>

            <FormSection
              id="sec-info"
              title="Info tambahan"
              description={`Pasangan nama/isi bebas, maksimal ${PRODUCT_ATTR_LIMITS.maxRows}.`}
            >
              <KeyValueEditor
                idPrefix="product-attrs"
                rows={form.attrs}
                onChange={(rows) => set("attrs", rows)}
                limits={PRODUCT_ATTR_LIMITS}
                suggestions={ATTR_SUGGESTIONS}
                serverError={errors.attributes ?? errors.attrs}
              />
            </FormSection>
          </div>
        </div>

        <StickyFooter
          summary={
            errorCount > 0 ? (
              <span role="status" className="font-medium text-destructive">
                {errorCount} isian perlu diperbaiki
              </span>
            ) : undefined
          }
        >
          <Button
            variant="secondary"
            onClick={() => navigate(isEdit ? `${ROUTE_PATHS.products}/${productId}` : ROUTE_PATHS.products)}
          >
            Batal
          </Button>
          <Button type="submit" loading={submitting} disabled={!allowed || archived}>
            {isEdit ? "Simpan" : "Buat produk"}
          </Button>
        </StickyFooter>
      </form>
    </Page>
  );
}
