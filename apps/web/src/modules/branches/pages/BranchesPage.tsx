import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  Modal,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { branchService } from "@/modules/branches/services/branch.service";
import { branchSchema } from "@/modules/branches/schemas/branch.schema";
import type { BranchItem } from "@/modules/branches/types";

const PAGE_LIMIT = 10;

type FormMode = { kind: "create" } | { kind: "edit"; branch: BranchItem };

export default function BranchesPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("branches.manage");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, reload } = useAsyncData(
    async () => {
      try {
        const { items, meta } = await branchService.list({
          search: search || undefined,
          status: status || undefined,
          page,
          limit: PAGE_LIMIT,
        });
        return { items, total: meta.total };
      } catch (err) {
        toast.danger("Gagal memuat cabang", toApiError(err).message);
        throw err;
      }
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
  const [isHead, setIsHead] = useState(false);
  const [formStatus, setFormStatus] = useState("active");
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const t = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 400);
    return () => window.clearTimeout(t);
  }, [searchInput]);

  function openCreate() {
    setCode("");
    setName("");
    setAddress("");
    setCity("");
    setPhone("");
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

  return (
    <div>
      <PageHeader
        eyebrow="Pengaturan"
        title="Cabang"
        description="Kelola daftar cabang dan status buka/tutup."
        actions={
          canManage ? <Button onClick={openCreate}>Tambah Cabang</Button> : undefined
        }
      />

      <FilterBar>
        <FormField label="Cari">
          <TextInput
            placeholder="Kode / nama / kota…"
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
            <option value="active">Buka</option>
            <option value="inactive">Tutup</option>
          </SelectInput>
        </FormField>
      </FilterBar>

      <SectionCard>
        {loading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat data cabang…</p>
        ) : (
          <>
            <DataTable<BranchItem>
              columns={[
                { header: "Kode", render: (r) => r.code },
                { header: "Nama", render: (r) => r.name },
                { header: "Kota", render: (r) => r.city || "-" },
                { header: "Telepon", render: (r) => r.phone || "-" },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>
                      {r.status === "active"
                        ? "Buka"
                        : r.status === "inactive"
                          ? "Tutup"
                          : statusLabel(r.status)}
                    </Badge>
                  ),
                },
                {
                  header: "Pusat",
                  render: (r) =>
                    r.isHead ? (
                      <Badge tone="info">Pusat</Badge>
                    ) : (
                      <span className="text-muted text-sm">-</span>
                    ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) =>
                    canManage ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => openEdit(r)}
                      >
                        Ubah
                      </Button>
                    ) : (
                      <span className="text-muted text-sm">-</span>
                    ),
                },
              ]}
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada cabang"
              emptyDescription="Tambah cabang baru untuk mulai beroperasi."
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
        title={isEdit ? "Ubah Cabang" : "Tambah Cabang"}
        description={
          isEdit
            ? "Tutup/buka cabang lewat pilihan status."
            : "Cabang baru otomatis berstatus buka."
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
            <Button type="submit" form="branch-form" disabled={saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="branch-form" onSubmit={(e) => void handleSubmit(e)}>
          <div className="grid gap-4 md:grid-cols-2">
            <FormField
              label="Kode"
              required
              helperText="Maks 10 karakter. Tidak dapat diubah bila sudah ada dokumen terbit."
              errorText={formErrors.code}
            >
              <TextInput
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="cth: JKT01"
                maxLength={10}
              />
            </FormField>
            <FormField label="Nama" required errorText={formErrors.name}>
              <TextInput
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="cth: Jakarta Pusat"
              />
            </FormField>
            <FormField label="Kota" errorText={formErrors.city}>
              <TextInput
                value={city}
                onChange={(e) => setCity(e.target.value)}
                placeholder="cth: Jakarta"
              />
            </FormField>
            <FormField label="Telepon" errorText={formErrors.phone}>
              <TextInput
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="cth: 021-xxxxxxx"
              />
            </FormField>
          </div>
          <div className="mt-4 space-y-4">
            <FormField label="Alamat" errorText={formErrors.address}>
              <TextArea
                rows={2}
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                placeholder="Jalan, nomor, patokan (opsional)"
              />
            </FormField>
            {isEdit && (
              <FormField
                label="Status"
                required
                helperText="Tutup untuk menonaktifkan operasional cabang."
                errorText={formErrors.status}
              >
                <SelectInput
                  value={formStatus}
                  onChange={(e) => setFormStatus(e.target.value)}
                >
                  <option value="active">Buka</option>
                  <option value="inactive">Tutup</option>
                </SelectInput>
              </FormField>
            )}
            <label className="flex cursor-pointer items-center gap-2 text-sm text-ink">
              <input
                type="checkbox"
                className="accent-brand h-4 w-4"
                checked={isHead}
                onChange={(e) => setIsHead(e.target.checked)}
              />
              Kantor pusat
            </label>
          </div>
        </form>
      </Modal>
    </div>
  );
}
