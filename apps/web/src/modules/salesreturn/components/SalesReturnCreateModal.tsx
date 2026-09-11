import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  DateInput,
  FormField,
  Modal,
  SearchSelect,
  SectionCard,
  Segmented,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { salesReturnSchema } from "@/modules/salesreturn/schemas/sales-return.schema";
import {
  salesReturnLookupService,
  salesReturnService,
} from "@/modules/salesreturn/services/sales-return.service";
import type {
  LocationOption,
  ReplacementProductOption,
  SalesReturn,
  SalesReturnPreview,
} from "@/modules/salesreturn/types";

interface LineDraft {
  key: string;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  orderedQty: number;
  delivered: number;
  returned: number;
  remaining: number;
  unitPrice: number;
  qty: string;
  locationId: number | null;
}

interface ReplDraft {
  key: string;
  productId: number | null;
  productLabel: string;
  variants: { id: number; code: string; name: string }[];
  variantId: number | null;
  uoms: string[];
  uom: string;
  qty: string;
  locationId: number | null;
  loadingDetail: boolean;
}

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (ret: SalesReturn) => void;
}

let replKey = 1;

function clampQty(raw: string, max: number): string {
  if (raw.trim() === "") return "";
  const n = Number(raw);
  if (Number.isNaN(n)) return raw;
  if (n < 0) return "0";
  if (n > max) return String(max);
  return raw;
}

