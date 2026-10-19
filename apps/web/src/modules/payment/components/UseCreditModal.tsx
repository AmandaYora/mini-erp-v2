import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, FormSheet, SelectInput, TextArea } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { UseCreditPayload } from "@/modules/payment/types";

interface UseCreditModalProps {
  open: boolean;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: UseCreditPayload) => void;
}

/**
 * Pakai saldo uang muka (N-41): saldo yang ditinggalkan retur terkonfirmasi
 * (kredit pelanggan / kredit pemasok) dipakai melunasi tagihan terbuka pihak
 * yang sama. Tidak ada kas bergerak; jurnalnya Dr Uang Muka / Cr Piutang
 * (beli: Dr Hutang / Cr Uang Muka). Nominal tidak bisa melebihi sisa saldo retur.
 */
export function UseCreditModal({ open, submitting, serverError, onClose, onSubmit }: UseCreditModalProps) {
  const [side, setSide] = useState<"customer" | "supplier">("customer");
  const [partyId, setPartyId] = useState<number | null>(null);
  const [returnId, setReturnId] = useState("");
  const [orderId, setOrderId] = useState("");
  const [amount, setAmount] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const formId = useId();

  const { data, loading, error } = useAsyncData(
    () => (partyId ? paymentService.creditBalances(partyId) : Promise.resolve(null)),
    [partyId],
  );
  const holders = (data?.items ?? []).filter((b) => b.remaining > 0);
  const chosen = holders.find((b) => String(b.returnId) === returnId);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const next: Record<string, string> = {};
    const amountNum = Number(amount);
    if (!partyId) next.partyId = "Pilih pihak dulu";
    if (!chosen) next.returnId = "Pilih retur sumber saldo";
    if (!orderId) next.orderId = "Pilih tagihan";
    if (!Number.isInteger(amountNum) || amountNum <= 0) next.amount = "Nominal harus bilangan bulat lebih dari 0";
    else if (chosen && amountNum > chosen.remaining) {
      next.amount = `Melebihi sisa saldo (${formatIDR(chosen.remaining)})`;
    }
    setErrors(next);
    if (Object.keys(next).length > 0 || !partyId || !chosen) return;
    onSubmit({
      partyId,
      returnType: chosen.returnType,
      returnId: chosen.returnId,
      orderType: side === "supplier" ? "purchase" : "sales",
      orderId: Number(orderId),
      amount: amountNum,
      notes: notes.trim() ? notes.trim() : undefined,
    });
  }

  return (
    <FormSheet
      open={open}
      title="Pakai saldo uang muka"
      description="Saldo dari retur dipakai melunasi tagihan terbuka pihak yang sama, tanpa kas bergerak."
      size="md"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            {submitting ? "Memproses…" : "Pakai saldo"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField label="Jenis pihak" required>
          <SelectInput
            value={side}
            onChange={(e) => {
              setSide(e.target.value as "customer" | "supplier");
              setPartyId(null);
              setReturnId("");
              setOrderId("");
            }}
          >
            <option value="customer">Pelanggan (saldo dari retur jual)</option>
            <option value="supplier">Pemasok (saldo dari retur beli)</option>
          </SelectInput>
        </FormField>

        <FormField label="Pihak" required errorText={errors.partyId}>
          <PartySearchSelect
            key={side}
            partyKind={side}
            value={partyId}
            onChange={(id) => {
              setPartyId(id);
              setReturnId("");
              setOrderId("");
            }}
          />
        </FormField>

        <FormField label="Retur sumber saldo" required errorText={errors.returnId}>
          {!partyId ? (
            <p className="py-2 text-sm text-muted-foreground">Pilih pihak lebih dulu.</p>
          ) : loading ? (
            <p className="py-2 text-sm text-muted-foreground">Memuat saldo…</p>
          ) : error ? (
            <p className="py-2 text-sm text-destructive">{toApiError(error).message}</p>
          ) : holders.length === 0 ? (
            <p className="py-2 text-sm text-muted-foreground">Pihak ini tidak punya saldo uang muka.</p>
          ) : (
            <SelectInput
              value={returnId}
              onChange={(e) => {
                const hit = holders.find((b) => String(b.returnId) === e.target.value);
                setReturnId(e.target.value);
                if (hit) setAmount(String(hit.remaining));
              }}
            >
              <option value="">Pilih retur…</option>
              {holders.map((b) => (
                <option key={b.returnId} value={b.returnId}>
                  {b.number} — sisa {formatIDR(b.remaining)} dari {formatIDR(b.credited)}
                </option>
              ))}
            </SelectInput>
          )}
        </FormField>

        <FormField label="Tagihan" required errorText={errors.orderId}>
          {!partyId ? (
            <p className="py-2 text-sm text-muted-foreground">Pilih pihak lebih dulu.</p>
          ) : side === "supplier" ? (
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

        <FormField label="Nominal dipakai" required errorText={errors.amount}>
          <RupiahInput
            value={amount}
            onValueChange={setAmount}
            fillAmount={chosen?.remaining}
            fillLabel="Seluruh saldo"
            invalid={Boolean(errors.amount)}
            aria-label="Nominal dipakai"
          />
        </FormField>

        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Keterangan (opsional)"
            rows={2}
            maxLength={255}
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
