import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";
import { issueMap, purchaseHeaderSchema } from "@/modules/purchasing/schemas/purchase-correction.schema";
import type { PurchaseHeaderValues } from "@/modules/purchasing/schemas/purchase-correction.schema";
import type { PurchaseOrder } from "@/modules/purchasing/types";

interface HeaderEditSheetProps {
  open: boolean;
  order: PurchaseOrder;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: PurchaseHeaderValues) => void;
}

const day = (v: string | null | undefined) => (v ?? "").slice(0, 10);

/**
 * Ubah info dokumen order yang sudah berjalan: jatuh tempo, faktur pemasok,
 * faktur pajak, penanggung jawab, catatan. Nilai uang dan barang tidak
 * tersentuh, jadi boleh walau sudah ada penerimaan, pembayaran, atau retur.
 */
export function HeaderEditSheet({ open, order, submitting, serverError, onClose, onSubmit }: HeaderEditSheetProps) {
  const can = useAuthStore((s) => s.can);
  const formId = useId();
  const [values, setValues] = useState<PurchaseHeaderValues>(() => ({
    dueDate: day(order.dueDate),
    paymentTermDays: order.paymentTermDays,
    notes: order.notes,
    supplierInvoiceNumber: order.supplierInvoiceNumber,
    supplierInvoiceDate: day(order.supplierInvoiceDate),
    taxInvoiceNumber: order.taxInvoiceNumber,
    taxInvoiceDate: day(order.taxInvoiceDate),
    assignedUserId: order.assignedUserId,
    reason: "",
  }));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const set = <K extends keyof PurchaseHeaderValues>(key: K, v: PurchaseHeaderValues[K]) =>
    setValues((prev) => ({ ...prev, [key]: v }));

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = purchaseHeaderSchema.safeParse(values);
    if (!parsed.success) {
      setErrors(issueMap(parsed.error));
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <FormSheet
      open={open}
      title="Ubah info dokumen"
      description="Harga, qty, pemasok, dan pajak tidak berubah. Perubahan tercatat di riwayat audit."
      size="md"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            Simpan perubahan
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Jatuh tempo" errorText={errors.dueDate}>
            <DateInput value={values.dueDate} onChange={(e) => set("dueDate", e.target.value)} />
          </FormField>
          <FormField label="Masa tempo (hari)" errorText={errors.paymentTermDays}>
            <TextInput
              type="number"
              min={0}
              max={3650}
              value={values.paymentTermDays}
              onChange={(e) => set("paymentTermDays", Number(e.target.value) || 0)}
            />
          </FormField>
          <FormField label="No. faktur pemasok" errorText={errors.supplierInvoiceNumber}>
            <TextInput
              value={values.supplierInvoiceNumber}
              onChange={(e) => set("supplierInvoiceNumber", e.target.value)}
            />
          </FormField>
          <FormField label="Tanggal faktur pemasok" errorText={errors.supplierInvoiceDate}>
            <DateInput value={values.supplierInvoiceDate} onChange={(e) => set("supplierInvoiceDate", e.target.value)} />
          </FormField>
          <FormField label="No. faktur pajak" errorText={errors.taxInvoiceNumber}>
            <TextInput value={values.taxInvoiceNumber} onChange={(e) => set("taxInvoiceNumber", e.target.value)} />
          </FormField>
          <FormField label="Tanggal faktur pajak" errorText={errors.taxInvoiceDate}>
            <DateInput value={values.taxInvoiceDate} onChange={(e) => set("taxInvoiceDate", e.target.value)} />
          </FormField>
        </div>
        {can("users.view") && (
          <FormField label="Penanggung jawab" errorText={errors.assignedUserId}>
            <UserSearchSelect
              value={values.assignedUserId > 0 ? values.assignedUserId : null}
              onChange={(uid) => set("assignedUserId", uid ?? 0)}
              placeholder="Tanpa penanggung jawab"
            />
          </FormField>
        )}
        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea rows={2} value={values.notes} onChange={(e) => set("notes", e.target.value)} />
        </FormField>
        <FormField label="Alasan perubahan" helperText="Opsional, tersimpan di riwayat audit." errorText={errors.reason}>
          <TextArea
            rows={2}
            maxLength={500}
            value={values.reason}
            placeholder="Mis. faktur pemasok baru diterima"
            onChange={(e) => set("reason", e.target.value)}
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
