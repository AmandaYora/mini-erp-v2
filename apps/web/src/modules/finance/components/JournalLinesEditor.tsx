import { SearchSelect, TextInput } from "@/shared/components/ui";
import { LineItemsEditor } from "@/shared/components/patterns";
import type { LineColumn } from "@/shared/components/patterns";
import type { JournalDraft } from "@/modules/finance/hooks/use-journal-draft";
import type { DraftLine } from "@/modules/finance/lib/journal-lines";
import { accountTypeLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

/**
 * Editor baris jurnal (Akun · Debit · Kredit · Jenis) di atas LineItemsEditor — satu pola untuk
 * jurnal manual, saldo awal, dan koreksi fiskal. Akun dipilih berdasarkan KODE (backend mencari
 * menurut kode). "Jenis" adalah info turunan dari akun; API tidak menerima keterangan per baris
 * (memo berlaku untuk seluruh jurnal).
 */
export function JournalLinesEditor({
  draft,
  accounts,
  errors,
  compact = false,
}: {
  draft: JournalDraft;
  accounts: Account[];
  errors: Record<string, string>;
  /** Tanpa kolom "Jenis" (dipakai di panel samping yang sempit). */
  compact?: boolean;
}) {
  const active = accounts.filter((a) => a.status === "active");
  const options = active.map((a) => ({ value: a.code, label: `${a.code} — ${a.name}` }));
  const byCode = new Map(active.map((a) => [a.code, a]));

  const columns: LineColumn<DraftLine>[] = [
    {
      id: "account",
      header: "Akun",
      render: (l, i) => (
        <div role="group" aria-label={`Akun baris ${i + 1}`}>
          <SearchSelect
            options={options}
            value={l.accountCode || null}
            onChange={(v) => draft.update(l.key, { accountCode: typeof v === "string" ? v : "" })}
            placeholder="Pilih akun…"
          />
          {errors[`lines.${i}.accountCode`] && (
            <p className="mt-1 text-left text-xs font-medium text-destructive">{errors[`lines.${i}.accountCode`]}</p>
          )}
        </div>
      ),
    },
    {
      id: "debit",
      header: "Debit (Rp)",
      width: "9rem",
      align: "right",
      render: (l, i) => (
        <TextInput
          aria-label={`Debit baris ${i + 1}`}
          value={l.debit}
          onChange={(e) => draft.update(l.key, { debit: e.target.value })}
          placeholder="0"
          inputMode="numeric"
          className="text-right"
          error={errors[`lines.${i}.debit`]}
        />
      ),
    },
    {
      id: "credit",
      header: "Kredit (Rp)",
      width: "9rem",
      align: "right",
      render: (l, i) => (
        <TextInput
          aria-label={`Kredit baris ${i + 1}`}
          value={l.credit}
          onChange={(e) => draft.update(l.key, { credit: e.target.value })}
          placeholder="0"
          inputMode="numeric"
          className="text-right"
        />
      ),
    },
  ];
  if (!compact) {
    columns.push({
      id: "type",
      header: "Keterangan",
      width: "9rem",
      render: (l) => {
        const a = byCode.get(l.accountCode);
        return <span className="block py-2 text-xs text-muted-foreground">{a ? `${accountTypeLabel(a.type)}${a.isCash ? " · kas" : ""}` : "–"}</span>;
      },
    });
  }

  return (
    <LineItemsEditor<DraftLine>
      lines={draft.lines}
      getKey={(l) => l.key}
      columns={columns}
      onAdd={draft.add}
      onRemove={draft.lines.length > 2 ? draft.remove : undefined}
      addLabel="Tambah baris"
    />
  );
}
