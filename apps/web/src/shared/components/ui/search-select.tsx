import { useState } from "react";
import { Check, ChevronDown, X } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { INPUT } from "./field-classes";
import { LIST_MAX_HEIGHT, SelectPopover, SelectPopoverContent, SelectPopoverTrigger } from "./select-popover";

export interface SearchSelectOption {
  value: string | number;
  label: string;
}

export interface SearchSelectProps {
  options: SearchSelectOption[];
  value: string | number | null | undefined;
  onChange: (value: string | number | null) => void;
  placeholder?: string;
  allowClear?: boolean;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
}

export function SearchSelect({
  options,
  value,
  onChange,
  placeholder = "Pilih…",
  allowClear,
  disabled,
  id,
  "aria-label": ariaLabel,
}: SearchSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const selected = options.find((o) => o.value === value);
  const filtered = options.filter((o) => o.label.toLowerCase().includes(query.toLowerCase()));
  const showClear = allowClear && value !== null && value !== undefined && value !== "" && !disabled;

  function handleOpenChange(next: boolean) {
    if (next) setQuery("");
    setOpen(next);
  }

  return (
    <SelectPopover open={open && !disabled} onOpenChange={handleOpenChange}>
      <SelectPopoverTrigger asChild>
        <button
          type="button"
          id={id}
          aria-label={ariaLabel}
          disabled={disabled}
          aria-haspopup="listbox"
          aria-expanded={open}
          className={`${INPUT} flex cursor-pointer items-center justify-between gap-2 text-left disabled:cursor-not-allowed disabled:opacity-50`}
        >
          <span className={cn("truncate", selected ? "text-foreground" : "text-muted-foreground")}>
            {selected ? selected.label : placeholder}
          </span>
          <span className="flex shrink-0 items-center gap-1">
            {showClear && (
              <span
                role="button"
                tabIndex={0}
                aria-label="Hapus pilihan"
                className="rounded p-0.5 text-muted-foreground hover:text-foreground"
                onClick={(e) => {
                  e.stopPropagation();
                  onChange(null);
                  setOpen(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    e.stopPropagation();
                    onChange(null);
                    setOpen(false);
                  }
                }}
              >
                <X size={14} />
              </span>
            )}
            <ChevronDown size={16} className="text-muted-foreground" />
          </span>
        </button>
      </SelectPopoverTrigger>
      <SelectPopoverContent className="overflow-hidden rounded-md border border-border bg-card shadow-[var(--shadow-pop)]">
        <div className="border-b border-border p-2">
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari…"
            className={INPUT}
          />
        </div>
        <ul role="listbox" className="overflow-auto p-1" style={{ maxHeight: LIST_MAX_HEIGHT }}>
          {filtered.length === 0 ? (
            <li role="presentation" className="px-3 py-2 text-sm text-muted-foreground">Tidak ada hasil</li>
          ) : (
            filtered.map((o) => {
              const active = o.value === value;
              return (
                <li key={o.value} role="presentation">
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    data-select-option=""
                    onClick={() => {
                      onChange(o.value);
                      setOpen(false);
                    }}
                    className={cn(
                      "flex w-full cursor-pointer items-center justify-between rounded px-3 py-2 text-left text-sm",
                      active ? "bg-primary/10 font-medium text-primary" : "text-foreground hover:bg-accent",
                    )}
                  >
                    <span className="truncate">{o.label}</span>
                    {active && <Check size={16} className="shrink-0" />}
                  </button>
                </li>
              );
            })
          )}
        </ul>
      </SelectPopoverContent>
    </SelectPopover>
  );
}
