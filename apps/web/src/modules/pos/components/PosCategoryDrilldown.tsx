import { useMemo, useState } from "react";
import type { PosCategory } from "@/modules/pos/types";

// Drilldown katalog per kategori (legacy: pos-category-drilldown.tsx).
// Kategori revamp datar + parentId (categoryView: id/code/name/parentId/status)
// sehingga pohon disusun di klien. "Semua" = tanpa filter kategori.
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

export function PosCategoryDrilldown({
  categories,
  value,
  onChange,
}: {
  categories: PosCategory[];
  value: number | null;
  onChange: (id: number | null) => void;
}) {
  const active = useMemo(
    () => categories.filter((c) => c.status === "active"),
    [categories],
  );
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

  function select(id: number | null, node?: CategoryNode) {
    onChange(id);
    if (node && node.children.length > 0) setParentId(node.id);
  }

  return (
    <div aria-label="Kategori produk" className="flex flex-col gap-2">
      <div className="flex flex-nowrap items-center gap-1.5 overflow-x-auto px-0.5 pb-0.5">
        <button
          type="button"
          onClick={() => {
            setParentId(null);
            onChange(null);
          }}
          className={`flex-none cursor-pointer rounded-full px-3.5 py-1.5 text-[0.82rem] font-bold transition-colors ${
            value === null
              ? "bg-brand text-white"
              : "bg-surface-subtle text-ink"
          }`}
        >
          Semua
        </button>
        {path.map((p) => (
          <span key={p.id} className="flex flex-none items-center gap-1.5">
            <span className="font-extrabold text-muted">/</span>
            <button
              type="button"
              onClick={() => {
                setParentId(p.id);
                onChange(p.id);
              }}
              className="cursor-pointer rounded-full bg-brand/10 px-3 py-1.5 text-[0.82rem] font-bold text-brand"
            >
              {p.name}
            </button>
          </span>
        ))}
        {parentId !== null && (
          <button
            type="button"
            onClick={() => {
              const up = path.length >= 2 ? path[path.length - 2].id : null;
              setParentId(up);
              onChange(up);
            }}
            className="ml-auto flex flex-none cursor-pointer items-center gap-1 rounded-full border border-hairline bg-surface-subtle px-3 py-1 text-[0.76rem] font-bold text-ink"
          >
            ← Naik
          </button>
        )}
      </div>
      {level.length > 0 && (
        <div className="flex gap-2 overflow-x-auto px-0.5 py-0.5">
          {level.map((node) => {
            const selected = node.id === value;
            return (
              <button
                key={node.id}
                type="button"
                onClick={() => select(node.id, node)}
                className={`flex flex-none cursor-pointer items-center gap-2 whitespace-nowrap rounded-full px-4 py-2 text-[0.82rem] font-bold transition-colors ${
                  selected
                    ? "border-2 border-brand bg-brand/10 text-brand"
                    : "border border-hairline bg-surface text-ink"
                }`}
              >
                {node.name}
                {node.children.length > 0 && (
                  <span
                    className={`flex h-5 w-5 items-center justify-center rounded-full text-[0.7rem] font-extrabold ${
                      selected ? "bg-brand text-white" : "bg-surface-subtle text-muted"
                    }`}
                  >
                    {node.children.length}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
