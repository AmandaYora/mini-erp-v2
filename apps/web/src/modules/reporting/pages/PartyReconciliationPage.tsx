import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  DataTable,
  ErrorState,
  FormField,
  PageHeader,
  SectionCard,
  SummaryCard,
  ToggleGroup,
  ToggleGroupItem,
} from "@/shared/components/ui";
import { Page, Toolbar } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { reportingService } from "@/modules/reporting/services/reporting.service";
import type { PartyReconRow, PartyReconSide } from "@/modules/reporting/types";

const SIDE_LABEL: Record<PartyReconSide, { books: string; party: string; path: string }> = {
  receivable: { books: "Piutang + Uang Muka Pelanggan", party: "Pelanggan", path: "/customers" },
  payable: { books: "Hutang + Uang Muka Supplier", party: "Pemasok", path: "/suppliers" },
};

// Rekonsiliasi buku vs dokumen per pihak (ADR-0023): alat pantau salah input.
// Pihak tanpa order berjalan wajib sama persis; selisihnya adalah kesalahan
// yang perlu diperbaiki lewat web (koreksi pembayaran, jurnal manual berpihak,
// pembatalan dokumen yang tertinggal jurnalnya).
export default function PartyReconciliationPage() {
  const [side, setSide] = useState<PartyReconSide>("receivable");
  const { data, loading, error, reload } = useAsyncData(
    (signal) => reportingService.partyReconciliation(side, signal),
    [side],
  );
  const labels = SIDE_LABEL[side];

  return (
    <Page>
      <PageHeader
        title="Rekonsiliasi Pihak"
        description="Membandingkan saldo buku besar (akun kendali) dengan saldo dokumen tiap pihak. Pihak tanpa order berjalan harus sama persis."
      />

      <Toolbar
        primary={
          <FormField label="Sisi">
            <ToggleGroup
              type="single"
              variant="outline"
              value={side}
              onValueChange={(v) => v && setSide(v as PartyReconSide)}
              aria-label="Sisi"
            >
              <ToggleGroupItem value="receivable">Piutang pelanggan</ToggleGroupItem>
              <ToggleGroupItem value="payable">Hutang pemasok</ToggleGroupItem>
            </ToggleGroup>
          </FormField>
        }
      />

      {error ? (
        <ErrorState title="Gagal memuat rekonsiliasi" message={error} onRetry={reload} />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <SummaryCard label={`${labels.party} diperiksa`} value={data ? formatNumber(data.partiesChecked) : "-"} tone="brand" />
            <SummaryCard label="Cocok" value={data ? formatNumber(data.matched) : "-"} tone="success" />
            <SummaryCard
              label="Selisih tak terjelaskan"
              value={data ? formatNumber(data.unexplained) : "-"}
              tone={data && data.unexplained > 0 ? "danger" : "success"}
            />
          </div>
          {data && data.unexplained > 0 && (
            <Notice tone="warning" title="Ada selisih yang harus ditelusuri">
              Pihak tanpa order berjalan seharusnya cocok persis. Periksa dulu dua penyebab wajar: dokumen yang belum dijurnal (mis.
              surat jalan bertanggal ke depan, cek antrean posting) dan barang pengganti retur tukar yang belum dikirim. Selain itu,
              buka pihaknya, cocokkan dokumen dengan jurnal, lalu perbaiki lewat web: koreksi pembayaran untuk pembayaran yang salah,
              batalkan dokumen yang salah, atau jurnal manual dengan memilih pihak untuk selisih saldo.
            </Notice>
          )}
          <SectionCard
            title="Pihak dengan selisih"
            description={`Buku = ${labels.books}. Dokumen = order − pembayaran − retur + refund. Selisih = buku − dokumen.`}
            flush
          >
            <DataTable<PartyReconRow>
              rows={data?.rows}
              rowKey={(r) => r.partyId}
              loading={loading}
              emptyTitle="Semua cocok"
              emptyDescription="Buku dan dokumen sama untuk setiap pihak."
              columns={[
                {
                  header: labels.party,
                  render: (r) => (
                    <Link to={`${labels.path}/${r.partyId}`} className="font-semibold text-primary hover:underline">
                      {r.partyName}
                    </Link>
                  ),
                },
                { header: "Buku", align: "right", render: (r) => formatIDR(r.books) },
                { header: "Dokumen", align: "right", render: (r) => formatIDR(r.documents) },
                { header: "Selisih", align: "right", render: (r) => <span className="font-semibold">{formatIDR(r.difference)}</span> },
                {
                  header: "Keterangan",
                  render: (r) =>
                    r.timing ? (
                      <Badge tone="neutral">{`${r.openOrders} order berjalan — bisa karena waktu`}</Badge>
                    ) : (
                      <Badge tone="danger">Perlu ditelusuri</Badge>
                    ),
                },
              ]}
            />
          </SectionCard>
        </>
      )}
    </Page>
  );
}
