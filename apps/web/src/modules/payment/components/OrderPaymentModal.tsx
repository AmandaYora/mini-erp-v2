import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, SelectInput, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";
import { paymentService } from "@/modules/payment/services/payment.service";
import { cashChange } from "@/modules/payment/lib/fifo";
import { referenceLabel } from "@/modules/payment/types";
import {
  buildOrderPaymentSchema,
  toOrderPaymentPayload,
} from "@/modules/payment/schemas/order-payment.schema";

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `pay-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

interface Props {
  open: boolean;
  onClose: () => void;
  onPaid: () => void;
  orderType: "sales" | "purchase";
  orderId: number;
  orderNumber: string;
  /** 0 = walk-in (tanpa pihak): alokasi tetap eksplisit ke order ini. */
  partyId: number;
  outstanding: number;
}

/**
 * Catat pembayaran untuk SATU order dari halaman detailnya. Alokasi dikunci
 * ke order ini (tanpa FIFO); pesan server (mis. "melebihi sisa") tampil apa
 * adanya di dalam panel.
 */
export function OrderPaymentModal({
  open,
  onClose,
  onPaid,
  orderType,
  orderId,
  orderNumber,
  partyId,
  outstanding,
}: Props) {
  const [amount, setAmount] = useState(String(outstanding));
  const [method, setMethod] = useState<"cash" | "transfer" | "check">("cash");
  const [paidAt, setPaidAt] = useState(todayWIB());
  const [reference, setReference] = useState("");
  const [tendered, setTendered] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  // Satu kunci idempoten per pembukaan modal: klik ganda/retry tidak menggandakan.
  const [clientRef, setClientRef] = useState(newClientRef);
  const formId = useId();

  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setAmount(String(outstanding));
      setMethod("cash");
      setPaidAt(todayWIB());
      setReference("");
      setTendered("");
      setNotes("");
      setErrors({});
      setServerError(null);
      setSaving(false);
      setClientRef(newClientRef());
    }
  }

  const isPurchase = orderType === "purchase";
  const amountNum = Number(amount);
  const tenderedNum = Number(tendered);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = buildOrderPaymentSchema(outstanding).safeParse({
      amount,
      method,
      paidAt: paidAt.trim(),
      reference,
      amountTendered: method === "cash" ? tendered : "",
      notes,
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        mapped[String(issue.path[0] ?? "_")] ??= issue.message;
      }
      setErrors(mapped);
      return;
    }
    setErrors({});
    setServerError(null);
    setSaving(true);
    try {
      const res = await paymentService.create(
        toOrderPaymentPayload(parsed.data, { partyId, orderType, orderId, clientRef }),
      );
      toast.fromServer(res.message, "Pembayaran dicatat", res.data.number);
      onPaid();
    } catch (err) {
      const apiErr = toApiError(err);
      setServerError(apiErr.message);
      const mapped: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) mapped[fe.field] ??= fe.message;
      if (Object.keys(mapped).length > 0) setErrors(mapped);
    } finally {
      setSaving(false);
    }
  }

  return (
    <FormSheet
      open={open}
      title={isPurchase ? "Bayar ke pemasok" : "Catat pembayaran"}
      description={`${orderNumber} · sisa tagihan ${formatIDR(outstanding)}`}
      size="md"
      onClose={() => {
        if (!saving) onClose();
      }}
      actions={
        <>
          <Button type="button" variant="secondary" disabled={saving} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={saving}>
            {saving ? "Menyimpan…" : "Simpan pembayaran"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Nominal" required errorText={errors.amount} helperText="Maksimal sisa tagihan.">
          <RupiahInput
            value={amount}
            onValueChange={setAmount}
            fillAmount={outstanding}
            invalid={Boolean(errors.amount)}
            aria-label="Nominal pembayaran"
          />
        </FormField>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Metode" required errorText={errors.method}>
            <SelectInput
              value={method}
              onChange={(e) => {
                setMethod(e.target.value as "cash" | "transfer" | "check");
                setTendered("");
              }}
            >
              <option value="cash">Tunai</option>
              <option value="transfer">Transfer</option>
              <option value="check">Cek</option>
            </SelectInput>
          </FormField>
          <FormField label="Tanggal bayar" errorText={errors.paidAt}>
            <DateInput value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
          </FormField>
        </div>
        {method === "cash" ? (
          <FormField
            label="Uang diterima"
            errorText={errors.amountTendered}
            helperText={
              tendered.trim() !== "" && Number.isFinite(tenderedNum) && Number.isFinite(amountNum)
                ? `Kembalian ${formatIDR(cashChange(tenderedNum, amountNum))}`
                : "Opsional, untuk kembalian."
            }
          >
            <RupiahInput
              value={tendered}
              onValueChange={setTendered}
              invalid={Boolean(errors.amountTendered)}
              aria-label="Uang diterima"
            />
          </FormField>
        ) : (
          <FormField label={referenceLabel(method)} errorText={errors.reference}>
            <TextInput value={reference} maxLength={100} onChange={(e) => setReference(e.target.value)} />
          </FormField>
        )}
        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </FormField>
      </form>
    </FormSheet>
  );
}
