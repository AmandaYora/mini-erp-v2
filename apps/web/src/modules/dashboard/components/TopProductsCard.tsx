import { useState } from "react";
import { Badge, DataTable, SectionCard, Tabs, TabsList, TabsTrigger, TabsContent } from "@/shared/components/ui";
import { Money, Qty } from "@/shared/components/patterns";
import { formatNumber } from "@/shared/lib/format";
import type { TopMarginProduct, TopSeller } from "@/modules/dashboard/types";

interface Source<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

type TabKey = "sellers" | "margin";

const DESCRIPTION: Record<TabKey, string> = {
  sellers: "5 produk dengan qty basis terbesar bulan ini; order batal dikecualikan.",
  margin: "5 produk dengan laba kotor terbesar bulan ini menurut buku.",
};

function ProductCell({ code, name }: { code: string; name: string }) {
  return (
    <span>
      <span className="font-semibold">{code}</span> <span className="text-muted-foreground">{name}</span>
    </span>
  );
}

/** Tabs "Terlaris | Margin tertinggi": dua daftar 5 produk bulan berjalan dalam satu kartu. */
export function TopProductsCard({ sellers, margin }: { sellers: Source<TopSeller[]>; margin: Source<TopMarginProduct[]> }) {
  const [tab, setTab] = useState<TabKey>("sellers");
  return (
    <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
      <SectionCard
        title="Produk bulan ini"
        description={DESCRIPTION[tab]}
        flush
        actions={
          <TabsList aria-label="Peringkat produk">
            <TabsTrigger value="sellers">Terlaris</TabsTrigger>
            <TabsTrigger value="margin">Margin tertinggi</TabsTrigger>
          </TabsList>
        }
      >
        <TabsContent value="sellers">
          <DataTable<TopSeller>
            rows={sellers.data}
            rowKey={(r) => `${r.productId}/${r.variantId}`}
            loading={sellers.loading}
            error={sellers.error}
            onRetry={sellers.reload}
            emptyTitle="Belum ada penjualan"
            emptyDescription="Belum ada order bulan berjalan."
            columns={[
              { header: "Produk", render: (r) => <ProductCell code={r.productCode} name={r.productName} /> },
              { header: "Terjual", align: "right", render: (r) => <Qty value={r.qtyBase} /> },
              { header: "Omzet", align: "right", render: (r) => <Money value={r.revenue} /> },
            ]}
          />
        </TabsContent>
        <TabsContent value="margin">
          <DataTable<TopMarginProduct>
            rows={margin.data}
            rowKey={(r) => `${r.productId}/${r.variantId}`}
            loading={margin.loading}
            error={margin.error}
            onRetry={margin.reload}
            emptyTitle="Belum ada margin"
            emptyDescription="Belum ada penjualan terposting bulan berjalan."
            columns={[
              { header: "Produk", render: (r) => <ProductCell code={r.productCode} name={r.productName} /> },
              {
                header: "Laba",
                align: "right",
                render: (r) => (
                  <span className="inline-flex items-center justify-end gap-2">
                    {r.costIncomplete && <Badge tone="warning">Estimasi</Badge>}
                    <Badge tone={r.gross > 0 ? "success" : "neutral"}>{`${formatNumber(Math.round(r.percent * 10) / 10)}%`}</Badge>
                    <Money value={r.gross} />
                  </span>
                ),
              },
              { header: "Omzet", align: "right", render: (r) => <Money value={r.revenue} /> },
            ]}
          />
        </TabsContent>
      </SectionCard>
    </Tabs>
  );
}
