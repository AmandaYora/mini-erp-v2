import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { adjustmentSchema } from "@/modules/stock/schemas/stock.schema";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductPicker } from "@/modules/stock/components/ProductPicker";

export default function AdjustmentsPage() {
  const can = useAuthStore((s) => s.can);
  const canAdjust = can("stock.adjust");

  const [productId, setProductId] = useState<number | null>(null);
  const [variantId, setVariantId] = useState<number | null>(null);
  const [locationId, setLocationId] = useState<number | null>(null);
  const [mode, setMode] = useState("in");
  const [qty, setQty] = useState("");
  const [reason, setReason] = useState("");
  const [approver, setApprover] = useState("");
  const [approverPassword, setApproverPassword] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = adjustmentSchema.safeParse({
      productId: productId ?? 0,
      variantId: variantId ?? 0,
      locationId: locationId ?? 0,
      mode,
      qty,
      reason: reason.trim(),
      approver: approver.trim(),
      approverPassword,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setErrors(errs);
      return;
    }
    setErrors({});
    setServerError(null);
    setSaving(true);
    try {
      // approver + approverPassword selalu dikirim (boleh string kosong);
      // backend memutuskan kapan persetujuan atasan wajib.
      const res = await stockService.createAdjustment({
        productId: parsed.data.productId,
        variantId: parsed.data.variantId,
        locationId: parsed.data.locationId,
        mode: parsed.data.mode,
        qtyAfter: parsed.data.mode === "set" ? parsed.data.qty : 0,
        qtyDelta: parsed.data.mode === "set" ? 0 : parsed.data.qty,
        reason: parsed.data.reason,
        approver: parsed.data.approver,
        approverPassword: parsed.data.approverPassword,
      });
      toast.fromServer(res.message, "Koreksi tersimpan");
      setQty("");
      setReason("");
      setApprover("");
      setApproverPassword("");
    } catch (err) {
      const apiErr = toApiError(err);
      setServerError(apiErr.message);
      toast.danger("Gagal menyimpan koreksi", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title="Koreksi Stok"
        description="Penyesuaian saldo: penambahan, pengurangan, atau penetapan hasil opname."
      />

      {!canAdjust && (
        <div className="mb-4">
          <Notice tone="warning" title="Izin terbatas">
            Akun Anda tidak memiliki izin stock.adjust. Formulir dikunci.
          </Notice>
        </div>
      )}
      {serverError && (
        <div className="mb-4">
          <Notice tone="danger" title="Koreksi ditolak server">
            {serverError}
          </Notice>
        </div>
      )}

      <SectionCard
        title="Formulir Koreksi"
        description="Alasan wajib diisi. Kolom penyetuju selalu tampil dan selalu dikirim."
      >
        <form onSubmit={(e) => void handleSubmit(e)}>
          <fieldset disabled={!canAdjust || saving} className="space-y-4">
            <ProductPicker
              productId={productId}
              variantId={variantId}
              onProductChange={setProductId}
              onVariantChange={setVariantId}
            />
            {(errors.productId ?? errors.variantId) && (
              <p className="text-bad text-[0.82rem]">
                {errors.productId ?? errors.variantId}
              </p>
            )}
            <div className="grid gap-4 md:grid-cols-3">
              <LocationSelect
                value={locationId}
                onChange={setLocationId}
                required
              />
              <FormField label="Mode" required>
                <SelectInput
                  value={mode}
                  onChange={(e) => setMode(e.target.value)}
                >
                  <option value="in">Penambahan</option>
                  <option value="out">Pengurangan</option>
                  <option value="set">Set ke jumlah pasti</option>
                </SelectInput>
              </FormField>
              <FormField
                label={mode === "set" ? "Jumlah Akhir" : "Selisih Jumlah"}
                required
                helperText={
                  mode === "set"
                    ? "Stok fisik hasil opname."
                    : "Jumlah yang ditambah/dikurangi."
                }
                errorText={errors.qty ?? errors.locationId}
              >
                <TextInput
                  type="number"
                  min="0"
                  step="any"
                  placeholder="cth: 10"
                  value={qty}
                  onChange={(e) => setQty(e.target.value)}
                />
              </FormField>
            </div>
            <FormField label="Alasan" required errorText={errors.reason}>
              <TextArea
                rows={2}
                placeholder="cth: Selisih hasil stok opname 10 Sep"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </FormField>
            <div className="grid gap-4 md:grid-cols-2">
              <FormField
                label="Nama Penyetuju"
                helperText="Diisi bila nilai koreksi mensyaratkan persetujuan atasan."
              >
                <TextInput
                  placeholder="cth: supervisor01"
                  value={approver}
                  onChange={(e) => setApprover(e.target.value)}
                />
              </FormField>
              <FormField label="Kata Sandi Penyetuju">
                <TextInput
                  type="password"
                  value={approverPassword}
                  onChange={(e) => setApproverPassword(e.target.value)}
                />
              </FormField>
            </div>
            <div>
              <Button type="submit" disabled={!canAdjust || saving}>
                {saving ? "Menyimpan…" : "Simpan Koreksi"}
              </Button>
            </div>
          </fieldset>
        </form>
      </SectionCard>
    </div>
  );
}
