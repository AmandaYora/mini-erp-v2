import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Badge, Button, DateInput, FormField, PageHeader, SectionCard, TextArea } from "@/shared/components/ui";
import { Money, Page, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { todayWIB } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { useJournalDraft } from "@/modules/finance/hooks/use-journal-draft";
import { draftDifference, draftTotals, isDraftBalanced, parseJournalDraft } from "@/modules/finance/lib/journal-lines";
import { JournalLinesEditor } from "@/modules/finance/components/JournalLinesEditor";
import type { Account } from "@/modules/finance/types";

/**
 * Jurnal manual (T3, halaman penuh /finance/journals/new). Akun dipilih menurut kode; jurnal harus
 * seimbang — footer menampilkan selisih debit − kredit (merah bila ≠ 0) dan menonaktifkan Simpan.
 */
export default function JournalFormPage() {
  usePageCrumb("Jurnal manual");
  const navigate = useNavigate();
  const { data: accounts, loading, error, reload } = useAsyncData(
    () => financeService.accounts("active").then((a) => a ?? ([] as Account[])),
    [],
  );

  const draft = useJournalDraft();
  const [date, setDate] = useState(todayWIB());
  const [memo, setMemo] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const totals = draftTotals(draft.lines);
  const diff = draftDifference(draft.lines);
  const balanced = isDraftBalanced(draft.lines);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = parseJournalDraft({ date, memo, lines: draft.lines });
    if (!parsed.ok) {
      setErrors(parsed.errors);
      return;
    }
    setErrors({});
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.manual({
        date: parsed.values.date,
        memo: parsed.values.memo,
        lines: parsed.values.lines.map((l) => ({ accountCode: l.accountCode, debit: l.debit, credit: l.credit })),
      });
      toast.fromServer(res.message, "Jurnal manual dicatat", res.data.number);
      navigate(`${ROUTE_PATHS.financeJournals}/${res.data.id}`);
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Page>
      <PageHeader
        title="Jurnal manual"
        description="Untuk koreksi dan penyesuaian. Jurnal harus seimbang: total debit sama dengan total kredit."
      />
      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal memuat daftar akun" />
      ) : (
        <form onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-6" noValidate>
          {serverError && (
            <Notice tone="danger" title="Jurnal gagal disimpan">
              {serverError}
            </Notice>
          )}
          <SectionCard title="Dokumen">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <FormField label="Tanggal" htmlFor="journal-date" helperText="Kosongkan = hari ini">
                <DateInput id="journal-date" value={date} onChange={(e) => setDate(e.target.value)} />
              </FormField>
              <div className="sm:col-span-2">
                <FormField label="Memo" htmlFor="journal-memo" required errorText={errors.memo}>
                  <TextArea
                    id="journal-memo"
                    value={memo}
                    onChange={(e) => setMemo(e.target.value)}
                    placeholder="cth. Koreksi saldo kas"
                    rows={2}
                  />
                </FormField>
              </div>
            </div>
          </SectionCard>

          <SectionCard title="Baris jurnal" description="Tiap baris hanya satu sisi: debit atau kredit.">
            {errors.lines && <p className="mb-3 text-sm font-medium text-destructive">{errors.lines}</p>}
            {loading && !accounts ? (
              <p role="status" className="text-sm text-muted-foreground">
                Menyiapkan daftar akun…
              </p>
            ) : (
              <JournalLinesEditor draft={draft} accounts={accounts ?? []} errors={errors} />
            )}
          </SectionCard>

          <StickyFooter
            summary={
              <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
                <span>
                  Debit <Money value={totals.debit} className="font-medium text-foreground" />
                </span>
                <span>
                  Kredit <Money value={totals.credit} className="font-medium text-foreground" />
                </span>
                <span className="inline-flex items-center gap-2">
                  Selisih D−K{" "}
                  <Money
                    value={diff}
                    className={diff === 0 ? "font-medium text-foreground" : "font-semibold text-destructive"}
                  />
                  {balanced && <Badge tone="success">Seimbang ✓</Badge>}
                </span>
              </span>
            }
          >
            <Button asChild variant="secondary">
              <Link to={ROUTE_PATHS.financeJournals}>Batal</Link>
            </Button>
            <Button type="submit" loading={submitting} disabled={!balanced}>
              Simpan jurnal
            </Button>
          </StickyFooter>
        </form>
      )}
    </Page>
  );
}
