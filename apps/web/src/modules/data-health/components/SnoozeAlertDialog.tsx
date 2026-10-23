import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, RadioGroup, RadioGroupItem, TextArea } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { SNOOZE_DAYS, snoozeSchema } from "@/modules/data-health/schemas/snooze.schema";
import type { SnoozeValues } from "@/modules/data-health/schemas/snooze.schema";
import { countLabel, snoozeEndLabel } from "@/modules/data-health/lib/alerts";
import type { HealthAlert } from "@/modules/data-health/types";

const REASON_CHIPS = ["Sedang dikerjakan", "Menunggu pemasok", "Data memang benar"];

export interface SnoozeAlertDialogProps {
  alert: HealthAlert | null;
  onClose: () => void;
  /** Kirim ke server; lempar galat agar dialog menampilkan pesan API dan tetap terbuka. */
  onSubmit: (alert: HealthAlert, values: SnoozeValues) => Promise<void>;
}

/**
 * Dialog "Tunda alert" (brief §6): durasi wajib (opsi di atas `snooze.maxDays` tidak ditawarkan),
 * alasan 5–255 karakter dengan chip saran. Bukan ReasonDialog: menunda bukan aksi merusak
 * (tombol primary, bukan danger) dan butuh isian durasi.
 */
export function SnoozeAlertDialog({ alert, ...rest }: SnoozeAlertDialogProps) {
  // Isi dipasang ulang setiap kali dibuka agar alasan lama tidak tersisa.
  return alert ? <SnoozeBody key={alert.id} alert={alert} {...rest} /> : null;
}

function SnoozeBody({ alert, onClose, onSubmit }: Omit<SnoozeAlertDialogProps, "alert"> & { alert: HealthAlert }) {
  const maxDays = alert.snooze.maxDays || 7;
  const options = SNOOZE_DAYS.filter((d) => d <= maxDays);
  const [days, setDays] = useState<number>(alert.severity === "critical" ? 1 : 7);
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<{ days?: string; reason?: string }>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const id = useId();
  const formId = `${id}-form`;
  const reasonId = `${id}-reason`;
  const daysLabelId = `${id}-days`;

  function addChip(text: string) {
    setReason((r) => (r.trim() === "" ? text : `${r.trimEnd()}. ${text}`).slice(0, 255));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = snoozeSchema(maxDays).safeParse({ days, reason });
    if (!parsed.success) {
      const f = parsed.error.flatten().fieldErrors;
      setErrors({ days: f.days?.[0], reason: f.reason?.[0] });
      return;
    }
    setErrors({});
    setServerError(null);
    setSubmitting(true);
    try {
      await onSubmit(alert, parsed.data);
    } catch (err) {
      setServerError(formatApiError(err));
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      size="sm"
      title="Tunda alert"
      description={`${alert.title} · ${countLabel(alert)}`}
      onClose={submitting ? () => undefined : onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            Tunda
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} noValidate className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title="Alert gagal ditunda">{serverError}</Notice>}
        <div>
          <p id={daysLabelId} className="mb-1.5 text-sm font-medium text-foreground">
            Tunda selama <span className="text-destructive">*</span>
          </p>
          <RadioGroup
            aria-labelledby={daysLabelId}
            value={String(days)}
            onValueChange={(v) => setDays(Number(v))}
            className="gap-2"
          >
            {options.map((d) => (
              <div key={d} className="flex items-center gap-2.5">
                <RadioGroupItem id={`${id}-d${d}`} value={String(d)} />
                <label htmlFor={`${id}-d${d}`} className="text-sm text-foreground">
                  {`${d} hari`}
                  <span className="text-muted-foreground">{` — sampai ${snoozeEndLabel(d)}`}</span>
                </label>
              </div>
            ))}
          </RadioGroup>
          {alert.severity === "critical" && (
            <p className="mt-1.5 text-xs text-muted-foreground">Alert kritis paling lama ditunda 7 hari.</p>
          )}
          {errors.days && <p className="mt-1.5 text-xs text-destructive">{errors.days}</p>}
        </div>
        <FormField
          label="Alasan"
          htmlFor={reasonId}
          required
          errorText={errors.reason}
        >
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap gap-2">
              {REASON_CHIPS.map((c) => (
                <Button key={c} type="button" variant="secondary" size="sm" onClick={() => addChip(c)}>
                  {c}
                </Button>
              ))}
            </div>
            <TextArea
              id={reasonId}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={3}
              maxLength={255}
              placeholder="Mis. revaluasi menunggu harga baru dari pemasok"
            />
          </div>
        </FormField>
        <p className="-mt-2 text-xs text-muted-foreground">
          Berlaku untuk semua pengguna di cabang ini. Tercatat di log audit bersama nama Anda.
        </p>
        <Notice tone="info" title="Alert muncul lagi lebih awal bila jumlah atau nilainya bertambah." />
      </form>
    </Modal>
  );
}
