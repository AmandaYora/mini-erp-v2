import { useEffect, useRef, useState } from "react";
import { INPUT } from "./field-classes";

export interface SearchInputProps {
  /** Nilai terapan (mis. dari URL). Berubah dari luar (reset) -> teks ikut. */
  value: string;
  /** Dipanggil setelah jeda ketik (debounce) atau saat Enter/blur. */
  onSearch: (value: string) => void;
  placeholder?: string;
  debounceMs?: number;
  id?: string;
  disabled?: boolean;
  "aria-label"?: string;
}

/**
 * SearchInput — kolom cari dengan debounce. Memiliki teks lokal agar ketikan
 * tidak tertahan; hanya nilai yang sudah berjeda yang dilaporkan, sehingga
 * tiap ketikan tidak memicu request. Timer dibuat di handler (bukan effect)
 * dan dibersihkan saat unmount.
 */
export function SearchInput({
  value,
  onSearch,
  placeholder,
  debounceMs = 400,
  id,
  disabled,
  "aria-label": ariaLabel,
}: SearchInputProps) {
  const [text, setText] = useState(value);
  const [prevValue, setPrevValue] = useState(value);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const valueRef = useRef(value);

  // Sinkron dari luar (reset/tautan) hanya bila berbeda dari teks yang sedang
  // diketik — spasi akhir yang sedang diketik tidak ikut terhapus.
  if (value !== prevValue) {
    setPrevValue(value);
    if (value !== text.trim()) setText(value);
  }

  useEffect(() => {
    valueRef.current = value;
  }, [value]);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  function emit(next: string) {
    const trimmed = next.trim();
    if (trimmed === valueRef.current) return;
    valueRef.current = trimmed;
    onSearch(trimmed);
  }

  function schedule(next: string) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => emit(next), debounceMs);
  }

  function flush() {
    if (timer.current) clearTimeout(timer.current);
    emit(text);
  }

  return (
    <input
      id={id}
      type="search"
      aria-label={ariaLabel}
      disabled={disabled}
      className={INPUT}
      value={text}
      placeholder={placeholder}
      onChange={(e) => {
        setText(e.target.value);
        schedule(e.target.value);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          flush();
        }
      }}
      onBlur={flush}
    />
  );
}
