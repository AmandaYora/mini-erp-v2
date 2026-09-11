import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { INPUT } from "./field-classes";
import {
  SS_CHECK,
  SS_CHEVRON,
  SS_EMPTY,
  SS_INPUT,
  SS_ITEM,
  SS_ITEM_ACTIVE,
  SS_ITEM_DESC,
  SS_ITEM_MAIN,
  SS_ITEM_TITLE,
  SS_LABEL,
  SS_LIST,
  SS_MORE,
  SS_POPOVER,
  SS_SEARCH,
  SS_TRIGGER,
  type SelectValue,
} from "./select-shared";

export interface AsyncSearchOption {
  value: SelectValue;
  label: string;
  description?: ReactNode;
}

export interface AsyncSearchLoadResult {
  options: AsyncSearchOption[];
  hasMore?: boolean;
}

export interface AsyncSearchSelectProps {
  loadOptions: (
    search: string,
  ) => Promise<AsyncSearchLoadResult | AsyncSearchOption[]>;
  value?: SelectValue | null;
  defaultValue?: SelectValue | null;
  onChange?: (value: SelectValue | null, option?: AsyncSearchOption) => void;
  name?: string;
  id?: string;
  placeholder?: string;
  searchPlaceholder?: string;
  className?: string;
  disabled?: boolean;
  /** Tampilkan tombol × untuk mengosongkan pilihan. */
  allowClear?: boolean;
  selectedOption?: AsyncSearchOption | null;
  debounceMs?: number;
  loadingText?: string;
  emptyText?: string;
  moreText?: string;
}

const SEARCH_BAR_HEIGHT = 44;
const GAP = 4;
const MIN_LIST_HEIGHT = 120;

interface PopoverPos {
  top: number;
  left: number;
  width: number;
  listMaxHeight: number;
}

/**
 * AsyncSearchSelect — dropdown dengan pencarian server-side penuh.
 *
 * Diport dari aplikasi lama. Bedanya dari SearchSelect (sync): opsi tidak
 * diberikan di muka, melainkan dimuat via `loadOptions` setiap popover dibuka
 * dan setiap ketikan (debounce). Nilai mendukung string|number agar cocok
 * dengan ID numerik backend.
 *
 * Bebas peringatan set-state-in-effect by design: sinkronisasi
 * controlled↔internal memakai render-phase adjustment (pola resmi React untuk
 * derived state), fetch dipicu imperatif dari handler buka/ketik (bukan dari
 * effect), dan effect luar-klik hanya berlangganan event (setState hanya di
 * callback).
 */
