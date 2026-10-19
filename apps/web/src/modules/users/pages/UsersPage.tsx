import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  Checkbox,
  ConfirmDialog,
  DataTable,
  FormField,
  FormSheet,
  Modal,
  PageHeader,
  Pagination,
  PasswordInput,
  RadioGroup,
  RadioGroupItem,
  SearchInput,
  SectionCard,
  SelectInput,
  Skeleton,
  StatusBadge,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, Page, Toolbar } from "@/shared/components/patterns";
import type { ActionMenuItem } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { formatDateTime } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { userService } from "@/modules/users/services/user.service";
import { roleService } from "@/modules/users/services/role.service";
import {
  createUserSchema,
  editUserSchema,
  passwordSchema,
  USER_PHONE_MAX,
  USER_TITLE_MAX,
} from "@/modules/users/schemas/user.schema";
import type {
  BranchOption,
  RoleItem,
  UserItem,
  UserListItem,
} from "@/modules/users/types";

const PAGE_LIMIT = 25;

type FormMode = { kind: "create" } | { kind: "edit"; id: number };
type BranchScope = "all" | "selected";

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

  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(
    async (signal) => {
      const { items, meta } = await userService.list(
        {
          search: search || undefined,
          status: status || undefined,
          page,
          limit: PAGE_LIMIT,
        },
        signal,
      );
      return { items, total: meta.total };
    },
    [search, status, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  const [formMode, setFormMode] = useState<FormMode | null>(null);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [fullName, setFullName] = useState("");
  const [phone, setPhone] = useState("");
  const [displayTitle, setDisplayTitle] = useState("");
  const [password, setPassword] = useState("");
  const [roleIds, setRoleIds] = useState<number[]>([]);
  const [branchIds, setBranchIds] = useState<number[]>([]);
  const [branchScope, setBranchScope] = useState<BranchScope>("selected");
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

  async function ensureOptions(): Promise<BranchOption[]> {
    try {
      if (roleOptions.length === 0) {
        setRoleOptions(await roleService.list());
      }
    } catch (err) {
      toast.danger("Gagal memuat daftar role", toApiError(err).message);
    }
    let branches = branchOptions;
    try {
      if (branchOptions.length === 0) {
        branches = await userService.branchOptions();
        setBranchOptions(branches);
      }
    } catch (err) {
      toast.danger("Gagal memuat daftar cabang", toApiError(err).message);
    }
    return branches;
  }

  function resetForm() {
    setUsername("");
    setEmail("");
    setFullName("");
    setPhone("");
    setDisplayTitle("");
    setPassword("");
    setRoleIds([]);
    setBranchIds([]);
    setBranchScope("selected");
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
    const optionsPromise = ensureOptions();
    try {
      const detail = await userService.get(row.id);
      const options = await optionsPromise;
      setUsername(detail.username);
      setEmail(detail.email ?? "");
      setFullName(detail.fullName);
      setPhone(detail.phone ?? "");
      setDisplayTitle(detail.displayTitle ?? "");
      // GET /users/{id} mengembalikan roleIds + branches — prefill agar
      // update (replace-all di backend) tak menghapus penugasan.
      setRoleIds(detail.roleIds ?? []);
      const assigned = (detail.branches ?? []).map((b) => b.branchId);
      setBranchIds(assigned);
      const activeIds = options.filter((b) => b.status === "active").map((b) => b.id);
      setBranchScope(
        activeIds.length > 0 && activeIds.every((id) => assigned.includes(id)) ? "all" : "selected",
      );
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

  function changeBranchScope(next: BranchScope) {
    setBranchScope(next);
    if (next !== "all") return;
    // "Semua cabang" = seluruh cabang aktif (cabang nonaktif yang sudah ditugaskan tetap dipertahankan).
    const activeIds = branchOptions.filter((b) => b.status === "active").map((b) => b.id);
    const merged = Array.from(new Set([...branchIds, ...activeIds]));
    setBranchIds(merged);
    if (defaultBranchId === null || !merged.includes(defaultBranchId)) {
      setDefaultBranchId(merged[0] ?? null);
    }
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
        phone,
        displayTitle,
        password,
      });
      if (!parsed.success) {
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "");
          errs[key] ??= issue.message;
        }
      }
    } else {
      const parsed = editUserSchema.safeParse({ username, email, fullName, phone, displayTitle });
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
          phone: phone.trim(),
          displayTitle: displayTitle.trim(),
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
          phone: phone.trim(),
          displayTitle: displayTitle.trim(),
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
      toast.fromServer(res.message, "Kata sandi disimpan");
      setPasswordTarget(null);
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      const apiErr = toApiError(err);
      setPasswordErrors({
        password: fieldMessage(apiErr.errors, "password") ?? apiErr.message,
      });
      toast.danger("Gagal menyimpan kata sandi", apiErr.message);
    } finally {
      setSavingPassword(false);
    }
  }

  function openPassword(row: UserItem) {
    setPasswordTarget(row);
    setNewPassword("");
    setConfirmPassword("");
    setPasswordErrors({});
  }

  function rowActions(r: UserItem): ActionMenuItem[] {
    const list: ActionMenuItem[] = [];
    if (canUpdate) {
      list.push({ key: "edit", label: "Ubah", onSelect: () => void openEdit(r) });
      list.push({ key: "password", label: "Ganti kata sandi", onSelect: () => openPassword(r) });
    }
    if (canArchive) {
      list.push({
        key: "status",
        label: r.status === "active" ? "Nonaktifkan" : "Aktifkan",
        destructive: r.status === "active",
        onSelect: () => setStatusTarget(r),
      });
    }
    return list;
  }

  const isEdit = formMode?.kind === "edit";
  // Role tercadang (reserved) disembunyikan API dari non-superadmin; id-nya
  // tetap ikut dikirim agar penugasan lama tidak hilang.
  const hiddenRoleCount = roleOptions.length === 0 ? 0 : roleIds.filter(
    (id) => !roleOptions.some((r) => r.id === id),
  ).length;
  const selectedBranches = branchOptions.filter((b) => branchIds.includes(b.id));
  const filtered = search !== "" || status !== "";

  return (
    <Page>
      <PageHeader
        title="Pengguna"
        description="Kelola akun pengguna, role, dan akses cabang."
        actions={
          canCreate ? <Button onClick={openCreate}>Tambah pengguna</Button> : undefined
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari pengguna"
            placeholder="Cari username, nama, atau email"
            value={search}
            onSearch={(v) => {
              setSearch(v);
              setPage(1);
            }}
          />
        }
        primary={
          <div className="w-40">
            <FormField label="Status" htmlFor="users-status">
              <SelectInput
                id="users-status"
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
          </div>
        }
        active={
          status
            ? [
                {
                  key: "status",
                  label: `Status: ${status === "active" ? "Aktif" : "Nonaktif"}`,
                  onRemove: () => {
                    setStatus("");
                    setPage(1);
                  },
                },
              ]
            : []
        }
        onClearAll={
          filtered
            ? () => {
                setSearch("");
                setStatus("");
                setPage(1);
              }
            : undefined
        }
      />

      <SectionCard flush>
        <DataTable<UserListItem>
          columns={[
            { header: "Username", render: (r) => <span className="font-medium">{r.username}</span> },
            { header: "Nama lengkap", render: (r) => r.fullName },
            { header: "Email", render: (r) => r.email ?? "–" },
            { header: "Jabatan", render: (r) => r.displayTitle || "–" },
            {
              header: "Peran",
              render: (r) => (r.roles.length ? r.roles.map((x) => x.name).join(", ") : "–"),
            },
            {
              header: "Cabang",
              render: (r) =>
                r.branches.length ? (
                  <span title={r.branches.map((b) => b.name).join(", ")}>
                    {r.branches.map((b) => b.name).join(", ")}
                  </span>
                ) : (
                  "–"
                ),
            },
            {
              header: "Login terakhir",
              render: (r) => (r.lastLoginAt ? formatDateTime(r.lastLoginAt) : "Belum pernah"),
            },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            {
              header: "Aksi",
              align: "right",
              width: 64,
              render: (r) => (
                // Menu dirender di portal; klik itemnya menggelembung ke <tr> lewat pohon React.
                <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                  <ActionMenu items={rowActions(r)} label={`Aksi untuk ${r.username}`} variant="ghost" />
                </div>
              ),
            },
          ]}
          rows={data ? items : null}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={canUpdate ? (r) => void openEdit(r) : undefined}
          emptyTitle={filtered ? "Tidak ada pengguna yang cocok" : "Belum ada pengguna"}
          emptyDescription={
            filtered
              ? "Ubah kata kunci atau filter status."
              : "Tambah pengguna baru untuk mulai memberi akses."
          }
          emptyAction={
            !filtered && canCreate ? <Button onClick={openCreate}>Tambah pengguna</Button> : undefined
          }
        />
        <Pagination page={page} limit={PAGE_LIMIT} total={total} onPageChange={setPage} />
      </SectionCard>

      <FormSheet
        open={formMode !== null}
        title={isEdit ? "Ubah pengguna" : "Tambah pengguna"}
        description={
          isEdit
            ? "Perubahan role dan cabang menimpa seluruh penugasan lama."
            : "Akun baru membutuhkan minimal satu role."
        }
        size="lg"
        onClose={() => {
          if (!saving) setFormMode(null);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setFormMode(null)} disabled={saving}>
              Batal
            </Button>
            <Button type="submit" form="user-form" loading={saving} disabled={loadingDetail}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        {loadingDetail ? (
          <div role="status" aria-busy="true" aria-label="Memuat detail pengguna" className="space-y-4">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-24 w-full" />
          </div>
        ) : (
          <form id="user-form" onSubmit={(e) => void handleSubmit(e)} className="space-y-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="Username" htmlFor="user-username" required errorText={formErrors.username}>
                <TextInput
                  id="user-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="cth: kasir01"
                />
              </FormField>
              <FormField label="Nama lengkap" htmlFor="user-fullname" required errorText={formErrors.fullName}>
                <TextInput
                  id="user-fullname"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  placeholder="cth: Kasir Satu"
                />
              </FormField>
              <FormField label="Email" htmlFor="user-email" errorText={formErrors.email}>
                <TextInput
                  id="user-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="opsional@perusahaan.id"
                />
              </FormField>
              <FormField label="Telepon" htmlFor="user-phone" errorText={formErrors.phone}>
                <TextInput
                  id="user-phone"
                  value={phone}
                  maxLength={USER_PHONE_MAX}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="opsional"
                />
              </FormField>
              <FormField label="Jabatan" htmlFor="user-title" errorText={formErrors.displayTitle}>
                <TextInput
                  id="user-title"
                  value={displayTitle}
                  maxLength={USER_TITLE_MAX}
                  onChange={(e) => setDisplayTitle(e.target.value)}
                  placeholder="cth: Kepala Gudang"
                />
              </FormField>
              {!isEdit && (
                <FormField
                  label="Kata sandi"
                  htmlFor="user-password"
                  required
                  helperText="Minimal 8 karakter."
                  errorText={formErrors.password}
                >
                  <PasswordInput
                    id="user-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Minimal 8 karakter"
                    autoComplete="new-password"
                  />
                </FormField>
              )}
            </div>

            {isEdit && (
              <Notice tone="info" title="Menggantikan seluruh penugasan">
                Pilihan role dan cabang di bawah menggantikan seluruh role dan cabang lama pengguna ini.
              </Notice>
            )}

            <fieldset>
              <legend className="mb-1.5 text-sm font-medium text-foreground">
                Role <span aria-hidden className="text-destructive">*</span>
                <span className="sr-only">(wajib diisi)</span>
              </legend>
              <p className="mb-2 text-xs text-muted-foreground">Pilih minimal satu role.</p>
              <div className="grid gap-2 sm:grid-cols-2">
                {roleOptions.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Tidak ada role tersedia.</p>
                ) : (
                  roleOptions.map((role) => (
                    <label
                      key={role.id}
                      htmlFor={`user-role-${role.id}`}
                      className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2 text-sm text-foreground hover:bg-muted"
                    >
                      <Checkbox
                        id={`user-role-${role.id}`}
                        checked={roleIds.includes(role.id)}
                        onCheckedChange={() => toggleRole(role.id)}
                      />
                      <span>
                        {role.name}
                        <span className="ml-1 text-xs text-muted-foreground">({role.code})</span>
                      </span>
                    </label>
                  ))
                )}
              </div>
              {hiddenRoleCount > 0 && (
                <p className="mt-2 text-xs text-muted-foreground">
                  {hiddenRoleCount} role lain pada pengguna ini tidak dapat ditampilkan dan tetap dipertahankan.
                </p>
              )}
              {formErrors.roleIds && (
                <p role="alert" className="mt-1.5 text-xs font-medium text-destructive">
                  {formErrors.roleIds}
                </p>
              )}
            </fieldset>

            <fieldset>
              <legend className="mb-1.5 text-sm font-medium text-foreground">Akses cabang</legend>
              {branchOptions.length === 0 ? (
                <p className="text-sm text-muted-foreground">Tidak ada cabang tersedia.</p>
              ) : (
                <>
                  <RadioGroup
                    value={branchScope}
                    onValueChange={(v) => changeBranchScope(v as BranchScope)}
                    aria-label="Cakupan akses cabang"
                  >
                    <label htmlFor="user-scope-all" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
                      <RadioGroupItem id="user-scope-all" value="all" />
                      Semua cabang
                    </label>
                    <label htmlFor="user-scope-selected" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
                      <RadioGroupItem id="user-scope-selected" value="selected" />
                      Pilih cabang
                    </label>
                  </RadioGroup>

                  {branchScope === "selected" && (
                    <div className="mt-3 grid gap-2 sm:grid-cols-2">
                      {branchOptions.map((b) => (
                        <label
                          key={b.id}
                          htmlFor={`user-branch-${b.id}`}
                          className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2 text-sm text-foreground hover:bg-muted"
                        >
                          <Checkbox
                            id={`user-branch-${b.id}`}
                            checked={branchIds.includes(b.id)}
                            onCheckedChange={() => toggleBranch(b.id)}
                          />
                          <span>
                            {b.code} — {b.name}
                            {b.status !== "active" && (
                              <span className="ml-1 text-xs text-muted-foreground">(nonaktif)</span>
                            )}
                          </span>
                        </label>
                      ))}
                    </div>
                  )}

                  {selectedBranches.length > 0 && (
                    <div className="mt-3 max-w-sm">
                      <FormField
                        label="Cabang default"
                        htmlFor="user-default-branch"
                        helperText="Cabang yang terbuka saat pengguna masuk."
                      >
                        <SelectInput
                          id="user-default-branch"
                          value={
                            defaultBranchId !== null && branchIds.includes(defaultBranchId)
                              ? String(defaultBranchId)
                              : String(selectedBranches[0].id)
                          }
                          onChange={(e) => setDefaultBranchId(Number(e.target.value))}
                        >
                          {selectedBranches.map((b) => (
                            <option key={b.id} value={b.id}>
                              {b.code} — {b.name}
                            </option>
                          ))}
                        </SelectInput>
                      </FormField>
                    </div>
                  )}
                </>
              )}
              {formErrors.branches && (
                <p role="alert" className="mt-1.5 text-xs font-medium text-destructive">
                  {formErrors.branches}
                </p>
              )}
            </fieldset>
          </form>
        )}
      </FormSheet>

      <ConfirmDialog
        open={statusTarget !== null}
        title={statusTarget?.status === "active" ? "Nonaktifkan pengguna" : "Aktifkan pengguna"}
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
        title="Ganti kata sandi"
        description={
          passwordTarget ? `Kata sandi baru untuk "${passwordTarget.username}".` : undefined
        }
        size="sm"
        onClose={() => {
          if (!savingPassword) setPasswordTarget(null);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setPasswordTarget(null)} disabled={savingPassword}>
              Batal
            </Button>
            <Button type="submit" form="password-form" loading={savingPassword}>
              {savingPassword ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="password-form" onSubmit={(e) => void handlePasswordSubmit(e)}>
          <div className="space-y-4">
            <FormField
              label="Kata sandi baru"
              htmlFor="pw-new"
              required
              helperText="Minimal 8 karakter."
              errorText={passwordErrors.password}
            >
              <PasswordInput
                id="pw-new"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                autoComplete="new-password"
              />
            </FormField>
            <FormField
              label="Konfirmasi kata sandi"
              htmlFor="pw-confirm"
              required
              errorText={passwordErrors.confirmPassword}
            >
              <PasswordInput
                id="pw-confirm"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                autoComplete="new-password"
              />
            </FormField>
          </div>
        </form>
      </Modal>
    </Page>
  );
}
