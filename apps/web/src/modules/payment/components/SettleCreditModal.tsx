import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, FormSheet, SelectInput, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { settleCreditSchema } from "@/modules/payment/schemas/payment.schema";
import type { SettleCreditFormValues } from "@/modules/payment/schemas/payment.schema";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";

interface SettleCreditModalProps {
  open: boolean;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: SettleCreditFormValues) => void;
}

/**
 * Offset retur terkonfirmasi ke tagihan terbuka tanpa kas bergerak
 * (dua kaki "offset"). Pasangan valid: retur jual↔order jual (customer),
 * retur beli↔order beli (supplier) — ditegakkan backend & skema.
 * Tampil sebagai panel samping (sheet).
 */
export function SettleCreditModal({ open, submitting, serverError, onClose, onSubmit }: SettleCreditModalProps) {
  const [partyId, setPartyId] = useState<number | null>(null);
  const [returnType, setReturnType] = useState("sales_return");
  const [returnId, setReturnId] = useState("");
  const [orderType, setOrderType] = useState("sales");
  const [orderId, setOrderId] = useState("");
  const [amount, setAmount] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const formId = useId();

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = settleCreditSchema.safeParse({
      partyId,
      returnType,
      returnId,
      orderType,
      orderId,
      amount,
      notes: notes.trim() ? notes.trim() : undefined,
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
      title="Offset kredit"
      description="Retur terkonfirmasi dikompensasi ke tagihan terbuka pihak yang sama."
      size="md"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            {submitting ? "Memproses…" : "Offset kredit"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField label="Jenis retur" required errorText={errors.returnType}>
          <SelectInput
            value={returnType}
            onChange={(e) => {
              setReturnType(e.target.value);
              setPartyId(null);
              setOrderId("");
              setOrderType(e.target.value === "purchase_return" ? "purchase" : "sales");
            }}
          >
            <option value="sales_return">Retur jual (pelanggan)</option>
            <option value="purchase_return">Retur beli (pemasok)</option>
          </SelectInput>
        </FormField>

        <FormField label="Pihak" required errorText={errors.partyId}>
          <PartySearchSelect
            // Retur jual = customer, retur beli = supplier.
            key={returnType}
            partyKind={returnType === "purchase_return" ? "supplier" : "customer"}
            value={partyId}
            onChange={(id) => {
              setPartyId(id);
              setOrderId("");
            }}
          />
        </FormField>

        <FormField label="ID retur" required errorText={errors.returnId}>
          <TextInput
            value={returnId}
            onChange={(e) => setReturnId(e.target.value)}
            placeholder="cth. 5"
            inputMode="numeric"
          />
        </FormField>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Jenis tagihan" required errorText={errors.orderType}>
            <SelectInput value={orderType} onChange={(e) => setOrderType(e.target.value)}>
              <option value="sales">Order jual</option>
              <option value="purchase">Order beli</option>
            </SelectInput>
          </FormField>
          <FormField label="Tagihan" required errorText={errors.orderId}>
            {!partyId ? (
              <p className="py-2 text-sm text-muted-foreground">Pilih pihak lebih dulu.</p>
            ) : orderType === "purchase" ? (
              <PurchaseOrderSearchSelect
                value={orderId ? Number(orderId) : null}
                partyId={partyId}
                status="confirmed,completed"
                onChange={(id) => setOrderId(id ? String(id) : "")}
              />
            ) : (
              <SalesOrderSearchSelect
                value={orderId ? Number(orderId) : null}
                partyId={partyId}
                status="confirmed,completed"
                onChange={(id) => setOrderId(id ? String(id) : "")}
              />
            )}
          </FormField>
        </div>

        <FormField label="Nominal offset" required errorText={errors.amount}>
          <RupiahInput
            value={amount}
            onValueChange={setAmount}
            placeholder="250000"
            invalid={Boolean(errors.amount)}
            aria-label="Nominal offset"
          />
        </FormField>

        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Keterangan (opsional)"
            rows={2}
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
