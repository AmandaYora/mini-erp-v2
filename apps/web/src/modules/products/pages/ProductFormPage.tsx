import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ActionRow,
  Button,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import { productSchema } from "@/modules/products/schemas/product.schema";
import type { ProductFormValues } from "@/modules/products/schemas/product.schema";
import { productsService } from "@/modules/products/services/products.service";
import type { ProductCategory, ProductPayload } from "@/modules/products/types";

const EMPTY: ProductFormValues = {
  code: "",
  name: "",
  type: "barang",
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
  variants: [],
};

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

export default function ProductFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const isEdit = id !== undefined;
  const productId = isEdit ? Number(id) : NaN;

  const [form, setForm] = useState<ProductFormValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

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

  const { data: editData, loading: editLoading, error: editFetchError } = useAsyncData(
    () => {
      if (!isEdit || !validId) return Promise.resolve(null);
      const key = formKey;
      return productsService.detail(productId).then((p) => ({
        key,
        form: {
          code: p.code,
          name: p.name,
          type: (p.type === "jasa" ? "jasa" : "barang") as "barang" | "jasa",
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
          variants: (p.variants ?? []).map((v) => ({
            code: v.code,
            name: v.name,
            barcode: v.barcode ?? "",
            isDefault: v.isDefault,
          })),
        } satisfies ProductFormValues,
      }));
    },
    [formKey],
  );
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
  }

  // Prefill edit: salin bundel fetch ke state form sekali per kunci.
  if (editData && editData.key === formKey && appliedKey !== formKey) {
    setAppliedKey(formKey);
    setForm(editData.form);
  }

  function set<K extends keyof ProductFormValues>(key: K, value: ProductFormValues[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function addVariant() {
    setForm((f) => ({
      ...f,
      variants: [...f.variants, { code: "", name: "", barcode: "", isDefault: f.variants.length === 0 }],
    }));
  }

  function removeVariant(index: number) {
    setForm((f) => ({ ...f, variants: f.variants.filter((_, i) => i !== index) }));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fieldErrors = flattenErrors(form);
    setErrors(fieldErrors);
    if (Object.keys(fieldErrors).length > 0) return;
    const parsed = productSchema.safeParse(form);
    if (!parsed.success) return;

    const v = parsed.data;
    const body: ProductPayload = {
      code: v.code.trim(),
      name: v.name.trim(),
      categoryId: v.categoryId,
      type: v.type,
      tracked: v.tracked,
      baseUom: v.baseUom.trim(),
      purchaseUom: v.purchaseUom.trim(),
      salesUom: v.salesUom.trim(),
      purchaseFactor: v.purchaseFactor,
      salesFactor: v.salesFactor,
      purchasePrice: v.purchasePrice,
      sellingPrice: v.sellingPrice,
      minSellingPrice: v.minSellingPrice,
      minStock: v.minStock,
      variants: v.variants.map((x) => ({
        code: x.code.trim(),
        name: x.name.trim(),
        barcode: (x.barcode ?? "").trim(),
        isDefault: x.isDefault,
      })),
    };

    setSubmitting(true);
    try {
      if (isEdit) {
        const updated = await productsService.update(productId, body);
        toast.fromServer(updated.message, "Produk disimpan");
        navigate(`/products/${updated.data.id}`);
      } else {
        const created = await productsService.create(body);
        toast.fromServer(created.message, "Produk dibuat");
        navigate(`/products/${created.data.id}`);
      }
    } catch (err) {
      const apiErr = toApiError(err);
      toast.danger(isEdit ? "Gagal menyimpan produk" : "Gagal membuat produk", apiErr.message);
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
        <PageHeader title={isEdit ? "Ubah Produk" : "Tambah Produk"} />
        <Notice tone="danger" title="Gagal memuat produk">
          {loadError}
        </Notice>
        <Button variant="secondary" onClick={() => navigate("/products")}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Produk"
        title={isEdit ? "Ubah Produk" : "Tambah Produk"}
        description={
          isEdit
            ? "Ubah data katalog. Satuan dan tipe terkunci bila sudah ada riwayat stok."
            : "Varian default dibuat otomatis bila varian dikosongkan."
        }
      />

      {!allowed && (
        <div className="mb-4">
          <Notice tone="warning" title="Akses terbatas">
            Anda tidak memiliki izin untuk {isEdit ? "mengubah" : "menambah"} produk.
          </Notice>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
          <SectionCard title="Identitas">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField label="Kode" required htmlFor="f-code" errorText={errors.code}>
                <TextInput
                  id="f-code"
                  value={form.code}
                  onChange={(e) => set("code", e.target.value)}
                  placeholder="BRG-001"
                />
              </FormField>
              <FormField label="Nama" required htmlFor="f-name" errorText={errors.name}>
                <TextInput
                  id="f-name"
                  value={form.name}
                  onChange={(e) => set("name", e.target.value)}
                  placeholder="Nama produk"
                />
              </FormField>
              <FormField label="Tipe" required htmlFor="f-type" errorText={errors.type}>
                <SelectInput
                  id="f-type"
                  value={form.type}
                  onChange={(e) => set("type", e.target.value as "barang" | "jasa")}
                >
                  <option value="barang">Barang</option>
                  <option value="jasa">Jasa</option>
                </SelectInput>
              </FormField>
              <div>
                <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">Kategori</span>
                <SearchSelect
                  options={categories.map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }))}
                  value={form.categoryId}
                  onChange={(v) => set("categoryId", typeof v === "number" ? v : null)}
                  placeholder="Tanpa kategori"
                  allowClear
                />
                {errors.categoryId && <p className="text-bad mt-1 text-[0.82rem]">{errors.categoryId}</p>}
              </div>
              <FormField label="Terpantau" htmlFor="f-tracked" helperText="Jasa biasanya tidak terpantau.">
                <SelectInput
                  id="f-tracked"
                  value={form.tracked ? "ya" : "tidak"}
                  onChange={(e) => set("tracked", e.target.value === "ya")}
                >
                  <option value="ya">Ya — stok dicatat</option>
                  <option value="tidak">Tidak</option>
                </SelectInput>
              </FormField>
              <FormField
                label="Stok Minimum"
                htmlFor="f-minstock"
                errorText={errors.minStock}
              >
                <TextInput
                  id="f-minstock"
                  type="number"
                  min={0}
                  step="any"
                  value={String(form.minStock)}
                  onChange={(e) => set("minStock", Number(e.target.value))}
                />
              </FormField>
            </div>
          </SectionCard>

          <SectionCard title="Satuan" description="Faktor konversi ke satuan dasar.">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <FormField label="Satuan Dasar" required htmlFor="f-buom" errorText={errors.baseUom}>
                <TextInput
                  id="f-buom"
                  value={form.baseUom}
                  onChange={(e) => set("baseUom", e.target.value)}
                  placeholder="pcs"
                />
              </FormField>
              <FormField label="Satuan Beli" required htmlFor="f-puom" errorText={errors.purchaseUom}>
                <TextInput
                  id="f-puom"
                  value={form.purchaseUom}
                  onChange={(e) => set("purchaseUom", e.target.value)}
                  placeholder="dus"
                />
              </FormField>
              <FormField label="Satuan Jual" required htmlFor="f-suom" errorText={errors.salesUom}>
                <TextInput
                  id="f-suom"
                  value={form.salesUom}
                  onChange={(e) => set("salesUom", e.target.value)}
                  placeholder="pcs"
                />
              </FormField>
              <FormField label="Faktor Beli" required htmlFor="f-pf" errorText={errors.purchaseFactor}>
                <TextInput
                  id="f-pf"
                  type="number"
                  min={0}
                  step="any"
                  value={String(form.purchaseFactor)}
                  onChange={(e) => set("purchaseFactor", Number(e.target.value))}
                />
              </FormField>
              <FormField label="Faktor Jual" required htmlFor="f-sf" errorText={errors.salesFactor}>
                <TextInput
                  id="f-sf"
                  type="number"
                  min={0}
                  step="any"
                  value={String(form.salesFactor)}
                  onChange={(e) => set("salesFactor", Number(e.target.value))}
                />
              </FormField>
            </div>
          </SectionCard>

          <SectionCard title="Harga" description="Rupiah penuh, tanpa desimal.">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <FormField label="Harga Beli" htmlFor="f-pp" errorText={errors.purchasePrice}>
                <TextInput
                  id="f-pp"
                  type="number"
                  min={0}
                  step={1}
                  value={String(form.purchasePrice)}
                  onChange={(e) => set("purchasePrice", Number(e.target.value))}
                />
              </FormField>
              <FormField label="Harga Jual" htmlFor="f-sp" errorText={errors.sellingPrice}>
                <TextInput
                  id="f-sp"
                  type="number"
                  min={0}
                  step={1}
                  value={String(form.sellingPrice)}
                  onChange={(e) => set("sellingPrice", Number(e.target.value))}
                />
              </FormField>
              <FormField
                label="Harga Jual Minimal"
                htmlFor="f-msp"
                errorText={errors.minSellingPrice}
              >
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
          </SectionCard>

          <SectionCard
            title="Varian"
            description="Kosongkan untuk varian default otomatis."
            actions={
              <Button variant="secondary" size="sm" onClick={addVariant}>
                Tambah varian
              </Button>
            }
          >
            {form.variants.length === 0 ? (
              <p className="text-muted text-sm">Belum ada varian — backend akan membuat varian default.</p>
            ) : (
              <div className="space-y-4">
                {form.variants.map((vv, i) => (
                  <div key={i} className="grid grid-cols-1 gap-3 rounded-lg border border-hairline p-4 sm:grid-cols-4">
                    <FormField label="Kode" required errorText={errors[`variants.${i}.code`]}>
                      <TextInput
                        value={vv.code}
                        onChange={(e) =>
                          set("variants", form.variants.map((x, j) => (j === i ? { ...x, code: e.target.value } : x)))
                        }
                        placeholder="BRG-001-A"
                      />
                    </FormField>
                    <FormField label="Nama" required errorText={errors[`variants.${i}.name`]}>
                      <TextInput
                        value={vv.name}
                        onChange={(e) =>
                          set("variants", form.variants.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))
                        }
                        placeholder="Varian A"
                      />
                    </FormField>
                    <FormField label="Barcode">
                      <TextInput
                        value={vv.barcode ?? ""}
                        onChange={(e) =>
                          set("variants", form.variants.map((x, j) => (j === i ? { ...x, barcode: e.target.value } : x)))
                        }
                        placeholder="Opsional"
                      />
                    </FormField>
                    <div className="flex items-end justify-between gap-2">
                      <label className="flex items-center gap-2 text-sm text-ink">
                        <input
                          type="checkbox"
                          checked={vv.isDefault}
                          onChange={(e) =>
                            set(
                              "variants",
                              form.variants.map((x, j) => (j === i ? { ...x, isDefault: e.target.checked } : x)),
                            )
                          }
                        />
                        Utama
                      </label>
                      <Button variant="ghost" size="sm" onClick={() => removeVariant(i)}>
                        Hapus
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </SectionCard>
        </div>

        <ActionRow>
          <Button variant="secondary" onClick={() => navigate(isEdit ? `/products/${productId}` : "/products")}>
            Batal
          </Button>
          <Button type="submit" disabled={submitting || !allowed}>
            {submitting ? "Menyimpan…" : isEdit ? "Simpan" : "Buat"}
          </Button>
        </ActionRow>
      </form>
    </div>
  );
}
