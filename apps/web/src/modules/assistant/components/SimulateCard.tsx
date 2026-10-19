import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, SectionCard, TextArea, TextInput } from "@/shared/components/ui";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { BranchPicker } from "@/modules/assistant/components/BranchPicker";
import { assistantService } from "@/modules/assistant/services/assistant.service";
import { simulateSchema } from "@/modules/assistant/schemas/assistant.schema";
import type { AssistantChatResult } from "@/modules/assistant/types";

/**
 * Simulasi pesan dari nomor WhatsApp tertentu (POST /assistant/simulate,
 * izin assistant.simulate). Tidak ada pesan terkirim; kanal harus terhubung.
 */
export default function SimulateCard() {
  const [phone, setPhone] = useState("");
  const [message, setMessage] = useState("");
  const [branch, setBranch] = useState("0");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<AssistantChatResult | null>(null);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = simulateSchema.safeParse({ phone, message });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) errs[String(issue.path[0] ?? "")] ??= issue.message;
      setErrors(errs);
      return;
    }
    setErrors({});
    setBusy(true);
    try {
      const res = await assistantService.simulate({ ...parsed.data, branchId: Number(branch) || 0 });
      setResult(res.data);
      toast.fromServer(res.message, "Simulasi selesai");
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) if (fe.field) next[fe.field] ??= fe.message;
      setErrors(next);
      toast.danger("Simulasi gagal", apiErr.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <SectionCard
      title="Simulasi nomor"
      description="Uji jawaban asisten seolah pesan datang dari nomor tertentu. Tidak ada pesan yang dikirim."
    >
      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
        <FormField label="Nomor pengirim" htmlFor="simulate-phone" required errorText={errors.phone}>
          <TextInput
            id="simulate-phone"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="cth: 6281234567890"
            inputMode="numeric"
            disabled={busy}
          />
        </FormField>
        <BranchPicker id="simulate-branch" value={branch} onChange={setBranch} disabled={busy} />
        <FormField label="Pesan" htmlFor="simulate-message" required errorText={errors.message}>
          <TextArea
            id="simulate-message"
            rows={3}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            disabled={busy}
          />
        </FormField>
        <div className="flex justify-end">
          <Button type="submit" loading={busy}>
            {busy ? "Menjalankan…" : "Jalankan simulasi"}
          </Button>
        </div>
      </form>
      {result && (
        <div className="mt-4 rounded-md border border-border bg-background p-3 text-sm text-foreground">
          <p className="text-xs text-muted-foreground">
            {result.branchName} · intent {result.intent} · mode {result.mode}
          </p>
          <p className="mt-1 whitespace-pre-wrap">{result.answer}</p>
        </div>
      )}
    </SectionCard>
  );
}
