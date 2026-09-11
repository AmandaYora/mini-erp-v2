import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  FormField,
  Modal,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR } from "@/shared/lib/format";
import {
  NOMINAL_CONFIRM_THRESHOLD,
  adjustTypeOptions,
  basisOptions,
  directionOptions,
  memberTypeCreateSchema,
  memberTypeUpdateSchema,
  roundingModeOptions,
} from "@/modules/party/schemas/member-type.schema";
import type {
  MemberTypeCreateValues,
  MemberTypeUpdateValues,
} from "@/modules/party/schemas/member-type.schema";
import type { MemberType } from "@/modules/party/types";

interface MemberTypeFormModalProps {
  open: boolean;
  /** null = buat baru (code wajib); terisi = ubah (code terkunci). */
  initial: MemberType | null;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: MemberTypeCreateValues | MemberTypeUpdateValues) => void;
}

export function MemberTypeFormModal({
  open,
  initial,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: MemberTypeFormModalProps) {
  const isEdit = initial !== null;
  const [code, setCode] = useState("");
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [basis, setBasis] = useState(initial?.basis ?? "selling_price");
  const [direction, setDirection] = useState(initial?.direction ?? "minus");
  const [adjType, setAdjType] = useState(initial?.type ?? "percent");
  const [value, setValue] = useState(String(initial?.value ?? 0));
  const [mode, setMode] = useState(initial?.roundingMode ?? "none");
  const [step, setStep] = useState(String(initial?.roundingStep ?? 0));
  const [confirmed, setConfirmed] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const numericValue = Number(value);
  const needsConfirm =
    adjType === "nominal" && Number.isFinite(numericValue) && numericValue > NOMINAL_CONFIRM_THRESHOLD;

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const raw = {
      name,
      description,
      basis,
      direction,
      type: adjType,
      value,
      mode,
      step,
      confirmed,
    };
    const parsed = isEdit
      ? memberTypeUpdateSchema.safeParse(raw)
      : memberTypeCreateSchema.safeParse({ ...raw, code });
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
    <Modal
      open={open}
      title={isEdit ? `Ubah Tipe Member ${initial.code}` : "Tambah Tipe Member"}
      description={
        isEdit
          ? "Kode terkunci setelah terbit — hanya nama, aturan, dan pembulatan yang dapat diubah."
          : "Kode terkunci setelah terbit dan tidak dapat dipakai ulang, walau tipe diarsipkan."
      }
      size="lg"
      onClose={onClose}
    >
      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
          {serverError && <Notice tone="danger" title={serverError} />}
          {!isEdit && (
            <FormField label="Kode" htmlFor="mt-code" required errorText={errors.code}>
              <TextInput
                id="mt-code"
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="cth. GOLD"
                maxLength={50}
              />
            </FormField>
          )}
          <FormField label="Nama" htmlFor="mt-name" required errorText={errors.name}>
            <TextInput
              id="mt-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="cth. Gold"
              maxLength={200}
            />
          </FormField>
          <FormField label="Deskripsi" htmlFor="mt-desc" errorText={errors.description}>
            <TextArea
              id="mt-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Keterangan (opsional)"
              rows={2}
              maxLength={500}
            />
          </FormField>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <FormField label="Basis harga" errorText={errors.basis}>
              <SelectInput value={basis} onChange={(e) => setBasis(e.target.value)}>
                {basisOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Arah" errorText={errors.direction}>
              <SelectInput value={direction} onChange={(e) => setDirection(e.target.value)}>
                {directionOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Jenis penyesuaian" errorText={errors.type}>
              <SelectInput value={adjType} onChange={(e) => setAdjType(e.target.value)}>
                {adjustTypeOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <FormField
              label={adjType === "percent" ? "Nilai (%)" : "Nilai (Rp)"}
              htmlFor="mt-value"
              required
              errorText={errors.value}
            >
              <TextInput
                id="mt-value"
                type="number"
                min={0}
                max={adjType === "percent" ? 100 : undefined}
                step="any"
                value={value}
                onChange={(e) => setValue(e.target.value)}
              />
            </FormField>
            <FormField label="Pembulatan" errorText={errors.mode}>
              <SelectInput
                value={mode}
                onChange={(e) => {
                  const next = e.target.value;
                  setMode(next);
                  if (next === "none") setStep("0");
                }}
              >
                {roundingModeOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField
              label="Kelipatan"
              htmlFor="mt-step"
              helperText={mode === "none" ? "Harus 0 bila tanpa pembulatan." : undefined}
              errorText={errors.step}
            >
              <TextInput
                id="mt-step"
                type="number"
                min={0}
                step="any"
                value={step}
                onChange={(e) => setStep(e.target.value)}
                disabled={mode === "none"}
              />
            </FormField>
          </div>
          {needsConfirm && (
            <Notice tone="warning" title={`Nominal di atas ${formatIDR(NOMINAL_CONFIRM_THRESHOLD)}`}>
              <label className="mt-1 flex cursor-pointer items-start gap-2">
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />
                <span>Saya yakin — kirim dengan konfirmasi eksplisit.</span>
              </label>
            </Notice>
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
