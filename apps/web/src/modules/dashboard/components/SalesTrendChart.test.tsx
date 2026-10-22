import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Candle } from "@/modules/dashboard/components/SalesTrendChart";
import type { TrendBucket } from "@/modules/dashboard/lib/trend";

const bucket = (open: number, close: number, low: number, high: number): TrendBucket => ({
  start: "2026-10-12", end: "2026-10-18", revenue: 0, open, close, low, high, range: [low, high], days: 7, partial: false,
});

function draw(b: TrendBucket) {
  // Batang rentang recharts: y = piksel nilai tertinggi, height = rentang sampai terendah.
  const { container } = render(<svg><Candle x={10} y={20} width={8} height={100} payload={b} /></svg>);
  return container.querySelector("g[data-candle]") as SVGGElement;
}

describe("Candle", () => {
  it("naik: badan dari tutup (atas) ke buka, warna success, sumbu sepanjang rentang", () => {
    const g = draw(bucket(20, 80, 0, 100));
    expect(g.dataset.candle).toBe("up");
    const line = g.querySelector("line")!;
    expect([line.getAttribute("y1"), line.getAttribute("y2")]).toEqual(["20", "120"]);
    const rect = g.querySelector("rect")!;
    expect(rect.getAttribute("y")).toBe("40"); // 80 dari 100 → 20 piksel di bawah puncak
    expect(rect.getAttribute("height")).toBe("60"); // 80 → 20
    expect(rect.getAttribute("stroke")).toBe("var(--success)");
  });
  it("turun: warna destructive; rentang datar tetap tergambar", () => {
    expect(draw(bucket(80, 20, 0, 100)).dataset.candle).toBe("down");
    const flat = draw(bucket(50, 50, 50, 50));
    expect(flat.querySelector("rect")!.getAttribute("height")).toBe("1");
  });
});
