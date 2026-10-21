import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  Checkbox,
  ConfirmDialog,
  DataTable,
  FormField,
  FormSheet,
  PageHeader,
  SearchInput,
  SectionCard,
  Skeleton,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, Page } from "@/shared/components/patterns";
import type { ActionMenuItem } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { roleService } from "@/modules/users/services/role.service";
import {
  createRoleSchema,
  editRoleSchema,
} from "@/modules/users/schemas/role.schema";
import { groupPermissions, type PermissionGroup } from "@/modules/users/lib/permission-groups";
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
  // Izin yang tersimpan di server saat panel dibuka: dasar selisih dan konfirmasi.
  const [initial, setInitial] = useState<string[]>([]);
  const [locked, setLocked] = useState(false);
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const [permSearch, setPermSearch] = useState("");
  const [loadingCatalog, setLoadingCatalog] = useState(false);
  const [savingPerms, setSavingPerms] = useState(false);

  const { data, loading, error, reload } = useAsyncData(() => roleService.list(), []);
  const roles = data ?? [];

  const groups = useMemo(() => groupPermissions(catalog), [catalog]);
  const checkedSet = useMemo(() => new Set(checked), [checked]);
  const needle = permSearch.trim().toLowerCase();
  const visibleGroups = useMemo(() => {
    if (!needle) return groups;
    return groups
      .map((g) => ({
        ...g,
        items: g.items.filter(
          (p) => p.code.toLowerCase().includes(needle) || p.name.toLowerCase().includes(needle),
        ),
      }))
      .filter((g) => g.items.length > 0);
  }, [groups, needle]);

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
      toast.danger("Gagal menghapus role", formatApiError(err));
    }
  }

  async function openPermissions(role: RoleItem) {
    setPermRole(role);
    setChecked([]);
    setInitial([]);
    setLocked(false);
    setPermSearch("");
    setLoadingCatalog(true);
    try {
      const [cat, detail] = await Promise.all([roleService.permissionCatalog(), roleService.detail(role.id)]);
      setCatalog(cat);
      setChecked(detail.permissions);
      setInitial(detail.permissions);
      setLocked(detail.locked);
    } catch (err) {
      toast.danger("Gagal memuat izin role", toApiError(err).message);
      setCatalog([]);
      // Tanpa data nyata, jangan biarkan simpan menimpa izin yang tidak terbaca.
      setLocked(true);
    } finally {
      setLoadingCatalog(false);
    }
  }

  const added = useMemo(() => checked.filter((c) => !initial.includes(c)).length, [checked, initial]);
  const removed = useMemo(() => initial.filter((c) => !checkedSet.has(c)).length, [initial, checkedSet]);

  function togglePerm(permCode: string, on: boolean) {
    if (locked) return;
    setChecked((prev) => (on ? (prev.includes(permCode) ? prev : [...prev, permCode]) : prev.filter((c) => c !== permCode)));
  }

  function toggleModule(group: PermissionGroup, on: boolean) {
    if (locked) return;
    const codes = group.items.map((p) => p.code);
    setChecked((prev) => {
      const rest = prev.filter((c) => !codes.includes(c));
      return on ? [...rest, ...codes] : rest;
    });
  }

  function requestSavePermissions() {
    if (removed > 0) setConfirmRevoke(true);
    else void handleSavePermissions();
  }

  async function handleSavePermissions() {
    if (!permRole || locked) return;
    setConfirmRevoke(false);
    setSavingPerms(true);
    try {
      // Key body `permissions` — diverifikasi di SetRolePermissions handler.go.
      const res = await roleService.setPermissions(permRole.id, checked);
      toast.fromServer(res.message, "Izin role disimpan");
      setPermRole(null);
    } catch (err) {
      toast.danger("Gagal menyimpan izin", formatApiError(err));
    } finally {
      setSavingPerms(false);
    }
  }

  function rowActions(r: RoleItem): ActionMenuItem[] {
    const list: ActionMenuItem[] = [];
    if (!r.isSystem) list.push({ key: "edit", label: "Ubah", onSelect: () => openEdit(r) });
    list.push({ key: "perm", label: "Atur izin", onSelect: () => void openPermissions(r) });
    if (!r.isSystem) {
      list.push({ key: "delete", label: "Hapus", destructive: true, onSelect: () => setDeleteTarget(r) });
    }
    return list;
  }

  const isEdit = formMode?.kind === "edit";

  return (
    <Page>
      <PageHeader
        title="Role"
        description="Kelola role dan izin akses pengguna."
        actions={canManage ? <Button onClick={openCreate}>Tambah role</Button> : undefined}
      />

      <SectionCard flush>
        <DataTable<RoleItem>
          columns={[
            { header: "Kode", render: (r) => <span className="font-mono text-sm">{r.code}</span> },
            { header: "Nama", render: (r) => <span className="font-medium">{r.name}</span> },
            { header: "Deskripsi", render: (r) => r.description || "–" },
            {
              header: "Tipe",
              render: (r) => (r.isSystem ? <Badge tone="info">Sistem</Badge> : <Badge tone="neutral">Kustom</Badge>),
            },
            {
              header: "Aksi",
              align: "right",
              render: (r) =>
                canManage ? (
                  <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                    <Button variant="secondary" size="sm" onClick={() => void openPermissions(r)}>
                      Atur izin
                    </Button>
                    {!r.isSystem && (
                      <ActionMenu
                        items={rowActions(r).filter((i) => i.key !== "perm")}
                        label={`Aksi untuk ${r.code}`}
                        variant="ghost"
                      />
                    )}
                  </div>
                ) : (
                  <span className="text-sm text-muted-foreground">–</span>
                ),
            },
          ]}
          rows={data ? roles : null}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={canManage ? (r) => void openPermissions(r) : undefined}
          emptyTitle="Belum ada role"
          emptyDescription="Tambah role kustom sesuai kebutuhan akses."
          emptyAction={canManage ? <Button onClick={openCreate}>Tambah role</Button> : undefined}
        />
      </SectionCard>

      <FormSheet
        open={formMode !== null}
        title={isEdit ? "Ubah role" : "Tambah role"}
        description="Kode role tidak dapat diubah setelah dibuat."
        onClose={() => {
          if (!saving) setFormMode(null);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setFormMode(null)} disabled={saving}>
              Batal
            </Button>
            <Button type="submit" form="role-form" loading={saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="role-form" onSubmit={(e) => void handleSubmit(e)}>
          <div className="space-y-4">
            <FormField
              label="Kode"
              htmlFor="role-code"
              required={!isEdit}
              helperText={
                isEdit
                  ? "Kode dikunci — tidak dapat diubah."
                  : "Huruf/angka/titik/strip/underscore, maks 64 karakter."
              }
              errorText={formErrors.code}
            >
              <TextInput
                id="role-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="cth: kasir"
                disabled={isEdit}
              />
            </FormField>
            <FormField label="Nama" htmlFor="role-name" required errorText={formErrors.name}>
              <TextInput
                id="role-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="cth: Kasir"
              />
            </FormField>
            <FormField label="Deskripsi" htmlFor="role-desc" errorText={formErrors.description}>
              <TextArea
                id="role-desc"
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Keterangan singkat role (opsional)"
              />
            </FormField>
          </div>
        </form>
      </FormSheet>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Hapus role"
        message={`Hapus role "${deleteTarget?.code}"? Role yang masih dipakai pengguna tidak dapat dihapus.`}
        confirmLabel="Hapus"
        onConfirm={() => void handleDeleteConfirm()}
        onCancel={() => setDeleteTarget(null)}
      />

      <FormSheet
        open={permRole !== null}
        title={permRole ? `Izin — ${permRole.code}` : "Izin role"}
        description="Menyimpan akan mengganti seluruh izin role ini."
        size="lg"
        onClose={() => {
          if (!savingPerms) setPermRole(null);
        }}
        actions={
          <>
            <span className="tabular mr-auto self-center text-sm text-muted-foreground">
              {checked.length}/{catalog.length} izin dipilih
              {!locked && (added > 0 || removed > 0) && (
                <span className="ml-2" data-testid="perm-diff">
                  (+{added} / −{removed} izin)
                </span>
              )}
            </span>
            <Button variant="secondary" onClick={() => setPermRole(null)} disabled={savingPerms}>
              Batal
            </Button>
            {!locked && (
              <Button
                onClick={requestSavePermissions}
                loading={savingPerms}
                disabled={loadingCatalog || (added === 0 && removed === 0)}
              >
                {savingPerms ? "Menyimpan…" : "Simpan izin"}
              </Button>
            )}
          </>
        }
      >
        <div className="space-y-4">
          {locked && !loadingCatalog && (
            <Notice tone="info" title="Izin role ini hanya-baca">
              Izin role pemilik/superadmin dikelola otomatis oleh sistem dan tidak dapat diubah dari sini.
            </Notice>
          )}

          {loadingCatalog ? (
            <div role="status" aria-busy="true" aria-label="Memuat katalog izin" className="space-y-3">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
            </div>
          ) : catalog.length === 0 ? (
            <EmptyState title="Katalog izin kosong" description="Belum ada izin yang terdaftar di server." />
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <div className="min-w-48 flex-1">
                  <SearchInput
                    aria-label="Cari izin"
                    placeholder="Cari izin atau kunci"
                    value={permSearch}
                    debounceMs={150}
                    onSearch={setPermSearch}
                  />
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={locked}
                  onClick={() => setChecked(catalog.map((p) => p.code))}
                >
                  Pilih semua
                </Button>
                <Button variant="ghost" size="sm" disabled={locked} onClick={() => setChecked([])}>
                  Bersihkan
                </Button>
              </div>

              {visibleGroups.length === 0 ? (
                <EmptyState title="Izin tidak ditemukan" description="Coba kata kunci lain." />
              ) : (
                <div className="space-y-4">
                  {visibleGroups.map((group) => {
                    const full = groups.find((g) => g.module === group.module) ?? group;
                    const selected = full.items.filter((p) => checkedSet.has(p.code)).length;
                    const state: boolean | "indeterminate" =
                      selected === 0 ? false : selected === full.items.length ? true : "indeterminate";
                    return (
                      <section
                        key={group.module}
                        aria-labelledby={`perm-mod-${group.module}`}
                        className="overflow-hidden rounded-lg border border-border"
                      >
                        <header className="flex flex-wrap items-center justify-between gap-2 border-b border-border bg-muted px-4 py-2.5">
                          <h3 id={`perm-mod-${group.module}`} className="text-sm font-semibold text-foreground capitalize">
                            {group.module}
                          </h3>
                          <div className="flex items-center gap-3">
                            <span className="tabular text-xs text-muted-foreground">
                              {selected}/{full.items.length} izin
                            </span>
                            <label
                              htmlFor={`perm-all-${group.module}`}
                              className="flex cursor-pointer items-center gap-2 text-xs font-medium text-foreground"
                            >
                              <Checkbox
                                id={`perm-all-${group.module}`}
                                checked={state}
                                disabled={locked}
                                onCheckedChange={(v) => toggleModule(full, v === true)}
                              />
                              Pilih semua di modul
                            </label>
                          </div>
                        </header>
                        <ul className="grid gap-px bg-border sm:grid-cols-2">
                          {group.items.map((p) => (
                            <li key={p.code} className="bg-card">
                              <label
                                htmlFor={`perm-${p.code}`}
                                className="flex cursor-pointer items-start gap-2.5 px-4 py-2.5 text-sm text-foreground hover:bg-muted"
                              >
                                <Checkbox
                                  id={`perm-${p.code}`}
                                  className="mt-0.5"
                                  checked={checkedSet.has(p.code)}
                                  disabled={locked}
                                  onCheckedChange={(v) => togglePerm(p.code, v === true)}
                                />
                                <span className="min-w-0">
                                  <span className="block font-medium">{p.name}</span>
                                  <span className="block font-mono text-xs break-all text-muted-foreground">
                                    {p.code}
                                  </span>
                                </span>
                              </label>
                            </li>
                          ))}
                        </ul>
                      </section>
                    );
                  })}
                </div>
              )}
            </>
          )}
        </div>
      </FormSheet>

      <ConfirmDialog
        open={confirmRevoke}
        title="Cabut izin dari role"
        message={`Simpan akan mencabut ${removed} izin dan menambah ${added} izin pada role "${permRole?.code}". Pengguna dengan role ini kehilangan akses tersebut segera.`}
        confirmLabel="Simpan perubahan"
        onConfirm={() => void handleSavePermissions()}
        onCancel={() => setConfirmRevoke(false)}
      />
    </Page>
  );
}
