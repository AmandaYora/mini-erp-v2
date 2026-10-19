import { useMemo, useState } from "react";
import { ChevronLeft } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import type { PosCategory } from "@/modules/pos/types";

// Drilldown katalog per kategori (legacy: pos-category-drilldown.tsx).
// Kategori revamp datar + parentId (categoryView: id/code/name/parentId/status)
// sehingga pohon disusun di klien. "Semua" = tanpa filter kategori.
// Chip 44px, bergulir horizontal.
interface CategoryNode extends PosCategory {
  children: CategoryNode[];
}

function buildTree(categories: PosCategory[], parentId: number | null): CategoryNode[] {
  return categories
    .filter((c) => (c.parentId ?? null) === parentId)
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((c) => ({ ...c, children: buildTree(categories, c.id) }));
}

function findPath(nodes: CategoryNode[], id: number | null, trail: CategoryNode[] = []): CategoryNode[] | null {
  if (id === null) return trail;
  for (const n of nodes) {
    if (n.id === id) return [...trail, n];
    const hit = findPath(n.children, id, [...trail, n]);
    if (hit) return hit;
  }
  return null;
}

const CHIP =
  "flex h-11 flex-none touch-manipulation items-center gap-2 rounded-full px-4 text-sm font-semibold whitespace-nowrap transition-[background-color,transform] duration-100 select-none active:scale-[0.98]";

export function PosCategoryDrilldown({
  categories,
  value,
  onChange,
}: {
  categories: PosCategory[];
  value: number | null;
  onChange: (id: number | null) => void;
}) {
  const active = useMemo(() => categories.filter((c) => c.status === "active"), [categories]);
  const tree = useMemo(() => buildTree(active, null), [active]);
  const [parentId, setParentId] = useState<number | null>(null);

  const path = useMemo(() => findPath(tree, parentId) ?? [], [tree, parentId]);
  const level: CategoryNode[] = useMemo(() => {
    if (parentId === null) return tree;
    const node = (function find(nodes: CategoryNode[]): CategoryNode | null {
      for (const n of nodes) {
        if (n.id === parentId) return n;
        const hit = find(n.children);
        if (hit) return hit;
      }
      return null;
    })(tree);
    return node ? node.children : tree;
  }, [tree, parentId]);

  if (active.length === 0) return null;

  function select(node: CategoryNode) {
    onChange(node.id);
    if (node.children.length > 0) setParentId(node.id);
  }

  return (
    <div role="group" aria-label="Kategori produk" className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto py-0.5">
      <button
        type="button"
        aria-pressed={value === null}
        onClick={() => {
          setParentId(null);
          onChange(null);
        }}
        className={cn(CHIP, value === null ? "bg-primary text-primary-foreground" : "border border-input bg-card text-foreground")}
      >
        Semua
      </button>
      {parentId !== null && (
        <button
          type="button"
          onClick={() => {
            const up = path.length >= 2 ? path[path.length - 2].id : null;
            setParentId(up);
            onChange(up);
          }}
          className={cn(CHIP, "border border-input bg-card text-foreground")}
        >
          <ChevronLeft aria-hidden className="size-4" />
          {path.length >= 2 ? path[path.length - 2].name : "Naik"}
        </button>
      )}
      {level.map((node) => {
        const selected = node.id === value;
        return (
          <button
            key={node.id}
            type="button"
            aria-pressed={selected}
            onClick={() => select(node)}
            className={cn(
              CHIP,
              selected ? "bg-accent text-accent-foreground ring-2 ring-primary" : "border border-input bg-card text-foreground",
            )}
          >
            {node.name}
            {node.children.length > 0 && (
              <span className="flex size-5 items-center justify-center rounded-full bg-muted text-2xs font-semibold text-muted-foreground">
                {node.children.length}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
