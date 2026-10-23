import type { DataHealthReport, HealthAlert } from "@/modules/data-health/types";

export function makeAlert(over: Partial<HealthAlert> = {}): HealthAlert {
  return {
    id: "inventory.cost_above_price",
    area: "persediaan",
    areaLabel: "Persediaan",
    severity: "critical",
    title: "Harga modal > 1,2× harga jual",
    impact: "Nilai persediaan membengkak dan HPP salah setiap kali produk ini terjual.",
    count: 64,
    countUnit: "produk",
    amountRp: 14_200_431_681,
    amountLabel: "nilai persediaan",
    meta: {},
    sample: [{ label: "GRT-1 Granit", value: "modal Rp19.700.000 / jual Rp300.000" }],
    action: { label: "Tinjau 64 produk", to: "/data-health?alert=inventory.cost_above_price", perm: "dashboard.alerts.view_all" },
    oldestAt: null,
    snooze: {
      snoozable: true, maxDays: 7, snoozedAt: null, snoozedUntil: null, reason: null, byName: null,
      countAtSnooze: null, amountAtSnooze: null, reactivated: false,
    },
    tier: "fast",
    computedAt: "2026-10-23T03:00:00Z",
    ...over,
  };
}

export const seedAlert = makeAlert({
  id: "security.seed_password", area: "keamanan", areaLabel: "Keamanan", title: "Superadmin masih memakai sandi bawaan",
  impact: "Siapa pun yang tahu sandi bawaan bisa masuk dengan akses penuh.", count: 1, countUnit: "akun", amountRp: null,
  amountLabel: "", sample: [{ label: "superadmin", value: "sandi bawaan" }], action: null,
  hint: "Minta pemegang akun superadmin login lalu mengganti sandinya.",
  snooze: { ...makeAlert().snooze, snoozable: false, maxDays: 0 },
});

export const minPriceAlert = makeAlert({
  id: "master.min_price_above_selling", area: "master", areaLabel: "Master", severity: "warning",
  title: "Harga minimum di atas harga jual", count: 38, amountRp: null, amountLabel: "",
  action: { label: "Perbaiki harga", to: "/products?priceIssue=min_above_selling", perm: "products.view" },
  snooze: { ...makeAlert().snooze, maxDays: 14 },
});

export const dupAlert = makeAlert({
  id: "master.duplicate_product_name", area: "master", areaLabel: "Master", severity: "info", title: "Nama produk ganda",
  count: 102, countUnit: "grup", amountRp: null, amountLabel: "",
  action: { label: "Tinjau duplikat", to: "/products?duplicateName=1", perm: "products.view" },
  snooze: { ...makeAlert().snooze, maxDays: 14 },
});

export function makeReport(over: Partial<DataHealthReport> = {}): DataHealthReport {
  return {
    areas: [
      { area: "keuangan", areaLabel: "Keuangan", items: [] },
      { area: "persediaan", areaLabel: "Persediaan", items: [makeAlert()] },
      { area: "master", areaLabel: "Master", items: [minPriceAlert, dupAlert] },
      { area: "dokumen", areaLabel: "Dokumen", items: [] },
      { area: "keamanan", areaLabel: "Keamanan", items: [seedAlert] },
    ],
    snoozed: [],
    checks: [
      { id: "finance.period_unclosed", label: "Periode yang belum ditutup", area: "keuangan", status: "ok", tier: "fast", computedAt: "" },
      { id: "finance.unbalanced_journal", label: "Keseimbangan jurnal", area: "keuangan", status: "ok", tier: "heavy", computedAt: "" },
    ],
    failedChecks: [],
    bySeverity: { critical: 2, warning: 1, info: 1 },
    pending: false,
    computedAt: "2026-10-23T03:00:00Z",
    ...over,
  };
}
