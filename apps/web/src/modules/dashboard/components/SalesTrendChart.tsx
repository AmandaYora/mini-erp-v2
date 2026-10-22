import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatIDR } from "@/shared/lib/format";
import { compactIDR } from "@/modules/dashboard/lib/dashboard";
import { bucketTick, bucketTitle } from "@/modules/dashboard/lib/trend";
import type { TrendBucket, TrendChartType, TrendInterval } from "@/modules/dashboard/lib/trend";

const TICK = { fontSize: 12, fill: "var(--muted-foreground)" } as const;
const MARGIN = { top: 8, right: 8, bottom: 0, left: 0 };

/**
 * Grafik tren omzet: batang, garis, atau candlestick (OHLC omzet harian per minggu/bulan).
 * Warna dari token DS (chart-1; naik = success, turun = destructive), tanpa hex.
 */
export function SalesTrendChart({
  data,
  interval,
  chart,
  summary,
}: {
  data: TrendBucket[];
  interval: TrendInterval;
  chart: TrendChartType;
  /** Ringkasan teks untuk pembaca layar. */
  summary: string;
}) {
  const long = data.length > 92;
  const xAxis = (
    <XAxis
      dataKey="start"
      tickFormatter={(v: string) => bucketTick(v, interval, long)}
      minTickGap={long ? 60 : 24}
      tick={TICK}
      tickLine={false}
      axisLine={{ stroke: "var(--border)" }}
    />
  );
  const yAxis = <YAxis tickFormatter={compactIDR} width={56} tick={TICK} tickLine={false} axisLine={false} />;
  const grid = <CartesianGrid stroke="var(--border)" vertical={false} />;
  const tooltip = (
    <Tooltip
      cursor={chart === "line" ? { stroke: "var(--border)" } : { fill: "var(--accent)" }}
      content={(props) => <TrendTooltip {...props} interval={interval} chart={chart} />}
    />
  );
  return (
    <div role="img" aria-label={summary} className="h-64 w-full">
      <ResponsiveContainer width="100%" height="100%">
        {chart === "line" ? (
          <LineChart data={data} margin={MARGIN}>
            {grid}
            {xAxis}
            {yAxis}
            {tooltip}
            <Line
              type="monotone"
              dataKey="revenue"
              name="Omzet"
              stroke="var(--chart-1)"
              strokeWidth={2}
              dot={data.length <= 31 ? { r: 2.5, fill: "var(--chart-1)" } : false}
              activeDot={{ r: 4 }}
              isAnimationActive={false}
            />
          </LineChart>
        ) : chart === "candle" ? (
          <BarChart data={data} margin={MARGIN} barCategoryGap="30%">
            {grid}
            {xAxis}
            {yAxis}
            {tooltip}
            <Bar dataKey="range" name="Omzet harian" shape={Candle} maxBarSize={18} isAnimationActive={false} />
          </BarChart>
        ) : (
          <BarChart data={data} margin={MARGIN} barCategoryGap="20%">
            {grid}
            {xAxis}
            {yAxis}
            {tooltip}
            <Bar dataKey="revenue" name="Omzet" fill="var(--chart-1)" radius={[3, 3, 0, 0]} maxBarSize={28} />
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}

interface CandleProps {
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  payload?: TrendBucket;
}

/**
 * Satu candle dari batang rentang [terendah, tertinggi] recharts: sumbu = rentang harian,
 * badan = buka→tutup. Naik (tutup ≥ buka) berwarna success, turun destructive.
 */
export function Candle({ x = 0, y = 0, width = 0, height = 0, payload }: CandleProps) {
  if (!payload) return null;
  const { open, close, high, low } = payload;
  const at = (v: number) => (high === low ? y : y + ((high - v) / (high - low)) * height);
  const up = close >= open;
  const color = up ? "var(--success)" : "var(--destructive)";
  const mid = x + width / 2;
  const top = at(Math.max(open, close));
  const body = Math.max(1, at(Math.min(open, close)) - top);
  return (
    <g data-candle={up ? "up" : "down"}>
      <line x1={mid} x2={mid} y1={y} y2={y + height} stroke={color} strokeWidth={1.5} />
      <rect x={x} y={top} width={width} height={body} fill={up ? "var(--background)" : color} stroke={color} strokeWidth={1.5} rx={1} />
    </g>
  );
}

function TrendTooltip({
  active,
  payload,
  interval,
  chart,
}: {
  active?: boolean;
  payload?: ReadonlyArray<{ payload?: unknown }>;
  interval: TrendInterval;
  chart: TrendChartType;
}) {
  const b = payload?.[0]?.payload as TrendBucket | undefined;
  if (!active || !b) return null;
  return (
    <div className="rounded-lg border border-border bg-popover px-3 py-2 text-xs text-foreground shadow-sm">
      <p className="mb-1 font-medium">
        {bucketTitle(b, interval)}
        {b.partial && interval !== "day" ? <span className="font-normal text-muted-foreground"> (berjalan)</span> : null}
      </p>
      <dl className="tabular grid grid-cols-[auto_auto] gap-x-4 gap-y-0.5">
        <dt className="text-muted-foreground">Omzet</dt>
        <dd className="text-right font-medium">{formatIDR(b.revenue)}</dd>
        {chart === "candle" && (
          <>
            <dt className="text-muted-foreground">Buka</dt>
            <dd className="text-right">{formatIDR(b.open)}</dd>
            <dt className="text-muted-foreground">Tertinggi</dt>
            <dd className="text-right">{formatIDR(b.high)}</dd>
            <dt className="text-muted-foreground">Terendah</dt>
            <dd className="text-right">{formatIDR(b.low)}</dd>
            <dt className="text-muted-foreground">Tutup</dt>
            <dd className="text-right">{formatIDR(b.close)}</dd>
          </>
        )}
      </dl>
    </div>
  );
}