function uniqueUoms(p: { baseUom: string; purchaseUom: string; salesUom: string }): string[] {
  return [p.salesUom, p.purchaseUom, p.baseUom].filter(
    (u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i,
  );
}

export function SalesReturnCreateModal({ open, onClose, onCreated }: Props) {
  const [sourceId, setSourceId] = useState<number | null>(null);
  const [lines, setLines] = useState<LineDraft[]>([]);
  const [returnMode, setReturnMode] = useState<"return_only" | "exchange">("return_only");
  const [replacements, setReplacements] = useState<ReplDraft[]>([]);
  const [preview, setPreview] = useState<SalesReturnPreview | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [returnDate, setReturnDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const { data: lookupData, loading: loadingLookups, error: lookupError } = useAsyncData(
    () => {
      if (!open) return Promise.resolve(null);
      return Promise.all([
        salesReturnLookupService.sourceOrders(),
        salesReturnLookupService.locationOptions(),
        // Best-effort: butuh perm products.view; editor pengganti tetap bisa
        // dibuka dan menampilkan error saat produk dipilih bila gagal.
        salesReturnLookupService.productOptions().catch(() => [] as ReplacementProductOption[]),
      ]).then(([orders, locs, prods]) => ({ orders, locations: locs, products: prods }));
    },
    [open],
  );
  const candidates = lookupData?.orders ?? [];
  const lookupLocations = lookupData?.locations ?? [];
  const productOptions = lookupData?.products ?? [];

  // Konteks membawa sisa bisa-diretur per produk (terkirim − diretur) +
  // lokasi aktif — pratinjau & clamp memakai angka ini, bukan qty order.
  // Gagal (mis. 403) → fallback ke detail SO dengan clamp qty order.
  const { data: ctxBundle, loading: ctxLoading, error: ctxFetchError } = useAsyncData(
    () => {
      if (!open || sourceId === null) return Promise.resolve(null);
      const sid = sourceId;
      return salesReturnService.context(sid).then(
        (ctx) => ({
          sourceId: sid,
          lines: ctx.lines.map(
            (it, i): LineDraft => ({
              key: `${it.productId}/${it.variantId}/${i}`,
              productId: it.productId,
              variantId: it.variantId,
              productCode: it.productCode,
              productName: it.productName,
              uom: it.uom,
              orderedQty: it.orderedQty,
              delivered: it.delivered,
              returned: it.returned,
              remaining: it.remaining,
              unitPrice: 0,
              qty: it.remaining > 0 ? String(it.remaining) : "",
              locationId: null,
            }),
          ),
          locations: ctx.locations.map(
            (l): LocationOption => ({ id: l.id, code: l.code, name: l.name, status: "active" }),
          ),
          contextError: null as string | null,
        }),
        (err: unknown) => {
          // Context butuh salesreturn.view; bila gagal (mis. 403) tampilkan
          // alasan dan fallback ke detail SO dengan clamp qty order.
          const firstMsg = toApiError(err).message;
          return salesReturnLookupService.sourceOrderDetail(sid).then((order) => ({
            sourceId: sid,
            lines: (order.items ?? []).map(
              (it, i): LineDraft => ({
                key: `${it.productId}/${it.variantId}/${i}`,
                productId: it.productId,
                variantId: it.variantId,
                productCode: it.productCode,
                productName: it.productName,
                uom: it.uom,
                orderedQty: it.qty,
                delivered: it.qty,
                returned: 0,
                remaining: it.qty,
                unitPrice: it.unitPrice,
                qty: String(it.qty),
                locationId: null,
              }),
            ),
            locations: [] as LocationOption[],
            contextError: firstMsg as string | null,
          }));
        },
      );
    },
    [open, sourceId],
  );
  const activeBundle = ctxBundle && ctxBundle.sourceId === sourceId ? ctxBundle : null;
  const contextError = activeBundle ? activeBundle.contextError : ctxFetchError;
  const loadingSource = sourceId === null ? false : ctxLoading;
  const locations =
    activeBundle && activeBundle.locations.length > 0
      ? activeBundle.locations
      : lookupLocations;

  // Reset saat modal dibuka.
  const [appliedCtx, setAppliedCtx] = useState<number | null>(null);
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setSourceId(null);
      setLines([]);
      setReturnMode("return_only");
      setReplacements([]);
      setAppliedCtx(null);
      setPreview(null);
      setPreviewError(null);
      setReturnDate(todayWIB());
      setNotes("");
      setFormError(null);
    }
  }

  // Terapkan baris konteks sekali per order sumber terpilih.
  if (open && activeBundle && appliedCtx !== activeBundle.sourceId) {
    setAppliedCtx(activeBundle.sourceId);
    setLines(activeBundle.lines);
  }

  function handleSourceChange(v: string | number | null) {
    const id = typeof v === "number" ? v : v === null ? null : Number(v);
    if (id === null || Number.isNaN(id)) {
      setSourceId(null);
      setLines([]);
      setAppliedCtx(null);
      setPreview(null);
      return;
    }
    setSourceId(id);
    setAppliedCtx(null);
    setFormError(null);
    setPreview(null);
    setPreviewError(null);
  }

  function patchLine(key: string, patch: Partial<LineDraft>) {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)));
    setPreview(null);
  }

  function handleModeChange(v: string) {
    setReturnMode(v === "exchange" ? "exchange" : "return_only");
    if (v !== "exchange") setReplacements([]);
    setPreview(null);
    setPreviewError(null);
    setFormError(null);
  }

  function addReplacementRow() {
    setReplacements((prev) => [
      ...prev,
      {
        key: `repl-${replKey++}`,
        productId: null,
        productLabel: "",
        variants: [],
        variantId: null,
        uoms: [],
        uom: "",
        qty: "1",
        locationId: null,
        loadingDetail: false,
      },
    ]);
    setPreview(null);
  }

  function patchRepl(key: string, patch: Partial<ReplDraft>) {
    setReplacements((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
    setPreview(null);
  }

  function removeRepl(key: string) {
    setReplacements((prev) => prev.filter((r) => r.key !== key));
    setPreview(null);
  }

  function handleReplProduct(key: string, v: string | number | null) {
    const pid = typeof v === "number" ? v : v === null ? null : Number(v);
    if (pid === null || Number.isNaN(pid)) {
      patchRepl(key, {
        productId: null,
        productLabel: "",
        variants: [],
        variantId: null,
        uoms: [],
        uom: "",
      });
      return;
    }
    const opt = productOptions.find((p) => p.id === pid);
    patchRepl(key, {
      productId: pid,
      productLabel: opt ? `${opt.code} — ${opt.name}` : `#${pid}`,
      loadingDetail: true,
    });
    salesReturnLookupService
      .productDetail(pid)
      .then((d) => {
        const uoms = uniqueUoms(d);
        const def = d.variants.find((x) => x.id > 0);
        patchRepl(key, {
          variants: d.variants.map((x) => ({ id: x.id, code: x.code, name: x.name })),
          variantId: def ? def.id : null,
          uoms,
          uom: d.salesUom || uoms[0] || "",
          loadingDetail: false,
        });
      })
      .catch((err) => {
        patchRepl(key, { productId: null, productLabel: "", loadingDetail: false });
        toast.danger("Gagal memuat varian produk", toApiError(err).message);
      });
  }

  const estimateTotal = lines.reduce((sum, l) => {
    const q = Number(l.qty);
    if (Number.isNaN(q) || q <= 0) return sum;
    return sum + Math.round(q * l.unitPrice);
  }, 0);

  function collectPayload() {
    if (sourceId === null) {
      return { error: "Order jual wajib dipilih." };
    }
    const parsed = salesReturnSchema.safeParse({
      salesOrderId: sourceId,
      returnDate: returnDate.trim(),
      notes: notes.trim(),
      returnMode,
      items: lines
        .filter((l) => Number(l.qty) > 0)
        .map((l) => ({
          productId: l.productId,
          variantId: l.variantId,
          locationId: l.locationId,
          uom: l.uom,
          qty: Number(l.qty),
        })),
      replacementItems:
        returnMode === "exchange"
          ? replacements
              .filter((r) => (r.productId ?? 0) > 0 && Number(r.qty) > 0)
              .map((r) => ({
                productId: r.productId as number,
                variantId: r.variantId,
                locationId: r.locationId,
                uom: r.uom,
                qty: Number(r.qty),
              }))
          : [],
    });
    if (!parsed.success) {
      return { error: parsed.error.issues[0]?.message ?? "Form belum valid." };
    }
    return {
      data: {
        salesOrderId: parsed.data.salesOrderId,
        returnDate: parsed.data.returnDate || undefined,
        notes: parsed.data.notes || undefined,
        returnMode: parsed.data.returnMode,
        items: parsed.data.items.map((it) => ({
          productId: it.productId,
          variantId: it.variantId,
          // Schema menjamin locationId positif.
          locationId: it.locationId as number,
          uom: it.uom,
          qty: it.qty,
        })),
        replacementItems: parsed.data.replacementItems.map((it) => ({
          productId: it.productId,
          variantId: it.variantId,
          locationId: it.locationId as number,
          uom: it.uom,
          qty: it.qty,
        })),
      },
    };
  }

  function handlePreview() {
    const collected = collectPayload();
    if (collected.error || !collected.data) {
      setPreviewError(collected.error ?? "Form belum valid.");
      setPreview(null);
      return;
    }
    setPreviewError(null);
    setPreviewBusy(true);
    salesReturnService
      .preview({
        salesOrderId: collected.data.salesOrderId,
        returnMode: collected.data.returnMode,
        items: collected.data.items,
        replacementItems:
          collected.data.replacementItems.length > 0
            ? collected.data.replacementItems
            : undefined,
      })
      .then((res) => setPreview(res.data))
      .catch((err) => {
        setPreview(null);
        setPreviewError(toApiError(err).message);
      })
      .finally(() => setPreviewBusy(false));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const collected = collectPayload();
    if (collected.error || !collected.data) {
      setFormError(collected.error ?? "Form belum valid.");
      return;
    }
    setFormError(null);
    setBusy(true);
    salesReturnService
      .create({
        salesOrderId: collected.data.salesOrderId,
        returnDate: collected.data.returnDate,
        notes: collected.data.notes,
        returnMode: collected.data.returnMode,
        items: collected.data.items,
        replacementItems:
          collected.data.replacementItems.length > 0
            ? collected.data.replacementItems
            : undefined,
      })
      .then((res) => {
        toast.fromServer(res.message, "Retur penjualan dibuat", res.data.number);
        onCreated(res.data);
        onClose();
      })
      .catch((err) => setFormError(toApiError(err).message))
      .finally(() => setBusy(false));
  }

  const isExchange = returnMode === "exchange";

  return (
    <Modal
      open={open}
      title="Buat Retur Penjualan"
      description="Retur dibuat sebagai draf terhadap order jual terkonfirmasi/selesai."
      size="xl"
      onClose={onClose}
    >
      {lookupError && (
        <div className="mb-4">
          <Notice tone="danger" title={lookupError} />
        </div>
      )}
      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Order Jual Sumber" required>
            <SearchSelect
              options={candidates.map((o) => ({
                value: o.id,
                label: `${o.number} — ${o.partyName || "Walk-in"}`,
              }))}
              value={sourceId}
              placeholder={loadingLookups ? "Memuat order…" : "Pilih order…"}
              disabled={loadingLookups || busy}
              onChange={handleSourceChange}
            />
          </FormField>
          <FormField label="Tanggal Retur" helperText="Kosongkan untuk hari ini (server).">
            <DateInput
              value={returnDate}
              onChange={(e) => setReturnDate(e.target.value)}
              disabled={busy}
            />
          </FormField>
        </div>

        <div className="mt-4">
          <FormField label="Mode Retur" helperText="Tukar barang mewajibkan minimal 1 baris barang pengganti.">
            <Segmented
              options={[
                { value: "return_only", label: "Retur saja" },
                { value: "exchange", label: "Tukar barang" },
              ]}
              value={returnMode}
              onChange={handleModeChange}
            />
          </FormField>
        </div>

        <div className="mt-4">
          <FormField label="Alasan / Catatan">
            <TextArea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Alasan retur…"
              rows={2}
              disabled={busy}
            />
          </FormField>
        </div>

        <div className="mt-4">
          <SectionCard
            title="Item Retur"
            description="Qty dibatasi sisa yang benar-benar bisa diretur (terkirim − sudah diretur) dari konteks SO."
          >
            {contextError && (
              <div className="mb-4">
                <Notice tone="warning" title="Konteks sisa tidak tersedia — memakai qty order">
                  {contextError}
                </Notice>
              </div>
            )}
            {loadingSource ? (
              <p className="text-muted py-4 text-center text-sm">Memuat item order…</p>
            ) : (
              <DataTable<LineDraft>
                rows={lines}
                rowKey={(r) => r.key}
                emptyTitle="Belum ada item"
                emptyDescription="Pilih order jual sumber untuk memuat baris item."
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div>
                        <p className="font-medium">{r.productName}</p>
                        <p className="text-muted text-xs">
                          {r.productCode} · {r.uom}
                        </p>
                        <p className="text-muted text-xs">
                          Sisa {formatNumber(r.remaining)} dari {formatNumber(r.orderedQty)} dipesan
                          (terkirim {formatNumber(r.delivered)}, diretur {formatNumber(r.returned)})
                        </p>
                      </div>
                    ),
                  },
                  {
                    header: "Qty Retur",
                    render: (r) => (
                      <input
                        type="number"
                        min={0}
                        max={r.remaining}
                        step="any"
                        value={r.qty}
                        disabled={busy}
                        onChange={(e) =>
                          patchLine(r.key, {
                            qty: clampQty(e.target.value, r.remaining),
                          })
                        }
                        className="w-28 rounded-md border border-hairline bg-surface px-2 py-1.5 text-right text-sm text-ink"
                      />
                    ),
                  },
                  {
                    header: "Lokasi Masuk",
                    render: (r) => (
                      <SelectInput
                        value={r.locationId ?? ""}
                        disabled={busy}
                        onChange={(e) =>
                          patchLine(r.key, {
                            locationId: e.target.value === "" ? null : Number(e.target.value),
                          })
                        }
                      >
                        <option value="">Pilih…</option>
                        {locations.map((l) => (
                          <option key={l.id} value={l.id}>
                            {l.code} — {l.name}
                          </option>
                        ))}
                      </SelectInput>
                    ),
                  },
                ]}
              />
            )}
            {lines.length > 0 && (
              <p className="mt-3 text-right text-sm text-ink">
                Estimasi total: <span className="font-bold">{formatIDR(estimateTotal)}</span>{" "}
                <span className="text-muted">(harga final dihitung server — gunakan Pratinjau)</span>
              </p>
            )}
          </SectionCard>
        </div>

        {isExchange && (
          <div className="mt-4">
            <SectionCard
              title="Barang Pengganti"
              description="Barang yang dikirim ke customer sebagai tukar. Stok dicek server saat pengiriman."
              actions={
                <Button variant="secondary" size="sm" onClick={addReplacementRow} disabled={busy}>
                  Tambah baris
                </Button>
              }
            >
              <DataTable<ReplDraft>
                rows={replacements}
                rowKey={(r) => r.key}
                emptyTitle="Belum ada barang pengganti"
                emptyDescription="Retur tukar wajib membawa minimal 1 baris barang pengganti."
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div className="min-w-44">
                        <SearchSelect
                          options={productOptions.map((p) => ({
                            value: p.id,
                            label: `${p.code} — ${p.name}`,
                          }))}
                          value={r.productId}
                          placeholder="Pilih produk…"
                          disabled={busy || r.loadingDetail}
                          onChange={(v) => handleReplProduct(r.key, v)}
                        />
                        {r.loadingDetail && (
                          <p className="text-muted mt-1 text-xs">Memuat varian…</p>
                        )}
                      </div>
                    ),
                  },
                  {
                    header: "Varian",
                    render: (r) => (
                      <SelectInput
                        value={r.variantId ?? ""}
                        disabled={busy || r.productId === null}
                        onChange={(e) =>
                          patchRepl(r.key, {
                            variantId: e.target.value === "" ? null : Number(e.target.value),
                          })
                        }
                      >
                        <option value="">Pilih…</option>
                        {r.variants.map((v) => (
                          <option key={v.id} value={v.id}>
                            {v.code} — {v.name}
                          </option>
                        ))}
                      </SelectInput>
                    ),
                  },
                  {
                    header: "Satuan",
                    render: (r) =>
                      r.uoms.length > 0 ? (
                        <SelectInput
                          value={r.uom}
                          disabled={busy}
                          onChange={(e) => patchRepl(r.key, { uom: e.target.value })}
                        >
                          <option value="">Pilih…</option>
                          {r.uoms.map((u) => (
                            <option key={u} value={u}>
                              {u}
                            </option>
                          ))}
                        </SelectInput>
                      ) : (
                        <TextInput
                          value={r.uom}
                          disabled={busy || r.productId === null}
                          placeholder="Satuan"
                          onChange={(e) => patchRepl(r.key, { uom: e.target.value })}
                        />
                      ),
                  },
                  {
                    header: "Qty",
                    render: (r) => (
                      <input
                        type="number"
                        min={0}
                        step="any"
                        value={r.qty}
                        disabled={busy}
                        onChange={(e) => patchRepl(r.key, { qty: e.target.value })}
                        className="w-24 rounded-md border border-hairline bg-surface px-2 py-1.5 text-right text-sm text-ink"
                      />
                    ),
                  },
                  {
                    header: "Lokasi Keluar",
                    render: (r) => (
                      <SelectInput
                        value={r.locationId ?? ""}
                        disabled={busy}
                        onChange={(e) =>
                          patchRepl(r.key, {
                            locationId: e.target.value === "" ? null : Number(e.target.value),
                          })
                        }
                      >
                        <option value="">Pilih…</option>
                        {locations.map((l) => (
                          <option key={l.id} value={l.id}>
                            {l.code} — {l.name}
                          </option>
                        ))}
                      </SelectInput>
                    ),
                  },
                  {
                    header: "",
                    align: "right",
                    render: (r) => (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => removeRepl(r.key)}
                        disabled={busy}
                      >
                        Hapus
                      </Button>
                    ),
                  },
                ]}
              />
            </SectionCard>
          </div>
        )}

        <div className="mt-4">
          <div className="flex flex-wrap items-center gap-3">
            <Button
              variant="secondary"
              onClick={handlePreview}
              disabled={busy || previewBusy || loadingSource || sourceId === null}
            >
              {previewBusy ? "Mempratinjau…" : "Pratinjau"}
            </Button>
            <span className="text-muted text-xs">
              Hitung subtotal, diskon, PPN, dan total pengganti di server tanpa menyimpan.
            </span>
          </div>
          {previewError && (
            <div className="mt-3">
              <Notice tone="danger" title={previewError} />
            </div>
          )}
          {preview && (
            <div className="mt-3">
              <Notice
                tone="info"
                title={`Pratinjau (${preview.mode === "exchange" ? "Tukar barang" : "Retur saja"}) — belum disimpan`}
              >
                <span className="mt-1 block">
                  Subtotal {formatIDR(preview.subtotal)} · Diskon {formatIDR(preview.discountTotal)} ·
                  PPN {formatIDR(preview.taxTotal)} · Total{" "}
                  <span className="font-bold">{formatIDR(preview.total)}</span>
                  {preview.mode === "exchange" && (
                    <>
                      {" "}· Total pengganti{" "}
                      <span className="font-bold">{formatIDR(preview.replacementTotal)}</span>
                    </>
                  )}
                </span>
              </Notice>
            </div>
          )}
        </div>

        {formError && (
          <div className="mt-4">
            <Notice tone="danger" title={formError} />
          </div>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Batal
          </Button>
          <Button type="submit" disabled={busy || loadingSource || lines.length === 0}>
            {busy ? "Menyimpan…" : "Simpan Draf"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
