import { useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Bookmark, Check, RotateCcw, Trash2 } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { Separator } from "@/shared/components/primitives/separator";
import { Button } from "@/shared/components/ui/button";
import { FormField } from "@/shared/components/ui/form-field";
import { TextInput } from "@/shared/components/ui/input";
import { cn } from "@/shared/lib/cn";
import {
  MAX_VIEW_NAME,
  deleteView,
  normalizeViewQuery,
  readHiddenColumns,
  readSavedViews,
  saveView,
  writeHiddenColumns,
  type SavedView,
} from "@/shared/lib/list-views";

function sameColumns(a: string[] | null, b: string[] | null): boolean {
  if (a === null || b === null) return a === b;
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((x) => set.has(x));
}

/**
 * Tampilan tersimpan per daftar (PLAN redesain-ui U6-3): simpan kombinasi filter + sortir (query
 * string) dan pilihan kolom tabel dengan sebuah nama, lalu terapkan lagi dengan satu klik.
 * Disimpan di localStorage per pengguna, tanpa endpoint (U11). Letakkan di slot `right` Toolbar.
 */
export function SavedViews({
  listKey,
  columnsKey,
  className,
}: {
  /** Kunci stabil daftar, mis. "sales-orders". */
  listKey: string;
  /** Sama dengan `columnsKey` DataTable bila daftar punya pilihan kolom. */
  columnsKey?: string;
  className?: string;
}) {
  const { search, pathname } = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [views, setViews] = useState<SavedView[]>(() => readSavedViews(listKey));
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const currentQuery = normalizeViewQuery(search);
  const currentColumns = columnsKey ? readHiddenColumns(columnsKey) : null;
  const active = views.find(
    (v) => v.query === currentQuery && (!columnsKey || sameColumns(v.hiddenColumns, currentColumns)),
  );

  function handleOpenChange(next: boolean) {
    if (next) {
      setViews(readSavedViews(listKey));
      setName(active?.name ?? "");
      setError(null);
    }
    setOpen(next);
  }

  function apply(query: string, hiddenColumns: string[] | null) {
    if (columnsKey) writeHiddenColumns(columnsKey, hiddenColumns);
    navigate({ pathname, search: query ? `?${query}` : "" });
    setOpen(false);
  }

  function handleSave(e: FormEvent) {
    e.preventDefault();
    const res = saveView(listKey, { name, query: search, hiddenColumns: currentColumns });
    if (!res.ok) {
      setError(res.message);
      return;
    }
    setViews(res.views);
    setError(null);
    setName("");
    setOpen(false);
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button variant="secondary" className={cn("max-w-56", className)}>
          <Bookmark aria-hidden />
          <span className="truncate">{active ? active.name : "Tampilan"}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-80 flex-col gap-3 p-3">
        <div>
          <p className="px-1 pb-1 text-xs font-semibold text-muted-foreground">Tampilan tersimpan</p>
          {views.length === 0 ? (
            <p className="px-1 py-2 text-sm text-muted-foreground">
              Belum ada. Atur filter dan kolom, lalu beri nama di bawah.
            </p>
          ) : (
            <ul className="flex flex-col" aria-label="Tampilan tersimpan">
              {views.map((v) => {
                const isActive = v.id === active?.id;
                return (
                  <li key={v.id} className="group flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => apply(v.query, v.hiddenColumns)}
                      aria-current={isActive || undefined}
                      className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent focus-visible:bg-accent focus-visible:outline-none"
                    >
                      <Check aria-hidden className={cn("size-4 flex-none", isActive ? "text-primary" : "invisible")} />
                      <span className="truncate">{v.name}</span>
                    </button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Hapus tampilan ${v.name}`}
                      onClick={() => setViews(deleteView(listKey, v.id))}
                      className="text-muted-foreground hover:text-destructive"
                    >
                      <Trash2 aria-hidden />
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
          <button
            type="button"
            onClick={() => apply("", null)}
            className="mt-1 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:outline-none"
          >
            <RotateCcw aria-hidden className="size-4" /> Kembali ke tampilan bawaan
          </button>
        </div>
        <Separator />
        <form onSubmit={handleSave} className="flex flex-col gap-2" noValidate>
          <FormField label="Simpan filter dan kolom saat ini" errorText={error ?? undefined}>
            <TextInput
              value={name}
              maxLength={MAX_VIEW_NAME}
              placeholder="Mis. Jatuh tempo minggu ini"
              onChange={(e) => {
                setName(e.target.value);
                setError(null);
              }}
            />
          </FormField>
          <Button type="submit" size="sm" className="self-end">
            Simpan tampilan
          </Button>
        </form>
      </PopoverContent>
    </Popover>
  );
}
