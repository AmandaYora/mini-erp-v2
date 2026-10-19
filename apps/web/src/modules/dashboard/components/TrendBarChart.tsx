import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { compactIDR } from "@/modules/dashboard/lib/dashboard";
import type { DashboardTrendPoint, TrendRangeKey } from "@/modules/dashboard/types";

const TICK = { fontSize: 12, fill: "var(--muted-foreground)" } as const;

/** Grafik batang omzet (warna chart-1 dari token DS; tanpa hex). */
export function TrendBarChart({
  data,
  range,
  summary,
}: {
  data: DashboardTrendPoint[];
  range: TrendRangeKey;
  /** Ringkasan teks untuk pembaca layar. */
  summary: string;
}) {
  return (
    <div role="img" aria-label={summary} className="h-64 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }} barCategoryGap="20%">
          <CartesianGrid stroke="var(--border)" vertical={false} />
          <XAxis
            dataKey="date"
            tickFormatter={(v: string) =>
              range === "12m" ? `${v.slice(5, 7)}/${v.slice(2, 4)}` : `${v.slice(8, 10)}/${v.slice(5, 7)}`
            }
            minTickGap={range === "12m" ? 60 : 24}
            tick={TICK}
            tickLine={false}
            axisLine={{ stroke: "var(--border)" }}
          />
          <YAxis tickFormatter={compactIDR} width={56} tick={TICK} tickLine={false} axisLine={false} />
          <Tooltip
            formatter={(value) => [formatIDR(Number(value)), "Omzet"]}
            labelFormatter={(label) => formatDate(String(label))}
            cursor={{ fill: "var(--accent)" }}
            contentStyle={{
              background: "var(--popover)",
              border: "1px solid var(--border)",
              borderRadius: 8,
              fontSize: 12,
              color: "var(--foreground)",
            }}
          />
          <Bar dataKey="revenue" name="Omzet" fill="var(--chart-1)" radius={[3, 3, 0, 0]} maxBarSize={28} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
