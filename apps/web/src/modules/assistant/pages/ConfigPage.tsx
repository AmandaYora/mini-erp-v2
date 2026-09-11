import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  FormField,
  PageHeader,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import type { DataTableColumn } from "@/shared/components/ui";
import { formatDateTime, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { assistantService } from "@/modules/assistant/services/assistant.service";
import { assistantConfigSchema } from "@/modules/assistant/schemas/assistant.schema";
import type {
  AssistantRunStat,
} from "@/modules/assistant/types";

const statColumns: DataTableColumn<AssistantRunStat>[] = [
  { header: "Hari", render: (r) => r.day },
  { header: "Intent", render: (r) => r.intent || "-" },
  { header: "Mode", render: (r) => r.mode || "-" },
  { header: "Total", align: "right", render: (r) => formatNumber(r.total) },
  {
    header: "Berhasil",
    align: "right",
    render: (r) => formatNumber(r.success),
  },
  {
    header: "Rata-rata",
    align: "right",
    render: (r) => `${formatNumber(r.avgDurationMs)} ms`,
  },
];

export default function ConfigPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("assistant.manage");

  const { data, loading, reload } = useAsyncData(
    () =>
      Promise.all([
        assistantService.getConfig(),
        assistantService.getStats(),
      ])
        .then(([cfg, rows]) => ({ cfg, rows: rows ?? [] }))
        .catch((err) => {
          toast.danger(
            "Gagal memuat konfigurasi asisten",
            toApiError(err).message,
          );
          throw err;
        }),
    [],
  );
  const config = data?.cfg ?? null;
  const stats = data?.rows ?? [];
  const configLoading = loading;
  const statsLoading = loading;

  const [rateLimit, setRateLimit] = useState("10");
  const [prevCfg, setPrevCfg] = useState(config);
  if (config !== prevCfg) {
    setPrevCfg(config);
    if (config) setRateLimit(String(config.rateLimitPerMinute ?? 10));
  }
  const [configErrors, setConfigErrors] = useState<Record<string, string>>({});
  const [configSaving, setConfigSaving] = useState(false);

  async function handleConfigSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = assistantConfigSchema.safeParse({
      mode: "rule_based",
      rateLimitPerMinute: rateLimit,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setConfigErrors(errs);
      return;
    }
    setConfigErrors({});
    setConfigSaving(true);
    try {
      const saved = await assistantService.saveConfig(parsed.data);
      setRateLimit(String(saved.data.rateLimitPerMinute));
      toast.fromServer(saved.message, "Konfigurasi asisten disimpan");
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field) next[fe.field] ??= fe.message;
      }
      setConfigErrors(next);
      toast.danger("Gagal menyimpan konfigurasi", apiErr.message);
    } finally {
      setConfigSaving(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Asisten"
        title="Konfigurasi"
        description="Mode balasan, batas pesan, dan statistik pemakaian asisten."
      />

      <div className="space-y-6">
        <SectionCard
          title="Pengaturan"
          description="Mode balasan dan batas pesan per nomor per menit."
        >
          {configLoading ? (
            <p className="text-muted py-2 text-sm">Memuat konfigurasi…</p>
          ) : (
            <form
              id="assistant-config-form"
              onSubmit={(e) => void handleConfigSubmit(e)}
            >
              <fieldset disabled={!canManage || configSaving}>
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField
                    label="Mode"
                    helperText="Terkunci rule_based — tanpa AI/RAG di skop ini."
                  >
                    <TextInput value="rule_based" disabled readOnly />
                  </FormField>
                  <FormField
                    label="Batas Pesan per Menit"
                    required
                    helperText="1-120 pesan per nomor per menit."
                    errorText={configErrors.rateLimitPerMinute}
                  >
                    <TextInput
                      type="number"
                      min={1}
                      max={120}
                      value={rateLimit}
                      onChange={(e) => setRateLimit(e.target.value)}
                    />
                  </FormField>
                </div>
              </fieldset>
              <p className="text-muted mt-3 text-xs">
                Mode efektif: {config?.effectiveMode || "-"}
                {config?.updatedAt
                  ? ` • Terakhir diubah ${formatDateTime(config.updatedAt)}`
                  : ""}
              </p>
              {canManage && (
                <div className="mt-6 flex justify-end border-t border-hairline pt-5">
                  <Button
                    type="submit"
                    form="assistant-config-form"
                    disabled={configSaving}
                  >
                    {configSaving ? "Menyimpan…" : "Simpan"}
                  </Button>
                </div>
              )}
            </form>
          )}
        </SectionCard>

        <SectionCard
          title="Statistik 7 Hari"
          description="Jumlah dan keberhasilan run asisten per hari, intent, dan mode."
        >
          {statsLoading ? (
            <p className="text-muted py-2 text-sm">Memuat statistik…</p>
          ) : (
            <DataTable<AssistantRunStat>
              columns={statColumns}
              rows={stats}
              rowKey={(r) => `${r.day}-${r.intent}-${r.mode}`}
              emptyTitle="Belum ada statistik"
              emptyDescription="Belum ada aktivitas asisten dalam 7 hari terakhir."
            />
          )}
        </SectionCard>
      </div>
    </div>
  );
}
