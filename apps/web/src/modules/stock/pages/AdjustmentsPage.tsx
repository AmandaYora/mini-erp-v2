import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  CardSkeleton,
  FormField,
  PageHeader,
  PasswordInput,
  SelectInput,
  SectionCard,
  Skeleton,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { cn } from "@/shared/lib/cn";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { adjustmentSchemaFor } from "@/modules/stock/schemas/stock.schema";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { VariantSelect } from "@/modules/stock/components/VariantSelect";
import { StockLocationSelect } from "@/modules/stock/components/StockLocationSelect";
import { parseQty, physicalDelta, resultingQty } from "@/modules/stock/lib/adjustment";
import type { AdjustmentMode } from "@/modules/stock/schemas/stock.schema";
import type { AdjustmentPurpose, StockBalance, StockLocation } from "@/modules/stock/types";

interface Line {
  key: number;
  productId: number | null;
  variantId: number | null;
  locationId: number | null;
  /** Isian jumlah: qty fisik (mode set) atau jumlah tambah/kurang (mode in/out). */
  physical: string;
}

let nextKey = 1;
const newLine = (): Line => ({ key: nextKey++, productId: null, variantId: null, locationId: null, physical: "" });
const MODE_OPTIONS: { value: AdjustmentMode; label: string }[] = [
  { value: "set", label: "Set ke jumlah fisik" },
  { value: "in", label: "Penambahan" },
  { value: "out", label: "Pengurangan" },
];
// Tujuan koreksi (ADR-0024): selisih stok masuk laba rugi; koreksi saldo awal
// membetulkan stok awal yang salah input dan dibukukan ke ekuitas.
const PURPOSE_OPTIONS: { value: "" | AdjustmentPurpose; label: string }[] = [
  { value: "", label: "Selisih stok (opname, hilang, rusak)" },
  { value: "opening_correction", label: "Koreksi saldo awal (stok awal salah)" },
];
const isBlank = (l: Line) => l.productId === null && l.locationId === null && l.physical.trim() === "";

function CellError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="mt-1 text-left text-xs font-medium text-destructive">
      {message}
    </p>
  );
}

/**
 * Koreksi stok: satu baris per produk/varian/lokasi, satu mode untuk seluruh baris
 * ("set" ke jumlah fisik, penambahan "in", pengurangan "out"). Tiap baris = satu POST
 * /stock/adjustments dengan payload yang sama seperti sebelumnya, diproses berurutan.
 */
