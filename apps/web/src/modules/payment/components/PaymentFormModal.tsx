import { useEffect, useId, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, SelectInput, TextArea, TextInput } from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { paymentFormSchema } from "@/modules/payment/schemas/payment.schema";
import type { PaymentFormValues } from "@/modules/payment/schemas/payment.schema";
import type { OrderBalance } from "@/modules/payment/types";
import { referenceLabel } from "@/modules/payment/types";
import { cashChange, previewFifo } from "@/modules/payment/lib/fifo";
import { paymentService } from "@/modules/payment/services/payment.service";

interface AllocRow {
  key: number;
  orderType: string;
  orderId: string;
  amount: string;
}

let nextRowKey = 1;

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `pay-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

interface PaymentFormModalProps {
  open: boolean;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: PaymentFormValues) => void;
  /** Pihak terisi di awal (tombol "Catat bayar" pada daftar pihak bersaldo, PY-01). */
  initialParty?: { id: number; type: "customer" | "supplier" };
}

function defaultOrderType(partyType: string | undefined): string {
  return partyType === "supplier" ? "purchase" : "sales";
}

function orderTypeOptions(partyType: string | undefined): SearchSelectOption[] {
  if (partyType === "supplier") {
    return [
      { value: "purchase", label: "Order beli" },
      { value: "purchase_return", label: "Retur beli (refund)" },
    ];
  }
  return [
    { value: "sales", label: "Order jual" },
    { value: "sales_return", label: "Retur jual (refund)" },
  ];
}

/**
 * Panel samping (sheet) catat pembayaran. Alokasi opsional: kosong = backend mengisi FIFO
 * otomatis (tagihan terbuka tertua dulu); bila diisi, totalnya harus sama
 * dengan nominal pembayaran (aturan backend, service.go: Create).
 */
export function PaymentFormModal({ open, submitting, serverError, onClose, onSubmit, initialParty }: PaymentFormModalProps) {
  const [partyId, setPartyId] = useState<number | null>(initialParty?.id ?? null);
  // Jenis pihak terpilih ("customer" | "supplier") dari hasil pencarian server.
  const [partyType, setPartyType] = useState<string | undefined>(initialParty?.type);
  const [method, setMethod] = useState("cash");
  const [amount, setAmount] = useState("");
  const [paidAt, setPaidAt] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [reference, setReference] = useState("");
  const [tendered, setTendered] = useState("");
  // Satu kunci idempoten per sesi panel: kirim ulang setelah gagal jaringan
  // tidak menggandakan pembayaran.
  const [clientRef] = useState(newClientRef);
  const [openBills, setOpenBills] = useState<OrderBalance[]>([]);
  const [rows, setRows] = useState<AllocRow[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const formId = useId();

  const allocTotal = rows.reduce((sum, r) => {
    const n = Number(r.amount);
    return Number.isFinite(n) && n > 0 ? sum + n : sum;
  }, 0);
  const amountNum = Number(amount);
  const tenderedNum = Number(tendered);

  // Saldo tagihan terbuka untuk pratinjau FIFO (hanya dokumen tagihan).
  useEffect(() => {
    let live = true;
    if (!partyId) return undefined;
    paymentService
      .balance(partyId)
      .then((b) => {
        if (live) setOpenBills(b.orders ?? []);
      })
      .catch(() => {
        if (live) setOpenBills([]);
      });
    return () => {
      live = false;
    };
  }, [partyId]);

  const fifo = useMemo(() => (partyId ? previewFifo(openBills, amountNum) : null), [partyId, openBills, amountNum]);
  const openTotal = useMemo(
    () => (partyId ? openBills.reduce((s, b) => (b.outstanding > 0 ? s + b.outstanding : s), 0) : 0),
    [partyId, openBills],
  );

  function addRow() {
    setRows((prev) => [
      ...prev,
      {
        key: nextRowKey++,
        orderType: defaultOrderType(partyType),
        orderId: "",
        amount: "",
      },
    ]);
  }

  function updateRow(key: number, patch: Partial<AllocRow>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function removeRow(key: number) {
    setRows((prev) => prev.filter((r) => r.key !== key));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const filled = rows.filter((r) => r.orderId.trim() !== "" || r.amount.trim() !== "");
    const parsed = paymentFormSchema.safeParse({
      partyId,
      method,
      amount,
      paidAt: paidAt.trim() ? paidAt.trim() : undefined,
      notes: notes.trim() ? notes.trim() : undefined,
      reference: reference.trim() ? reference.trim() : undefined,
      amountTendered: method === "cash" && tendered.trim() ? tendered.trim() : undefined,
      clientRef,
      allocations: filled.map((r) => ({
        orderType: r.orderType,
        orderId: r.orderId,
        amount: r.amount,
      })),
    });
    if (!parsed.success) {
      const fieldErrors = parsed.error.flatten().fieldErrors;
      const mapped: Record<string, string> = {};
      for (const [k, v] of Object.entries(fieldErrors)) {
        if (v && v.length > 0 && v[0]) mapped[k] = v[0];
      }
      setErrors(mapped);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <FormSheet
      open={open}
      title="Catat pembayaran"
      description="Alokasi kosong = otomatis ke tagihan terbuka tertua (FIFO)."
      size="lg"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            {submitting ? "Menyimpan…" : "Simpan pembayaran"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField label="Pihak" required errorText={errors.partyId}>
          <PartySearchSelect
            partyKind="any"
            value={partyId}
            placeholder="Cari customer / supplier…"
            onChange={(id, party) => {
              if (id !== partyId) setRows([]);
              setPartyId(id);
              setPartyType(party?.type);
            }}
          />
        </FormField>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Metode" required errorText={errors.method}>
            <SelectInput
              value={method}
              onChange={(e) => {
                setMethod(e.target.value);
                if (e.target.value !== "cash") setTendered("");
              }}
            >
              <option value="cash">Tunai</option>
              <option value="transfer">Transfer</option>
              <option value="check">Cek</option>
            </SelectInput>
          </FormField>
          <FormField label="Tanggal bayar" helperText="Kosongkan = hari ini">
            <DateInput value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
          </FormField>
        </div>

        <FormField label="Nominal" required errorText={errors.amount}>
          <RupiahInput
            value={amount}
            onValueChange={setAmount}
            placeholder="1500000"
            invalid={Boolean(errors.amount)}
            aria-label="Nominal pembayaran"
            fillAmount={openTotal > 0 ? openTotal : undefined}
            fillLabel="Lunasi semua"
          />
        </FormField>

        {method === "cash" ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField
              label="Uang diterima"
              errorText={errors.amountTendered}
              helperText="Opsional. Isi bila uang yang diterima lebih dari nominal."
            >
              <RupiahInput
                value={tendered}
                onValueChange={setTendered}
                placeholder="2000000"
                invalid={Boolean(errors.amountTendered)}
                aria-label="Uang diterima"
              />
            </FormField>
            <div className="flex flex-col justify-end pb-2 text-sm">
              <span className="text-xs text-muted-foreground">Kembalian</span>
              <Money
                className="font-semibold text-foreground"
                value={
                  Number.isFinite(tenderedNum) && Number.isFinite(amountNum) ? cashChange(tenderedNum, amountNum) : 0
                }
              />
            </div>
          </div>
        ) : (
          <FormField
            label={referenceLabel(method)}
            errorText={errors.reference}
            helperText="Opsional, maksimal 100 karakter."
          >
            <TextInput
              value={reference}
              onChange={(e) => setReference(e.target.value)}
              placeholder={method === "check" ? "cth. CEK-001234" : "cth. TRX-20261009"}
              maxLength={100}
            />
          </FormField>
        )}

        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Keterangan (opsional)"
            rows={2}
          />
        </FormField>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <p className="text-sm font-medium text-foreground">
              Alokasi dokumen <span className="font-normal text-muted-foreground">(opsional)</span>
            </p>
            <Button type="button" variant="secondary" size="sm" onClick={addRow}>
              Tambah baris
            </Button>
          </div>
          {errors.allocations && <p className="mb-2 text-sm text-destructive">{errors.allocations}</p>}
          {rows.length === 0 ? (
            <div>
              <p className="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                Tanpa baris alokasi — pembayaran dibagi otomatis ke tagihan terbuka tertua.
              </p>
              {fifo && Number.isFinite(amountNum) && amountNum > 0 && (
                <div className="mt-2 rounded-md border border-border px-3 py-2 text-sm">
                  <p className="mb-1 font-medium text-foreground">Pratinjau alokasi FIFO (perkiraan)</p>
                  {fifo.rows.length === 0 ? (
                    <p className="text-muted-foreground">Tidak ada tagihan terbuka untuk pihak ini.</p>
                  ) : (
                    <ul className="flex flex-col gap-0.5">
                      {fifo.rows.map((r) => (
                        <li key={`${r.orderType}-${r.orderId}`} className="flex justify-between">
                          <span>{r.number || `#${r.orderId}`}</span>
                          <Money value={r.amount} className="font-medium" />
                        </li>
                      ))}
                    </ul>
                  )}
                  {fifo.unallocated > 0 && (
                    <p className="mt-1 text-destructive">
                      {formatIDR(fifo.unallocated)} melebihi tagihan terbuka — server akan menolak.
                    </p>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {rows.map((r) => (
                <div key={r.key} className="flex flex-col gap-3 rounded-md border border-border p-3">
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <FormField label="Jenis">
                      <SelectInput value={r.orderType} onChange={(e) => updateRow(r.key, { orderType: e.target.value })}>
                        {orderTypeOptions(partyType).map((o) => (
                          <option key={o.value} value={o.value}>
                            {o.label}
                          </option>
                        ))}
                      </SelectInput>
                    </FormField>
                    <FormField label="Dokumen">
                      {r.orderType === "sales" && partyId ? (
                        <SalesOrderSearchSelect
                          value={r.orderId ? Number(r.orderId) : null}
                          partyId={partyId}
                          status="confirmed,completed"
                          onChange={(id) => updateRow(r.key, { orderId: id ? String(id) : "" })}
                        />
                      ) : r.orderType === "purchase" && partyId ? (
                        <PurchaseOrderSearchSelect
                          value={r.orderId ? Number(r.orderId) : null}
                          partyId={partyId}
                          status="confirmed,completed"
                          onChange={(id) => updateRow(r.key, { orderId: id ? String(id) : "" })}
                        />
                      ) : (
                        <TextInput
                          value={r.orderId}
                          onChange={(e) => updateRow(r.key, { orderId: e.target.value })}
                          placeholder="ID retur, cth. 12"
                          inputMode="numeric"
                        />
                      )}
                    </FormField>
                  </div>
                  <div className="flex items-end gap-3">
                    <div className="flex-1">
                      <FormField label="Nominal alokasi">
                        <RupiahInput
                          value={r.amount}
                          onValueChange={(v) => updateRow(r.key, { amount: v })}
                          placeholder="500000"
                          aria-label="Nominal alokasi"
                        />
                      </FormField>
                    </div>
                    <Button type="button" variant="ghost" size="sm" onClick={() => removeRow(r.key)}>
                      Hapus baris
                    </Button>
                  </div>
                </div>
              ))}
              <p className="text-sm text-muted-foreground">
                Total alokasi {formatIDR(allocTotal)}
                {Number.isFinite(amountNum) && amountNum > 0 && (
                  <>
                    {" "}
                    dari {formatIDR(amountNum)}
                    {allocTotal !== amountNum && " — harus sama dengan nominal (aturan backend)"}
                  </>
                )}
              </p>
            </div>
          )}
        </div>
      </form>
    </FormSheet>
  );
}
