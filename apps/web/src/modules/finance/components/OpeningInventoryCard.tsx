import { Button, DataTable, SectionCard, SummaryCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import type { MissingCostItem, OpeningInventory } from "@/modules/finance/types";

/**
 * Persediaan saldo awal yang dihitung server (stok awal x harga modal, dari ledger biaya).
 * Barisnya tidak bisa diketik bebas: jurnal cutover memuatnya otomatis berikut Modal Awal penyeimbangnya.
 */
export function OpeningInventorySummary({ inventory }: { inventory: OpeningInventory }) {
  return (
    <SectionCard
      title="Persediaan terhitung otomatis"
      description="Dihitung server dari saldo stok awal x harga modal — sama dengan Laporan Nilai Persediaan. Tidak diisi manual."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <SummaryCard
          label="Total persediaan"
          value={formatIDR(inventory.total)}
          hint="Debit akun Persediaan; Modal Awal ditambahkan otomatis sebagai kredit."
        />
        <SummaryCard label="Produk berstok" value={formatNumber(inventory.positions)} />
        <SummaryCard
          label="Harga dari katalog"
          value={formatNumber(inventory.estimated)}
          tone={inventory.estimated > 0 ? "warning" : "success"}
          hint={inventory.estimated > 0 ? "Harga modal diperkirakan dari harga beli katalog." : undefined}
        />
      </div>
      {inventory.missingCost.length > 0 && (
        <div className="mt-4">
          <Notice tone="warning" title={`${formatNumber(inventory.missingCost.length)} produk berstok tanpa harga modal`}>
            Produk ini bernilai nol di persediaan. Isi harga beli di master produk lalu kunci saldo awal, atau lengkapi setelah dikunci.
          </Notice>
        </div>
      )}
    </SectionCard>
  );
}

/** Daftar produk berstok tanpa harga modal + jalur "lengkapi" setelah saldo awal dikunci. */
export function MissingCostCard({
  items,
  canManage,
  busy,
  onComplete,
}: {
  items: MissingCostItem[];
  canManage: boolean;
  busy: boolean;
  onComplete: () => void;
}) {
  const fixable = items.filter((i) => i.suggestedCost > 0).length;
  return (
    <SectionCard
      flush
      title="Produk berstok tanpa harga modal"
      description="Nilai persediaannya nol di buku. Lengkapi harga modal agar persediaan, HPP, dan rekonsiliasi benar."
      actions={
        canManage ? (
          <Button size="sm" loading={busy} disabled={fixable === 0} onClick={onComplete}>
            Lengkapi dari harga beli ({fixable})
          </Button>
        ) : undefined
      }
    >
      <DataTable<MissingCostItem>
        caption="Produk berstok tanpa harga modal"
        rows={items}
        rowKey={(r) => `${r.productId}-${r.variantId}`}
        emptyTitle="Semua produk berstok sudah punya harga modal"
        columns={[
          { header: "Kode", render: (r) => r.productCode || `#${r.productId}` },
          { header: "Produk", render: (r) => r.productName || "-" },
          {
            header: "Stok",
            align: "right",
            render: (r) => formatNumber(r.qty),
          },
          {
            header: "Harga beli katalog",
            align: "right",
            render: (r) =>
              r.suggestedCost > 0 ? <Money value={r.suggestedCost} /> : <span className="text-muted-foreground">Belum diisi</span>,
          },
        ]}
      />
    </SectionCard>
  );
}
