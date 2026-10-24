import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { isAbort } from "@/shared/services/http-client";
import { INPUT } from "./field-classes";
import { LIST_MAX_HEIGHT, SelectPopover, SelectPopoverContent, SelectPopoverTrigger } from "./select-popover";
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
  /** Pemuat opsi. `signal` dibatalkan saat ketikan berikutnya menggantikan
   * request ini atau saat komponen dilepas — teruskan ke http-client agar
   * request usang berhenti, bukan sekadar diabaikan. Opsional: pemuat lama
   * yang mengabaikannya tetap benar, hanya tidak ikut berhenti. `page` (mulai 1)
   * hanya dikirim pada mode `paged`. */
  loadOptions: (
    search: string,
    signal?: AbortSignal,
    page?: number,
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
  /** Panjang minimal kata kunci sebelum request dikirim. Kata kunci kosong
   * tetap memuat daftar awal; kata kunci 1..minChars-1 huruf tidak memanggil
   * `loadOptions` sama sekali (mis. pencarian pihak minimal 2 huruf). */
  minChars?: number;
  "aria-label"?: string;
  minCharsText?: string;
  /** Mode berhalaman (pola katalog POS): selama `hasMore`, halaman berikutnya
   * dimuat otomatis saat daftar digulir mendekati bawah; opsi digabung tanpa
   * duplikat dan paging kembali ke halaman 1 saat kata kunci berubah. */
  paged?: boolean;
  loadingMoreText?: string;
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
  "aria-label": ariaLabel,
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
  minChars = 0,
  minCharsText,
  paged = false,
  loadingMoreText = "Memuat lainnya…",
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
  const [options, setOptions] = useState<AsyncSearchOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [tooShort, setTooShort] = useState(false);
  // Kata kunci yang hasilnya sedang tampil; beda dengan `search` = daftar basi (debounce/fetch berjalan).
  const [settledQuery, setSettledQuery] = useState<string | null>(null);
  // Enter ditekan saat daftar basi: pilih hasil pertama begitu hasil kata kunci terbaru tiba (U7-2).
  const pendingEnterRef = useRef(false);

  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<string | null>(null);

  const seqRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  // Pengaman sinkron mode paged: IntersectionObserver bisa memicu muat-berikutnya
  // dua kali sebelum render berikutnya (pola use-pos-catalog).
  const pageRef = useRef(0);
  const queryRef = useRef("");
  const idsRef = useRef<Set<SelectValue>>(new Set());
  const hasMoreRef = useRef(false);
  const moreInflightRef = useRef(false);
  const moreErrorRef = useRef(false);
  const listRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  function runFetch(query: string) {
    const requestId = seqRef.current + 1;
    seqRef.current = requestId;
    // Ketikan berikutnya menggantikan request ini: hentikan yang lama, jangan
    // hanya abaikan hasilnya. Urutan tetap dijaga seqRef — abort mengurus
    // biayanya (koneksi dan kerja server), seqRef mengurus kebenarannya.
    abortRef.current?.abort();
    const trimmedQuery = query.trim();
    // Generasi baru: muat-berikutnya generasi lama (bila ada) ikut dibatalkan di atas.
    moreInflightRef.current = false;
    moreErrorRef.current = false;
    hasMoreRef.current = false;
    setLoadingMore(false);
    setMoreError(null);
    if (trimmedQuery.length > 0 && trimmedQuery.length < minChars) {
      // Terlalu pendek: tidak ada request; hasil lama dikosongkan.
      abortRef.current = null;
      setOptions([]);
      setHasMore(false);
      setFetchError(null);
      setLoading(false);
      setTooShort(true);
      setSettledQuery(trimmedQuery);
      pendingEnterRef.current = false;
      return;
    }
    setTooShort(false);
    const controller = new AbortController();
    abortRef.current = controller;
    // Nilai saat fetch dimulai — dipakai mencocokkan opsi terpilih saat
    // respons tiba (closure, bukan ref: tanpa tulis ref saat render).
    const valueAtStart = internalValue;
    const load = loadOptions;
    setLoading(true);
    setFetchError(null);
    void Promise.resolve()
      .then(() => (paged ? load(trimmedQuery, controller.signal, 1) : load(trimmedQuery, controller.signal)))
      .then((result) => {
        if (seqRef.current !== requestId) return;
        const raw = Array.isArray(result) ? result : result.options;
        // Mode paged: buang nilai ganda di dalam halaman agar key opsi unik.
        const ids = new Set<SelectValue>();
        const next = paged ? raw.filter((o) => (ids.has(o.value) ? false : (ids.add(o.value), true))) : raw;
        setOptions(next);
        setSettledQuery(trimmedQuery);
        if (pendingEnterRef.current) {
          pendingEnterRef.current = false;
          const first = next[0];
          if (first) handleSelect(first);
        }
        const more = Array.isArray(result) ? false : Boolean(result.hasMore);
        setHasMore(more);
        if (paged) {
          // Daftar baru mulai dari atas (seperti grid POS); tanpa ini sentinel langsung
          // terlihat dari posisi gulir lama dan halaman 2 ikut termuat tanpa digulir.
          if (listRef.current) listRef.current.scrollTop = 0;
          queryRef.current = trimmedQuery;
          pageRef.current = 1;
          idsRef.current = ids;
          hasMoreRef.current = more && next.length > 0;
        }
        const selected = next.find((o) => o.value === valueAtStart);
        if (selected) setSelectedCache(selected);
      })
      .catch((err) => {
        if (seqRef.current !== requestId) return;
        // Pembatalan kita sendiri bukan kegagalan pemuatan.
        if (isAbort(err)) return;
        setOptions([]);
        setHasMore(false);
        setFetchError("Gagal memuat pilihan");
        setSettledQuery(trimmedQuery);
        pendingEnterRef.current = false;
      })
      .finally(() => {
        if (seqRef.current === requestId) setLoading(false);
      });
  }

  /** Mode paged: muat halaman berikutnya kata kunci yang sedang tampil; no-op
   * saat memuat, galat, atau daftar habis. */
  function loadMore() {
    if (!paged || moreInflightRef.current || moreErrorRef.current || !hasMoreRef.current) return;
    const gen = seqRef.current;
    const pageNo = pageRef.current + 1;
    const controller = new AbortController();
    abortRef.current = controller;
    moreInflightRef.current = true;
    setLoadingMore(true);
    setMoreError(null);
    const valueAtStart = internalValue;
    const load = loadOptions;
    void Promise.resolve()
      .then(() => load(queryRef.current, controller.signal, pageNo))
      .then((result) => {
        if (seqRef.current !== gen) return;
        const raw = Array.isArray(result) ? result : result.options;
        const ids = new Set(idsRef.current);
        const fresh: AsyncSearchOption[] = [];
        for (const o of raw) {
          if (ids.has(o.value)) continue;
          ids.add(o.value);
          fresh.push(o);
        }
        // Halaman tanpa nilai baru = akhir daftar (hindari muat tanpa henti).
        const more = !Array.isArray(result) && Boolean(result.hasMore) && fresh.length > 0;
        idsRef.current = ids;
        pageRef.current = pageNo;
        hasMoreRef.current = more;
        setOptions((prev) => [...prev, ...fresh]);
        setHasMore(more);
        const selected = fresh.find((o) => o.value === valueAtStart);
        if (selected) setSelectedCache(selected);
      })
      .catch((err) => {
        if (seqRef.current !== gen || isAbort(err)) return;
        // Opsi yang sudah tampil tidak dihapus; pengguna bisa mengulang.
        moreErrorRef.current = true;
        setMoreError("Gagal memuat pilihan berikutnya");
      })
      .finally(() => {
        if (seqRef.current !== gen) return;
        moreInflightRef.current = false;
        if (abortRef.current === controller) abortRef.current = null;
        setLoadingMore(false);
      });
  }

  function retryMore() {
    moreErrorRef.current = false;
    loadMore();
  }

  // Versi terbaru loadMore untuk callback observer (tanpa memasang ulang tiap render).
  const loadMoreRef = useRef(loadMore);
  useEffect(() => {
    loadMoreRef.current = loadMore;
  });

  // Sentinel di ujung daftar: terlihat (atau daftar belum penuh) → muat halaman berikutnya.
  // Dipasang ulang saat jumlah opsi berubah agar observer memeriksa ulang posisinya.
  const showSentinel = paged && open && hasMore && !loading && moreError === null;
  useEffect(() => {
    const root = listRef.current;
    const sentinel = sentinelRef.current;
    if (!showSentinel || !root || !sentinel || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) loadMoreRef.current();
      },
      { root, rootMargin: "0px 0px 120px 0px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [showSentinel, options.length]);

  function scheduleFetch(query: string) {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => runFetch(query), debounceMs);
  }

  // Bersihkan timer debounce dan request yang masih jalan saat unmount —
  // tanpa setState sinkron di body.
  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      abortRef.current?.abort();
    };
  }, []);

  // Buka/tutup dikelola Popover Radix (klik luar, Esc, fokus); fetch awal dipicu dari sini.
  function handleOpenChange(next: boolean) {
    if (disabled) return;
    setOpen(next);
    if (!next) pendingEnterRef.current = false;
    if (next) {
      setSearch("");
      runFetch("");
    }
  }

  function handleSelect(option: AsyncSearchOption) {
    pendingEnterRef.current = false;
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
  const stale = loading || settledQuery !== search.trim();

  return (
    <div className={["async-search-select", "relative w-full min-w-0", className].filter(Boolean).join(" ")}>
      {name ? <input type="hidden" name={name} value={internalValue ?? ""} /> : null}
      <SelectPopover open={open && !disabled} onOpenChange={handleOpenChange}>
      <SelectPopoverTrigger asChild>
      <button
        type="button"
        id={id}
        aria-label={ariaLabel}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
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
            className="inline-flex cursor-pointer items-center px-0.5 text-muted-foreground transition-colors hover:text-foreground"
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
      </SelectPopoverTrigger>

      <SelectPopoverContent className={`async-search-select-popover ${SS_POPOVER}`}>
              <div className={SS_SEARCH}>
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    pendingEnterRef.current = false;
                    scheduleFetch(e.target.value);
                  }}
                  onKeyDown={(e) => {
                    // Daftar basi: jangan pilih hasil kata kunci lama; tunggu hasil terbaru.
                    if (e.key === "Enter" && stale) {
                      e.preventDefault();
                      pendingEnterRef.current = true;
                    }
                  }}
                  placeholder={searchPlaceholder}
                  className={["async-search-select-input", INPUT, SS_INPUT].join(" ")}
                />
              </div>
              <div
                ref={listRef}
                className={SS_LIST}
                role="listbox"
                aria-busy={stale || loadingMore || undefined}
                style={{ maxHeight: LIST_MAX_HEIGHT }}
              >
                {options.map((option) => {
                  const active = option.value === internalValue;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      role="option"
                      data-select-option=""
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
                {tooShort ? (
                  <div className={SS_EMPTY}>
                    {minCharsText ?? `Ketik minimal ${minChars} huruf untuk mencari`}
                  </div>
                ) : null}
                {!loading && !fetchError && !tooShort && options.length === 0 ? (
                  <div className={SS_EMPTY}>{emptyText}</div>
                ) : null}
                {!loading && !fetchError && hasMore && !paged ? (
                  <div className={SS_MORE}>{moreText}</div>
                ) : null}
                {showSentinel ? <div ref={sentinelRef} aria-hidden className="h-px shrink-0" /> : null}
                {paged && loadingMore ? (
                  <div className={SS_MORE} role="status">
                    {loadingMoreText}
                  </div>
                ) : null}
                {paged && moreError !== null ? (
                  <div className={SS_MORE}>
                    {moreError}{" "}
                    <button
                      type="button"
                      onClick={retryMore}
                      className="font-medium text-primary underline-offset-2 hover:underline"
                    >
                      Coba lagi
                    </button>
                  </div>
                ) : null}
              </div>
      </SelectPopoverContent>
      </SelectPopover>
    </div>
  );
}
