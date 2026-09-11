import { useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import { INPUT } from "./field-classes";
import {
  SS_CHEVRON,
  SS_EMPTY,
  SS_INPUT,
  SS_ITEM_ACTIVE,
  SS_LABEL,
  SS_LIST_HIER,
  SS_POPOVER_HIER,
  SS_SEARCH,
  SS_TRIGGER,
  TS_CONTENT,
  TS_GROUP_BADGE,
  TS_GUIDE,
  TS_GUIDE_LINES,
  TS_ITEM,
  TS_LINES_ACTIVE,
  TS_LINES_IDLE,
  TS_PATH,
  TS_TITLE,
  TS_TITLE_ROW,
  getOptionDisplayLabel,
  getOptionSearchText,
  type SharedSelectOption,
} from "./select-shared";
import { SelectCheckIcon } from "./select-check-icon";

export type { SharedSelectOption as HierarchicalSelectOption };

export interface HierarchicalSelectProps {
  options: SharedSelectOption[];
  value?: string | number | null;
  defaultValue?: string | number | null;
  onChange?: (value: string | number | null) => void;
  name?: string;
  id?: string;
  placeholder?: string;
  searchPlaceholder?: string;
  className?: string;
  disabled?: boolean;
  required?: boolean;
  emptyOptionLabel?: string;
  emptyText?: string;
}

const SEARCH_BAR_HEIGHT = 44;
const GAP = 4;
const MIN_LIST_HEIGHT = 140;

interface PopoverPos {
  left: number;
  width: number;
  listMaxHeight: number;
  top?: number;
  bottom?: number;
}

/**
 * HierarchicalSelect — dropdown sync dengan visual pohon (tree guide).
 *
 * Diport dari aplikasi lama. Murni client-side (opsi diberikan di muka),
 * cocok untuk hierarki kecil seperti lokasi stok / kategori produk. Nilai
 * mendukung string|number. Bebas set-state-in-effect: sinkronisasi controlled
 * memakai render-phase adjustment; effect hanya untuk klik-luar.
 */
export function HierarchicalSelect({
  options,
  value,
  defaultValue,
  onChange,
  name,
  id,
  placeholder = "Pilih…",
  searchPlaceholder = "Cari…",
  className,
  disabled,
  required,
  emptyOptionLabel,
  emptyText = "Tidak ada pilihan",
}: HierarchicalSelectProps) {
  const [internalValue, setInternalValue] = useState<string | number | null>(
    value ?? defaultValue ?? null,
  );
  const [prevControlled, setPrevControlled] = useState(value);
  if (value !== undefined && value !== prevControlled) {
    setPrevControlled(value);
    setInternalValue(value);
  }

  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<PopoverPos | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  function calcPos(): PopoverPos | null {
    if (!containerRef.current || typeof window === "undefined") return null;
    const rect = containerRef.current.getBoundingClientRect();
    const vh = window.innerHeight;
    const spaceBelow = vh - rect.bottom - GAP - 8;
    const spaceAbove = rect.top - GAP - 8;
    const openUpward =
      spaceAbove > spaceBelow && spaceBelow < MIN_LIST_HEIGHT + SEARCH_BAR_HEIGHT;
    if (openUpward) {
      return {
        bottom: vh - rect.top + GAP,
        left: rect.left,
        width: rect.width,
        listMaxHeight: Math.max(MIN_LIST_HEIGHT, spaceAbove - SEARCH_BAR_HEIGHT),
      };
    }
    return {
      top: rect.bottom + GAP,
      left: rect.left,
      width: rect.width,
      listMaxHeight: Math.max(MIN_LIST_HEIGHT, spaceBelow - SEARCH_BAR_HEIGHT),
    };
  }

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node | null;
      if (!target) return;
      if (containerRef.current?.contains(target)) return;
      if (popoverRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onScrollOrResize = () => setOpen(false);
    document.addEventListener("mousedown", onPointerDown);
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open]);

  function handleToggle() {
    if (disabled) return;
    if (!open) setPos(calcPos());
    setOpen((v) => !v);
  }

  function handleSelect(option: SharedSelectOption) {
    if (option.disabled) return;
    setInternalValue(option.value);
    onChange?.(option.value);
    setOpen(false);
    setSearch("");
  }

  const normalizedSearch = search.trim().toLowerCase();
  const selectedOption = options.find((o) => o.value === internalValue);
  const selectedLabel = selectedOption
    ? getOptionDisplayLabel(selectedOption, placeholder)
    : (emptyOptionLabel ?? placeholder);
  const hasValue =
    internalValue !== null && internalValue !== undefined && internalValue !== "";
  const filtered = normalizedSearch
    ? options.filter((o) => getOptionSearchText(o).includes(normalizedSearch))
    : options;
  const showEmptyOption = Boolean(
    emptyOptionLabel &&
      (!normalizedSearch || emptyOptionLabel.toLowerCase().includes(normalizedSearch)),
  );

  return (
    <div
      className={["hierarchical-select", "relative w-full min-w-0", className]
        .filter(Boolean)
        .join(" ")}
      ref={containerRef}
    >
      {name ? (
        <input
          type="hidden"
          name={name}
          required={required}
          value={internalValue ?? ""}
        />
      ) : null}
      <button
        type="button"
        id={id}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        onClick={handleToggle}
        className={["hierarchical-select-trigger", INPUT, SS_TRIGGER].join(" ")}
      >
        <span
          className={[SS_LABEL, "text-[0.95rem]"].join(" ")}
          data-empty={!hasValue ? "true" : "false"}
        >
          {selectedLabel}
        </span>
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
              className={`hierarchical-select-popover ${SS_POPOVER_HIER}`}
              style={{
                position: "fixed",
                top: pos.top,
                bottom: pos.bottom,
                left: pos.left,
                width: pos.width,
                zIndex: 9999,
              }}
            >
              <div className={SS_SEARCH}>
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={searchPlaceholder}
                  className={["hierarchical-select-input", INPUT, SS_INPUT].join(" ")}
                />
              </div>
              <div className={SS_LIST_HIER} style={{ maxHeight: pos.listMaxHeight }}>
                {showEmptyOption ? (
                  <button
                    type="button"
                    onClick={() => handleSelect({ label: emptyOptionLabel ?? "", value: "" })}
                    className={["hierarchical-select-item", TS_ITEM, !hasValue ? SS_ITEM_ACTIVE : ""]
                      .filter(Boolean)
                      .join(" ")}
                  >
                    <span className={TS_CONTENT}>
                      <span className={TS_TITLE_ROW}>
                        <span className={`${TS_TITLE} italic text-muted`}>
                          {emptyOptionLabel}
                        </span>
                      </span>
                    </span>
                    {!hasValue ? <SelectCheckIcon /> : null}
                  </button>
                ) : null}
                {filtered.map((option) => {
                  const depth = option.depth ?? 0;
                  const isChild = depth > 0;
                  const isActive = option.value === internalValue;
                  const guideWidth = depth * 18;
                  const connectorLeft = Math.max(0, (depth - 1) * 18 + 8);
                  const pathText =
                    option.path && option.path.length > 1
                      ? option.path.join(" > ")
                      : undefined;
                  const guideCls = !isChild
                    ? TS_GUIDE
                    : `${TS_GUIDE} ${TS_GUIDE_LINES} ${
                        isActive
                          ? TS_LINES_ACTIVE
                          : option.disabled
                            ? "before:border-hairline after:border-hairline"
                            : TS_LINES_IDLE
                      }`;
                  const titleCls = `${TS_TITLE} ${
                    isActive
                      ? "font-semibold text-ink"
                      : option.disabled
                        ? "font-medium text-muted"
                        : "font-medium text-heading"
                  }`;
                  return (
                    <button
                      key={`${option.value}-${option.label}`}
                      type="button"
                      disabled={option.disabled}
                      aria-disabled={option.disabled}
                      onClick={() => handleSelect(option)}
                      style={
                        {
                          "--tree-guide-width": `${guideWidth}px`,
                          "--tree-connector-left": `${connectorLeft}px`,
                        } as CSSProperties
                      }
                      className={["hierarchical-select-item", TS_ITEM, isActive ? "bg-brand/10" : ""]
                        .filter(Boolean)
                        .join(" ")}
                    >
                      <span className={guideCls} aria-hidden="true" />
                      <span className={TS_CONTENT}>
                        <span className={TS_TITLE_ROW}>
                          <span className={titleCls}>{option.label}</span>
                          {option.code ? (
                            <span className="inline-flex min-h-[20px] max-w-[120px] flex-none items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-hairline bg-surface-subtle px-[7px] py-0.5 text-[0.72rem] font-semibold leading-[1.2] text-muted">
                              {option.code}
                            </span>
                          ) : null}
                          {option.isGroup ? (
                            <span className={TS_GROUP_BADGE}>Grup</span>
                          ) : null}
                        </span>
                        {option.description ? (
                          <span className={TS_PATH}>{option.description}</span>
                        ) : null}
                        {!option.description && pathText ? (
                          <span className={TS_PATH}>{pathText}</span>
                        ) : null}
                        {option.disabledReason ? (
                          <span className={TS_PATH}>{option.disabledReason}</span>
                        ) : null}
                      </span>
                      {isActive ? <SelectCheckIcon /> : null}
                    </button>
                  );
                })}
                {filtered.length === 0 && !showEmptyOption ? (
                  <div className={SS_EMPTY}>{emptyText}</div>
                ) : null}
              </div>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
