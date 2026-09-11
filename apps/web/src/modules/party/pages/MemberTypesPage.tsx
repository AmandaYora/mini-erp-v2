import { useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { memberTypeService } from "@/modules/party/services/party.service";
import { MemberTypeFormModal } from "@/modules/party/components/MemberTypeFormModal";
import {
  basisOptions,
  optionLabel,
  roundingModeOptions,
} from "@/modules/party/schemas/member-type.schema";
import type {
  MemberTypeCreateValues,
  MemberTypeUpdateValues,
} from "@/modules/party/schemas/member-type.schema";
import type { MemberType } from "@/modules/party/types";

/** Ringkasan aturan untuk kolom tabel, mis. "−10% dari Harga jual · Terdekat 100". */
function ruleSummary(m: MemberType): string {
  const dir = m.direction === "plus" ? "+" : "−";
  const val = m.type === "percent" ? `${m.value}%` : formatIDR(m.value);
  return `${dir}${val} dari ${optionLabel(basisOptions, m.basis)} · ${optionLabel(roundingModeOptions, m.roundingMode)}`;
}

export default function MemberTypesPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("member_types.manage");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<MemberType | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<{ target: MemberType; action: "archive" | "restore" } | null>(null);
  const [busy, setBusy] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () => memberTypeService.list(search, status),
    [search, status],
  );
  const items = data ?? [];

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput);
  }

  function openCreate() {
    setEditing(null);
    setModalError(null);
    setModalOpen(true);
  }

  function openEdit(row: MemberType) {
    setEditing(row);
    setModalError(null);
    setModalOpen(true);
  }

  async function handleSubmit(values: MemberTypeCreateValues | MemberTypeUpdateValues) {
    setSubmitting(true);
    setModalError(null);
    try {
      if (editing) {
        const v = values as MemberTypeUpdateValues;
        const res = await memberTypeService.update(editing.id, {
          name: v.name,
          description: v.description === "" ? undefined : v.description,
          basis: v.basis,
          direction: v.direction,
          type: v.type,
          value: v.value,
          mode: v.mode,
          step: v.step,
          confirmed: v.confirmed,
        });
        toast.fromServer(res.message, "Tipe member disimpan");
      } else {
        const v = values as MemberTypeCreateValues;
        const res = await memberTypeService.create({
          code: v.code,
          name: v.name,
          description: v.description === "" ? undefined : v.description,
          basis: v.basis,
          direction: v.direction,
          type: v.type,
          value: v.value,
          mode: v.mode,
          step: v.step,
          confirmed: v.confirmed,
        });
        toast.fromServer(res.message, "Tipe member dibuat");
      }
      setModalOpen(false);
      setEditing(null);
      reload();
    } catch (err) {
      setModalError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleConfirm() {
    if (!confirm) return;
    setBusy(true);
    try {
      if (confirm.action === "archive") {
        const res = await memberTypeService.archive(confirm.target.id);
        toast.fromServer(res.message, "Tipe member diarsipkan");
      } else {
        const res = await memberTypeService.restore(confirm.target.id);
        toast.fromServer(res.message, "Tipe member dipulihkan");
      }
      setConfirm(null);
      reload();
    } catch (err) {
      toast.danger("Gagal", toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Master"
        title="Tipe Member"
        description="Aturan harga member — basis, penyesuaian, dan pembulatan."
        actions={
          canManage ? <Button onClick={openCreate}>Tambah Tipe Member</Button> : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField label="Cari">
              <TextInput
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Kode / nama…"
              />
            </FormField>
          </div>
          <FormField label="Status">
            <SelectInput value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">Semua status</option>
              <option value="active">Aktif</option>
              <option value="archived">Diarsipkan</option>
            </SelectInput>
          </FormField>
          <Button type="submit" variant="secondary">
            Cari
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard>
        {loading && items.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <DataTable<MemberType>
            rows={items}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada tipe member"
            emptyDescription="Tambah tipe member baru atau ubah kata kunci pencarian."
            columns={[
              { header: "Kode", render: (r) => <span className="font-medium">{r.code}</span> },
              { header: "Nama", render: (r) => r.name },
              { header: "Aturan", render: (r) => ruleSummary(r) },
              {
                header: "Status",
                render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>,
              },
              ...(canManage
                ? [
                    {
                      header: "Aksi",
                      align: "right" as const,
                      render: (r: MemberType) => (
                        <span className="flex justify-end gap-3">
                          <button
                            type="button"
                            onClick={() => openEdit(r)}
                            className="cursor-pointer font-medium text-brand hover:underline"
                          >
                            Ubah
                          </button>
                          {r.status === "archived" ? (
                            <button
                              type="button"
                              onClick={() => setConfirm({ target: r, action: "restore" })}
                              className="cursor-pointer font-medium text-brand hover:underline"
                            >
                              Pulihkan
                            </button>
                          ) : (
                            <button
                              type="button"
                              onClick={() => setConfirm({ target: r, action: "archive" })}
                              className="cursor-pointer font-medium text-bad hover:underline"
                            >
                              Arsipkan
                            </button>
                          )}
                        </span>
                      ),
                    },
                  ]
                : []),
            ]}
          />
        )}
      </SectionCard>

      {modalOpen && (
        <MemberTypeFormModal
          open
          initial={editing}
          submitting={submitting}
          serverError={modalError}
          onClose={() => {
            setModalOpen(false);
            setEditing(null);
          }}
          onSubmit={(v) => void handleSubmit(v)}
        />
      )}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm?.action === "archive" ? "Arsipkan tipe member?" : "Pulihkan tipe member?"}
        message={
          confirm?.action === "archive"
            ? `Tipe ${confirm?.target.code} akan diarsipkan. Riwayat tetap menunjuk ke tipe ini, tetapi customer baru tidak dapat memakainya.`
            : `Tipe ${confirm?.target.code} akan aktif kembali.`
        }
        confirmLabel={confirm?.action === "archive" ? "Arsipkan" : "Pulihkan"}
        tone={confirm?.action === "archive" ? "danger" : "primary"}
        onConfirm={() => {
          if (!busy) void handleConfirm();
        }}
        onCancel={() => setConfirm(null)}
      />
    </div>
  );
}
