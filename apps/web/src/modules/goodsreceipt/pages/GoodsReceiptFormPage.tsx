import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import {
  Button,
  CardSkeleton,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  DateInput,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Page, Qty, StickyFooter, type LineColumn } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { SettlementFields } from "@/modules/goodsreceipt/components/SettlementFields";
import {
  emptySettlementDraft,
  resolveSettlement,
  settlementFieldErrors,
} from "@/modules/goodsreceipt/lib/settlement-draft";
import type { SettlementDraft } from "@/modules/goodsreceipt/lib/settlement-draft";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import { receiveSchema } from "@/modules/goodsreceipt/schemas/goodsreceipt.schema";
import type { StockLocationOption } from "@/modules/goodsreceipt/types";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { accumulateProgress, remainingQty } from "@/modules/payment/lib/order-history";

interface ReceiveLineRow {
  key: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  /** Qty pada PO. */
  ordered: number;
  /** Sudah diterima (posted); null = progres tak terbaca (mis. tanpa izin lihat penerimaan). */
  received: number | null;
  /** Menunggu posting (penerimaan pending). */
  pending: number;
  /** Batas atas klien = sisa terima (sisa sejati dihitung server). */
  maxQty: number;
  qty: string;
  locationId: number | null;
  /** false = non-stok: lokasi tidak diperlukan. */
  stockTracked: boolean;
}

