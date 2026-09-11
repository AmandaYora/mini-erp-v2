import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  DateInput,
  FormField,
  Modal,
  SearchSelect,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { paymentFormSchema } from "@/modules/payment/schemas/payment.schema";
import type { PaymentFormValues } from "@/modules/payment/schemas/payment.schema";
import type { PartyLookup } from "@/modules/payment/types";

interface AllocRow {
  key: number;
  orderType: string;
  orderId: string;
  amount: string;
}

let nextRowKey = 1;

interface PaymentFormModalProps {
  open: boolean;
  parties: PartyLookup[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: PaymentFormValues) => void;
}

function defaultOrderType(partyType: string | undefined): string {
  return partyType === "supplier" ? "purchase" : "sales";
}

function orderTypeOptions(partyType: string | undefined): SearchSelectOption[] {
  if (partyType === "supplier") {
    return [
      { value: "purchase", label: "Order Beli" },
      { value: "purchase_return", label: "Retur Beli (refund)" },
    ];
  }
  return [
    { value: "sales", label: "Order Jual" },
    { value: "sales_return", label: "Retur Jual (refund)" },
  ];
}

/**
 * Modal catat pembayaran. Alokasi opsional: kosong = backend mengisi FIFO
 * otomatis (tagihan terbuka tertua dulu); bila diisi, totalnya harus sama
 * dengan nominal pembayaran (aturan backend, service.go: Create).
 */
export function PaymentFormModal({
  open,
  parties,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: PaymentFormModalProps) {
  const [partyId, setPartyId] = useState<number | null>(null);
  const [method, setMethod] = useState("cash");
  const [amount, setAmount] = useState("");
  const [paidAt, setPaidAt] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [rows, setRows] = useState<AllocRow[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const partyOptions: SearchSelectOption[] = parties.map((p) => ({
    value: p.id,
    label: `${p.name} (${p.type === "supplier" ? "Supplier" : "Customer"})`,
  }));
  const partyType = parties.find((p) => p.id === partyId)?.type;

  const allocTotal = rows.reduce((sum, r) => {
    const n = Number(r.amount);
    return Number.isFinite(n) && n > 0 ? sum + n : sum;
  }, 0);
  const amountNum = Number(amount);

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
    const filled = rows.filter(
      (r) => r.orderId.trim() !== "" || r.amount.trim() !== "",
    );
    const parsed = paymentFormSchema.safeParse({
      partyId,
      method,
      amount,
      paidAt: paidAt.trim() ? paidAt.trim() : undefined,
      notes: notes.trim() ? notes.trim() : undefined,
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
    <Modal
      open={open}
      title="Catat Pembayaran"
      description="Alokasi kosong = otomatis ke tagihan terbuka tertua (FIFO)."
      size="lg"
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField label="Pihak" required errorText={errors.partyId}>
          <SearchSelect
            options={partyOptions}
            value={partyId}
            onChange={(v) =>
              setPartyId(typeof v === "number" ? v : v === null ? null : Number(v))
            }
            placeholder="Pilih customer / supplier…"
          />
        </FormField>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <FormField label="Metode" required errorText={errors.method}>
            <SelectInput
              value={method}
              onChange={(e) => setMethod(e.target.value)}
            >
              <option value="cash">Tunai</option>
              <option value="transfer">Transfer</option>
            </SelectInput>
          </FormField>
          <FormField label="Nominal (Rp)" required errorText={errors.amount}>
            <TextInput
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="cth. 1500000"
              inputMode="numeric"
            />
          </FormField>
          <FormField label="Tanggal Bayar" helperText="Kosongkan = hari ini">
            <DateInput
              value={paidAt}
              onChange={(e) => setPaidAt(e.target.value)}
            />
          </FormField>
        </div>

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
            <p className="text-heading text-[0.85rem] font-medium">
              Alokasi Dokumen{" "}
              <span className="text-muted font-normal">(opsional)</span>
            </p>
            <Button type="button" variant="secondary" size="sm" onClick={addRow}>
              Tambah Baris
            </Button>
          </div>
          {errors.allocations && (
            <p className="text-bad mb-2 text-[0.82rem]">{errors.allocations}</p>
          )}
          {rows.length === 0 ? (
            <p className="text-muted rounded-md border border-dashed border-hairline px-3 py-3 text-sm">
              Tanpa baris alokasi — pembayaran dibagi otomatis ke tagihan
              terbuka tertua.
            </p>
          ) : (
            <div className="flex flex-col gap-2">
              {rows.map((r) => (
                <div
                  key={r.key}
                  className="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2"
                >
                  <FormField label="Tipe">
                    <SelectInput
                      value={r.orderType}
                      onChange={(e) =>
                        updateRow(r.key, { orderType: e.target.value })
                      }
                    >
                      {orderTypeOptions(partyType).map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </SelectInput>
                  </FormField>
                  <FormField label="ID Dokumen">
                    <TextInput
                      value={r.orderId}
                      onChange={(e) =>
                        updateRow(r.key, { orderId: e.target.value })
                      }
                      placeholder="cth. 12"
                      inputMode="numeric"
                    />
                  </FormField>
                  <FormField label="Nominal (Rp)">
                    <TextInput
                      value={r.amount}
                      onChange={(e) =>
                        updateRow(r.key, { amount: e.target.value })
                      }
                      placeholder="cth. 500000"
                      inputMode="numeric"
                    />
                  </FormField>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeRow(r.key)}
                  >
                    Hapus
                  </Button>
                </div>
              ))}
              <p className="text-muted text-[0.82rem]">
                Total alokasi {formatIDR(allocTotal)}
                {Number.isFinite(amountNum) && amountNum > 0 && (
                  <>
                    {" "}
                    dari {formatIDR(amountNum)}
                    {allocTotal !== amountNum &&
                      " — harus sama dengan nominal (aturan backend)"}
                  </>
                )}
              </p>
            </div>
          )}
        </div>

        <ActionRow>
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={submitting}
          >
            Batal
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? "Menyimpan…" : "Simpan Pembayaran"}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
