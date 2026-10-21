import { useState } from "react";
import type { CSSProperties } from "react";
import { INPUT } from "./field-classes";
import { LIST_MAX_HEIGHT, SelectPopover, SelectPopoverContent, SelectPopoverTrigger } from "./select-popover";
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

  function handleOpenChange(next: boolean) {
    if (disabled) return;
    setOpen(next);
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
    <div className={["hierarchical-select", "relative w-full min-w-0", className].filter(Boolean).join(" ")}>
      {name ? (
        <input
          type="hidden"
          name={name}
          required={required}
          value={internalValue ?? ""}
        />
      ) : null}
      <SelectPopover open={open && !disabled} onOpenChange={handleOpenChange}>
      <SelectPopoverTrigger asChild>
      <button
        type="button"
        id={id}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
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
      </SelectPopoverTrigger>

      <SelectPopoverContent className={`hierarchical-select-popover ${SS_POPOVER_HIER}`}>
              <div className={SS_SEARCH}>
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={searchPlaceholder}
                  className={["hierarchical-select-input", INPUT, SS_INPUT].join(" ")}
                />
              </div>
              <div className={SS_LIST_HIER} style={{ maxHeight: LIST_MAX_HEIGHT }}>
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
                        <span className={`${TS_TITLE} italic text-muted-foreground`}>
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
                            ? "before:border-border after:border-border"
                            : TS_LINES_IDLE
                      }`;
                  const titleCls = `${TS_TITLE} ${
                    isActive
                      ? "font-semibold text-foreground"
                      : option.disabled
                        ? "font-medium text-muted-foreground"
                        : "font-medium text-foreground"
                  }`;
                  return (
                    <button
                      key={`${option.value}-${option.label}`}
                      type="button"
                      data-select-option=""
                      disabled={option.disabled}
                      aria-disabled={option.disabled}
                      onClick={() => handleSelect(option)}
                      style={
                        {
                          "--tree-guide-width": `${guideWidth}px`,
                          "--tree-connector-left": `${connectorLeft}px`,
                        } as CSSProperties
                      }
                      className={["hierarchical-select-item", TS_ITEM, isActive ? "bg-primary/10" : ""]
                        .filter(Boolean)
                        .join(" ")}
                    >
                      <span className={guideCls} aria-hidden="true" />
                      <span className={TS_CONTENT}>
                        <span className={TS_TITLE_ROW}>
                          <span className={titleCls}>{option.label}</span>
                          {option.code ? (
                            <span className="inline-flex min-h-[20px] max-w-[120px] flex-none items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-border bg-background px-[7px] py-0.5 text-[0.72rem] font-semibold leading-[1.2] text-muted-foreground">
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
      </SelectPopoverContent>
      </SelectPopover>
    </div>
  );
}
