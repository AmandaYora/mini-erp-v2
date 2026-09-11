import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  DateInput,
  FilterBar,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import type { Account, LedgerLine } from "@/modules/finance/types";

function isOpening(row: LedgerLine): boolean {
  return !row.entryId && row.memo === "Saldo awal";
}

export default function GeneralLedgerPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [account, setAccount] = useState<string | number | null>(null);
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const [lines, setLines] = useState<LedgerLine[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    financeReportsService
      .accounts()
      .then((list) => {
        if (!cancelled) setAccounts(list);
      })
      .catch((err) => {
        if (!cancelled)
          toast.danger("Gagal memuat daftar akun", toApiError(err).message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const load = useCallback(async () => {
    if (account === null || account === undefined || account === "") {
      toast.warning("Pilih akun terlebih dahulu");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const rows = await financeReportsService.generalLedger(
        String(account),
        from,
        to,
      );
      setLines(rows);
      setLoaded(true);
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal memuat buku besar", apiErr.message);
    } finally {
      setLoading(false);
    }
  }, [account, from, to]);

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    void load();
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Buku Besar"
        description="Mutasi satu akun beserta saldo berjalan — hanya baca."
      />

      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-64 flex-1">
            <FormField label="Akun" required>
              <SearchSelect
                options={accounts.map((a) => ({
                  value: a.code,
                  label: `${a.code} — ${a.name}`,
                }))}
                value={account}
                onChange={(v) => setAccount(v)}
                placeholder="Pilih akun…"
              />
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Dari">
              <DateInput
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Sampai">
              <DateInput value={to} onChange={(e) => setTo(e.target.value)} />
            </FormField>
          </div>
          <Button type="submit" variant="secondary" disabled={loading}>
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => void load()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title="Mutasi Akun"
        description={
          loaded
            ? `${formatDate(from)} – ${formatDate(to)}`
            : "Pilih akun lalu tekan Tampilkan."
        }
      >
        {loading && !loaded ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <DataTable<LedgerLine>
            rows={loaded ? lines : []}
            rowKey={(r) =>
              r.entryId
                ? `line-${r.entryId}-${r.debit}-${r.credit}`
                : "saldo-awal"
            }
            emptyTitle={loaded ? "Belum ada mutasi" : "Belum ada data"}
            emptyDescription={
              loaded
                ? "Tidak ada jurnal untuk akun ini pada rentang tersebut."
                : "Pilih akun dan rentang tanggal terlebih dahulu."
            }
            columns={[
              { header: "Tanggal", render: (r) => formatDate(r.date) },
              {
                header: "Nomor",
                render: (r) => (r.number ? r.number : "-"),
              },
              {
                header: "Keterangan",
                render: (r) =>
                  isOpening(r) ? <strong>{r.memo}</strong> : (r.memo || "-"),
              },
              {
                header: "Debit",
                align: "right",
                render: (r) => formatIDR(r.debit),
              },
              {
                header: "Kredit",
                align: "right",
                render: (r) => formatIDR(r.credit),
              },
              {
                header: "Saldo",
                align: "right",
                render: (r) => (
                  <span className="font-medium">{formatIDR(r.balance)}</span>
                ),
              },
            ]}
          />
        )}
      </SectionCard>
    </div>
  );
}
