import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  CardSkeleton,
  DataTable,
  ErrorState,
  PageHeader,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { DocNumber, Page } from "@/shared/components/patterns";
import { formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { opsService } from "@/modules/ops/services/ops.service";
import { groupByDocType, issueActionHint, issueDocLabel, issueDocPath } from "@/modules/ops/lib/integrity";
import type { IntegrityIssue } from "@/modules/ops/types";

// Konsol operasi (T7): dokumen yang tertinggal setengah jalan oleh alur bertahap yang gagal.
// Hanya baca — operator mengulang atau membatalkan di dokumennya.
export default function OpsIntegrityPage() {
  const navigate = useNavigate();
  const { data, loading, error, reload } = useAsyncData((signal) => opsService.integrity(signal), []);
  const [docFilter, setDocFilter] = useState<string | null>(null);

  const issues = useMemo(() => data?.issues ?? [], [data]);
  const groups = useMemo(() => [...groupByDocType(issues).entries()], [issues]);
  const total = data?.count ?? issues.length;
  // Filter yang sudah tak punya temuan (setelah periksa ulang) diabaikan.
  const activeFilter = docFilter && groups.some(([t]) => t === docFilter) ? docFilter : null;
  const rows = activeFilter ? issues.filter((i) => i.docType === activeFilter) : issues;

  return (
    <Page>
      <PageHeader
        title="Konsistensi Dokumen"
        description="Dokumen setengah jadi akibat alur bertahap yang gagal (pengiriman, penerimaan, retur) di cabang aktif."
        actions={
          <Button variant="secondary" onClick={() => reload()} loading={loading}>
            Periksa ulang
          </Button>
        }
      />

      {error ? (
        <ErrorState title="Gagal memuat laporan konsistensi" message={error} onRetry={reload} />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {!data ? (
              Array.from({ length: 4 }, (_, i) => <CardSkeleton key={i} />)
            ) : (
              <>
                <SummaryCard
                  label="Status"
                  value={total === 0 ? "Konsisten" : `${formatNumber(total)} perlu tindakan`}
                  tone={total === 0 ? "success" : "warning"}
                  hint={
                    total === 0
                      ? "Tidak ada dokumen setengah jadi."
                      : "Buka dokumennya lalu ulangi atau batalkan sesuai petunjuk."
                  }
                  onClick={total > 0 ? () => setDocFilter(null) : undefined}
                />
                {groups.map(([docType, list]) => (
                  <SummaryCard
                    key={docType}
                    label={issueDocLabel(docType)}
                    value={formatNumber(list.length)}
                    tone="warning"
                    hint={activeFilter === docType ? "Disaring — klik untuk batal" : "Klik untuk menyaring temuan"}
                    onClick={() => setDocFilter(activeFilter === docType ? null : docType)}
                  />
                ))}
              </>
            )}
          </div>

          <SectionCard title="Temuan" description="Dokumen yang tertinggal setengah jalan." flush>
            <DataTable<IntegrityIssue>
              caption="Temuan konsistensi dokumen"
              rows={rows}
              rowKey={(r) => `${r.docType}:${r.docId}:${r.kind}`}
              loading={loading}
              onRowClick={(r) => {
                const path = issueDocPath(r.docType, r.docId);
                if (path) navigate(path);
              }}
              emptyTitle="Semua dokumen konsisten"
              emptyDescription="Tidak ada dokumen setengah jadi di cabang ini."
              columns={[
                {
                  header: "Dokumen",
                  render: (r) => {
                    const path = issueDocPath(r.docType, r.docId);
                    return (
                      <span>
                        {path ? (
                          <Link to={path} className="font-semibold text-primary hover:underline">
                            <DocNumber value={r.number} />
                          </Link>
                        ) : (
                          <DocNumber value={r.number} className="font-semibold" />
                        )}{" "}
                        <span className="text-muted-foreground">{issueDocLabel(r.docType)}</span>
                      </span>
                    );
                  },
                },
                { header: "Jenis", render: (r) => <Badge tone="warning">{r.kind}</Badge> },
                { header: "Keterangan", render: (r) => r.detail },
                { header: "Tindakan", render: (r) => issueActionHint(r.kind) },
              ]}
            />
          </SectionCard>
        </>
      )}
    </Page>
  );
}
