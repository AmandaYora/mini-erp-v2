import { useId } from "react";
import { Area, AreaChart, CartesianGrid, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { compactIDR } from "@/modules/reporting/lib/trend";
import type { TrendPoint } from "@/modules/reporting/types";

const TICK = { fontSize: 12, fill: "var(--muted-foreground)" } as const;

/** Grafik tren: omzet (area, chart-1) dan laba (garis, chart-2). Warna dari token DS, bukan hex. */
export function TrendChart({ data, summary }: { data: TrendPoint[]; summary: string }) {
  const gradientId = `rev-${useId().replace(/:/g, "")}`;
  const long = data.length > 92;
  return (
    <div role="img" aria-label={summary} className="h-72 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--chart-1)" stopOpacity={0.28} />
              <stop offset="100%" stopColor="var(--chart-1)" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="var(--border)" vertical={false} />
          <XAxis
            dataKey="date"
            tickFormatter={(v: string) => (long ? `${v.slice(5, 7)}/${v.slice(2, 4)}` : `${v.slice(8, 10)}/${v.slice(5, 7)}`)}
            minTickGap={long ? 60 : 24}
            tick={TICK}
            tickLine={false}
            axisLine={{ stroke: "var(--border)" }}
          />
          <YAxis tickFormatter={compactIDR} width={56} tick={TICK} tickLine={false} axisLine={false} />
          <Tooltip
            formatter={(value, name) => [formatIDR(Number(value)), String(name)]}
            labelFormatter={(label) => formatDate(String(label))}
            contentStyle={{
              background: "var(--popover)",
              border: "1px solid var(--border)",
              borderRadius: 8,
              fontSize: 12,
              color: "var(--foreground)",
            }}
          />
          <Legend iconType="circle" wrapperStyle={{ fontSize: 12 }} />
          <Area
            type="monotone"
            dataKey="revenue"
            name="Omzet"
            stroke="var(--chart-1)"
            strokeWidth={2}
            fill={`url(#${gradientId})`}
            activeDot={{ r: 4 }}
          />
          <Line type="monotone" dataKey="profit" name="Laba" stroke="var(--chart-2)" strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
