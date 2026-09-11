import type { StockLocation } from "@/modules/stock/types";
import type { SharedSelectOption } from "@/shared/components/ui";

export interface LocationNode extends StockLocation {
  depth: number;
  children: LocationNode[];
}

export const EMPTY_LOCATIONS: StockLocation[] = [];

export function getStockLocationLabel(
  locations: StockLocation[],
  targetId: number,
): string {
  const map = new Map(locations.map((l) => [l.id, l]));
  const parts: string[] = [];
  let current = map.get(targetId);
  while (current) {
    parts.unshift(current.name);
    current = current.parentId != null ? map.get(current.parentId) : undefined;
  }
  return parts.join(" > ");
}

function buildTree(
  locations: StockLocation[],
  parentId: number | null = null,
  depth = 0,
): LocationNode[] {
  return locations
    .filter((l) => (l.parentId ?? null) === parentId && l.status === "active")
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((l) => ({ ...l, depth, children: buildTree(locations, l.id, depth + 1) }));
}

function flatten(nodes: LocationNode[]): LocationNode[] {
  return nodes.flatMap((n) => [n, ...flatten(n.children)]);
}

/** Ratakan pohon lokasi menjadi opsi HierarchicalSelect (daun saja bisa dipilih). */
export function buildLocationOptions(
  locations: StockLocation[],
  excludeIds: number[],
): SharedSelectOption[] {
  return flatten(buildTree(locations)).map((l) => {
    const path = getStockLocationLabel(locations, l.id)
      .split(" > ")
      .filter(Boolean);
    const isLeaf = l.children.length === 0;
    const excluded = excludeIds.includes(l.id);
    return {
      value: l.id,
      label: l.name,
      code: l.code,
      depth: l.depth,
      disabled: !isLeaf || excluded,
      disabledReason: !isLeaf
        ? "Pilih lokasi paling bawah"
        : excluded
          ? "Tidak tersedia untuk pilihan ini"
          : undefined,
      isGroup: !isLeaf,
      path,
      searchText: [l.code, path.join(" ")].filter(Boolean).join(" "),
    };
  });
}
