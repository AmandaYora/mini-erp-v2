import { SearchSelect, TextInput } from "@/shared/components/ui";
import { LineItemsEditor } from "@/shared/components/patterns";
import type { LineColumn } from "@/shared/components/patterns";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import type { JournalDraft } from "@/modules/finance/hooks/use-journal-draft";
import { partyKindLabel } from "@/modules/finance/lib/journal-lines";
import type { DraftLine, PartyAccounts } from "@/modules/finance/lib/journal-lines";
import { accountTypeLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

/**
 * Editor baris jurnal (Akun · Debit · Kredit · Jenis) di atas LineItemsEditor — satu pola untuk
 * jurnal manual, saldo awal, dan koreksi fiskal. Akun dipilih berdasarkan KODE (backend mencari
 * menurut kode). "Jenis" adalah info turunan dari akun; API tidak menerima keterangan per baris
 * (memo berlaku untuk seluruh jurnal). Baris ke akun kendali per pihak (`partyAccounts`, dari
 * pemetaan akun) menampilkan pemilih pelanggan/pemasok di bawah akunnya; mengganti akun
 * mengosongkan pihaknya.
 */
export function JournalLinesEditor({
  draft,
  accounts,
  errors,
  compact = false,
  partyAccounts,
}: {
  draft: JournalDraft;
  accounts: Account[];
  errors: Record<string, string>;
  /** Tanpa kolom "Jenis" (dipakai di panel samping yang sempit). */
  compact?: boolean;
  /** Akun kendali per pihak → jenis pihak. Tanpa ini tidak ada pemilih pihak (koreksi fiskal). */
  partyAccounts?: PartyAccounts;
}) {
  const active = accounts.filter((a) => a.status === "active");
  const options = active.map((a) => ({ value: a.code, label: `${a.code} — ${a.name}` }));
  const byCode = new Map(active.map((a) => [a.code, a]));

  const columns: LineColumn<DraftLine>[] = [
    {
      id: "account",
      header: "Akun",
      render: (l, i) => {
        const partyKind = partyAccounts?.get(l.accountCode);
        return (
          <div role="group" aria-label={`Akun baris ${i + 1}`}>
            <SearchSelect
              aria-label={`Akun baris ${i + 1}`}
              options={options}
              value={l.accountCode || null}
              onChange={(v) => {
                const code = typeof v === "string" ? v : "";
                draft.update(l.key, code === l.accountCode ? { accountCode: code } : { accountCode: code, partyId: null });
              }}
              placeholder="Pilih akun…"
            />
            {errors[`lines.${i}.accountCode`] && (
              <p className="mt-1 text-left text-xs font-medium text-destructive">{errors[`lines.${i}.accountCode`]}</p>
            )}
            {partyKind && (
              <div className="mt-2">
                <PartySearchSelect
                  aria-label={`${partyKind === "supplier" ? "Pemasok" : "Pelanggan"} baris ${i + 1}`}
                  partyKind={partyKind}
                  value={l.partyId}
                  onChange={(v) => draft.update(l.key, { partyId: v })}
                  placeholder={`Pilih ${partyKindLabel(partyKind)}…`}
                />
                {errors[`lines.${i}.partyId`] && (
                  <p className="mt-1 text-left text-xs font-medium text-destructive">{errors[`lines.${i}.partyId`]}</p>
                )}
              </div>
            )}
          </div>
        );
      },
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