export default function AdjustmentsPage() {
  const can = useAuthStore((s) => s.can);
  const canAdjust = can("stock.adjust");
  const canOpening = can("stock.adjust_opening");

  const [lines, setLines] = useState<Line[]>(() => [newLine()]);
  const [mode, setMode] = useState<AdjustmentMode>("set");
  const [purpose, setPurpose] = useState<"" | AdjustmentPurpose>("");
  const [reason, setReason] = useState("");
  const [approver, setApprover] = useState("");
  const [approverPassword, setApproverPassword] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Kebijakan "semua koreksi perlu penyetuju" — ketat secara default (hanya
  // stock_adjust_always_approve=false yang melonggarkan). Bila tak berizin membaca,
  // server tetap otoritas dan pesannya ditampilkan.
  const { data: strictData, loading: policyLoading } = useAsyncData(
    () => stockService.adjustAlwaysApprove().catch(() => true),
    [],
  );
  const strictApprover = strictData ?? true;

  const { data: locations } = useAsyncData(
    () => stockService.listLocations().catch(() => [] as StockLocation[]),
    [],
  );
  const locationList = useMemo(() => (locations ?? []).filter((l) => l.status !== "archived"), [locations]);

  // Qty sistem: saldo per produk (GET /stock/balances?productId=), dimuat untuk produk yang dipilih.
  const productKey = [...new Set(lines.map((l) => l.productId).filter((p): p is number => p !== null))]
    .sort((a, b) => a - b)
    .join(",");
  const {
    data: balanceMap,
    loading: balancesLoading,
    error: balancesError,
    reload: reloadBalances,
  } = useAsyncData(async () => {
    const map = new Map<number, StockBalance[]>();
    const ids = productKey === "" ? [] : productKey.split(",").map(Number);
    await Promise.all(
      ids.map(async (id) => {
        map.set(id, (await stockService.balancesByProduct(id)) ?? []);
      }),
    );
    return map;
  }, [productKey]);

  function systemQty(l: Line): number | null {
    if (l.productId === null || l.variantId === null || l.locationId === null) return null;
    const rows = balanceMap?.get(l.productId);
    if (!rows) return null;
    return rows.find((b) => b.variantId === l.variantId && b.locationId === l.locationId)?.onHand ?? 0;
  }

  const patch = (key: number, p: Partial<Line>) =>
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...p } : l)));

  const diffCount = lines.filter((l) => {
    const q = parseQty(l.physical);
    const d = mode === "set" ? physicalDelta(systemQty(l), q) : q;
    return d !== null && d !== 0;
  }).length;
  const errorCount = Object.keys(errors).length;

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const filled = lines.filter((l) => !isBlank(l));
    const errs: Record<string, string> = {};
    if (filled.length === 0) errs.lines = "Isi minimal satu baris koreksi.";

    const schema = adjustmentSchemaFor(strictApprover);
    const seen = new Set<string>();
    const payloads: { line: Line; index: number; qty: number }[] = [];
    filled.forEach((l) => {
      const index = lines.indexOf(l);
      // Kosong dibaca 0 seperti form lama (skema meng-coerce); angka tak valid ditolak.
      const raw = l.physical.trim();
      const qty = raw === "" ? 0 : parseQty(raw);
      if (qty === null) errs[`${index}.qty`] = "Jumlah harus berupa angka 0 atau lebih";
      const parsed = schema.safeParse({
        productId: l.productId ?? 0,
        variantId: l.variantId ?? 0,
        locationId: l.locationId ?? 0,
        mode,
        qty: qty ?? 0,
        reason: reason.trim(),
        approver: approver.trim(),
        approverPassword,
      });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const field = String(issue.path[0] ?? "");
          if (field === "reason" || field === "approver" || field === "approverPassword") {
            errs[field] ??= issue.message;
          } else if (field !== "qty" || qty !== null) {
            errs[`${index}.${field}`] ??= issue.message;
          }
        }
      }
      const dupKey = `${l.productId}/${l.variantId}/${l.locationId}`;
      if (l.productId !== null && l.variantId !== null && l.locationId !== null) {
        if (seen.has(dupKey)) errs[`${index}.locationId`] ??= "Produk, varian, dan lokasi ini sudah ada di baris lain";
        seen.add(dupKey);
      }
      if (qty !== null) payloads.push({ line: l, index, qty });
    });
    setErrors(errs);
    setServerError(null);
    if (Object.keys(errs).length > 0) return;
    if (payloads.length === 0) {
      setServerError("Isi minimal satu baris koreksi.");
      return;
    }

    setSaving(true);
    const done = new Set<number>();
    try {
      for (const p of payloads) {
        try {
          // approver + approverPassword selalu dikirim (boleh string kosong);
          // backend memutuskan kapan persetujuan atasan wajib.
          await stockService.createAdjustment({
            productId: p.line.productId!,
            variantId: p.line.variantId!,
            locationId: p.line.locationId!,
            mode,
            qtyAfter: mode === "set" ? p.qty : 0,
            qtyDelta: mode === "set" ? 0 : p.qty,
            reason: reason.trim(),
            approver: approver.trim(),
            approverPassword,
            ...(purpose ? { purpose } : {}),
          });
          done.add(p.line.key);
        } catch (err) {
          const apiErr = toApiError(err);
          setServerError(`Baris ${p.index + 1}: ${apiErr.message}`);
          toast.danger("Gagal menyimpan koreksi", apiErr.message);
          return;
        }
      }
      toast.success(done.size > 1 ? `${done.size} koreksi tersimpan` : "Koreksi tersimpan");
      setLines([newLine()]);
      setReason("");
      setApprover("");
      setApproverPassword("");
    } finally {
      // Baris yang sudah tersimpan dilepas dari daftar agar tidak terkirim dua kali bila ada yang gagal.
      if (done.size > 0) {
        setLines((prev) => {
          const rest = prev.filter((l) => !done.has(l.key));
          return rest.length > 0 ? rest : [newLine()];
        });
      }
      reloadBalances();
      setSaving(false);
    }
  }

  return (
    <Page>
      <PageHeader
        title="Koreksi Stok"
        description="Samakan saldo sistem dengan hasil hitung fisik (opname). Selisih dihitung otomatis."
      />

      {!canAdjust && (
        <Notice tone="warning" title="Izin terbatas">
          Akun Anda tidak memiliki izin stock.adjust. Formulir dikunci.
        </Notice>
      )}
      {serverError && (
        <Notice tone="danger" title="Koreksi ditolak server">
          {serverError}
        </Notice>
      )}
      {balancesError && (
        <Notice tone="warning" title="Qty sistem belum bisa dimuat">
          {balancesError}{" "}
          <Button variant="ghost" size="sm" onClick={reloadBalances} className="h-auto px-1.5 py-0.5 align-baseline">
            Coba lagi
          </Button>
        </Notice>
      )}

      <form id="adjustment-form" className="flex flex-col gap-6" onSubmit={(e) => void handleSubmit(e)} noValidate>
        <fieldset disabled={!canAdjust || saving} className="flex min-w-0 flex-col gap-6">
          <SectionCard title="Barang yang dikoreksi" description="Satu baris per produk, varian, dan lokasi.">
            <div className="mb-4 flex flex-wrap gap-4">
              <div className="w-64 max-w-full">
                <FormField label="Mode koreksi" htmlFor="adjust-mode">
                  <SelectInput id="adjust-mode" value={mode} onChange={(e) => setMode(e.target.value as AdjustmentMode)}>
                    {MODE_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
              </div>
              {canOpening && (
                <div className="w-80 max-w-full">
                  <FormField
                    label="Tujuan koreksi"
                    htmlFor="adjust-purpose"
                    helperText={
                      purpose
                        ? "Untuk stok awal yang salah input/impor. Dibukukan ke Koreksi Saldo Awal (ekuitas), tidak memengaruhi laba."
                        : "Selisih stok dibukukan ke laba rugi (akun Selisih Stok)."
                    }
                  >
                    <SelectInput
                      id="adjust-purpose"
                      value={purpose}
                      onChange={(e) => setPurpose(e.target.value as "" | AdjustmentPurpose)}
                    >
                      {PURPOSE_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </SelectInput>
                  </FormField>
                </div>
              )}
            </div>
            <CellError message={errors.lines} />
            <LineItemsEditor<Line>
              lines={lines}
              getKey={(l) => l.key}
              onAdd={() => setLines((prev) => [...prev, newLine()])}
              onRemove={(i) => setLines((prev) => (prev.length > 1 ? prev.filter((_, idx) => idx !== i) : prev))}
              addLabel="Tambah baris"
              columns={[
                {
                  id: "product",
                  header: "Produk",
                  width: "18rem",
                  render: (l, i) => (
                    <div role="group" aria-label={`Produk baris ${i + 1}`}>
                      <ProductSelect
                        productId={l.productId}
                        allowClear={false}
                        onChange={(id) => patch(l.key, { productId: id, variantId: null })}
                      />
                      <CellError message={errors[`${i}.productId`]} />
                    </div>
                  ),
                },
                {
                  id: "variant",
                  header: "Varian",
                  width: "12rem",
                  render: (l, i) => (
                    <div>
                      <VariantSelect
                        aria-label={`Varian baris ${i + 1}`}
                        productId={l.productId}
                        variantId={l.variantId}
                        autoPick
                        onChange={(id) => patch(l.key, { variantId: id })}
                      />
                      <CellError message={errors[`${i}.variantId`]} />
                    </div>
                  ),
                },
                {
                  id: "location",
                  header: "Lokasi",
                  width: "14rem",
                  render: (l, i) => (
                    <div role="group" aria-label={`Lokasi baris ${i + 1}`}>
                      <StockLocationSelect
                        locations={locationList}
                        value={l.locationId}
                        onChange={(id) => patch(l.key, { locationId: id })}
                      />
                      <CellError message={errors[`${i}.locationId`]} />
                    </div>
                  ),
                },
                {
                  id: "system",
                  header: "Qty sistem",
                  width: "7rem",
                  align: "right",
                  render: (l) => {
                    const sys = systemQty(l);
                    if (sys !== null) return <span className="inline-block py-2"><Qty value={sys} /></span>;
                    if (l.productId !== null && balancesLoading)
                      return <Skeleton className="ml-auto mt-2.5 h-4 w-12" />;
                    return <span className="inline-block py-2 text-muted-foreground">–</span>;
                  },
                },
                {
                  id: "physical",
                  header: mode === "set" ? "Qty fisik" : mode === "in" ? "Jumlah tambah" : "Jumlah kurang",
                  width: "8rem",
                  align: "right",
                  render: (l, i) => (
                    <div>
                      <TextInput
                        aria-label={`${mode === "set" ? "Qty fisik" : "Jumlah"} baris ${i + 1}`}
                        type="number"
                        min="0"
                        step="any"
                        inputMode="decimal"
                        className="tabular text-right"
                        placeholder="0"
                        value={l.physical}
                        onChange={(e) => patch(l.key, { physical: e.target.value })}
                      />
                      <CellError message={errors[`${i}.qty`]} />
                    </div>
                  ),
                },
                {
                  id: "delta",
                  header: mode === "set" ? "Selisih" : "Hasil",
                  width: "7rem",
                  align: "right",
                  render: (l) => {
                    const q = parseQty(l.physical);
                    if (mode !== "set") {
                      const r = resultingQty(systemQty(l), q, mode);
                      if (r === null) return <span className="inline-block py-2 text-muted-foreground">�</span>;
                      return (
                        <span className={cn("tabular inline-block py-2 font-medium", r < 0 && "text-destructive")}>
                          <Qty value={r} />
                        </span>
                      );
                    }
                    const d = physicalDelta(systemQty(l), q);
                    if (d === null) return <span className="inline-block py-2 text-muted-foreground">–</span>;
                    return (
                      <span
                        className={cn(
                          "tabular inline-block py-2 font-medium",
                          d > 0 && "text-success",
                          d < 0 && "text-destructive",
                          d === 0 && "text-muted-foreground",
                        )}
                      >
                        {d > 0 ? "+" : ""}
                        <Qty value={d} />
                      </span>
                    );
                  },
                },
              ]}
            />
          </SectionCard>

          {policyLoading ? (
            <CardSkeleton />
          ) : (
            <SectionCard
              title="Alasan dan persetujuan"
              description={
                strictApprover
                  ? "Alasan wajib diisi. Kebijakan perusahaan: setiap koreksi wajib disetujui atasan."
                  : "Alasan wajib diisi. Koreksi di atas ambang nilai perlu persetujuan atasan."
              }
            >
              <div className="flex flex-col gap-4">
                <FormField label="Alasan" htmlFor="adjust-reason" required errorText={errors.reason}>
                  <TextArea
                    id="adjust-reason"
                    rows={2}
                    placeholder="cth: Selisih hasil stok opname 10 Sep"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </FormField>
                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label="Nama penyetuju"
                    htmlFor="adjust-approver"
                    required={strictApprover}
                    errorText={errors.approver}
                    helperText={
                      strictApprover
                        ? "Wajib untuk semua koreksi."
                        : "Diisi bila nilai koreksi mensyaratkan persetujuan atasan."
                    }
                  >
                    <TextInput
                      id="adjust-approver"
                      placeholder="cth: supervisor01"
                      autoComplete="off"
                      value={approver}
                      onChange={(e) => setApprover(e.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="Kata sandi penyetuju"
                    htmlFor="adjust-approver-password"
                    required={strictApprover}
                    errorText={errors.approverPassword}
                  >
                    <PasswordInput
                      id="adjust-approver-password"
                      autoComplete="new-password"
                      value={approverPassword}
                      onChange={(e) => setApproverPassword(e.target.value)}
                    />
                  </FormField>
                </div>
              </div>
            </SectionCard>
          )}
        </fieldset>
      </form>

      <StickyFooter
        summary={
          errorCount > 0 ? (
            <span className="font-medium text-destructive">{errorCount} isian perlu diperbaiki</span>
          ) : (
            <span>
              <Qty value={lines.filter((l) => !isBlank(l)).length} /> baris · <Qty value={diffCount} /> diubah
            </span>
          )
        }
      >
        <Button type="submit" form="adjustment-form" loading={saving} disabled={!canAdjust || policyLoading}>
          Simpan koreksi
        </Button>
      </StickyFooter>
    </Page>
  );
}
