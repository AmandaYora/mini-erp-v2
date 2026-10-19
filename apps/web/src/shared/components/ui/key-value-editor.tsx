import { Button } from "./button";
import { TextInput } from "./input";
import type { KeyValueLimits, KeyValueRow } from "@/shared/lib/key-value";
import { validateRows } from "@/shared/lib/key-value";

export interface KeyValueEditorProps {
  rows: KeyValueRow[];
  onChange: (rows: KeyValueRow[]) => void;
  limits: KeyValueLimits;
  /** Saran nama (datalist) — pengguna tetap bebas mengetik nama lain. */
  suggestions?: readonly string[];
  /** Galat dari server untuk blok ini. */
  serverError?: string;
  disabled?: boolean;
  /** Prefiks id unik per instance (untuk datalist). */
  idPrefix: string;
}

/** Editor pasangan nama/isi bebas (Info Tambahan). Tanpa pengetahuan domain. */
export function KeyValueEditor({
  rows,
  onChange,
  limits,
  suggestions,
  serverError,
  disabled,
  idPrefix,
}: KeyValueEditorProps) {
  const check = validateRows(rows, limits);
  const listId = `${idPrefix}-suggest`;

  function update(i: number, patch: Partial<KeyValueRow>) {
    onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  }

  return (
    <div className="space-y-3">
      {rows.length === 0 && <p className="text-muted-foreground text-sm">Belum ada informasi tambahan.</p>}
      {suggestions && suggestions.length > 0 && (
        <datalist id={listId}>
          {suggestions.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      )}
      {rows.map((r, i) => (
        <div key={i} className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_2fr_auto]">
          <TextInput
            aria-label={`Nama info tambahan ${i + 1}`}
            list={suggestions && suggestions.length > 0 ? listId : undefined}
            value={r.key}
            maxLength={limits.maxKey}
            placeholder="Nama (mis. NPWP)"
            disabled={disabled}
            onChange={(e) => update(i, { key: e.target.value })}
            error={check.byRow[i]}
          />
          <TextInput
            aria-label={`Isi info tambahan ${i + 1}`}
            value={r.value}
            maxLength={limits.maxValue}
            placeholder="Isi"
            disabled={disabled}
            onChange={(e) => update(i, { value: e.target.value })}
          />
          <div>
            <Button
              variant="ghost"
              size="sm"
              disabled={disabled}
              onClick={() => onChange(rows.filter((_, j) => j !== i))}
            >
              Hapus
            </Button>
          </div>
        </div>
      ))}
      {check.general && <p className="text-destructive text-[0.82rem]">{check.general}</p>}
      {serverError && <p className="text-destructive text-[0.82rem]">{serverError}</p>}
      <Button
        variant="secondary"
        size="sm"
        disabled={disabled || rows.length >= limits.maxRows}
        onClick={() => onChange([...rows, { key: "", value: "" }])}
      >
        Tambah info
      </Button>
    </div>
  );
}
