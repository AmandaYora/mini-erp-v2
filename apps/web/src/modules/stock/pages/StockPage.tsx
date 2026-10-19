import { useMemo } from "react";
import { PageHeader, Tabs, TabsContent, TabsList, TabsTrigger } from "@/shared/components/ui";
import { Page, useUrlTab } from "@/shared/components/patterns";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { StockBalanceTab } from "@/modules/stock/components/StockBalanceTab";
import { StockLocationTab } from "@/modules/stock/components/StockLocationTab";
import { StockMovementTab } from "@/modules/stock/components/StockMovementTab";
import type { StockLocation } from "@/modules/stock/types";

const TABS = ["saldo", "mutasi", "lokasi"] as const;

export default function StockPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.update");
  const [tab, setTab] = useUrlTab(TABS, "saldo");

  // Daftar lokasi dipakai ketiga tab (nama lokasi di tabel + filter).
  const { data, loading, error, reload } = useAsyncData(() => stockService.listLocations(), []);
  const locations: StockLocation[] = useMemo(() => data ?? [], [data]);

  return (
    <Page>
      <PageHeader
        title="Stok"
        description="Saldo per produk, riwayat mutasi, dan daftar lokasi penyimpanan."
      />
      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-4">
        <TabsList variant="line" aria-label="Bagian stok">
          <TabsTrigger value="saldo">Saldo</TabsTrigger>
          <TabsTrigger value="mutasi">Mutasi</TabsTrigger>
          <TabsTrigger value="lokasi">Lokasi</TabsTrigger>
        </TabsList>
        <TabsContent value="saldo">
          <StockBalanceTab locations={locations} />
        </TabsContent>
        <TabsContent value="mutasi">
          <StockMovementTab locations={locations} />
        </TabsContent>
        <TabsContent value="lokasi">
          <StockLocationTab
            locations={locations}
            loading={loading}
            error={error}
            onReload={reload}
            canManage={canManage}
          />
        </TabsContent>
      </Tabs>
    </Page>
  );
}
