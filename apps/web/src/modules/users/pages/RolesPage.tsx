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
  SectionCard,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { roleService } from "@/modules/users/services/role.service";
import {
  createRoleSchema,
  editRoleSchema,
} from "@/modules/users/schemas/role.schema";
import type { PermissionItem, RoleItem } from "@/modules/users/types";

type FormMode = { kind: "create" } | { kind: "edit"; role: RoleItem };

export default function RolesPage() {
  const can = useAuthStore((s) => s.can);
  // Halaman dikunci roles.manage di registry; seluruh mutasi juga mensyaratkan
  // roles.manage sesuai routes.go backend.
  const canManage = can("roles.manage");

  const [formMode, setFormMode] = useState<FormMode | null>(null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<RoleItem | null>(null);

  const [permRole, setPermRole] = useState<RoleItem | null>(null);
  const [catalog, setCatalog] = useState<PermissionItem[]>([]);
  const [checked, setChecked] = useState<string[]>([]);
  const [loadingCatalog, setLoadingCatalog] = useState(false);
  const [savingPerms, setSavingPerms] = useState(false);

  const { data, loading, reload } = useAsyncData(
    async () => {
      try {
        return await roleService.list();
      } catch (err) {
        toast.danger("Gagal memuat role", toApiError(err).message);
        throw err;
      }
    },
    [],
  );
  const roles = data ?? [];

  function openCreate() {
    setCode("");
    setName("");
    setDescription("");
    setFormErrors({});
    setFormMode({ kind: "create" });
  }

  function openEdit(role: RoleItem) {
    if (role.isSystem) return;
    setCode(role.code);
    setName(role.name);
    setDescription(role.description ?? "");
    setFormErrors({});
    setFormMode({ kind: "edit", role });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!formMode) return;
    const errs: Record<string, string> = {};
    if (formMode.kind === "create") {
      const parsed = createRoleSchema.safeParse({ code, name, description });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          errs[String(issue.path[0] ?? "")] ??= issue.message;
        }
      }
    } else {
      const parsed = editRoleSchema.safeParse({ name, description });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          errs[String(issue.path[0] ?? "")] ??= issue.message;
        }
      }
    }
    if (Object.keys(errs).length > 0) {
      setFormErrors(errs);
      return;
    }
    setFormErrors({});
    setSaving(true);
    try {
      if (formMode.kind === "create") {
        const res = await roleService.create({
          code: code.trim(),
          name: name.trim(),
          description: description.trim(),
        });
        toast.fromServer(res.message, "Role dibuat");
      } else {
        const res = await roleService.update(formMode.role.id, {
          name: name.trim(),
          description: description.trim(),
        });
        toast.fromServer(res.message, "Role disimpan");
      }
      setFormMode(null);
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field) next[fe.field] ??= fe.message;
      }
      setFormErrors(next);
      toast.danger("Gagal menyimpan role", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget) return;
    try {
      const res = await roleService.remove(deleteTarget.id);
      toast.fromServer(res.message, "Role dihapus");
      setDeleteTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal menghapus role", toApiError(err).message);
    }
  }

  async function openPermissions(role: RoleItem) {
    setPermRole(role);
    setChecked([]);
    setLoadingCatalog(true);
    try {
      setCatalog(await roleService.permissionCatalog());
    } catch (err) {
      toast.danger("Gagal memuat katalog permission", toApiError(err).message);
      setCatalog([]);
    } finally {
      setLoadingCatalog(false);
    }
  }

  function togglePerm(code: string) {
    setChecked((prev) =>
      prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code],
    );
  }

  async function handleSavePermissions() {
    if (!permRole) return;
    setSavingPerms(true);
    try {
      // Key body `permissions` — diverifikasi di SetRolePermissions handler.go.
      const res = await roleService.setPermissions(permRole.id, checked);
      toast.fromServer(res.message, "Izin role disimpan");
      setPermRole(null);
    } catch (err) {
      toast.danger("Gagal menyimpan izin", toApiError(err).message);
    } finally {
      setSavingPerms(false);
    }
  }

  const isEdit = formMode?.kind === "edit";

  return (
    <div>
      <PageHeader
        eyebrow="Pengaturan"
        title="Role"
        description="Kelola role dan izin akses pengguna."
        actions={
          canManage ? <Button onClick={openCreate}>Tambah Role</Button> : undefined
        }
      />

      <SectionCard>
        {loading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat data role…</p>
        ) : (
          <DataTable<RoleItem>
            columns={[
              { header: "Kode", render: (r) => r.code },
              { header: "Nama", render: (r) => r.name },
              {
                header: "Deskripsi",
                render: (r) => r.description || "-",
              },
              {
                header: "Tipe",
                render: (r) =>
                  r.isSystem ? (
                    <Badge tone="info">Sistem</Badge>
                  ) : (
                    <Badge tone="neutral">Kustom</Badge>
                  ),
              },
              {
                header: "Aksi",
                align: "right",
                render: (r) =>
                  canManage ? (
                    <div className="flex justify-end gap-2">
                      {!r.isSystem && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => openEdit(r)}
                        >
                          Ubah
                        </Button>
                      )}
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => void openPermissions(r)}
                      >
                        Izin
                      </Button>
                      {!r.isSystem && (
                        <Button
                          variant="danger"
                          size="sm"
                          onClick={() => setDeleteTarget(r)}
                        >
                          Hapus
                        </Button>
                      )}
                    </div>
                  ) : (
                    <span className="text-muted text-sm">-</span>
                  ),
              },
            ]}
            rows={roles}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada role"
            emptyDescription="Tambah role kustom sesuai kebutuhan akses."
          />
        )}
      </SectionCard>

      <Modal
        open={formMode !== null}
        title={isEdit ? "Ubah Role" : "Tambah Role"}
        description={
          isEdit
            ? "Kode role tidak dapat diubah setelah dibuat."
            : "Kode role tidak dapat diubah setelah dibuat."
        }
        size="md"
        onClose={() => {
          if (!saving) setFormMode(null);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={() => setFormMode(null)}
              disabled={saving}
            >
              Batal
            </Button>
            <Button type="submit" form="role-form" disabled={saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="role-form" onSubmit={(e) => void handleSubmit(e)}>
          <div className="space-y-4">
            <FormField
              label="Kode"
              required={!isEdit}
              helperText={
                isEdit
                  ? "Kode dikunci — tidak dapat diubah."
                  : "Huruf/angka/titik/strip/underscore, maks 64 karakter."
              }
              errorText={formErrors.code}
            >
              <TextInput
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="cth: kasir"
                disabled={isEdit}
              />
            </FormField>
            <FormField label="Nama" required errorText={formErrors.name}>
              <TextInput
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="cth: Kasir"
              />
            </FormField>
            <FormField label="Deskripsi" errorText={formErrors.description}>
              <TextArea
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Keterangan singkat role (opsional)"
              />
            </FormField>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Hapus Role"
        message={`Hapus role "${deleteTarget?.code}"? Role yang masih dipakai pengguna tidak dapat dihapus.`}
        confirmLabel="Hapus"
        onConfirm={() => void handleDeleteConfirm()}
        onCancel={() => setDeleteTarget(null)}
      />

      <Modal
        open={permRole !== null}
        title={permRole ? `Izin — ${permRole.code}` : "Izin Role"}
        description="Menyimpan akan mengganti seluruh izin role ini."
        size="lg"
        onClose={() => {
          if (!savingPerms) setPermRole(null);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={() => setPermRole(null)}
              disabled={savingPerms}
            >
              Batal
            </Button>
            <Button
              onClick={() => void handleSavePermissions()}
              disabled={savingPerms || loadingCatalog}
            >
              {savingPerms ? "Menyimpan…" : "Simpan Izin"}
            </Button>
          </>
        }
      >
        <div className="mb-4">
          <Notice tone="warning" title="Izin saat ini tidak dapat dibaca">
            API belum menyediakan endpoint bacaan izin per role, jadi daftar
            mulai kosong. Centang lengkap sesuai kebutuhan sebelum menyimpan
            agar izin lama tidak hilang.
          </Notice>
        </div>
        {loadingCatalog ? (
          <p className="text-muted text-sm">Memuat katalog permission…</p>
        ) : catalog.length === 0 ? (
          <p className="text-muted text-sm">Katalog permission kosong.</p>
        ) : (
          <>
            <div className="mb-3 flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setChecked(catalog.map((p) => p.code))}
              >
                Pilih Semua
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setChecked([])}>
                Bersihkan
              </Button>
              <span className="text-muted ml-auto self-center text-sm">
                {checked.length} dipilih
              </span>
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              {catalog.map((p) => (
                <label
                  key={p.code}
                  className="flex cursor-pointer items-start gap-2 rounded-md border border-hairline px-3 py-2 text-sm text-ink"
                >
                  <input
                    type="checkbox"
                    className="accent-brand mt-0.5 h-4 w-4"
                    checked={checked.includes(p.code)}
                    onChange={() => togglePerm(p.code)}
                  />
                  <span>
                    <span className="block font-medium">{p.name}</span>
                    <span className="text-muted block text-xs">{p.code}</span>
                  </span>
                </label>
              ))}
            </div>
          </>
        )}
      </Modal>
    </div>
  );
}
