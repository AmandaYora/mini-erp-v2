// Bentuk JSON backend: apps/api/internal/modules/dashboard/contracts/alerts.go (ADR-0025).
// Alert dihitung langsung dari data; hanya penundaan yang disimpan.

export type AlertSeverity = "critical" | "warning" | "info";
export type AlertArea = "keuangan" | "persediaan" | "master" | "dokumen" | "keamanan";
export type CheckState = "ok" | "alert" | "pending" | "error";

export interface AlertSample {
  label: string;
  value: string;
}

export interface AlertAction {
  label: string;
  to: string;
  /** Tombol disembunyikan bila sesi tidak punya izin ini. */
  perm: string;
}

export interface AlertSnooze {
  snoozable: boolean;
  maxDays: number;
  snoozedAt: string | null;
  snoozedUntil: string | null;
  reason: string | null;
  byName: string | null;
  countAtSnooze: number | null;
  amountAtSnooze: number | null;
  /** Tunda gugur karena masalah bertambah sejak ditunda. */
  reactivated: boolean;
}

export interface HealthAlert {
  id: string;
  area: AlertArea;
  areaLabel: string;
  severity: AlertSeverity;
  title: string;
  impact: string;
  count: number;
  countUnit: string;
  amountRp: number | null;
  amountLabel: string;
  meta: Record<string, number>;
  sample: AlertSample[];
  action: AlertAction | null;
  /** Petunjuk bila tidak ada tombol (A1 untuk owner). */
  hint?: string;
  oldestAt: string | null;
  snooze: AlertSnooze;
  tier: "fast" | "heavy";
  computedAt: string;
}

export interface BySeverity {
  critical: number;
  warning: number;
  info: number;
}

export interface FailedCheck {
  id: string;
  label: string;
  message: string;
}

export interface AlertSummary {
  items: HealthAlert[];
  totalActive: number;
  bySeverity: BySeverity;
  snoozedCount: number;
  failedChecks: FailedCheck[];
  pending: boolean;
  computedAt: string;
}

export interface CheckStatus {
  id: string;
  label: string;
  area: AlertArea;
  status: CheckState;
  tier: "fast" | "heavy";
  computedAt: string;
}

export interface AreaAlerts {
  area: AlertArea;
  areaLabel: string;
  items: HealthAlert[];
}

export interface DataHealthReport {
  areas: AreaAlerts[];
  snoozed: HealthAlert[];
  checks: CheckStatus[];
  failedChecks: FailedCheck[];
  bySeverity: BySeverity;
  pending: boolean;
  computedAt: string;
}

export interface AlertItemLink {
  label: string;
  to: string;
  perm: string;
}

export interface AlertItemRow {
  key: string;
  label: string;
  columns: Record<string, string | number>;
  links: AlertItemLink[];
}
