import type { ReactNode } from "react";
import { PageHeader } from "@/shared/components/ui";
import { NavTabs, Page } from "@/shared/components/patterns";
import type { NavTabItem } from "@/shared/components/patterns";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

const REPORT_TABS: (NavTabItem & { perm: string })[] = [
  { to: ROUTE_PATHS.trialBalance, label: "Neraca Saldo", perm: "finance.report.view" },
  { to: ROUTE_PATHS.profitLoss, label: "Laba Rugi", perm: "finance.report.view" },
  { to: ROUTE_PATHS.balanceSheet, label: "Neraca", perm: "finance.report.view" },
  { to: ROUTE_PATHS.cashReport, label: "Kas", perm: "finance.report.view" },
  { to: ROUTE_PATHS.taxSummary, label: "Pajak", perm: "finance.tax_report.view" },
  { to: ROUTE_PATHS.taxDetail, label: "Rincian PPN", perm: "finance.tax_report.view" },
  { to: ROUTE_PATHS.inventoryValue, label: "Persediaan", perm: "finance.report.view" },
  { to: ROUTE_PATHS.margin, label: "Marjin", perm: "finance.report.view" },
  { to: ROUTE_PATHS.receivablesPayables, label: "Piutang & Hutang", perm: "finance.report.view" },
];

/**
 * Kerangka laporan (T5): judul <h1> = nama laporan, tab antarlaporan berupa tautan
 * (satu URL per laporan), lalu isi laporan. Tab disaring menurut izin pengguna.
 */
export function ReportShell({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const can = useAuthStore((s) => s.can);
  const items = REPORT_TABS.filter((t) => can(t.perm)).map(({ to, label }) => ({ to, label }));
  return (
    <Page>
      <PageHeader title={title} description={description} actions={actions} />
      {items.length > 1 && <NavTabs items={items} label="Laporan keuangan" />}
      {children}
    </Page>
  );
}
