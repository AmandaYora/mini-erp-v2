import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  FormField,
  Modal,
  SearchSelect,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { settleCreditSchema } from "@/modules/payment/schemas/payment.schema";
import type { SettleCreditFormValues } from "@/modules/payment/schemas/payment.schema";
import type { PartyLookup } from "@/modules/payment/types";

interface SettleCreditModalProps {
  open: boolean;
  parties: PartyLookup[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: SettleCreditFormValues) => void;
}

/**
 * Offset retur terkonfirmasi ke tagihan terbuka tanpa kas bergerak
 * (dua kaki "offset"). Pasangan valid: retur jual↔order jual (customer),
 * retur beli↔order beli (supplier) — ditegakkan backend & skema.
 */
export function SettleCreditModal({
  open,
  parties,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: SettleCreditModalProps) {
  const [partyId, setPartyId] = useState<number | null>(null);
  const [returnType, setReturnType] = useState("sales_return");
  const [returnId, setReturnId] = useState("");
  const [orderType, setOrderType] = useState("sales");
  const [orderId, setOrderId] = useState("");
  const [amount, setAmount] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const partyOptions: SearchSelectOption[] = parties.map((p) => ({
    value: p.id,
    label: `${p.name} (${p.type === "supplier" ? "Supplier" : "Customer"})`,
  }));

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
    <Modal
      open={open}
      title="Offset Kredit (Retur → Tagihan)"
      description="Retur terkonfirmasi dikompensasi ke tagihan terbuka pihak yang sama."
      size="md"
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

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Tipe Retur" required errorText={errors.returnType}>
            <SelectInput
              value={returnType}
              onChange={(e) => {
                setReturnType(e.target.value);
                setOrderType(
                  e.target.value === "purchase_return" ? "purchase" : "sales",
                );
              }}
            >
              <option value="sales_return">Retur Jual (customer)</option>
              <option value="purchase_return">Retur Beli (supplier)</option>
            </SelectInput>
          </FormField>
          <FormField label="ID Retur" required errorText={errors.returnId}>
            <TextInput
              value={returnId}
              onChange={(e) => setReturnId(e.target.value)}
              placeholder="cth. 5"
              inputMode="numeric"
            />
          </FormField>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Tipe Tagihan" required errorText={errors.orderType}>
            <SelectInput
              value={orderType}
              onChange={(e) => setOrderType(e.target.value)}
            >
              <option value="sales">Order Jual</option>
              <option value="purchase">Order Beli</option>
            </SelectInput>
          </FormField>
          <FormField label="ID Tagihan" required errorText={errors.orderId}>
            <TextInput
              value={orderId}
              onChange={(e) => setOrderId(e.target.value)}
              placeholder="cth. 12"
              inputMode="numeric"
            />
          </FormField>
        </div>

        <FormField label="Nominal Offset (Rp)" required errorText={errors.amount}>
          <TextInput
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="cth. 250000"
            inputMode="numeric"
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
            {submitting ? "Memproses…" : "Offset Kredit"}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