let nextLineKey = 1;

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `gr-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/** PO + progres penerimaannya (null = tak terbaca, mis. tanpa izin: qty PO penuh). */
async function fetchPurchaseOrderWithProgress(poId: number) {
  const [po, receipts] = await Promise.all([
    goodsreceiptService.purchaseOrderDetail(poId),
    goodsreceiptService
      .list({ purchaseOrderId: poId, page: 1, limit: 100 })
      .then((r) => r.items)
      .catch(() => null),
  ]);
  return { po, progress: receipts ? accumulateProgress(receipts, "posted", "pending") : null };
}

/**
 * Catat penerimaan barang (T3): pilih PO terkonfirmasi, atau datang dari detail PO lewat
 * `?from=<purchaseOrderId>` (PO terkunci, baris langsung terisi sisa terima).
 */
export default function GoodsReceiptFormPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const fromParam = Number(searchParams.get("from"));
  const fromId = Number.isInteger(fromParam) && fromParam > 0 ? fromParam : null;

  const can = useAuthStore((s) => s.can);
  const canPay = can("payment.create");
  const canSwitchNet = can("purchasing.update");

  const [locations, setLocations] = useState<StockLocationOption[]>([]);
  const [poId, setPoId] = useState<number | null>(fromId);
  const [poLabel, setPoLabel] = useState("");
  const [loadingPo, setLoadingPo] = useState(fromId !== null);
  const [lines, setLines] = useState<ReceiveLineRow[]>([]);
  const [receivedAt, setReceivedAt] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [supplierDeliveryNumber, setSupplierDeliveryNumber] = useState("");
  const [paymentTerms, setPaymentTerms] = useState("");
  const [settlement, setSettlement] = useState<SettlementDraft>(() => emptySettlementDraft(todayWIB()));
  const [settlementErrors, setSettlementErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [detailOpenState, setDetailOpenState] = useState<boolean | null>(null);
  // Satu UUID per pembukaan halaman: retry setelah gagal melanjutkan penerimaan yang sama
  // (server menjawab dengan receipt itu), bukan mencatat ganda.
  const [clientRef] = useState(newClientRef);

  const locationOptions = useMemo(
    () => locations.map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` })),
    [locations],
  );

  // Lokasi dimuat sekali; PO dicari server-side lewat PurchaseOrderSearchSelect.
  useEffect(() => {
    let alive = true;
    goodsreceiptService
      .locationOptions()
      .then((rows) => {
        if (alive) setLocations(rows);
      })
      .catch((err) => {
        if (alive) toast.danger("Gagal memuat lokasi", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
  }, []);

  function applyPurchaseOrder({ po, progress }: Awaited<ReturnType<typeof fetchPurchaseOrderWithProgress>>) {
    // Prefill = sisa yang belum diterima/belum dicatat (qty PO penuh bila progres tak terbaca);
    // server tetap menolak kelebihan.
    setPoLabel(`${po.number} — ${po.partyName || "-"}`);
    setPaymentTerms(po.paymentTerms ?? "");
    // COD: usulkan bayar sekarang bila boleh (server mengabaikannya bila bukan penerimaan penuntas).
    setSettlement({
      ...emptySettlementDraft(todayWIB()),
      mode: po.paymentTerms === "cod" ? (canPay ? "pay_now" : canSwitchNet ? "switch_to_net" : "") : "",
    });
    setLines(
      po.items
        .map((it) => {
          const p = progress?.get(`${it.productId}/${it.variantId}`);
          const rest = progress ? remainingQty(it.qty, p) : it.qty;
          return {
            key: nextLineKey++,
            productId: it.productId,
            variantId: it.variantId,
            productCode: it.productCode,
            productName: it.productName,
            uom: it.uom,
            ordered: it.qty,
            received: progress ? (p?.done ?? 0) : null,
            pending: p?.pending ?? 0,
            maxQty: rest,
            qty: String(rest),
            locationId: null,
            stockTracked: it.stockTracked !== false,
          };
        })
        .filter((r) => r.maxQty > 0),
    );
  }

  // Dari detail order: muat baris (sisa terima) sekali saat halaman dibuka.
  useEffect(() => {
    if (fromId === null) return;
    let alive = true;
    fetchPurchaseOrderWithProgress(fromId)
      .then((loaded) => {
        if (alive) applyPurchaseOrder(loaded);
      })
      .catch((err) => {
        if (alive) setFormError(toApiError(err).message);
      })
      .finally(() => {
        if (alive) setLoadingPo(false);
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fromId]);

  async function handlePoChange(v: string | number | null) {
    const id = toId(v);
    setPoId(id);
    setLines([]);
    setPaymentTerms("");
    setPoLabel("");
    setSettlementErrors({});
    setFormError(null);
    if (!id) return;
    setLoadingPo(true);
    try {
      applyPurchaseOrder(await fetchPurchaseOrderWithProgress(id));
    } catch (err) {
      setFormError(toApiError(err).message);
    } finally {
      setLoadingPo(false);
    }
  }

  function patchLine(key: number, patch: Partial<ReceiveLineRow>) {
    setLines((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  /** Jepit qty klien ke [0, maxQty] — backend menolak over-receipt. */
  function handleQtyChange(key: number, maxQty: number, raw: string) {
    if (raw.trim() === "") {
      patchLine(key, { qty: "" });
      return;
    }
    const n = Number(raw);
    if (Number.isNaN(n)) return;
    if (n > maxQty) {
      patchLine(key, { qty: String(maxQty) });
      return;
    }
    patchLine(key, { qty: raw });
  }

  const backTo = fromId !== null ? `${ROUTE_PATHS.purchaseOrders}/${fromId}` : ROUTE_PATHS.goodsReceipts;

  async function handleSubmit() {
    const isCod = paymentTerms === "cod";
    const settle = isCod ? resolveSettlement(settlement) : ({ ok: true, payload: undefined } as const);
    if (!settle.ok) {
      setSettlementErrors(settle.errors);
      setFormError("Periksa isian pelunasan.");
      return;
    }
    setSettlementErrors({});
    const missingLoc = lines.find((r) => r.stockTracked && r.locationId === null);
    if (missingLoc) {
      setFormError(`Lokasi wajib dipilih untuk barang stok (${missingLoc.productName}).`);
      return;
    }
    const parsed = receiveSchema.safeParse({
      purchaseOrderId: poId ?? 0,
      receivedAt: receivedAt.trim(),
      notes,
      supplierDeliveryNumber,
      clientRef,
      items: lines.map((r) => ({
        productId: r.productId,
        variantId: r.variantId,
        locationId: r.locationId ?? 0,
        uom: r.uom,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      const first = parsed.error.issues[0];
      setFormError(first?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setFormError(null);
    setSaving(true);
    try {
      const res = await goodsreceiptService.create({
        purchaseOrderId: parsed.data.purchaseOrderId,
        receivedAt: parsed.data.receivedAt.trim() !== "" ? parsed.data.receivedAt.trim() : undefined,
        notes: parsed.data.notes,
        supplierDeliveryNumber:
          parsed.data.supplierDeliveryNumber !== "" ? parsed.data.supplierDeliveryNumber : undefined,
        clientRef: parsed.data.clientRef,
        settlement: settle.payload,
        // locationId 0 = dihilangkan (barang non-stok).
        items: parsed.data.items.map((it) => (it.locationId > 0 ? it : { ...it, locationId: undefined })),
      });
      toast.fromServer(res.message, "Penerimaan dicatat", res.data.number || res.data.orderNumber);
      // Pembayaran pay_now dibawa ke detail agar slip transfer bisa dilampirkan.
      void navigate(`${ROUTE_PATHS.goodsReceipts}/${res.data.id}`, {
        state: res.data.payment ? { payment: res.data.payment } : undefined,
      });
    } catch (err) {
      // Kegagalan posting stok meninggalkan receipt `pending` yang nomornya ada di pesan server;
      // halaman tetap terbuka agar bisa dicoba lagi (clientRef sama = melanjutkan receipt itu)
      // atau diselesaikan dari daftar.
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      setSettlementErrors(settlementFieldErrors(apiErr.errors));
      toast.danger("Gagal mencatat penerimaan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  const detailHasValue = notes.trim() !== "";
  const detailOpen = detailOpenState ?? detailHasValue;

  const columns: LineColumn<ReceiveLineRow>[] = [
    {
      id: "product",
      header: "Produk",
      render: (r) => (
        <div className="py-1.5">
          <span className="font-medium">{r.productName}</span>
          <p className="text-xs text-muted-foreground">
            {r.productCode} · {r.uom}
          </p>
        </div>
      ),
    },
    {
      id: "ordered",
      header: "Dipesan",
      width: "6rem",
      align: "right",
      render: (r) => (
        <span className="inline-flex h-9 items-center">
          <Qty value={r.ordered} />
        </span>
      ),
    },
    {
      id: "received",
      header: "Sudah diterima",
      width: "8rem",
      align: "right",
      render: (r) => (
        <div className="py-1.5">
          {r.received === null ? <span className="text-muted-foreground">-</span> : <Qty value={r.received} />}
          {r.pending > 0 && (
            <p className="text-xs text-muted-foreground">
              +<Qty value={r.pending} /> menunggu
            </p>
          )}
        </div>
      ),
    },
    {
      id: "receive",
      header: "Terima sekarang",
      width: "9rem",
      align: "right",
      render: (r) => (
        <div>
          <TextInput
            type="number"
            aria-label={`Qty terima ${r.productName}`}
            min="0"
            step="any"
            className="text-right"
            value={r.qty}
            onChange={(e) => handleQtyChange(r.key, r.maxQty, e.target.value)}
          />
          <p className="mt-0.5 text-right text-xs text-muted-foreground">
            Maks <Qty value={r.maxQty} />
          </p>
        </div>
      ),
    },
    {
      id: "location",
      header: "Lokasi",
      width: "14rem",
      render: (r) =>
        !r.stockTracked ? (
          <span className="inline-flex h-9 items-center text-xs text-muted-foreground">Non-stok (tanpa lokasi)</span>
        ) : (
          <SearchSelect
            aria-label={`Lokasi ${r.productName}`}
            options={locationOptions}
            value={r.locationId}
            placeholder="Pilih lokasi…"
            onChange={(v) => patchLine(r.key, { locationId: toId(v) })}
          />
        ),
    },
  ];

  const filledCount = lines.filter((r) => r.qty.trim() !== "" && Number(r.qty) > 0).length;

  return (
    <Page>
      <PageHeader
        title="Catat penerimaan"
        description="Terima barang dari purchase order terkonfirmasi. Stok masuk dicatat server per baris."
      />

      {formError && (
        <Notice tone="danger" title="Penerimaan belum tersimpan">
          {formError}
        </Notice>
      )}

      <SectionCard title="Dokumen">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <FormField label="Order beli" required>
            {fromId !== null ? (
              <TextInput aria-label="Order beli" value={poLabel} readOnly disabled placeholder="Mengambil data PO…" />
            ) : (
              <PurchaseOrderSearchSelect
                value={poId}
                status="confirmed"
                placeholder="Cari PO terkonfirmasi (nomor atau pemasok)…"
                onChange={(id) => void handlePoChange(id)}
              />
            )}
          </FormField>
          <FormField label="Tanggal terima" helperText="Kosongkan untuk memakai tanggal hari ini.">
            <DateInput value={receivedAt} onChange={(e) => setReceivedAt(e.target.value)} />
          </FormField>
          <FormField label="No. surat jalan pemasok" helperText="Opsional, maksimal 60 karakter.">
            <TextInput
              value={supplierDeliveryNumber}
              maxLength={60}
              onChange={(e) => setSupplierDeliveryNumber(e.target.value)}
              placeholder="cth. SJ-2026-0001"
            />
          </FormField>
        </div>
        <Collapsible open={detailOpen} onOpenChange={setDetailOpenState} className="mt-4 border-t border-border pt-3">
          <CollapsibleTrigger className="group inline-flex cursor-pointer items-center gap-1 text-sm font-medium text-muted-foreground hover:text-foreground">
            <ChevronRight size={14} aria-hidden className="transition-transform group-data-[state=open]:rotate-90" />
            Detail lain (catatan)
          </CollapsibleTrigger>
          <CollapsibleContent className="mt-4">
            <FormField label="Catatan">
              <TextArea
                rows={2}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="Keterangan tambahan (opsional)"
              />
            </FormField>
          </CollapsibleContent>
        </Collapsible>
      </SectionCard>

      <SectionCard
        title="Barang diterima"
        description="Qty terima dijepit ke sisa PO; server menolak kelebihan penerimaan."
      >
        {loadingPo ? (
          <CardSkeleton />
        ) : lines.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {poId === null
              ? "Pilih purchase order untuk memuat baris barang."
              : "Tidak ada sisa barang yang bisa diterima dari order ini."}
          </p>
        ) : (
          <LineItemsEditor<ReceiveLineRow> lines={lines} getKey={(r) => r.key} columns={columns} />
        )}
      </SectionCard>

      {paymentTerms === "cod" && lines.length > 0 && (
        <SettlementFields
          value={settlement}
          errors={settlementErrors}
          canPay={canPay}
          canSwitchNet={canSwitchNet}
          onChange={setSettlement}
        />
      )}

      <StickyFooter
        summary={
          <span>
            {filledCount} dari {lines.length} baris akan diterima
          </span>
        }
      >
        <Button variant="secondary" disabled={saving} onClick={() => void navigate(backTo)}>
          Batal
        </Button>
        <Button onClick={() => void handleSubmit()} loading={saving} disabled={loadingPo}>
          Simpan penerimaan
        </Button>
      </StickyFooter>
    </Page>
  );
}
