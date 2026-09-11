import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, X } from "lucide-react";
import { INPUT } from "./field-classes";

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
}

interface PopoverPos {
  top: number;
  left: number;
  width: number;
}

export function SearchSelect({
  options,
  value,
  onChange,
  placeholder = "Pilih…",
  allowClear,
  disabled,
}: SearchSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [pos, setPos] = useState<PopoverPos>({ top: 0, left: 0, width: 0 });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const selected = options.find((o) => o.value === value);

  const openPanel = () => {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect) {
      setPos({ top: rect.bottom + 4, left: rect.left, width: rect.width });
    }
    setQuery("");
    setOpen(true);
  };

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (panelRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onScrollOrResize = () => setOpen(false);
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open ]);

  const filtered = options.filter((o) =>
    o.label.toLowerCase().includes(query.toLowerCase()),
  );

  const showClear =
    allowClear && value !== null && value !== undefined && value !== "" && !disabled;

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        disabled={disabled}
        onClick={() => {
          if (open) setOpen(false);
          else openPanel();
        }}
        className={`${INPUT} flex cursor-pointer items-center justify-between gap-2 text-left disabled:cursor-not-allowed disabled:opacity-50`}
      >
        <span
          className={`truncate ${selected ? "text-ink" : "text-muted"}`}
        >
          {selected ? selected.label : placeholder}
        </span>
        <span className="flex shrink-0 items-center gap-1">
          {showClear && (
            <span
              role="button"
              tabIndex={0}
              aria-label="Hapus pilihan"
              className="text-muted rounded p-0.5 hover:text-ink"
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
          <ChevronDown size={16} className="text-muted" />
        </span>
      </button>
      {open &&
        !disabled &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={panelRef}
            style={{ top: pos.top, left: pos.left, width: pos.width }}
            className="fixed z-[1000] overflow-hidden rounded-md border border-hairline bg-surface shadow-md"
          >
            <div className="border-b border-hairline p-2">
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Cari…"
                className={INPUT}
              />
            </div>
            <ul className="max-h-60 overflow-auto p-1">
              {filtered.length === 0 ? (
                <li className="text-muted px-3 py-2 text-sm">
                  Tidak ada hasil
                </li>
              ) : (
                filtered.map((o) => {
                  const active = o.value === value;
                  return (
                    <li key={o.value}>
                      <button
                        type="button"
                        onClick={() => {
                          onChange(o.value);
                          setOpen(false);
                        }}
                        className={`flex w-full cursor-pointer items-center justify-between rounded px-3 py-2 text-left text-sm ${
                          active
                            ? "bg-brand/10 text-brand font-medium"
                            : "text-ink hover:bg-surface-subtle"
                        }`}
                      >
                        <span className="truncate">{o.label}</span>
                        {active && <Check size={16} className="shrink-0" />}
                      </button>
                    </li>
                  );
                })
              )}
            </ul>
          </div>,
          document.body,
        )}
    </>
  );
}
