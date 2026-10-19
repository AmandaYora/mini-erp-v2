import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  FormField,
  FormSheet,
  SearchSelect,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { KeyValueEditor } from "@/shared/components/ui/key-value-editor";
import { PARTY_ATTR_LIMITS, attrsToRows } from "@/shared/lib/key-value";
import type { KeyValueRow } from "@/shared/lib/key-value";
import { partyFormSchema } from "@/modules/party/schemas/party.schema";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import type { PartyListFilter } from "@/modules/party/schemas/party-list-filter.schema";
import type { Party } from "@/modules/party/types";

const ATTR_SUGGESTIONS = ["NPWP", "NIK", "Bank", "No. Rekening", "Nama PIC", "Termin"] as const;

interface PartyFormModalProps {
  open: boolean;
  title: string;
  description?: string;
  /** null = buat baru. */
  initial: Party | null;
  /** true untuk customer; supplier tidak punya member. */
  withMember: boolean;
  /** false (gagal muat) + tanpa member lama = field member disembunyikan. */
  memberLoadOk: boolean;
  memberOptions: SearchSelectOption[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: PartyFormValues) => void;
}

/**
 * Form bersama customer & supplier. Dipasang fresh (conditional render) tiap
 * dibuka sehingga state selalu diinisialisasi dari `initial`.
 */
export function PartyFormModal({
  open,
  title,
  description,
  initial,
  withMember,
  memberLoadOk,
  memberOptions,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: PartyFormModalProps) {
  const isEdit = initial !== null;
  const [code, setCode] = useState(initial?.code ?? "");
  const [name, setName] = useState(initial?.name ?? "");
  const [phone, setPhone] = useState(initial?.phone ?? "");
  const [email, setEmail] = useState(initial?.email ?? "");
  const [address, setAddress] = useState(initial?.address ?? "");
  const [notes, setNotes] = useState(initial?.notes ?? "");
  const [memberTypeId, setMemberTypeId] = useState<number | null>(initial?.member?.id ?? null);
  const [attrs, setAttrs] = useState<KeyValueRow[]>(() => attrsToRows(initial?.attributes));
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Member lama yang tak ada di opsi (mis. sudah diarsipkan) tetap
  // ditampilkan agar tidak terhapus diam-diam saat simpan.
  const options: SearchSelectOption[] = [...memberOptions];
  if (initial?.member && !options.some((o) => o.value === initial.member?.id)) {
    options.unshift({ value: initial.member.id, label: `${initial.member.code} (saat ini)` });
  }
  const showMember = withMember && (memberLoadOk || initial?.member != null);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = partyFormSchema.safeParse({
      code,
      name,
      phone,
      email,
      address,
      notes,
      memberTypeId: showMember ? memberTypeId : undefined,
      attrs,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const key = String(issue.path[0] ?? "");
        if (key) errs[key] ??= issue.message;
      }
      setErrors(errs);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <FormSheet
      open={open}
      title={title}
      description={description}
      size="lg"
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form="party-form" loading={submitting}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="party-form" onSubmit={handleSubmit}>
        <div className="space-y-4">
          {serverError && (
            <Notice tone="danger" title="Gagal menyimpan">
              {serverError}
            </Notice>
          )}
          <FormField
            label="Kode"
            htmlFor="party-code"
            helperText={
              isEdit
                ? "Kode tidak dapat diubah."
                : `Opsional — kosongkan untuk kode otomatis (${withMember ? "CUS-001" : "SUP-001"}, berurutan).`
            }
            errorText={errors.code}
          >
            <TextInput
              id="party-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={isEdit ? "" : "Otomatis bila kosong"}
              disabled={isEdit}
              maxLength={50}
            />
          </FormField>
          <FormField label="Nama" htmlFor="party-name" required errorText={errors.name}>
            <TextInput
              id="party-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nama customer / supplier"
              maxLength={200}
            />
          </FormField>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Telepon" htmlFor="party-phone" errorText={errors.phone}>
              <TextInput
                id="party-phone"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="08…"
                maxLength={30}
              />
            </FormField>
            <FormField label="Email" htmlFor="party-email" errorText={errors.email}>
              <TextInput
                id="party-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="nama@email.com"
                maxLength={100}
              />
            </FormField>
          </div>
          <FormField label="Alamat" htmlFor="party-address" errorText={errors.address}>
            <TextArea
              id="party-address"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="Alamat utama"
              rows={2}
              maxLength={500}
            />
          </FormField>
          <FormField label="Catatan" htmlFor="party-notes" errorText={errors.notes}>
            <TextArea
              id="party-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Catatan (opsional)"
              rows={2}
              maxLength={500}
            />
          </FormField>
          <div>
            <span className="mb-1.5 block text-sm font-medium text-foreground">Informasi tambahan</span>
            <p className="mb-2 text-xs text-muted-foreground">
              Data bebas seperti NPWP, bank, atau nama PIC (maksimal {PARTY_ATTR_LIMITS.maxRows}).
            </p>
            <KeyValueEditor
              idPrefix="party-attrs"
              rows={attrs}
              onChange={setAttrs}
              limits={PARTY_ATTR_LIMITS}
              suggestions={ATTR_SUGGESTIONS}
              serverError={errors.attrs}
            />
          </div>
          {showMember && (
            <FormField
              label="Tipe member"
              helperText="Kosongkan untuk melepas membership."
              errorText={errors.memberTypeId}
            >
              <SearchSelect
                options={options}
                value={memberTypeId}
                onChange={(v) => setMemberTypeId(typeof v === "number" ? v : null)}
                placeholder={options.length === 0 ? "Tidak ada tipe member aktif" : "Pilih tipe member…"}
                allowClear
              />
            </FormField>
          )}
        </div>
      </form>
    </FormSheet>
  );
}

/** Select status untuk FilterBar list party. */
export function PartyStatusFilter({
  value,
  onChange,
}: {
  value: PartyListFilter["status"];
  onChange: (v: PartyListFilter["status"]) => void;
}) {
  return (
    <FormField label="Status" htmlFor="party-status-filter">
      <SelectInput
        id="party-status-filter"
        value={value}
        onChange={(e) => onChange(e.target.value as PartyListFilter["status"])}
      >
        <option value="active">Aktif</option>
        <option value="archived">Diarsipkan</option>
        <option value="all">Semua status</option>
      </SelectInput>
    </FormField>
  );
}