export function AsyncSearchSelect({
  loadOptions,
  value,
  defaultValue,
  onChange,
  name,
  id,
  placeholder = "Pilih…",
  searchPlaceholder = "Cari…",
  className,
  disabled,
  allowClear = false,
  selectedOption,
  debounceMs = 250,
  loadingText = "Memuat…",
  emptyText = "Tidak ada hasil",
  moreText = "Ketik lebih spesifik untuk melihat hasil lainnya",
}: AsyncSearchSelectProps) {
  const [internalValue, setInternalValue] = useState<SelectValue | null>(
    value ?? defaultValue ?? null,
  );
  const [prevControlled, setPrevControlled] = useState(value);
  if (value !== undefined && value !== prevControlled) {
    setPrevControlled(value);
    setInternalValue(value);
  }
  const [selectedCache, setSelectedCache] =
    useState<AsyncSearchOption | null>(selectedOption ?? null);
  const [prevSelected, setPrevSelected] = useState(selectedOption);
  if (selectedOption !== prevSelected) {
    setPrevSelected(selectedOption);
    setSelectedCache(selectedOption ?? null);
  }

  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<PopoverPos | null>(null);
  const [options, setOptions] = useState<AsyncSearchOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const seqRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  function calcPos(): PopoverPos | null {
    if (!containerRef.current || typeof window === "undefined") return null;
    const rect = containerRef.current.getBoundingClientRect();
    const vh = window.innerHeight;
    const listMaxHeight = Math.max(MIN_LIST_HEIGHT, vh - rect.bottom - GAP - 8 - SEARCH_BAR_HEIGHT);
    return {
      top: rect.bottom + GAP,
      left: rect.left,
      width: rect.width,
      listMaxHeight,
    };
  }

  function runFetch(query: string) {
    const requestId = seqRef.current + 1;
    seqRef.current = requestId;
    // Nilai saat fetch dimulai — dipakai mencocokkan opsi terpilih saat
    // respons tiba (closure, bukan ref: tanpa tulis ref saat render).
    const valueAtStart = internalValue;
    const load = loadOptions;
    setLoading(true);
    setFetchError(null);
    void Promise.resolve()
      .then(() => load(query.trim()))
      .then((result) => {
        if (seqRef.current !== requestId) return;
        const next = Array.isArray(result) ? result : result.options;
        setOptions(next);
        setHasMore(Array.isArray(result) ? false : Boolean(result.hasMore));
        const selected = next.find((o) => o.value === valueAtStart);
        if (selected) setSelectedCache(selected);
      })
      .catch(() => {
        if (seqRef.current !== requestId) return;
        setOptions([]);
        setHasMore(false);
        setFetchError("Gagal memuat pilihan");
      })
      .finally(() => {
        if (seqRef.current === requestId) setLoading(false);
      });
  }

  function scheduleFetch(query: string) {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => runFetch(query), debounceMs);
  }

  // Bersihkan timer debounce saat unmount — tanpa setState sinkron di body.
  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  // Tutup saat klik di luar / Escape / scroll — setState hanya di callback.
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node | null;
      if (!target) return;
      if (containerRef.current?.contains(target)) return;
      if (popoverRef.current?.contains(target)) return;
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
  }, [open]);

  function handleToggle() {
    if (disabled) return;
    if (open) {
      setOpen(false);
      return;
    }
    setPos(calcPos());
    setSearch("");
    setOpen(true);
    runFetch("");
  }

  function handleSelect(option: AsyncSearchOption) {
    setInternalValue(option.value);
    setSelectedCache(option);
    onChange?.(option.value, option);
    setOpen(false);
    setSearch("");
  }

  function handleClear(e: React.MouseEvent | React.KeyboardEvent) {
    e.stopPropagation();
    setInternalValue(null);
    setSelectedCache(null);
    onChange?.(null, undefined);
    setOpen(false);
    setSearch("");
  }

  const selectedFromOptions = options.find((o) => o.value === internalValue);
  const selectedLabel =
    selectedFromOptions?.label ??
    (selectedCache && selectedCache.value === internalValue
      ? selectedCache.label
      : undefined) ??
    placeholder;
  const hasValue = internalValue !== null && internalValue !== undefined && internalValue !== "";
  const showClear = allowClear && hasValue && !disabled;

  return (
    <div
      className={["async-search-select", "relative w-full min-w-0", className]
        .filter(Boolean)
        .join(" ")}
      ref={containerRef}
    >
      {name ? <input type="hidden" name={name} value={internalValue ?? ""} /> : null}
      <button
        type="button"
        id={id}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        onClick={handleToggle}
        className={["async-search-select-trigger", INPUT, SS_TRIGGER].join(" ")}
      >
        <span className={[SS_LABEL, "text-[0.95rem]"].join(" ")} data-empty={!hasValue ? "true" : "false"}>
          {selectedLabel}
        </span>
        {showClear ? (
          <span
            role="button"
            tabIndex={0}
            aria-label="Hapus pilihan"
            className="inline-flex cursor-pointer items-center px-0.5 text-muted transition-colors hover:text-ink"
            onClick={handleClear}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                handleClear(e);
              }
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </span>
        ) : null}
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={SS_CHEVRON}
          style={{ transform: open ? "rotate(180deg)" : undefined }}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && pos && typeof document !== "undefined"
        ? createPortal(
            <div
              ref={popoverRef}
              className={`async-search-select-popover ${SS_POPOVER}`}
              style={{
                position: "fixed",
                top: pos.top,
                left: pos.left,
                width: pos.width,
                zIndex: 9999,
              }}
            >
              <div className={SS_SEARCH}>
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    scheduleFetch(e.target.value);
                  }}
                  placeholder={searchPlaceholder}
                  className={["async-search-select-input", INPUT, SS_INPUT].join(" ")}
                />
              </div>
              <div className={SS_LIST} role="listbox" style={{ maxHeight: pos.listMaxHeight }}>
                {options.map((option) => {
                  const active = option.value === internalValue;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      role="option"
                      aria-selected={active}
                      onClick={() => handleSelect(option)}
                      className={[
                        "async-search-select-item",
                        SS_ITEM,
                        active ? SS_ITEM_ACTIVE : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                    >
                      <span className={SS_ITEM_MAIN}>
                        <span className={SS_ITEM_TITLE}>{option.label}</span>
                        {option.description ? (
                          <span className={SS_ITEM_DESC}>{option.description}</span>
                        ) : null}
                      </span>
                      {active ? (
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          className={SS_CHECK}
                        >
                          <polyline points="20 6 9 17 4 12" />
                        </svg>
                      ) : null}
                    </button>
                  );
                })}
                {loading ? <div className={SS_EMPTY}>{loadingText}</div> : null}
                {!loading && fetchError ? <div className={SS_EMPTY}>{fetchError}</div> : null}
                {!loading && !fetchError && options.length === 0 ? (
                  <div className={SS_EMPTY}>{emptyText}</div>
                ) : null}
                {!loading && !fetchError && hasMore ? (
                  <div className={SS_MORE}>{moreText}</div>
                ) : null}
              </div>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
