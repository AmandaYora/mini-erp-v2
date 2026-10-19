import { useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  Checkbox,
  DataTable,
  FormField,
  FormSheet,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, Page, Toolbar } from "@/shared/components/patterns";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { statusLabel } from "@/shared/lib/format";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { branchService } from "@/modules/branches/services/branch.service";
import { BRANCH_CODE_MAX, BRANCH_LOCATION_LABEL_MAX, branchSchema } from "@/modules/branches/schemas/branch.schema";
import type { BranchItem } from "@/modules/branches/types";

const PAGE_LIMIT = 25;

type FormMode = { kind: "create" } | { kind: "edit"; branch: BranchItem };

function branchStatusLabel(status: string): string {
  if (status === "active") return "Buka";
  if (status === "inactive") return "Tutup";
  return statusLabel(status);
}

export default function BranchesPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("branches.manage");

  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(
    async (signal) => {
      const { items, meta } = await branchService.list(
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
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [phone, setPhone] = useState("");
  const [locationLabel, setLocationLabel] = useState("");
  const [isHead, setIsHead] = useState(false);
  const [formStatus, setFormStatus] = useState("active");
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  function openCreate() {
    setCode("");
    setName("");
    setAddress("");
    setCity("");
    setPhone("");
    setLocationLabel("");
    setIsHead(false);
    setFormStatus("active");
    setFormErrors({});
    setFormMode({ kind: "create" });
  }

  function openEdit(branch: BranchItem) {
    setCode(branch.code);
    setName(branch.name);
    setAddress(branch.address ?? "");
    setCity(branch.city ?? "");
    setPhone(branch.phone ?? "");
    setLocationLabel(branch.defaultLocationLabel ?? "");
    setIsHead(branch.isHead);
    setFormStatus(branch.status);
    setFormErrors({});
    setFormMode({ kind: "edit", branch });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!formMode) return;
    const parsed = branchSchema.safeParse({
      code,
      name,
      address,
      city,
      phone,
      defaultLocationLabel: locationLabel,
      isHead,
      status: formStatus,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setFormErrors(errs);
      return;
    }
    setFormErrors({});
    setSaving(true);
    const normalizedCode = parsed.data.code.toUpperCase();
    try {
      if (formMode.kind === "create") {
        const res = await branchService.create({
          code: normalizedCode,
          name: parsed.data.name,
          address: parsed.data.address ?? "",
          city: parsed.data.city ?? "",
          phone: parsed.data.phone ?? "",
          defaultLocationLabel: parsed.data.defaultLocationLabel ?? "",
          isHead: parsed.data.isHead ?? false,
        });
        toast.fromServer(res.message, "Cabang dibuat");
      } else {
        const res = await branchService.update(formMode.branch.id, {
          code: normalizedCode,
          name: parsed.data.name,
          address: parsed.data.address ?? "",
          city: parsed.data.city ?? "",
          phone: parsed.data.phone ?? "",
          status: parsed.data.status ?? "active",
          defaultLocationLabel: parsed.data.defaultLocationLabel ?? "",
          isHead: parsed.data.isHead ?? false,
        });
        toast.fromServer(res.message, "Cabang disimpan");
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
      toast.danger("Gagal menyimpan cabang", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  const isEdit = formMode?.kind === "edit";
  const filtered = search !== "" || status !== "";

  return (
    <Page>
      <PageHeader
        title="Cabang"
        description="Kelola daftar cabang dan status buka/tutup."
        actions={canManage ? <Button onClick={openCreate}>Tambah cabang</Button> : undefined}
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari cabang"
            placeholder="Cari kode, nama, atau kota"
            value={search}
            onSearch={(v) => {
              setSearch(v);
              setPage(1);
            }}
          />
        }
        primary={
          <div className="w-40">
            <FormField label="Status" htmlFor="branches-status">
              <SelectInput
                id="branches-status"
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Semua</option>
                <option value="active">Buka</option>
                <option value="inactive">Tutup</option>
              </SelectInput>
            </FormField>
          </div>
        }
        active={
          status
            ? [
                {
                  key: "status",
                  label: `Status: ${branchStatusLabel(status)}`,
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
        <DataTable<BranchItem>
          columns={[
            { header: "Kode", render: (r) => <span className="font-mono text-sm">{r.code}</span> },
            { header: "Nama", render: (r) => <span className="font-medium">{r.name}</span> },
            { header: "Kota", render: (r) => r.city || "–" },
            { header: "Telepon", render: (r) => r.phone || "–" },
            {
              header: "Status",
              render: (r) => <StatusBadge status={r.status} label={branchStatusLabel(r.status)} />,
            },
            {
              header: "Pusat",
              render: (r) => (r.isHead ? <Badge tone="info">Pusat</Badge> : <span className="text-sm text-muted-foreground">–</span>),
            },
            {
              header: "Aksi",
              align: "right",
              width: 64,
              render: (r) =>
                canManage ? (
                  // Menu dirender di portal; klik itemnya menggelembung ke <tr> lewat pohon React.
                  <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                    <ActionMenu
                      items={[{ key: "edit", label: "Ubah", onSelect: () => openEdit(r) }]}
                      label={`Aksi untuk ${r.code}`}
                      variant="ghost"
                    />
                  </div>
                ) : (
                  <span className="text-sm text-muted-foreground">–</span>
                ),
            },
          ]}
          rows={data ? items : null}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={canManage ? openEdit : undefined}
          emptyTitle={filtered ? "Tidak ada cabang yang cocok" : "Belum ada cabang"}
          emptyDescription={
            filtered ? "Ubah kata kunci atau filter status." : "Tambah cabang baru untuk mulai beroperasi."
          }
          emptyAction={!filtered && canManage ? <Button onClick={openCreate}>Tambah cabang</Button> : undefined}
        />
        <Pagination page={page} limit={PAGE_LIMIT} total={total} onPageChange={setPage} />
      </SectionCard>

      <FormSheet
        open={formMode !== null}
        title={isEdit ? "Ubah cabang" : "Tambah cabang"}
        description={isEdit ? "Tutup/buka cabang lewat pilihan status." : "Cabang baru otomatis berstatus buka."}
        onClose={() => {
          if (!saving) setFormMode(null);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setFormMode(null)} disabled={saving}>
              Batal
            </Button>
            <Button type="submit" form="branch-form" loading={saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="branch-form" onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <FormField
            label="Kode"
            htmlFor="branch-code"
            required
            helperText={`Maks ${BRANCH_CODE_MAX} karakter. Tidak dapat diubah bila sudah ada dokumen terbit.`}
            errorText={formErrors.code}
          >
            <TextInput
              id="branch-code"
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
              placeholder="cth: JKT01"
              maxLength={BRANCH_CODE_MAX}
            />
          </FormField>
          <FormField label="Nama" htmlFor="branch-name" required errorText={formErrors.name}>
            <TextInput
              id="branch-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="cth: Jakarta Pusat"
            />
          </FormField>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Kota" htmlFor="branch-city" errorText={formErrors.city}>
              <TextInput
                id="branch-city"
                value={city}
                onChange={(e) => setCity(e.target.value)}
                placeholder="cth: Jakarta"
              />
            </FormField>
            <FormField label="Telepon" htmlFor="branch-phone" errorText={formErrors.phone}>
              <TextInput
                id="branch-phone"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="cth: 021-xxxxxxx"
              />
            </FormField>
          </div>
          <FormField label="Alamat" htmlFor="branch-address" errorText={formErrors.address}>
            <TextArea
              id="branch-address"
              rows={2}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="Jalan, nomor, patokan (opsional)"
            />
          </FormField>
          <FormField
            label="Label lokasi bawaan"
            htmlFor="branch-location"
            helperText="Nama lokasi stok utama cabang (opsional). Kosong = nama bawaan sistem."
            errorText={formErrors.defaultLocationLabel}
          >
            <TextInput
              id="branch-location"
              value={locationLabel}
              onChange={(e) => setLocationLabel(e.target.value)}
              placeholder="cth: Gudang Utama"
              maxLength={BRANCH_LOCATION_LABEL_MAX}
            />
          </FormField>
          {isEdit && (
            <FormField
              label="Status"
              htmlFor="branch-status"
              required
              helperText="Tutup untuk menonaktifkan operasional cabang."
              errorText={formErrors.status}
            >
              <SelectInput id="branch-status" value={formStatus} onChange={(e) => setFormStatus(e.target.value)}>
                <option value="active">Buka</option>
                <option value="inactive">Tutup</option>
              </SelectInput>
            </FormField>
          )}
          <label htmlFor="branch-head" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
            <Checkbox id="branch-head" checked={isHead} onCheckedChange={(v) => setIsHead(v === true)} />
            Kantor pusat
          </label>
        </form>
      </FormSheet>
    </Page>
  );
}
