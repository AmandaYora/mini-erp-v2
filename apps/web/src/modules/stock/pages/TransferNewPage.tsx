import { useId, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import { transferSchemaFor } from "@/modules/stock/schemas/stock.schema";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { todayWIB } from "@/shared/lib/format";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { VariantSelect } from "@/modules/stock/components/VariantSelect";
import { toId } from "@/modules/stock/lib/ids";
import { parseQty } from "@/modules/stock/lib/adjustment";
import type { BranchOption } from "@/modules/stock/types";

interface Line {
  key: number;
  productId: number | null;
  variantId: number | null;
  qty: string;
}

let nextKey = 1;
const newLine = (): Line => ({ key: nextKey++, productId: null, variantId: null, qty: "" });

function CellError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="mt-1 text-left text-xs font-medium text-destructive">
      {message}
    </p>
  );
}

/** Buat transfer antar cabang (T3). Stok keluar dicatat saat pengiriman, bukan saat dibuat. */
export default function TransferNewPage() {
  const uid = useId();
  const navigate = useNavigate();
  const listPath = ROUTE_PATHS.stockTransfers;

  const { data: branches } = useAsyncData(
    () =>
      stockService.listAccessibleBranches().catch((err) => {
        toast.danger("Gagal memuat cabang", toApiError(err).message);
        return [] as BranchOption[];
      }),
    [],
  );

  const [toBranchId, setToBranchId] = useState<number | null>(null);
  const [fromLocationId, setFromLocationId] = useState<number | null>(null);
  const [toLocationId, setToLocationId] = useState<number | null>(null);
  const [notes, setNotes] = useState("");
  const [driverName, setDriverName] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [transferDate, setTransferDate] = useState(todayWIB());
  const sessionBranchId = useAuthStore((s) => s.branch?.id ?? null);
  // Antar cabang bila tujuan beda dari cabang sesi; sopir wajib (BR-20). Cabang sesi tak
  // diketahui klien = diperlakukan antar cabang (server tetap penentu).
  const interBranch = toBranchId !== null && (sessionBranchId === null || toBranchId !== sessionBranchId);
  const [lines, setLines] = useState<Line[]>(() => [newLine()]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const patch = (key: number, p: Partial<Line>) =>
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...p } : l)));

  const totalQty = useMemo(() => lines.reduce((s, l) => s + (parseQty(l.qty) ?? 0), 0), [lines]);
  const filledLines = lines.filter((l) => l.productId !== null).length;
  const errorCount = Object.keys(errors).length;

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = transferSchemaFor(!interBranch).safeParse({
      toBranchId: toBranchId ?? 0,
      fromLocationId: fromLocationId ?? 0,
      toLocationId: toLocationId ?? 0,
      notes,
      driverName,
      vehiclePlate,
      transferDate,
      items: lines.map((l) => ({ productId: l.productId ?? 0, variantId: l.variantId ?? 0, qty: l.qty })),
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) errs[issue.path.join(".")] ??= issue.message;
      setErrors(errs);
      setServerError(null);
      return;
    }
    setErrors({});
    setServerError(null);
    setSaving(true);
    try {
      const res = await stockService.createTransfer({
        toBranchId: parsed.data.toBranchId,
        fromLocationId: parsed.data.fromLocationId,
        toLocationId: parsed.data.toLocationId,
        notes: parsed.data.notes,
        driverName: parsed.data.driverName || undefined,
        vehiclePlate: parsed.data.vehiclePlate || undefined,
        transferDate: parsed.data.transferDate || undefined,
        items: parsed.data.items.map((it) => ({ ...it, notes: "" })),
      });
      toast.fromServer(res.message, "Transfer dibuat", res.data.number);
      navigate(`${listPath}/${res.data.id}`);
    } catch (err) {
      const apiErr = toApiError(err);
      setServerError(apiErr.message);
      toast.danger("Gagal membuat transfer", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Page>
      <PageHeader
        title="Buat Transfer"
        description="Stok keluar dicatat saat pengiriman, bukan saat transfer dibuat."
      />

      {serverError && (
        <Notice tone="danger" title="Transfer belum tersimpan">
          {serverError}
        </Notice>
      )}

      <form id="transfer-form" className="flex flex-col gap-6" onSubmit={(e) => void handleSubmit(e)} noValidate>
        <SectionCard title="Tujuan dan lokasi">
          <fieldset disabled={saving} className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <FormField label="Cabang tujuan" required errorText={errors.toBranchId}>
                <SearchSelect
                  options={(branches ?? []).map((b) => ({ value: b.id, label: `${b.code} — ${b.name}` }))}
                  value={toBranchId}
                  placeholder="Pilih cabang…"
                  onChange={(v) => {
                    const next = toId(v);
                    // Lokasi tujuan milik cabang tujuan: ganti cabang = pilihan lama tak berlaku.
                    if (next !== toBranchId) setToLocationId(null);
                    setToBranchId(next);
                  }}
                />
              </FormField>
              <div>
                <LocationSelect label="Lokasi asal" value={fromLocationId} onChange={setFromLocationId} required />
                <CellError message={errors.fromLocationId} />
              </div>
              <div>
                <LocationSelect
                  label="Lokasi tujuan"
                  value={toLocationId}
                  onChange={setToLocationId}
                  branchId={toBranchId}
                  requireBranch
                  placeholder={toBranchId ? "Pilih lokasi…" : "Pilih cabang tujuan dulu"}
                  required
                />
                <CellError message={errors.toLocationId} />
              </div>
            </div>
            <div className="grid gap-4 sm:grid-cols-3">
              <FormField
                label="Sopir"
                htmlFor={`${uid}-driver`}
                required={interBranch}
                errorText={errors.driverName}
                helperText={interBranch ? "Wajib untuk transfer antar cabang (tercetak di surat transfer)." : undefined}
              >
                <TextInput id={`${uid}-driver`} value={driverName} maxLength={150} onChange={(e) => setDriverName(e.target.value)} />
              </FormField>
              <FormField label="Kendaraan" htmlFor={`${uid}-plate`} errorText={errors.vehiclePlate}>
                <TextInput id={`${uid}-plate`} value={vehiclePlate} maxLength={30} placeholder="cth: B 1234 XY" onChange={(e) => setVehiclePlate(e.target.value)} />
              </FormField>
              <FormField label="Tanggal transfer" htmlFor={`${uid}-date`} errorText={errors.transferDate}>
                <TextInput id={`${uid}-date`} type="date" value={transferDate} onChange={(e) => setTransferDate(e.target.value)} />
              </FormField>
            </div>
            <Collapsible>
              <CollapsibleTrigger asChild>
                <Button variant="ghost" size="sm">
                  Detail lain
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-2">
                <FormField label="Catatan" htmlFor={`${uid}-notes`}>
                  <TextArea
                    id={`${uid}-notes`}
                    rows={2}
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    placeholder="Keterangan tambahan (opsional)"
                  />
                </FormField>
              </CollapsibleContent>
            </Collapsible>
          </fieldset>
        </SectionCard>

        <SectionCard title="Barang" description="Produk, varian, dan jumlah yang akan dipindahkan.">
          <CellError message={errors.items} />
          <LineItemsEditor<Line>
            lines={lines}
            getKey={(l) => l.key}
            onAdd={() => setLines((prev) => [...prev, newLine()])}
            onRemove={(i) => setLines((prev) => (prev.length > 1 ? prev.filter((_, idx) => idx !== i) : prev))}
            addLabel="Tambah barang"
            columns={[
              {
                id: "product",
                header: "Produk",
                width: "20rem",
                render: (l, i) => (
                  <div role="group" aria-label={`Produk baris ${i + 1}`}>
                    <ProductSelect
                      productId={l.productId}
                      allowClear={false}
                      onChange={(id) => patch(l.key, { productId: id, variantId: null })}
                    />
                    <CellError message={errors[`items.${i}.productId`]} />
                  </div>
                ),
              },
              {
                id: "variant",
                header: "Varian",
                width: "14rem",
                render: (l, i) => (
                  <div>
                    <VariantSelect
                      aria-label={`Varian baris ${i + 1}`}
                      productId={l.productId}
                      variantId={l.variantId}
                      autoPick
                      onChange={(id) => patch(l.key, { variantId: id })}
                    />
                    <CellError message={errors[`items.${i}.variantId`]} />
                  </div>
                ),
              },
              {
                id: "qty",
                header: "Jumlah",
                width: "8rem",
                align: "right",
                render: (l, i) => (
                  <div>
                    <TextInput
                      aria-label={`Jumlah baris ${i + 1}`}
                      type="number"
                      min="0"
                      step="any"
                      inputMode="decimal"
                      className="tabular text-right"
                      placeholder="0"
                      value={l.qty}
                      onChange={(e) => patch(l.key, { qty: e.target.value })}
                    />
                    <CellError message={errors[`items.${i}.qty`]} />
                  </div>
                ),
              },
            ]}
          />
        </SectionCard>
      </form>

      <StickyFooter
        summary={
          errorCount > 0 ? (
            <span className="font-medium text-destructive">{errorCount} isian perlu diperbaiki</span>
          ) : (
            <span>
              <Qty value={filledLines} /> barang · total jumlah <Qty value={totalQty} />
            </span>
          )
        }
      >
        <Button asChild variant="secondary">
          <Link to={listPath}>Batal</Link>
        </Button>
        <Button type="submit" form="transfer-form" loading={saving}>
          Simpan transfer
        </Button>
      </StickyFooter>
    </Page>
  );
}
