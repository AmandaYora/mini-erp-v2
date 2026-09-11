import { useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  Modal,
  PageHeader,
  SearchSelect,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import {
  categoryCreateSchema,
  categoryUpdateSchema,
} from "@/modules/products/schemas/category.schema";
import { productsService } from "@/modules/products/services/products.service";
import type { ProductCategory } from "@/modules/products/types";

interface ModalState {
  mode: "create" | "edit";
  target: ProductCategory | null;
}

export default function CategoriesPage() {
  const can = useAuthStore((s) => s.can);
  const manageable = can("product_categories.manage");

  const [modal, setModal] = useState<ModalState | null>(null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [archiveTarget, setArchiveTarget] = useState<ProductCategory | null>(null);
  const [archiving, setArchiving] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () => productsService.listCategories(),
    [],
  );
  const items = data ?? [];

  function parentName(parentIdValue: number | null): string {
    if (parentIdValue === null) return "-";
    return items.find((c) => c.id === parentIdValue)?.name ?? `#${parentIdValue}`;
  }

  function openCreate() {
    setCode("");
    setName("");
    setParentId(null);
    setFieldErrors({});
    setModal({ mode: "create", target: null });
  }

  function openEdit(target: ProductCategory) {
    setCode(target.code);
    setName(target.name);
    setParentId(target.parentId);
    setFieldErrors({});
    setModal({ mode: "edit", target });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!modal) return;
    if (modal.mode === "create") {
      const parsed = categoryCreateSchema.safeParse({ code, name, parentId });
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
        });
        toast.fromServer(res.message, "Kategori dibuat");
        setModal(null);
        reload();
      } catch (err) {
        toast.danger("Gagal membuat kategori", toApiError(err).message);
      } finally {
        setSaving(false);
      }
    } else {
      const parsed = categoryUpdateSchema.safeParse({ name, parentId });
      if (!parsed.success) {
        const errs: Record<string, string> = {};
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "_");
          errs[key] ??= issue.message;
        }
        setFieldErrors(errs);
        return;
      }
      if (!modal.target) return;
      if (parentId === modal.target.id) {
        setFieldErrors({ parentId: "Kategori tidak boleh menjadi induk dirinya sendiri" });
        return;
      }
      setFieldErrors({});
      setSaving(true);
      try {
        const res = await productsService.updateCategory(modal.target.id, {
          name: parsed.data.name.trim(),
          parentId: parsed.data.parentId,
        });
        toast.fromServer(res.message, "Kategori disimpan");
        setModal(null);
        reload();
      } catch (err) {
        toast.danger("Gagal menyimpan kategori", toApiError(err).message);
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
      toast.danger("Gagal mengarsipkan kategori", toApiError(err).message);
    } finally {
      setArchiving(false);
    }
  }

  const parentOptions = (modal?.mode === "edit" && modal.target
    ? items.filter((c) => c.id !== modal.target!.id)
    : items
  ).map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }));

  return (
    <div>
      <PageHeader
        eyebrow="Master"
        title="Kategori Produk"
        description="Kelompok produk, opsional bertingkat via induk."
        actions={
          manageable ? <Button onClick={openCreate}>Tambah</Button> : undefined
        }
      />

      {!manageable && (
        <div className="mb-4">
          <Notice tone="info" title="Mode baca">
            Anda hanya dapat melihat kategori — pengelolaan butuh izin product_categories.manage.
          </Notice>
        </div>
      )}

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat kategori">
            {error}
          </Notice>
        </div>
      )}

      <div className="rounded-lg border border-hairline bg-surface">
        {loading ? (
          <p className="text-muted px-6 py-10 text-center text-sm">Memuat…</p>
        ) : (
          <DataTable<ProductCategory>
            columns={[
              { header: "Kode", render: (r) => <span className="font-medium">{r.code}</span> },
              { header: "Nama", render: (r) => r.name },
              { header: "Induk", render: (r) => parentName(r.parentId) },
              {
                header: "Status",
                render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>,
              },
              {
                header: "Aksi",
                align: "right",
                render: (r) =>
                  manageable && r.status !== "archived" ? (
                    <div className="flex justify-end gap-2">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(r)}>
                        Ubah
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setArchiveTarget(r)}>
                        Arsipkan
                      </Button>
                    </div>
                  ) : (
                    <span className="text-muted">-</span>
                  ),
              },
            ]}
            rows={items}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada kategori"
            emptyDescription="Buat kategori pertama untuk mengelompokkan produk."
          />
        )}
      </div>

      <Modal
        open={modal !== null}
        title={modal?.mode === "edit" ? "Ubah Kategori" : "Tambah Kategori"}
        description={modal?.mode === "edit" ? "Kode tidak dapat diubah." : undefined}
        onClose={() => setModal(null)}
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <FormField label="Kode" required htmlFor="cat-code" errorText={fieldErrors.code}>
            <TextInput
              id="cat-code"
              value={code}
              disabled={modal?.mode === "edit"}
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
          <div>
            <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">Induk</span>
            <SearchSelect
              options={parentOptions}
              value={parentId}
              onChange={(v) => setParentId(typeof v === "number" ? v : null)}
              placeholder="Tanpa induk"
              allowClear
            />
            {fieldErrors.parentId && (
              <p className="text-bad mt-1 text-[0.82rem]">{fieldErrors.parentId}</p>
            )}
          </div>
          <div className="flex justify-end gap-3">
            <Button variant="secondary" size="sm" onClick={() => setModal(null)}>
              Batal
            </Button>
            <Button type="submit" size="sm" disabled={saving}>
              {saving ? "Menyimpan…" : modal?.mode === "edit" ? "Simpan" : "Buat"}
            </Button>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan kategori?"
        message={`Kategori ${archiveTarget?.code} — ${archiveTarget?.name} akan diarsipkan. Hanya kategori kosong yang bisa diarsipkan.`}
        confirmLabel={archiving ? "Mengarsipkan…" : "Arsipkan"}
        onConfirm={handleArchive}
        onCancel={() => setArchiveTarget(null)}
      />
    </div>
  );
}
