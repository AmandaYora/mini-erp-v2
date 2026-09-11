import type { ReactNode } from "react";

/**
 * Kelas Tailwind + tipe + helper murni untuk trio select
 * (sync / hierarchical / async). Diport dari aplikasi lama (select-shared) —
 * token identik agar tampilan sama. Domain-agnostik: tanpa pengetahuan
 * party/produk/lokasi.
 *
 * File ini SENGAJA tanpa komponen (nol JSX): konstanta bersama agar tidak
 * memicu react-refresh/only-export-components. Ikon centang ada di
 * select-check-icon.tsx.
 */

export type SelectValue = string | number;

export interface SharedSelectOption {
  value: SelectValue;
  label: string;
  description?: ReactNode;
  code?: ReactNode;
  depth?: number;
  disabled?: boolean;
  disabledReason?: ReactNode;
  isGroup?: boolean;
  path?: string[];
  searchText?: string;
}

export const SS_TRIGGER =
  "flex w-full cursor-pointer items-center justify-between overflow-hidden whitespace-nowrap bg-surface text-left";
export const SS_LABEL =
  "min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap data-[empty=true]:text-muted data-[empty=false]:text-ink";
export const SS_CHEVRON = "ml-2 shrink-0 text-muted transition-transform";
export const SS_POPOVER =
  "flex max-w-[calc(100vw-24px)] flex-col rounded-lg border border-hairline bg-surface shadow-lg animate-[slideDown_0.15s_ease-out_forwards]";
export const SS_POPOVER_HIER = `${SS_POPOVER} min-w-[min(360px,calc(100vw-24px))]`;
export const SS_SEARCH = "border-b border-hairline p-2";
export const SS_INPUT = "!px-2.5 !py-1.5 !text-[0.85rem] !shadow-none";
export const SS_LIST = "flex flex-col overflow-y-auto p-1";
export const SS_LIST_HIER = `${SS_LIST} gap-0.5`;
export const SS_ITEM =
  "flex w-full items-center justify-between gap-2 rounded-md border-0 bg-transparent px-3 py-2 text-left text-[0.9rem] text-heading transition-all enabled:hover:bg-surface-subtle enabled:hover:text-ink disabled:cursor-not-allowed disabled:opacity-[0.55]";
export const SS_ITEM_ACTIVE = "bg-brand/10 font-medium text-brand";
export const SS_ITEM_MAIN = "flex min-w-0 flex-1 flex-col gap-0.5";
export const SS_ITEM_TITLE =
  "block min-w-0 whitespace-normal [overflow-wrap:break-word] [word-break:normal]";
export const SS_ITEM_DESC =
  "text-[0.76rem] font-normal leading-[1.3] text-muted [overflow-wrap:anywhere]";
export const SS_CODE =
  "inline-flex min-h-[20px] max-w-[120px] flex-none items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-hairline bg-surface-subtle px-[7px] py-0.5 text-[0.72rem] font-semibold leading-[1.2] text-muted";
export const SS_CHECK = "ml-1 shrink-0";
export const SS_EMPTY = "p-3 text-center text-[0.85rem] text-muted";
export const SS_MORE =
  "px-3 py-2 text-center text-[0.78rem] leading-[1.35] text-muted";

// Hierarchical (tree) — item pakai items-start + guide connector.
// Lebar guide & posisi konektor dinamis per-depth → via CSS var inline
// (--tree-guide-width / --tree-connector-left).
export const TS_ITEM =
  "group flex min-h-[40px] w-full items-start justify-between gap-2 rounded-md border-0 bg-transparent py-[7px] pl-2.5 pr-3 text-left text-[0.9rem] transition-all disabled:cursor-not-allowed enabled:hover:bg-surface-subtle";
export const TS_GUIDE =
  "relative min-h-[26px] shrink-0 grow-0 basis-[var(--tree-guide-width)] self-stretch";
export const TS_GUIDE_LINES =
  "before:absolute before:bottom-1/2 before:left-[var(--tree-connector-left)] before:top-[-8px] before:border-l before:opacity-[0.85] before:content-[''] " +
  "after:absolute after:left-[var(--tree-connector-left)] after:top-[calc(50%-1px)] after:w-[13px] after:border-t after:opacity-[0.85] after:content-['']";
export const TS_LINES_IDLE =
  "before:border-hairline after:border-hairline group-hover:before:border-brand group-hover:after:border-brand";
export const TS_LINES_ACTIVE = "before:border-brand after:border-brand";
export const TS_CONTENT = "flex min-w-0 flex-1 flex-col gap-[3px]";
export const TS_TITLE_ROW = "flex min-w-0 flex-wrap items-start gap-x-[7px] gap-y-1";
export const TS_TITLE =
  "min-w-0 flex-[1_1_160px] leading-[1.25] [overflow-wrap:break-word] [word-break:normal]";
export const TS_PATH =
  "text-[0.74rem] font-normal leading-[1.3] text-muted [overflow-wrap:anywhere]";
export const TS_GROUP_BADGE =
  "inline-flex min-h-[20px] max-w-[120px] flex-none items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-hairline bg-surface-sunken px-[7px] py-0.5 text-[0.68rem] font-semibold uppercase leading-[1.2] text-muted";

function textFromNode(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join(" ");
  return "";
}

export function getOptionSearchText(option: SharedSelectOption): string {
  return [
    option.value,
    option.label,
    textFromNode(option.description),
    textFromNode(option.code),
    option.path?.join(" "),
    option.searchText,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export function getOptionDisplayLabel(
  option: SharedSelectOption | undefined,
  placeholder: string,
): string {
  if (!option) return placeholder;
  if (option.path && option.path.length > 0) return option.path.join(" > ");
  return option.label;
}
