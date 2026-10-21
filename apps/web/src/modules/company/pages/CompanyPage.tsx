import { useState } from "react";
import { PageHeader } from "@/shared/components/ui";
import { Page } from "@/shared/components/patterns";
import { DetailSkeleton } from "@/shared/components/feedback/skeleton";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { cn } from "@/shared/lib/cn";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { companyService } from "@/modules/company/services/company.service";
import CompanyProfileForm from "@/modules/company/components/CompanyProfileForm";
import DocumentProfileForm from "@/modules/company/components/DocumentProfileForm";
import OperationalSettingsForm from "@/modules/company/components/OperationalSettingsForm";
import OtherSettingsForm from "@/modules/company/components/OtherSettingsForm";
import type { CompanySettings, SettingDefinition } from "@/modules/company/types";

const SECTIONS = [
  { id: "profil", label: "Profil" },
  { id: "dokumen", label: "Dokumen & cetak" },
  { id: "operasional", label: "Operasional" },
] as const;

export default function CompanyPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("company.manage");

  const [definitions, setDefinitions] = useState<SettingDefinition[]>([]);
  const [settings, setSettings] = useState<CompanySettings>({});
  const [settingsVersion, setSettingsVersion] = useState(0);
  const [activeSection, setActiveSection] = useState<string>("profil");

  function applySettings(next: CompanySettings) {
    setSettings(next);
    setSettingsVersion((v) => v + 1);
  }

  async function reloadSettings() {
    try {
      applySettings((await companyService.getSettings()) ?? {});
    } catch (err) {
      toast.danger("Gagal memuat pengaturan", toApiError(err).message);
    }
  }

  const { data, loading, error, reload } = useAsyncData(async () => {
    const [profile, settings, defs] = await Promise.all([
      companyService.getProfile(),
      companyService.getSettings(),
      companyService.getSettingDefinitions(),
    ]);
    return { profile, settings, defs };
  }, []);

  // Salin hasil fetch ke state pengaturan sekali saat tiba.
  const [applied, setApplied] = useState<typeof data>(null);
  if (data && applied !== data) {
    setApplied(data);
    setDefinitions(data.defs);
    applySettings(data.settings);
  }

  function goTo(id: string) {
    setActiveSection(id);
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <Page>
      <PageHeader title="Perusahaan" description="Profil identitas dan pengaturan operasional perusahaan." />

      {!canManage && (
        <Notice tone="info" title="Mode baca saja">
          Anda tidak memiliki izin mengubah pengaturan perusahaan. Semua isian ditampilkan terkunci dan tidak dapat disimpan.
        </Notice>
      )}

      {loading && !data ? (
        <DetailSkeleton />
      ) : error || !data ? (
        <ErrorState message={error ?? "Data perusahaan tidak tersedia."} onRetry={reload} />
      ) : (
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[11rem_minmax(0,1fr)]">
          <nav aria-label="Seksi pengaturan" className="flex gap-1 overflow-x-auto lg:sticky lg:top-20 lg:flex-col">
            {SECTIONS.map((s) => (
              <button
                key={s.id}
                type="button"
                onClick={() => goTo(s.id)}
                aria-current={activeSection === s.id ? "true" : undefined}
                className={cn(
                  "rounded-md px-3 py-2 text-left text-sm font-medium whitespace-nowrap transition-colors hover:bg-muted",
                  activeSection === s.id ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                )}
              >
                {s.label}
              </button>
            ))}
          </nav>

          <div className="min-w-0 divide-y divide-border rounded-xl border border-border bg-card px-6">
            <CompanyProfileForm initial={data.profile} canManage={canManage} />

            <DocumentProfileForm raw={settings.document_profile} canManage={canManage} onSaved={() => void reloadSettings()} />

            <div id="operasional" className="scroll-mt-20 divide-y divide-border">
              <OperationalSettingsForm
                key={`op-${settingsVersion}`}
                definitions={definitions}
                settings={settings}
                canManage={canManage}
                onSaved={applySettings}
              />
              <OtherSettingsForm
                key={`other-${settingsVersion}`}
                settings={settings}
                registeredKeys={definitions.map((d) => d.key)}
                canManage={canManage}
                onSaved={applySettings}
              />
            </div>
          </div>
        </div>
      )}
    </Page>
  );
}
