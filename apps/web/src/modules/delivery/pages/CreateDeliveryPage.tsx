import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { z } from "zod";
import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  DateInput,
  DetailSkeleton,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  Skeleton,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { accumulateProgress, remainingQty } from "@/modules/payment/lib/order-history";
import { deliverSchema } from "@/modules/delivery/schemas/delivery.schema";
import type { SalesOrderOption } from "@/modules/delivery/types";

// Halaman Buat Surat Jalan (T3). Menggantikan CreateDeliveryModal: rute tetap
// `/deliveries/new?from=<salesOrderId>` (PANDUAN §3), izin delivery.create.

interface DeliverLineRow {
  key: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  /** Batas atas klien = sisa kirim baris ini (sisa sejati dihitung server). */
  maxQty: number;
  qty: string;
  locationId: number | null;
}

let nextLineKey = 1;

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `sj-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

// Dokumen pengiriman opsional — batas disalin dari
// delivery/application/service.go: validateDoc (driver/staff maks 100,
// plat maks 20).
const docSchema = z.object({
  driverName: z.string().max(100, "Nama supir maksimal 100 karakter").default(""),
  vehiclePlate: z.string().max(20, "Plat kendaraan maksimal 20 karakter").default(""),
  warehouseStaffName: z.string().max(100, "Nama petugas gudang maksimal 100 karakter").default(""),
  dropLocationNote: z.string().default(""),
});

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/**
 * Baris kirim dari SO + sisa yang belum dikirim/belum dibuatkan draf. Sisa
 * dihitung dari daftar surat jalan SO itu (gagal/403 = qty SO penuh; server
 * tetap menolak kelebihan).
 */
async function fetchDeliverLines(soId: number): Promise<{ so: SalesOrderOption; rows: DeliverLineRow[] }> {
  const [so, notes] = await Promise.all([
    deliveryService.salesOrderDetail(soId),
    deliveryService
      .list({ salesOrderId: soId, page: 1, limit: 100 })
      .then((r) => r.items)
      .catch(() => null),
  ]);
  const progress = notes ? accumulateProgress(notes, "confirmed", "draft") : null;
  const rows = so.items
    .map((it) => {
      const rest = progress ? remainingQty(it.qty, progress.get(`${it.productId}/${it.variantId}`)) : it.qty;
      return {
        key: nextLineKey++,
        productId: it.productId,
        variantId: it.variantId,
        productCode: it.productCode,
        productName: it.productName,
        uom: it.uom,
        maxQty: rest,
        qty: String(rest),
        locationId: null,
      };
    })
    .filter((r) => r.maxQty > 0);
  return { so, rows };
}

export default function CreateDeliveryPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const fromRaw = Number(params.get("from"));
  const soId = Number.isInteger(fromRaw) && fromRaw > 0 ? fromRaw : null;

  // Baris + SO dimuat tiap `?from=` berubah (dipilih lewat picker atau tautan dari order).
  const { data: loaded, loading: loadingSo, error: loadError } = useAsyncData(
    () => (soId === null ? Promise.resolve(null) : fetchDeliverLines(soId)),
    [soId],
  );
  const { data: locations } = useAsyncData(
    () => deliveryService.locationOptions().catch((err) => {
      toast.danger("Gagal memuat lokasi", toApiError(err).message);
      return [];
    }),
    [],
  );

  // Edit lokal baris: disalin dari hasil muat sekali per `?from=` (render-phase adjustment).
  const [lines, setLines] = useState<DeliverLineRow[]>([]);
  const [appliedFor, setAppliedFor] = useState<unknown>(null);
  if (loaded && appliedFor !== loaded) {
    setAppliedFor(loaded);
    setLines(loaded.rows);
  }
  if (!loaded && appliedFor !== null) {
    setAppliedFor(null);
    setLines([]);
  }

  const [deliveryDate, setDeliveryDate] = useState(todayWIB());
  const [driverName, setDriverName] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [warehouseStaffName, setWarehouseStaffName] = useState("");
  const [dropLocationNote, setDropLocationNote] = useState("");
  const [notes, setNotes] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  // Galat per kolom dari server (driverName, warehouseStaffName, ...).
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  // Satu UUID per halaman dibuka: klik ganda/retry = replay server, bukan SJ ganda.
  const [clientRef] = useState(newClientRef);

  function pickOrder(v: string | number | null) {
    const id = toId(v);
    setFormError(null);
    setParams(id ? { from: String(id) } : {}, { replace: true });
  }

  function patchLine(key: number, patch: Partial<DeliverLineRow>) {
    setLines((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  /** Jepit qty klien ke [0, maxQty] — backend menolak over-SO. */
  function handleQtyChange(key: number, maxQty: number, raw: string) {
    if (raw.trim() === "") {
      patchLine(key, { qty: "" });
      return;
    }
    const n = Number(raw);
    if (Number.isNaN(n)) return;
    patchLine(key, { qty: n > maxQty ? String(maxQty) : raw });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = deliverSchema.safeParse({
      salesOrderId: soId ?? 0,
      deliveryDate: deliveryDate.trim(),
      notes,
      items: lines.map((r) => ({
        productId: r.productId,
        variantId: r.variantId,
        locationId: r.locationId ?? 0,
        uom: r.uom,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      setFormError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    const doc = docSchema.safeParse({ driverName, vehiclePlate, warehouseStaffName, dropLocationNote });
    if (!doc.success) {
      setFormError(doc.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setFormError(null);
    setFieldErrors({});
    setSaving(true);
    try {
      const trimOrUndefined = (v: string) => (v.trim() !== "" ? v.trim() : undefined);
      const res = await deliveryService.create({
        salesOrderId: parsed.data.salesOrderId,
        deliveryDate: parsed.data.deliveryDate.trim() !== "" ? parsed.data.deliveryDate.trim() : undefined,
        driverName: trimOrUndefined(doc.data.driverName),
        vehiclePlate: trimOrUndefined(doc.data.vehiclePlate),
        warehouseStaffName: trimOrUndefined(doc.data.warehouseStaffName),
        dropLocationNote: trimOrUndefined(doc.data.dropLocationNote),
        notes: parsed.data.notes,
        clientRef,
        // locationId 0 = dihilangkan: server mengalokasikan lintas lokasi.
        items: parsed.data.items.map((it) => (it.locationId > 0 ? it : { ...it, locationId: undefined })),
      });
      // Replay (HTTP 200, replayed:true): SJ sudah ada — pesan server dipakai apa adanya.
      toast.fromServer(res.message, "Surat jalan dibuat", res.data.number);
      const warnings = res.data.warnings ?? [];
      if (warnings.length > 1) toast.warning("Periksa ketersediaan stok", warnings.slice(1).join(" "));
      navigate(`${ROUTE_PATHS.deliveries}/${res.data.id}`, warnings.length > 0 ? { state: { warnings } } : undefined);
    } catch (err) {
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      const mapped: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) mapped[fe.field] ??= fe.message;
      setFieldErrors(mapped);
      toast.danger("Gagal membuat surat jalan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  const so = loaded?.so ?? null;
  const cancelTo = soId ? `${ROUTE_PATHS.salesOrders}/${soId}` : ROUTE_PATHS.deliveries;
  const totalQty = lines.reduce((sum, r) => sum + (Number(r.qty) || 0), 0);
  const errorCount = Object.keys(fieldErrors).length;

  return (
    <Page>
      <PageHeader
        title="Buat Surat Jalan"
        description="Draf tidak menggerakkan stok; stok keluar dicatat saat konfirmasi. Lokasi asal boleh dikosongkan: server mengalokasikan stok lintas lokasi."
      />

      {formError && (
        <Notice tone="danger" title="Surat jalan belum tersimpan">
          {formError}
        </Notice>
      )}

      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="flex flex-col gap-6">
        <SectionCard title="Dokumen">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="sm:col-span-2">
              <FormField label="Order jual" required>
                {soId !== null && so ? (
                  <div className="flex items-start gap-2">
                    <div className="min-w-0 flex-1">
                      <TextInput value={`${so.number} — ${so.partyName || "Walk-in"}`} readOnly disabled aria-label="Order jual" />
                    </div>
                    <Button variant="ghost" size="sm" onClick={() => pickOrder(null)}>
                      Ganti order
                    </Button>
                  </div>
                ) : soId !== null && loadingSo ? (
                  <Skeleton className="h-10 w-full" />
                ) : (
                  <SalesOrderSearchSelect
                    value={soId}
                    status="confirmed"
                    placeholder="Cari order terkonfirmasi (nomor atau pelanggan)…"
                    onChange={pickOrder}
                  />
                )}
              </FormField>
            </div>
            <FormField label="Tanggal kirim" helperText="Kosongkan untuk memakai tanggal hari ini.">
              <DateInput value={deliveryDate} onChange={(e) => setDeliveryDate(e.target.value)} />
            </FormField>
          </div>
          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <FormField label="Supir" required errorText={fieldErrors.driverName}>
              <TextInput
                value={driverName}
                maxLength={100}
                onChange={(e) => setDriverName(e.target.value)}
                placeholder="Nama supir"
              />
            </FormField>
            <FormField label="Plat kendaraan" errorText={fieldErrors.vehiclePlate}>
              <TextInput
                value={vehiclePlate}
                maxLength={20}
                onChange={(e) => setVehiclePlate(e.target.value)}
                placeholder="cth. B 1234 CD (opsional)"
              />
            </FormField>
            <FormField label="Petugas gudang" required errorText={fieldErrors.warehouseStaffName}>
              <TextInput
                value={warehouseStaffName}
                maxLength={100}
                onChange={(e) => setWarehouseStaffName(e.target.value)}
                placeholder="Nama petugas gudang"
              />
            </FormField>
          </div>
          <p className="mt-3 text-xs text-muted-foreground">
            Pengiriman non-POS wajib mencantumkan nama sopir dan petugas gudang pada surat jalan. Saat
            dikonfirmasi, nama penerima, status tanda tangan, dan minimal satu foto bukti kirim juga wajib
            (kecuali perusahaan menonaktifkannya). Sopir dan petugas tidak bisa diubah setelah draf
            disimpan — isi sekarang.
          </p>

          <Collapsible open={detailOpen} onOpenChange={setDetailOpen} className="mt-4 border-t border-border pt-3">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="-ml-2">
                <ChevronRight
                  aria-hidden
                  className={detailOpen ? "rotate-90 transition-transform" : "transition-transform"}
                />
                Detail lain
                <span className="font-normal text-muted-foreground">(catatan turun barang, catatan)</span>
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-3">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField label="Catatan turun barang">
                  <TextArea
                    rows={2}
                    value={dropLocationNote}
                    onChange={(e) => setDropLocationNote(e.target.value)}
                    placeholder="Titik/lokasi turun barang (opsional)"
                  />
                </FormField>
                <FormField label="Catatan">
                  <TextArea
                    rows={2}
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    placeholder="Keterangan tambahan (opsional)"
                  />
                </FormField>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </SectionCard>

        <SectionCard title="Barang">
          {loadError ? (
            <ErrorState title="Gagal memuat order" message={loadError} />
          ) : loadingSo && soId !== null && lines.length === 0 ? (
            <DetailSkeleton />
          ) : (
            <LineItemsEditor<DeliverLineRow>
              lines={lines}
              getKey={(r) => r.key}
              emptyText={
                soId === null
                  ? "Pilih order jual untuk memuat baris barang."
                  : "Semua barang order ini sudah dikirim atau dibuatkan surat jalan."
              }
              columns={[
                {
                  id: "product",
                  header: "Produk",
                  render: (r) => (
                    <span className="inline-block py-2">
                      <span className="font-medium">{r.productName}</span>{" "}
                      <span className="text-xs text-muted-foreground">{r.productCode}</span>
                    </span>
                  ),
                },
                { id: "uom", header: "Satuan", width: "6rem", render: (r) => <span className="inline-block py-2">{r.uom}</span> },
                {
                  id: "max",
                  header: "Maks",
                  width: "6rem",
                  align: "right",
                  render: (r) => (
                    <span className="inline-block py-2">
                      <Qty value={r.maxQty} />
                    </span>
                  ),
                },
                {
                  id: "qty",
                  header: "Qty kirim",
                  width: "8rem",
                  align: "right",
                  render: (r) => (
                    <TextInput
                      type="number"
                      min="0"
                      step="any"
                      value={r.qty}
                      onChange={(e) => handleQtyChange(r.key, r.maxQty, e.target.value)}
                      aria-label={`Qty kirim ${r.productName}`}
                      className="text-right"
                    />
                  ),
                },
                {
                  id: "location",
                  header: "Lokasi asal (opsional)",
                  width: "16rem",
                  render: (r) => (
                    <SearchSelect
                      aria-label={`Lokasi asal ${r.productName}`}
                      options={(locations ?? []).map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` }))}
                      value={r.locationId}
                      placeholder="Otomatis (server)"
                      allowClear
                      onChange={(v) => patchLine(r.key, { locationId: toId(v) })}
                    />
                  ),
                },
              ]}
            />
          )}
        </SectionCard>

        <StickyFooter
          summary={
            <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span>
                <span className="tabular">{lines.length}</span> baris
              </span>
              <span>
                Total qty <Qty value={totalQty} className="font-semibold text-foreground" />
              </span>
              {errorCount > 0 && (
                <span role="alert" className="font-medium text-destructive">
                  {errorCount} isian perlu diperbaiki
                </span>
              )}
            </span>
          }
        >
          <Button variant="secondary" asChild>
            <Link to={cancelTo}>Batal</Link>
          </Button>
          <Button type="submit" loading={saving}>
            Simpan surat jalan
          </Button>
        </StickyFooter>
      </form>
    </Page>
  );
}
