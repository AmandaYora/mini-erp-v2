import { useState } from "react";
import type { FormEvent } from "react";
import { Lock } from "lucide-react";
import {
  Button,
  DataTable,
  FormField,
  PageHeader,
  SectionCard,
  Skeleton,
  TextInput,
} from "@/shared/components/ui";
import type { DataTableColumn } from "@/shared/components/ui";
import { FormSection, Page, StickyFooter } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { assistantService } from "@/modules/assistant/services/assistant.service";
import { assistantConfigSchema } from "@/modules/assistant/schemas/assistant.schema";
import type { AssistantRunStat } from "@/modules/assistant/types";

const statColumns: DataTableColumn<AssistantRunStat>[] = [
  { header: "Hari", render: (r) => r.day },
  { header: "Intent", render: (r) => r.intent || "–" },
  { header: "Mode", render: (r) => r.mode || "–" },
  { header: "Total", align: "right", render: (r) => formatNumber(r.total) },
  { header: "Berhasil", align: "right", render: (r) => formatNumber(r.success) },
  { header: "Rata-rata", align: "right", render: (r) => `${formatNumber(r.avgDurationMs)} ms` },
];

export default function ConfigPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("assistant.manage");

  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([assistantService.getConfig(), assistantService.getStats()]).then(([cfg, rows]) => ({
        cfg,
        rows: rows ?? [],
      })),
    [],
  );
  const config = data?.cfg ?? null;
  const stats = data?.rows ?? [];

  const [rateLimit, setRateLimit] = useState("10");
  const [savedRate, setSavedRate] = useState("10");
  const [prevCfg, setPrevCfg] = useState(config);
  if (config !== prevCfg) {
    setPrevCfg(config);
    if (config) {
      const v = String(config.rateLimitPerMinute ?? 10);
      setRateLimit(v);
      setSavedRate(v);
    }
  }
  const [configErrors, setConfigErrors] = useState<Record<string, string>>({});
  const [configSaving, setConfigSaving] = useState(false);

  const changed = rateLimit !== savedRate ? 1 : 0;

  async function handleConfigSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    // Lapis ke-3 mode baca-saja: submit diabaikan.
    if (!canManage) return;
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
      const v = String(saved.data.rateLimitPerMinute);
      setRateLimit(v);
      setSavedRate(v);
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
    <Page>
      <PageHeader
        title="Konfigurasi asisten"
        description="Mode balasan, batas pesan, dan statistik pemakaian asisten."
      />

      {!canManage && (
        <Notice tone="info" title="Mode baca saja">
          Anda tidak memiliki izin mengubah konfigurasi asisten. Isian terkunci dan tidak dapat disimpan.
        </Notice>
      )}

      {error && !data ? (
        <ErrorState message={error} onRetry={reload} />
      ) : (
        <>
          <form id="assistant-config-form" onSubmit={(e) => void handleConfigSubmit(e)}>
            <FormSection title="Pengaturan" description="Mode balasan dan batas pesan per nomor per menit.">
              {loading && !data ? (
                <div role="status" aria-busy="true" aria-label="Memuat konfigurasi" className="space-y-3">
                  <Skeleton className="h-9 w-full" />
                  <Skeleton className="h-9 w-full" />
                </div>
              ) : (
                <>
                  {!canManage && (
                    <p className="mb-4 flex items-center gap-1.5 text-xs text-muted-foreground">
                      <Lock size={12} aria-hidden /> Hanya baca — Anda tidak memiliki izin mengubah bagian ini.
                    </p>
                  )}
                  <fieldset disabled={!canManage || configSaving} className="grid gap-4 md:grid-cols-2">
                    <FormField
                      label="Mode"
                      htmlFor="assistant-mode"
                      helperText="Terkunci rule_based — tanpa AI/RAG di skop ini."
                    >
                      <TextInput id="assistant-mode" value="rule_based" disabled readOnly />
                    </FormField>
                    <FormField
                      label="Batas pesan per menit"
                      htmlFor="assistant-rate"
                      required
                      helperText="1-120 pesan per nomor per menit."
                      errorText={configErrors.rateLimitPerMinute}
                    >
                      <TextInput
                        id="assistant-rate"
                        type="number"
                        min={1}
                        max={120}
                        value={rateLimit}
                        onChange={(e) => setRateLimit(e.target.value)}
                      />
                    </FormField>
                  </fieldset>
                  <p className="mt-3 text-xs text-muted-foreground">
                    Mode efektif: {config?.effectiveMode || "–"}
                    {config?.updatedAt ? ` · Terakhir diubah ${formatDateTime(config.updatedAt)}` : ""}
                  </p>
                </>
              )}
            </FormSection>

            {canManage && changed > 0 && (
              <StickyFooter summary={<span role="status">{changed} perubahan belum disimpan</span>}>
                <Button variant="secondary" onClick={() => setRateLimit(savedRate)} disabled={configSaving}>
                  Batalkan
                </Button>
                <Button type="submit" loading={configSaving}>
                  {configSaving ? "Menyimpan…" : "Simpan"}
                </Button>
              </StickyFooter>
            )}
          </form>

          <SectionCard
            flush
            title="Statistik 7 hari"
            description="Jumlah dan keberhasilan run asisten per hari, intent, dan mode."
          >
            <DataTable<AssistantRunStat>
              columns={statColumns}
              rows={data ? stats : null}
              rowKey={(r) => `${r.day}-${r.intent}-${r.mode}`}
              loading={loading}
              emptyTitle="Belum ada statistik"
              emptyDescription="Belum ada aktivitas asisten dalam 7 hari terakhir."
            />
          </SectionCard>
        </>
      )}
    </Page>
  );
}
