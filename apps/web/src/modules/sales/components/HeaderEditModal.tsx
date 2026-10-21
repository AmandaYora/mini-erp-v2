import { useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, Modal, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { headerEditSchema } from "@/modules/sales/schemas/order-correction.schema";
import type { HeaderEditValues } from "@/modules/sales/schemas/order-correction.schema";
import { headerEditDirty, headerEditInitial } from "@/modules/sales/lib/order-correction";
import type { SalesOrder } from "@/modules/sales/types";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";

interface HeaderEditModalProps {
  order: SalesOrder;
  /** Pesan penjaga server: dokumen apa yang mengunci kolom ekonomi. */
  lockReason?: string;
  canPickUser: boolean;
  submitting: boolean;
  /** Pesan API terakhir (gagal simpan) — tampil di dalam dialog. */
  serverError: string | null;
  onClose: () => void;
  onSubmit: (initial: HeaderEditValues, values: HeaderEditValues) => void;
}

// Ubah info order terkonfirmasi yang sudah punya surat jalan/pembayaran/retur:
// hanya kolom yang tidak mengubah nilai order (service.go lightOnly).
export function HeaderEditModal({
  order,
  lockReason,
  canPickUser,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: HeaderEditModalProps) {
  const [initial] = useState(() => headerEditInitial(order));
  const [v, setV] = useState<HeaderEditValues>(initial);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function set<K extends keyof HeaderEditValues>(key: K, value: HeaderEditValues[K]) {
    setV((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = headerEditSchema.safeParse(v);
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const issue of parsed.error.issues) mapped[issue.path.join(".")] ??= issue.message;
      setErrors(mapped);
      return;
    }
    setErrors({});
    onSubmit(initial, parsed.data);
  }

  const dirty = headerEditDirty(initial, v);

  return (
    <Modal
      open
      size="md"
      title="Ubah info order"
      description={`Barang, harga, pelanggan, dan termin ${order.number} tidak bisa diubah lagi karena sudah ada dokumen turunan.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="header-edit-form" loading={submitting} disabled={!dirty}>
            Simpan perubahan
          </Button>
        </>
      }
    >
      <form id="header-edit-form" onSubmit={handleSubmit} noValidate className="space-y-4">
        {lockReason && (
          <Notice tone="info" title="Kenapa terbatas">
            {lockReason}
          </Notice>
        )}
        {serverError && (
          <Notice tone="danger" title="Gagal menyimpan">
            {serverError}
          </Notice>
        )}
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            label="Jatuh tempo"
            htmlFor="he-due"
            required={order.paymentTerms === "net"}
            errorText={errors.dueDate}
          >
            <DateInput id="he-due" value={v.dueDate} onChange={(e) => set("dueDate", e.target.value)} />
          </FormField>
          {canPickUser && (
            <FormField label="Penanggung jawab" errorText={errors.assignedUserId}>
              <UserSearchSelect
                value={v.assignedUserId}
                onChange={(uid) => set("assignedUserId", uid)}
                placeholder="Tanpa penanggung jawab"
              />
            </FormField>
          )}
          <FormField label="No. faktur pajak" htmlFor="he-tax-no" errorText={errors.taxInvoiceNumber}>
            <TextInput
              id="he-tax-no"
              value={v.taxInvoiceNumber}
              onChange={(e) => set("taxInvoiceNumber", e.target.value)}
              maxLength={100}
            />
          </FormField>
          <FormField label="Tanggal faktur pajak" htmlFor="he-tax-date" errorText={errors.taxInvoiceDate}>
            <DateInput id="he-tax-date" value={v.taxInvoiceDate} onChange={(e) => set("taxInvoiceDate", e.target.value)} />
          </FormField>
          <FormField label="Penerima" htmlFor="he-recipient" errorText={errors.shipToRecipient}>
            <TextInput
              id="he-recipient"
              value={v.shipToRecipient}
              onChange={(e) => set("shipToRecipient", e.target.value)}
              maxLength={150}
            />
          </FormField>
          <FormField label="Telepon penerima" htmlFor="he-phone" errorText={errors.shipToPhone}>
            <TextInput id="he-phone" value={v.shipToPhone} onChange={(e) => set("shipToPhone", e.target.value)} maxLength={30} />
          </FormField>
          <div className="sm:col-span-2">
            <FormField
              label="Alamat kirim"
              htmlFor="he-address"
              errorText={errors.shipToAddress}
              helperText={
                order.shipToAddressId > 0
                  ? "Diambil dari buku alamat pelanggan. Bila disunting, alamat ini hanya berlaku untuk order ini."
                  : undefined
              }
            >
              <TextArea
                id="he-address"
                value={v.shipToAddress}
                onChange={(e) => set("shipToAddress", e.target.value)}
                rows={2}
                maxLength={1000}
              />
            </FormField>
          </div>
          <div className="sm:col-span-2">
            <FormField label="Catatan" htmlFor="he-notes" errorText={errors.notes}>
              <TextArea id="he-notes" value={v.notes} onChange={(e) => set("notes", e.target.value)} rows={2} />
            </FormField>
          </div>
          <div className="sm:col-span-2">
            <FormField label="Alasan perubahan" htmlFor="he-reason" errorText={errors.reason}>
              <TextInput
                id="he-reason"
                value={v.reason}
                onChange={(e) => set("reason", e.target.value)}
                maxLength={255}
                placeholder="Opsional, dicatat di jejak audit"
              />
            </FormField>
          </div>
        </div>
      </form>
    </Modal>
  );
}
