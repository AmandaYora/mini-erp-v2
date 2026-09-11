import { useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  PageHeader,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { financeService } from "@/modules/finance/services/finance.service";
import { periodSchema } from "@/modules/finance/schemas/finance.schema";
import { periodLabel } from "@/modules/finance/types";
import type { FiscalPeriod, PeriodReadiness } from "@/modules/finance/types";

interface PendingAction {
  kind: "close" | "reopen" | "safeclose";
  scope: "fiscal" | "tax";
  year: number;
  month: number;
}

// Periode fiskal + periode pajak. Tutup/kunci bulan via ConfirmDialog →
// POST /periods/close|/reopen dan /tax-periods/close|/reopen {year, month}
// (gate finance.manage). List hanya berisi bulan yang pernah tersentuh.
export default function PeriodsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([financeService.periods(), financeService.taxPeriods()])
        .then(([p, t]) => ({ periods: p ?? [], taxPeriods: t ?? [] }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat periode", apiErr.message);
          throw err;
        }),
    [],
  );
  const periods = data?.periods ?? [];
  const taxPeriods = data?.taxPeriods ?? [];

  const [yearInput, setYearInput] = useState(todayWIB().slice(0, 4));
  const [monthInput, setMonthInput] = useState(
    String(Number(todayWIB().slice(5, 7))),
  );
  const [formError, setFormError] = useState<string | undefined>(undefined);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [readiness, setReadiness] = useState<PeriodReadiness | null>(null);
  const [readinessLoading, setReadinessLoading] = useState(false);

  function parsePeriodForm(): { year: number; month: number } | null {
    const parsed = periodSchema.safeParse({
      year: yearInput,
      month: monthInput,
    });
    if (!parsed.success) {
      const flat = parsed.error.flatten().fieldErrors;
      setFormError(flat.year?.[0] ?? flat.month?.[0] ?? "Periode tidak valid");
      return null;
    }
    setFormError(undefined);
    return parsed.data;
  }

  function askClose(e: FormEvent<HTMLFormElement>, scope: "fiscal" | "tax") {
    e.preventDefault();
    const p = parsePeriodForm();
    if (!p) return;
    setPending({ kind: "close", scope, year: p.year, month: p.month });
  }

  async function handleConfirm() {
    if (!pending) return;
    const body = { year: pending.year, month: pending.month };
    try {
      if (pending.kind === "safeclose") {
        const res = await financeService.safeClose(body);
        toast.fromServer(
          res.message,
          "Periode ditutup aman",
          periodLabel(pending.year, pending.month),
        );
      } else if (pending.scope === "fiscal") {
        if (pending.kind === "close") {
          const res = await financeService.closePeriod(body);
          toast.fromServer(
            res.message,
            "Periode ditutup",
            periodLabel(pending.year, pending.month),
          );
        } else {
          const res = await financeService.reopenPeriod(body);
          toast.fromServer(
            res.message,
            "Periode dibuka",
            periodLabel(pending.year, pending.month),
          );
        }
      } else {
        if (pending.kind === "close") {
          const res = await financeService.closeTaxPeriod(body);
          toast.fromServer(
            res.message,
            "Periode ditutup",
            periodLabel(pending.year, pending.month),
          );
        } else {
          const res = await financeService.reopenTaxPeriod(body);
          toast.fromServer(
            res.message,
            "Periode dibuka",
            periodLabel(pending.year, pending.month),
          );
        }
      }
      setPending(null);
      setReadiness(null);
      reload();
    } catch (err) {
      toast.danger("Gagal memproses periode", toApiError(err).message);
    }
  }

  async function checkReadiness() {
    const p = parsePeriodForm();
    if (!p) return;
    setReadinessLoading(true);
    try {
      const res = await financeService.readiness(p.year, p.month);
      setReadiness(res);
      if (res.ready) {
        setPending({ kind: "safeclose", scope: "fiscal", year: p.year, month: p.month });
      }
    } catch (err) {
      toast.danger("Gagal memeriksa kesiapan", toApiError(err).message);
    } finally {
      setReadinessLoading(false);
    }
  }

  function periodTable(
    rows: FiscalPeriod[],
    scope: "fiscal" | "tax",
    emptyTitle: string,
  ) {
    return (
      <DataTable<FiscalPeriod>
        rows={rows}
        rowKey={(r) => `${r.year}-${r.month}`}
        emptyTitle={emptyTitle}
        emptyDescription="Tutup bulan berjalan lewat form di atas untuk mengunci pembukuan."
        columns={[
          {
            header: "Periode",
            render: (r) => periodLabel(r.year, r.month),
          },
          {
            header: "Status",
            render: (r) => (
              <Badge tone={statusTone(r.status)}>
                {statusLabel(r.status)}
              </Badge>
            ),
          },
          {
            header: "Aksi",
            align: "right",
            render: (r) =>
              canManage ? (
                r.status === "closed" ? (
                  <button
                    type="button"
                    onClick={() =>
                      setPending({
                        kind: "reopen",
                        scope,
                        year: r.year,
                        month: r.month,
                      })
                    }
                    className="cursor-pointer font-medium text-brand hover:underline"
                  >
                    Buka Ulang
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() =>
                      setPending({
                        kind: "close",
                        scope,
                        year: r.year,
                        month: r.month,
                      })
                    }
                    className="cursor-pointer font-medium text-bad hover:underline"
                  >
                    Tutup
                  </button>
                )
              ) : (
                <span className="text-muted">-</span>
              ),
          },
        ]}
      />
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Periode"
        description="Kunci bulan fiskal dan pajak agar jurnal tak bisa ditambah ke bulan tertutup."
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      {loading && periods.length === 0 && taxPeriods.length === 0 ? (
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      ) : (
        <>
          <SectionCard
            title="Periode Fiskal"
            description="Bulan tertutup menolak jurnal baru (termasuk posting, manual, biaya)."
          >
            {canManage && (
              <form
                onSubmit={(e) => askClose(e, "fiscal")}
                className="mb-4 flex flex-wrap items-end gap-4"
              >
                <div className="min-w-32">
                  <FormField label="Tahun" errorText={formError}>
                    <TextInput
                      value={yearInput}
                      onChange={(e) => setYearInput(e.target.value)}
                      placeholder="2026"
                      inputMode="numeric"
                    />
                  </FormField>
                </div>
                <div className="min-w-32">
                  <FormField label="Bulan (1–12)">
                    <TextInput
                      value={monthInput}
                      onChange={(e) => setMonthInput(e.target.value)}
                      placeholder="9"
                      inputMode="numeric"
                    />
                  </FormField>
                </div>
                <Button type="submit" variant="secondary">
                  Tutup Periode
                </Button>
                <Button
                  type="button"
                  disabled={readinessLoading}
                  onClick={() => void checkReadiness()}
                >
                  {readinessLoading ? "Memeriksa…" : "Cek & Tutup Aman"}
                </Button>
              </form>
            )}
            {readiness && (
              <div className="mb-4">
                <Notice
                  tone={readiness.ready ? "success" : "warning"}
                  title={
                    readiness.ready
                      ? "Siap ditutup — konfirmasi di dialog"
                      : "Belum siap ditutup"
                  }
                >
                  {!readiness.ready && (
                    <ul className="list-disc pl-5">
                      {readiness.blockers.map((b) => (
                        <li key={b}>{b}</li>
                      ))}
                    </ul>
                  )}
                  {readiness.ready && (
                    <span>
                      Antrean kosong, jurnal seimbang, faktur pajak lengkap.
                    </span>
                  )}
                </Notice>
              </div>
            )}
            {periodTable(periods, "fiscal", "Belum ada periode fiskal")}
          </SectionCard>

          <div className="mt-4">
            <SectionCard
              title="Periode Pajak"
              description="Mengunci snapshot ringkasan pajak bulan tersebut."
            >
              {canManage && (
                <form
                  onSubmit={(e) => askClose(e, "tax")}
                  className="mb-4 flex flex-wrap items-end gap-4"
                >
                  <div className="min-w-32">
                    <FormField label="Tahun">
                      <TextInput
                        value={yearInput}
                        onChange={(e) => setYearInput(e.target.value)}
                        placeholder="2026"
                        inputMode="numeric"
                      />
                    </FormField>
                  </div>
                  <div className="min-w-32">
                    <FormField label="Bulan (1–12)">
                      <TextInput
                        value={monthInput}
                        onChange={(e) => setMonthInput(e.target.value)}
                        placeholder="9"
                        inputMode="numeric"
                      />
                    </FormField>
                  </div>
                  <Button type="submit" variant="secondary">
                    Tutup Periode Pajak
                  </Button>
                </form>
              )}
              {periodTable(taxPeriods, "tax", "Belum ada periode pajak")}
            </SectionCard>
          </div>
        </>
      )}

      <ConfirmDialog
        open={pending !== null}
        title={
          pending?.kind === "safeclose"
            ? "Tutup Aman Periode?"
            : pending?.kind === "close"
              ? "Tutup Periode?"
              : "Buka Ulang Periode?"
        }
        message={
          pending?.kind === "safeclose"
            ? `Periode ${pending ? periodLabel(pending.year, pending.month) : ""} lolos semua gerbang (antrean kosong, jurnal seimbang, faktur lengkap) dan akan dikunci. Lanjutkan?`
            : pending?.kind === "close"
              ? `Periode ${pending ? periodLabel(pending.year, pending.month) : ""} (${pending?.scope === "tax" ? "pajak" : "fiskal"}) akan dikunci. Jurnal baru ke bulan ini akan ditolak. Lanjutkan?`
              : `Periode ${pending ? periodLabel(pending.year, pending.month) : ""} (${pending?.scope === "tax" ? "pajak" : "fiskal"}) akan dibuka kembali. Lanjutkan?`
        }
        confirmLabel={pending?.kind === "reopen" ? "Buka Ulang" : "Tutup"}
        tone={pending?.kind === "reopen" ? "primary" : "danger"}
        onConfirm={() => void handleConfirm()}
        onCancel={() => setPending(null)}
      />
    </div>
  );
}
