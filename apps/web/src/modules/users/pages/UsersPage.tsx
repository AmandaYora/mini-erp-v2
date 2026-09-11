import { useEffect, useState } from "react";
import type { FormEvent } from "react";import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FilterBar,
  FormField,
  Modal,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { userService } from "@/modules/users/services/user.service";
import { roleService } from "@/modules/users/services/role.service";
import {
  createUserSchema,
  editUserSchema,
  passwordSchema,
} from "@/modules/users/schemas/user.schema";
import type {
  BranchOption,
  RoleItem,
  UserItem,
} from "@/modules/users/types";

const PAGE_LIMIT = 10;

type FormMode = { kind: "create" } | { kind: "edit"; id: number };

function fieldMessage(
  errors: { field: string; message: string }[] | undefined,
  field: string,
): string | undefined {
  return errors?.find((e) => e.field === field)?.message;
}

export default function UsersPage() {
  const can = useAuthStore((s) => s.can);
  const canCreate = can("users.create");
  const canUpdate = can("users.update");
  const canArchive = can("users.archive");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, reload } = useAsyncData(
    async () => {
      try {
        const { items, meta } = await userService.list({
          search: search || undefined,
          status: status || undefined,
          page,
          limit: PAGE_LIMIT,
        });
        return { items, total: meta.total };
      } catch (err) {
        toast.danger("Gagal memuat pengguna", toApiError(err).message);
        throw err;
      }
    },
    [search, status, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  const [formMode, setFormMode] = useState<FormMode | null>(null);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [fullName, setFullName] = useState("");
  const [password, setPassword] = useState("");
  const [roleIds, setRoleIds] = useState<number[]>([]);
  const [branchIds, setBranchIds] = useState<number[]>([]);
  const [defaultBranchId, setDefaultBranchId] = useState<number | null>(null);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);

  const [roleOptions, setRoleOptions] = useState<RoleItem[]>([]);
  const [branchOptions, setBranchOptions] = useState<BranchOption[]>([]);

  const [statusTarget, setStatusTarget] = useState<UserItem | null>(null);
  const [passwordTarget, setPasswordTarget] = useState<UserItem | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordErrors, setPasswordErrors] = useState<Record<string, string>>(
    {},
  );
  const [savingPassword, setSavingPassword] = useState(false);

  // Debounce pencarian agar tidak menembak API tiap ketikan.
  useEffect(() => {
    const t = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 400);
    return () => window.clearTimeout(t);
  }, [searchInput]);

  async function ensureOptions() {
    try {
      if (roleOptions.length === 0) {
        setRoleOptions(await roleService.list());
      }
    } catch (err) {
      toast.danger("Gagal memuat daftar role", toApiError(err).message);
    }
    try {
      if (branchOptions.length === 0) {
        setBranchOptions(await userService.branchOptions());
      }
    } catch (err) {
      toast.danger("Gagal memuat daftar cabang", toApiError(err).message);
    }
  }

  function resetForm() {
    setUsername("");
    setEmail("");
    setFullName("");
    setPassword("");
    setRoleIds([]);
    setBranchIds([]);
    setDefaultBranchId(null);
    setFormErrors({});
  }

  function openCreate() {
    resetForm();
    setFormMode({ kind: "create" });
    void ensureOptions();
  }

  async function openEdit(row: UserItem) {
    resetForm();
    setFormMode({ kind: "edit", id: row.id });
    setLoadingDetail(true);
    void ensureOptions();
    try {
      const detail = await userService.get(row.id);
      setUsername(detail.username);
      setEmail(detail.email ?? "");
      setFullName(detail.fullName);
      // GET /users/{id} mengembalikan roleIds + branches — prefill agar
      // update (replace-all di backend) tak menghapus penugasan.
      setRoleIds(detail.roleIds ?? []);
      setBranchIds((detail.branches ?? []).map((b) => b.branchId));
      setDefaultBranchId(
        (detail.branches ?? []).find((b) => b.isDefault)?.branchId ??
          detail.branches?.[0]?.branchId ??
          null,
      );
    } catch (err) {
      toast.danger("Gagal memuat detail pengguna", toApiError(err).message);
      setFormMode(null);
    } finally {
      setLoadingDetail(false);
    }
  }

  function toggleRole(id: number) {
    setRoleIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );
  }

  function toggleBranch(id: number) {
    setBranchIds((prev) => {
      const next = prev.includes(id)
        ? prev.filter((x) => x !== id)
        : [...prev, id];
      if (defaultBranchId !== null && !next.includes(defaultBranchId)) {
        setDefaultBranchId(next.length > 0 ? next[0] : null);
      }
      if (defaultBranchId === null && next.length > 0) {
        setDefaultBranchId(next[0]);
      }
      return next;
    });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!formMode) return;
    const errs: Record<string, string> = {};

    if (formMode.kind === "create") {
      const parsed = createUserSchema.safeParse({
        username,
        email,
        fullName,
        password,
      });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "");
          errs[key] ??= issue.message;
        }
      }
    } else {
      const parsed = editUserSchema.safeParse({ username, email, fullName });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "");
          errs[key] ??= issue.message;
        }
      }
    }
    if (roleIds.length === 0) {
      errs.roleIds = "Pengguna wajib memiliki minimal satu role";
    }
    const effectiveDefault =
      branchIds.length === 0
        ? null
        : (defaultBranchId !== null && branchIds.includes(defaultBranchId)
            ? defaultBranchId
            : branchIds[0]);
    if (Object.keys(errs).length > 0) {
      setFormErrors(errs);
      return;
    }
    setFormErrors({});
    setSaving(true);
    const branches = branchIds.map((branchId) => ({
      branchId,
      isDefault: branchId === effectiveDefault,
    }));
    try {
      if (formMode.kind === "create") {
        const res = await userService.create({
          username: username.trim(),
          email: email.trim() || undefined,
          fullName: fullName.trim(),
          password,
          roleIds,
          branches,
        });
        toast.fromServer(res.message, "Pengguna dibuat");
      } else {
        const res = await userService.update(formMode.id, {
          username: username.trim(),
          email: email.trim() || undefined,
          fullName: fullName.trim(),
          roleIds,
          branches,
        });
        toast.fromServer(res.message, "Pengguna disimpan");
      }
      setFormMode(null);
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field === "roleIds") next.roleIds = fe.message;
        else if (fe.field === "branches") next.branches = fe.message;
        else if (fe.field) next[fe.field] ??= fe.message;
      }
      setFormErrors(next);
      toast.danger("Gagal menyimpan pengguna", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleStatusConfirm() {
    if (!statusTarget) return;
    const nextStatus = statusTarget.status === "active" ? "inactive" : "active";
    try {
      const res = await userService.setStatus(statusTarget.id, nextStatus);
      toast.fromServer(
        res.message,
        nextStatus === "active" ? "Pengguna diaktifkan" : "Pengguna dinonaktifkan",
      );
      setStatusTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal mengubah status", toApiError(err).message);
    }
  }

  async function handlePasswordSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!passwordTarget) return;
    const parsed = passwordSchema.safeParse({
      password: newPassword,
      confirmPassword,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const key = String(issue.path[0] ?? "");
        errs[key] ??= issue.message;
      }
      setPasswordErrors(errs);
      return;
    }
    setPasswordErrors({});
    setSavingPassword(true);
    try {
      const res = await userService.changePassword(passwordTarget.id, newPassword);
      toast.fromServer(res.message, "Password disimpan");
      setPasswordTarget(null);
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      const apiErr = toApiError(err);
      setPasswordErrors({
        password: fieldMessage(apiErr.errors, "password") ?? apiErr.message,
      });
      toast.danger("Gagal menyimpan password", apiErr.message);
    } finally {
      setSavingPassword(false);
    }
  }

  const isEdit = formMode?.kind === "edit";

  return (
    <div>
      <PageHeader
        eyebrow="Pengaturan"
        title="Pengguna"
        description="Kelola akun pengguna, role, dan akses cabang."
        actions={
          canCreate ? <Button onClick={openCreate}>Tambah Pengguna</Button> : undefined
        }
      />

      <FilterBar>
        <FormField label="Cari">
          <TextInput
            placeholder="Username / nama / email…"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </FormField>
        <FormField label="Status">
          <SelectInput
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
          >
            <option value="">Semua</option>
            <option value="active">Aktif</option>
            <option value="inactive">Nonaktif</option>
          </SelectInput>
        </FormField>
      </FilterBar>

      <SectionCard>
        {loading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat data pengguna…</p>
        ) : (
          <>
            <DataTable<UserItem>
              columns={[
                { header: "Username", render: (r) => r.username },
                { header: "Nama Lengkap", render: (r) => r.fullName },
                { header: "Email", render: (r) => r.email ?? "-" },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>
                      {statusLabel(r.status)}
                    </Badge>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <div className="flex justify-end gap-2">
                      {canUpdate && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => void openEdit(r)}
                        >
                          Ubah
                        </Button>
                      )}
                      {canUpdate && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => {
                            setPasswordTarget(r);
                            setNewPassword("");
                            setConfirmPassword("");
                            setPasswordErrors({});
                          }}
                        >
                          Password
                        </Button>
                      )}
                      {canArchive && (
                        <Button
                          variant={r.status === "active" ? "danger" : "secondary"}
                          size="sm"
                          onClick={() => setStatusTarget(r)}
                        >
                          {r.status === "active" ? "Nonaktifkan" : "Aktifkan"}
                        </Button>
                      )}
                    </div>
                  ),
                },
              ]}
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada pengguna"
              emptyDescription="Tambah pengguna baru untuk mulai memberi akses."
            />
            <Pagination
              page={page}
              limit={PAGE_LIMIT}
              total={total}
              onPageChange={setPage}
            />
          </>
        )}
      </SectionCard>

      <Modal
        open={formMode !== null}
        title={isEdit ? "Ubah Pengguna" : "Tambah Pengguna"}
        description={
          isEdit
            ? "Perubahan role & cabang menimpa seluruh penugasan lama."
            : "Akun baru membutuhkan minimal satu role."
        }
        size="lg"
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
            <Button type="submit" form="user-form" disabled={saving || loadingDetail}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        {loadingDetail ? (
          <p className="text-muted text-sm">Memuat detail pengguna…</p>
        ) : (
          <form id="user-form" onSubmit={(e) => void handleSubmit(e)}>
            <div className="grid gap-4 md:grid-cols-2">
              <FormField label="Username" required errorText={formErrors.username}>
                <TextInput
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="cth: kasir01"
                />
              </FormField>
              <FormField label="Nama Lengkap" required errorText={formErrors.fullName}>
                <TextInput
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  placeholder="cth: Kasir Satu"
                />
              </FormField>
              <FormField label="Email" errorText={formErrors.email}>
                <TextInput
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="opsional@perusahaan.id"
                />
              </FormField>
              {!isEdit && (
                <FormField
                  label="Password"
                  required
                  helperText="Minimal 8 karakter."
                  errorText={formErrors.password}
                >
                  <TextInput
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Minimal 8 karakter"
                  />
                </FormField>
              )}
            </div>

            {isEdit && (
              <div className="mt-4">
                <Notice tone="info" title="Menggantikan seluruh penugasan">
                  Pilihan role &amp; cabang di bawah menggantikan seluruh role
                  &amp; cabang lama pengguna ini.
                </Notice>
              </div>
            )}

            <div className="mt-4">
              <FormField
                label="Role"
                required
                helperText="Minimal satu role."
                errorText={formErrors.roleIds}
              >
                <div className="grid gap-2 md:grid-cols-2">
                  {roleOptions.length === 0 ? (
                    <p className="text-muted text-sm">Tidak ada role tersedia.</p>
                  ) : (
                    roleOptions.map((role) => (
                      <label
                        key={role.id}
                        className="flex cursor-pointer items-center gap-2 rounded-md border border-hairline px-3 py-2 text-sm text-ink"
                      >
                        <input
                          type="checkbox"
                          className="accent-brand h-4 w-4"
                          checked={roleIds.includes(role.id)}
                          onChange={() => toggleRole(role.id)}
                        />
                        <span>
                          {role.name}
                          <span className="text-muted ml-1 text-xs">
                            ({role.code})
                          </span>
                        </span>
                      </label>
                    ))
                  )}
                </div>
              </FormField>
            </div>

            <div className="mt-4">
              <FormField
                label="Cabang"
                helperText="Centang cabang yang boleh diakses, lalu pilih satu sebagai default (radio)."
                errorText={formErrors.branches}
              >
                <div className="space-y-2">
                  {branchOptions.length === 0 ? (
                    <p className="text-muted text-sm">Tidak ada cabang tersedia.</p>
                  ) : (
                    branchOptions.map((b) => {
                      const checked = branchIds.includes(b.id);
                      return (
                        <div
                          key={b.id}
                          className="flex cursor-pointer flex-wrap items-center gap-3 rounded-md border border-hairline px-3 py-2 text-sm text-ink"
                        >
                          <label className="flex flex-1 cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              className="accent-brand h-4 w-4"
                              checked={checked}
                              onChange={() => toggleBranch(b.id)}
                            />
                            <span>
                              {b.code} — {b.name}
                              {b.status !== "active" && (
                                <span className="text-muted ml-1 text-xs">
                                  (nonaktif)
                                </span>
                              )}
                            </span>
                          </label>
                          <label className="flex cursor-pointer items-center gap-1 text-xs text-muted">
                            <input
                              type="radio"
                              name="default-branch"
                              className="accent-brand h-4 w-4"
                              disabled={!checked}
                              checked={defaultBranchId === b.id}
                              onChange={() => setDefaultBranchId(b.id)}
                            />
                            Default
                          </label>
                        </div>
                      );
                    })
                  )}
                </div>
              </FormField>
            </div>
          </form>
        )}
      </Modal>

      <ConfirmDialog
        open={statusTarget !== null}
        title={statusTarget?.status === "active" ? "Nonaktifkan Pengguna" : "Aktifkan Pengguna"}
        message={
          statusTarget?.status === "active"
            ? `Nonaktifkan akun "${statusTarget?.username}"? Pengguna tidak bisa masuk sampai diaktifkan kembali.`
            : `Aktifkan kembali akun "${statusTarget?.username}"?`
        }
        confirmLabel={statusTarget?.status === "active" ? "Nonaktifkan" : "Aktifkan"}
        tone={statusTarget?.status === "active" ? "danger" : "primary"}
        onConfirm={() => void handleStatusConfirm()}
        onCancel={() => setStatusTarget(null)}
      />

      <Modal
        open={passwordTarget !== null}
        title="Ubah Password"
        description={
          passwordTarget
            ? `Password baru untuk "${passwordTarget.username}".`
            : undefined
        }
        size="sm"
        onClose={() => {
          if (!savingPassword) setPasswordTarget(null);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={() => setPasswordTarget(null)}
              disabled={savingPassword}
            >
              Batal
            </Button>
            <Button
              type="submit"
              form="password-form"
              disabled={savingPassword}
            >
              {savingPassword ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="password-form" onSubmit={(e) => void handlePasswordSubmit(e)}>
          <div className="space-y-4">
            <FormField
              label="Password Baru"
              required
              helperText="Minimal 8 karakter."
              errorText={passwordErrors.password}
            >
              <TextInput
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </FormField>
            <FormField
              label="Konfirmasi Password"
              required
              errorText={passwordErrors.confirmPassword}
            >
              <TextInput
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </FormField>
          </div>
        </form>
      </Modal>
    </div>
  );
}
