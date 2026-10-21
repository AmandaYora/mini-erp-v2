import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  DateInput,
  DetailSkeleton,
  FormField,
  PageHeader,
  SectionCard,
  StatusBadge,
  TextArea,
} from "@/shared/components/ui";
import { DescriptionList, DocNumber, Money, Page, Stepper, StickyFooter } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { useJournalDraft } from "@/modules/finance/hooks/use-journal-draft";
import { draftDifference, draftTotals, isDraftBalanced, isEntryBalanced, parseJournalDraft } from "@/modules/finance/lib/journal-lines";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { JournalLinesEditor } from "@/modules/finance/components/JournalLinesEditor";
import { BalanceBadge, JournalLinesTable } from "@/modules/finance/components/JournalLinesTable";
import { MissingCostCard, OpeningInventorySummary } from "@/modules/finance/components/OpeningInventoryCard";
import type { Account, OpeningInventory, OpeningPreview } from "@/modules/finance/types";

const STEPS = [
  { key: "draft", label: "Susun baris" },
  { key: "preview", label: "Tinjau" },
  { key: "done", label: "Selesai" },
];

// Saldo awal (T9, wizard): satu jurnal cutover per cabang (tanpa tabel saldo awal).
// Susun baris → Tinjau (pratinjau memvalidasi seimbang tanpa menulis) → Posting final dan idempoten.
// Cabang yang sudah cutover langsung melihat jurnalnya.
export default function OpeningPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeService.openingStatus(),
        financeService.accounts("active").catch(() => [] as Account[]),
        // Persediaan dihitung server dari stok awal x harga modal (K-05).
        financeService.openingInventory().catch(() => null as OpeningInventory | null),
      ]).then(([st, acc, inv]) => ({
        status: st,
        accounts: acc ?? [],
        inventory: inv,
      })),
    [],
  );
  const status = data?.status ?? null;
  const accounts = data?.accounts ?? [];
  const inventory = data?.inventory ?? null;

  const draft = useJournalDraft();
  const [date, setDate] = useState(todayWIB());
  const [memo, setMemo] = useState("Saldo awal");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [preview, setPreview] = useState<OpeningPreview | null>(null);
  const [pending, setPending] = useState<ManualJournalValues | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [completing, setCompleting] = useState(false);

  const totals = draftTotals(draft.lines);
  const diff = draftDifference(draft.lines);
  // Hanya persediaan (tanpa saldo akun lain): semua baris kosong, jurnal murni dari stok awal.
  const stockOnly = (inventory?.total ?? 0) > 0 && draft.lines.every((l) => !l.accountCode && !l.debit.trim() && !l.credit.trim());
  const balanced = isDraftBalanced(draft.lines) || stockOnly;

  async function handlePreview() {
    const parsed: ReturnType<typeof parseJournalDraft> = stockOnly
      ? {
          ok: true,
          values: { date: date.trim() || undefined, memo: memo.trim() || "Saldo awal", lines: [] } as unknown as ManualJournalValues,
        }
      : parseJournalDraft({ date, memo, lines: draft.lines });
    if (!parsed.ok) {
      setErrors(parsed.errors);
      return;
    }
    setErrors({});
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.previewOpening({
        date: parsed.values.date,
        memo: parsed.values.memo,
        lines: parsed.values.lines,
      });
      setPreview(res.data);
      setPending(parsed.values);
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  async function handlePost() {
    if (!pending) return;
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.postOpening({
        date: pending.date,
        memo: pending.memo,
        lines: pending.lines,
      });
      setPreview(null);
      setPending(null);
      setConfirmOpen(false);
      toast.fromServer(res.message, "Saldo awal diposting", res.data.number);
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      setConfirmOpen(false);
      setServerError(apiErr.message);
      toast.danger("Gagal memposting saldo awal", apiErr.message);
    } finally {
      setSubmitting(false);
    }
  }

  // Lengkapi harga modal produk berstok setelah saldo awal dikunci (jurnal suplemen Persediaan / Modal Awal).
  async function handleComplete() {
    if (!inventory) return;
    const items = inventory.missingCost.filter((i) => i.suggestedCost > 0).map((i) => ({ productId: i.productId, variantId: i.variantId }));
    if (items.length === 0) return;
    setCompleting(true);
    try {
      const res = await financeService.completeOpening({ items });
      toast.fromServer(res.message, "Harga modal dilengkapi", res.data.number);
      reload();
    } catch (err) {
      toast.danger("Gagal melengkapi harga modal", formatApiError(err));
    } finally {
      setCompleting(false);
    }
  }

  const step = status ? "done" : preview ? "preview" : "draft";

  return (
    <Page>
      <PageHeader
        title="Saldo Awal"
        description="Jurnal cutover satu kali per cabang — tanpa tabel saldo awal, buku langsung membacanya."
        meta={status ? <StatusBadge status="posted" /> : undefined}
      >
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          <Stepper steps={STEPS} current={step} />
        </div>
      </PageHeader>

      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal memuat saldo awal" />
      ) : loading && !data ? (
        <DetailSkeleton />
      ) : status ? (
        <>
          <SectionCard title="Sudah cutover" description="Cabang ini sudah memposting saldo awal — posting bersifat final dan idempoten.">
            <DescriptionList
              columns={3}
              items={[
                {
                  label: "Nomor jurnal",
                  value: (
                    <Link to={`${ROUTE_PATHS.financeJournals}/${status.id}`} className="font-medium text-primary hover:underline">
                      <DocNumber value={status.number} />
                    </Link>
                  ),
                },
                {
                  label: "Tanggal",
                  value: formatDate(status.date || todayWIB()),
                },
                { label: "Memo", value: status.memo },
              ]}
            />
          </SectionCard>
          {inventory && inventory.missingCost.length > 0 && (
            <MissingCostCard
              items={inventory.missingCost}
              canManage={canManage}
              busy={completing}
              onComplete={() => void handleComplete()}
            />
          )}
        </>
      ) : !canManage ? (
        <Notice tone="info" title="Saldo awal belum diposting">
          Cabang ini belum cutover. Hanya pengguna berizin kelola keuangan yang dapat menyusun dan memposting saldo awal.
        </Notice>
      ) : preview ? (
        <>
          {serverError && (
            <Notice tone="danger" title="Saldo awal gagal diposting">
              {serverError}
            </Notice>
          )}
          {(preview.inventory ?? inventory) && <OpeningInventorySummary inventory={(preview.inventory ?? inventory) as OpeningInventory} />}
          <SectionCard
            flush
            title="Tinjau saldo awal"
            description={`${preview.memo || "-"} · ${formatDate(preview.date || todayWIB())}`}
            actions={<BalanceBadge balanced={isEntryBalanced(preview.lines)} />}
          >
            <JournalLinesTable lines={preview.lines} />
          </SectionCard>
          <StickyFooter summary="Posting bersifat final: setelah diposting, saldo awal tidak dapat disusun ulang.">
            <Button
              variant="secondary"
              disabled={submitting}
              onClick={() => {
                setPreview(null);
                setPending(null);
              }}
            >
              Ubah baris
            </Button>
            <Button disabled={!isEntryBalanced(preview.lines)} onClick={() => setConfirmOpen(true)}>
              Posting saldo awal
            </Button>
          </StickyFooter>
        </>
      ) : (
        <>
          <Notice tone="info" title="Belum cutover">
            Susun baris saldo awal per akun, tinjau validasinya, lalu posting satu kali. Total debit harus sama dengan kredit dan lebih dari
            0 — pratinjau menolak yang pincang sebelum apa pun tertulis ke buku.
          </Notice>
          {serverError && (
            <Notice tone="danger" title="Pratinjau gagal">
              {serverError}
            </Notice>
          )}
          {inventory && <OpeningInventorySummary inventory={inventory} />}
          <SectionCard title="Dokumen">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <FormField label="Tanggal" htmlFor="opening-date" helperText="Kosongkan = hari ini">
                <DateInput id="opening-date" value={date} onChange={(e) => setDate(e.target.value)} />
              </FormField>
              <div className="sm:col-span-2">
                <FormField label="Memo" htmlFor="opening-memo" required errorText={errors.memo}>
                  <TextArea id="opening-memo" value={memo} onChange={(e) => setMemo(e.target.value)} rows={2} />
                </FormField>
              </div>
            </div>
          </SectionCard>
          <SectionCard
            title="Baris saldo awal"
            description="Tiap baris hanya satu sisi: debit atau kredit. Akun Persediaan jangan diisi — ditambahkan otomatis dari stok awal, lengkap dengan Modal Awal penyeimbangnya; baris yang Anda isi harus seimbang sendiri."
          >
            {errors.lines && <p className="mb-3 text-sm font-medium text-destructive">{errors.lines}</p>}
            <JournalLinesEditor draft={draft} accounts={accounts} errors={errors} />
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
                <span>
                  Selisih D−K{" "}
                  <Money value={diff} className={diff === 0 ? "font-medium text-foreground" : "font-semibold text-destructive"} />
                </span>
              </span>
            }
          >
            <Button loading={submitting} disabled={!balanced} onClick={() => void handlePreview()}>
              Tinjau saldo awal
            </Button>
          </StickyFooter>
        </>
      )}

      <ConfirmDialog
        open={confirmOpen}
        title="Posting saldo awal?"
        message={`Jurnal cutover ${preview ? `dengan ${preview.lines?.length ?? 0} baris` : ""} akan diposting untuk cabang ini. Tindakan ini final dan tidak dapat diulang.`}
        confirmLabel={submitting ? "Memposting…" : "Posting"}
        tone="primary"
        onConfirm={() => void handlePost()}
        onCancel={() => (submitting ? undefined : setConfirmOpen(false))}
      />
    </Page>
  );
}
