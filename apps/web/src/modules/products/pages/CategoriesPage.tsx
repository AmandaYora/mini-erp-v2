import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Archive, ChevronDown, ChevronRight, SquarePen } from "lucide-react";
import {
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  FormSheet,
  PageHeader,
  Pagination,
  SearchSelect,
  SectionCard,
  StatusBadge,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, DocNumber, Page } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import { buildCategoryRows, parentIds } from "@/modules/products/lib/category-tree";
import type { CategoryTreeRow } from "@/modules/products/lib/category-tree";
import {
  categoryCreateSchema,
  categoryUpdateSchema,
} from "@/modules/products/schemas/category.schema";
import { productsService } from "@/modules/products/services/products.service";
import type { ProductCategory } from "@/modules/products/types";

const LIMIT = 20;
const FORM_ID = "category-form";

interface SheetState {
  mode: "create" | "edit";
  target: ProductCategory | null;
}

export default function CategoriesPage() {
  const can = useAuthStore((s) => s.can);
  const manageable = can("product_categories.manage");

  const [sheet, setSheet] = useState<SheetState | null>(null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [sortOrder, setSortOrder] = useState("0");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [archiveTarget, setArchiveTarget] = useState<ProductCategory | null>(null);
  const [archiving, setArchiving] = useState(false);
  const [collapsed, setCollapsed] = useState<ReadonlySet<number>>(new Set());
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(() => productsService.listCategories(), []);
  const items = useMemo(() => data ?? [], [data]);

  // Pohon diratakan lalu dipaginasi di klien (endpoint mengembalikan seluruh kategori).
  const treeRows = useMemo(() => buildCategoryRows(items, collapsed), [items, collapsed]);
  const totalPages = Math.max(1, Math.ceil(treeRows.length / LIMIT));
  const safePage = Math.min(page, totalPages);
  const pageRows = treeRows.slice((safePage - 1) * LIMIT, safePage * LIMIT);
  const hasParents = useMemo(() => parentIds(items).length > 0, [items]);
  const allCollapsed = hasParents && parentIds(items).every((id) => collapsed.has(id));

  function toggleNode(id: number) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function openCreate() {
    setCode("");
    setName("");
    setParentId(null);
    setSortOrder("0");
    setFieldErrors({});
    setSheet({ mode: "create", target: null });
  }

  function openEdit(target: ProductCategory) {
    setCode(target.code);
    setName(target.name);
    setParentId(target.parentId);
    setSortOrder(String(target.sortOrder ?? 0));
    setFieldErrors({});
    setSheet({ mode: "edit", target });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!sheet) return;
    if (sheet.mode === "create") {
      const parsed = categoryCreateSchema.safeParse({ code, name, parentId, sortOrder: Number(sortOrder.trim() === "" ? 0 : sortOrder) });
      if (!parsed.success) {
        const errs: Record<string, string> = {};
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "_");
          errs[key] ??= issue.message;
        }
        setFieldErrors(errs);
        return;
      }
      setFieldErrors({});
      setSaving(true);
      try {
        const res = await productsService.createCategory({
          code: parsed.data.code.trim(),
          name: parsed.data.name.trim(),
          parentId: parsed.data.parentId,
          sortOrder: parsed.data.sortOrder,
        });
        toast.fromServer(res.message, "Kategori dibuat");
        setSheet(null);
        reload();
      } catch (err) {
        toast.danger("Gagal membuat kategori", formatApiError(err));
      } finally {
        setSaving(false);
      }
    } else {
      const parsed = categoryUpdateSchema.safeParse({ name, parentId, sortOrder: Number(sortOrder.trim() === "" ? 0 : sortOrder) });
      if (!parsed.success) {
        const errs: Record<string, string> = {};
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "_");
          errs[key] ??= issue.message;
        }
        setFieldErrors(errs);
        return;
      }
      if (!sheet.target) return;
      if (parentId === sheet.target.id) {
        setFieldErrors({ parentId: "Kategori tidak boleh menjadi induk dirinya sendiri" });
        return;
      }
      setFieldErrors({});
      setSaving(true);
      try {
        const res = await productsService.updateCategory(sheet.target.id, {
          name: parsed.data.name.trim(),
          parentId: parsed.data.parentId,
          sortOrder: parsed.data.sortOrder,
        });
        toast.fromServer(res.message, "Kategori disimpan");
        setSheet(null);
        reload();
      } catch (err) {
        toast.danger("Gagal menyimpan kategori", formatApiError(err));
      } finally {
        setSaving(false);
      }
    }
  }

  async function handleArchive() {
    if (!archiveTarget) return;
    setArchiving(true);
    try {
      const res = await productsService.archiveCategory(archiveTarget.id);
      toast.fromServer(res.message, "Kategori diarsipkan");
      setArchiveTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal mengarsipkan kategori", formatApiError(err));
    } finally {
      setArchiving(false);
    }
  }

  const parentOptions = (sheet?.mode === "edit" && sheet.target
    ? items.filter((c) => c.id !== sheet.target!.id)
    : items
  ).map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }));

  const addButton = manageable ? <Button onClick={openCreate}>Tambah kategori</Button> : undefined;

  return (
    <Page>
      <PageHeader
        title="Kategori produk"
        description="Kelompok produk, opsional bertingkat via induk."
        actions={
          <>
            {hasParents && (
              <Button
                variant="secondary"
                onClick={() => {
                  setCollapsed(allCollapsed ? new Set() : new Set(parentIds(items)));
                  setPage(1);
                }}
              >
                {allCollapsed ? "Buka semua" : "Lipat semua"}
              </Button>
            )}
            {addButton}
          </>
        }
      />

      {!manageable && (
        <Notice tone="info" title="Mode baca">
          Anda hanya dapat melihat kategori — pengelolaan butuh izin product_categories.manage.
        </Notice>
      )}

      <SectionCard flush>
        <DataTable<CategoryTreeRow>
          caption="Pohon kategori produk"
          rows={loading && data === null ? null : pageRows}
          rowKey={(r) => r.category.id}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada kategori"
          emptyDescription="Buat kategori pertama untuk mengelompokkan produk."
          emptyAction={addButton}
          columns={[
            {
              header: "Nama",
              render: (r) => (
                <div className="flex items-center gap-1" style={{ paddingLeft: `${r.depth * 1.5}rem` }}>
                  {r.hasChildren ? (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-expanded={!collapsed.has(r.category.id)}
                      aria-label={`${collapsed.has(r.category.id) ? "Buka" : "Lipat"} ${r.category.name}`}
                      onClick={() => toggleNode(r.category.id)}
                    >
                      {collapsed.has(r.category.id) ? <ChevronRight aria-hidden /> : <ChevronDown aria-hidden />}
                    </Button>
                  ) : (
                    <span aria-hidden className="inline-block size-7 flex-none" />
                  )}
                  <span className={r.depth === 0 ? "font-medium" : undefined}>{r.category.name}</span>
                </div>
              ),
            },
            { header: "Kode", render: (r) => <DocNumber value={r.category.code} /> },
            { header: "Urutan", align: "right" as const, render: (r) => r.category.sortOrder ?? 0 },
            { header: "Status", render: (r) => <StatusBadge status={r.category.status} /> },
            ...(manageable
              ? [
                  {
                    header: "Aksi",
                    width: 56,
                    align: "right" as const,
                    render: (r: CategoryTreeRow) =>
                      r.category.status !== "archived" ? (
                        <ActionMenu
                          variant="ghost"
                          label={`Aksi untuk ${r.category.name}`}
                          items={[
                            {
                              key: "edit",
                              label: "Ubah",
                              icon: <SquarePen aria-hidden />,
                              onSelect: () => openEdit(r.category),
                            },
                            {
                              key: "archive",
                              label: "Arsipkan",
                              icon: <Archive aria-hidden />,
                              destructive: true,
                              onSelect: () => setArchiveTarget(r.category),
                            },
                          ]}
                        />
                      ) : (
                        <span className="text-muted-foreground">–</span>
                      ),
                  },
                ]
              : []),
          ]}
        />
        {!error && (
          <Pagination page={safePage} limit={LIMIT} total={treeRows.length} onPageChange={setPage} />
        )}
      </SectionCard>

      <FormSheet
        open={sheet !== null}
        title={sheet?.mode === "edit" ? "Ubah kategori" : "Tambah kategori"}
        description={sheet?.mode === "edit" ? "Kode tidak dapat diubah." : undefined}
        onClose={() => setSheet(null)}
        actions={
          <>
            <Button variant="secondary" onClick={() => setSheet(null)} disabled={saving}>
              Batal
            </Button>
            <Button type="submit" form={FORM_ID} loading={saving}>
              {sheet?.mode === "edit" ? "Simpan" : "Buat kategori"}
            </Button>
          </>
        }
      >
        <form id={FORM_ID} onSubmit={handleSubmit} className="space-y-4">
          <FormField label="Kode" required htmlFor="cat-code" errorText={fieldErrors.code}>
            <TextInput
              id="cat-code"
              value={code}
              disabled={sheet?.mode === "edit"}
              onChange={(e) => setCode(e.target.value)}
              placeholder="KTG-001"
            />
          </FormField>
          <FormField label="Nama" required htmlFor="cat-name" errorText={fieldErrors.name}>
            <TextInput
              id="cat-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nama kategori"
            />
          </FormField>
          <FormField label="Induk" errorText={fieldErrors.parentId}>
            <SearchSelect
              options={parentOptions}
              value={parentId}
              onChange={(v) => setParentId(typeof v === "number" ? v : null)}
              placeholder="Tanpa induk"
              allowClear
            />
          </FormField>
          <FormField
            label="Urutan tampil"
            htmlFor="cat-sort"
            helperText="Angka kecil tampil lebih dulu di daftar dan pemilih kategori."
            errorText={fieldErrors.sortOrder}
          >
            <TextInput
              id="cat-sort"
              type="number"
              inputMode="numeric"
              step={1}
              value={sortOrder}
              onChange={(e) => setSortOrder(e.target.value)}
            />
          </FormField>
        </form>
      </FormSheet>

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan kategori?"
        message={`Kategori ${archiveTarget?.code} — ${archiveTarget?.name} akan diarsipkan. Hanya kategori kosong yang bisa diarsipkan.`}
        confirmLabel={archiving ? "Mengarsipkan…" : "Arsipkan"}
        onConfirm={() => {
          if (!archiving) void handleArchive();
        }}
        onCancel={() => setArchiveTarget(null)}
      />
    </Page>
  );
}
