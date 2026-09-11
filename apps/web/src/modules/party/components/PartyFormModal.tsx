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
import { partyFormSchema } from "@/modules/party/schemas/party.schema";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import type { Party } from "@/modules/party/types";

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
    <Modal open={open} title={title} description={description} size="lg" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
          {serverError && <Notice tone="danger" title={serverError} />}
          <FormField
            label="Kode"
            htmlFor="party-code"
            helperText={isEdit ? "Kode tidak dapat diubah." : "Opsional — kosongkan untuk kode otomatis."}
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
          <ActionRow>
            <Button variant="secondary" onClick={onClose} disabled={submitting}>
              Batal
            </Button>
            <Button type="submit" variant="primary" disabled={submitting}>
              {submitting ? "Menyimpan…" : "Simpan"}
            </Button>
          </ActionRow>
        </div>
      </form>
    </Modal>
  );
}

/** Select status untuk FilterBar list party. */
export function PartyStatusFilter({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <FormField label="Status">
      <SelectInput value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">Semua status</option>
        <option value="active">Aktif</option>
        <option value="archived">Diarsipkan</option>
      </SelectInput>
    </FormField>
  );
}
