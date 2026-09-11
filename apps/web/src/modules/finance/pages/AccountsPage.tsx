import { useState } from "react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { financeService } from "@/modules/finance/services/finance.service";
import type {
  AccountCreateValues,
  AccountEditValues,
} from "@/modules/finance/schemas/finance.schema";
import {
  MAPPING_KEYS,
  accountTypeLabel,
  mappingKeyLabel,
} from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";
import { AccountFormModal } from "@/modules/finance/components/AccountFormModal";
import { MappingModal } from "@/modules/finance/components/MappingModal";

// Akun + pemetaan. Gate finance.manage (seluruh halaman di balik menu
// "Akun" ber-perm finance.manage; tombol juga dicek via can()).
export default function AccountsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeService.accounts(),
        financeService.mappings(),
      ])
        .then(([accs, maps]) => ({
          accounts: accs,
          mappings: maps ?? {},
        }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat akun", apiErr.message);
          throw err;
        }),
    [],
  );
  const accounts = data?.accounts ?? [];
  const mappings = data?.mappings ?? {};

  const [modalMode, setModalMode] = useState<"create" | "edit" | null>(null);
  const [editing, setEditing] = useState<Account | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<Account | null>(null);

  const [mappingKey, setMappingKey] = useState<string | null>(null);
  const [mappingSubmitting, setMappingSubmitting] = useState(false);
  const [mappingError, setMappingError] = useState<string | null>(null);

  async function handleAccountSubmit(
    values: AccountCreateValues | AccountEditValues,
  ) {
    setSubmitting(true);
    setServerError(null);
    try {
      if (modalMode === "create") {
        const v = values as AccountCreateValues;
        const res = await financeService.createAccount({
          code: v.code,
          name: v.name,
          type: v.type,
          isCash: v.isCash,
        });
        toast.fromServer(res.message, "Akun dibuat", res.data.code);
      } else if (editing) {
        const v = values as AccountEditValues;
        const res = await financeService.updateAccount(editing.id, {
          name: v.name,
          type: v.type,
          isCash: v.isCash,
        });
        toast.fromServer(res.message, "Akun disimpan", res.data.code);
      }
      setModalMode(null);
      setEditing(null);
      reload();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleArchive() {
    if (!archiveTarget) return;
    try {
      const res = await financeService.archiveAccount(archiveTarget.id);
      toast.fromServer(res.message, "Akun diarsipkan", archiveTarget.code);
      setArchiveTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal mengarsipkan akun", toApiError(err).message);
    }
  }

  async function handleMappingSubmit(accountId: number) {
    if (!mappingKey) return;
    setMappingSubmitting(true);
    setMappingError(null);
    try {
      const res = await financeService.setMapping(mappingKey, accountId);
      toast.fromServer(res.message, "Pemetaan disimpan", mappingKeyLabel(mappingKey));
      setMappingKey(null);
      reload();
    } catch (err) {
      setMappingError(toApiError(err).message);
    } finally {
      setMappingSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Akun"
        description="Bagan akun dan pemetaan akun otomatis untuk posting."
        actions={
          canManage ? (
            <Button
              onClick={() => {
                setEditing(null);
                setServerError(null);
                setModalMode("create");
              }}
            >
              Tambah Akun
            </Button>
          ) : undefined
        }
      />

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

      <SectionCard title="Bagan Akun">
        {loading && accounts.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <DataTable<Account>
            rows={accounts}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada akun"
            columns={[
              { header: "Kode", render: (r) => r.code },
              { header: "Nama", render: (r) => r.name },
              { header: "Tipe", render: (r) => accountTypeLabel(r.type) },
              {
                header: "Kas",
                render: (r) =>
                  r.isCash ? (
                    <Badge tone="info">Kas</Badge>
                  ) : (
                    <span className="text-muted">-</span>
                  ),
              },
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
                render: (r) =>
                  canManage && r.status === "active" ? (
                    <span className="flex justify-end gap-3">
                      <button
                        type="button"
                        onClick={() => {
                          setEditing(r);
                          setServerError(null);
                          setModalMode("edit");
                        }}
                        className="cursor-pointer font-medium text-brand hover:underline"
                      >
                        Ubah
                      </button>
                      <button
                        type="button"
                        onClick={() => setArchiveTarget(r)}
                        className="cursor-pointer font-medium text-bad hover:underline"
                      >
                        Arsipkan
                      </button>
                    </span>
                  ) : (
                    <span className="text-muted">-</span>
                  ),
              },
            ]}
          />
        )}
      </SectionCard>

      <div className="mt-4">
        <SectionCard
          title="Pemetaan Akun"
          description="Akun otomatis yang dipakai builder posting per kejadian."
        >
          <DataTable<string>
            rows={[...MAPPING_KEYS]}
            rowKey={(k) => k}
            emptyTitle="Tidak ada kunci pemetaan"
            columns={[
              { header: "Kunci", render: (k) => mappingKeyLabel(k) },
              {
                header: "Akun",
                render: (k) => {
                  const a = mappings[k];
                  return a ? `${a.code} — ${a.name}` : "-";
                },
              },
              {
                header: "Aksi",
                align: "right",
                render: (k) =>
                  canManage ? (
                    <button
                      type="button"
                      onClick={() => {
                        setMappingError(null);
                        setMappingKey(k);
                      }}
                      className="cursor-pointer font-medium text-brand hover:underline"
                    >
                      Ubah
                    </button>
                  ) : (
                    <span className="text-muted">-</span>
                  ),
              },
            ]}
          />
        </SectionCard>
      </div>

      {modalMode && (
        <AccountFormModal
          key={modalMode === "edit" ? editing?.id : "create"}
          open
          mode={modalMode}
          initial={editing}
          submitting={submitting}
          serverError={serverError}
          onClose={() => {
            setModalMode(null);
            setEditing(null);
          }}
          onSubmit={(v) => void handleAccountSubmit(v)}
        />
      )}

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan Akun?"
        message={`Akun ${archiveTarget?.code ?? ""} (${archiveTarget?.name ?? ""}) akan diarsipkan. Akun berjurnal atau terpetakan ditolak backend. Lanjutkan?`}
        confirmLabel="Arsipkan"
        onConfirm={() => void handleArchive()}
        onCancel={() => setArchiveTarget(null)}
      />

      {mappingKey && (
        <MappingModal
          open
          mappingKey={mappingKey}
          currentAccountId={mappings[mappingKey]?.id ?? null}
          accounts={accounts}
          submitting={mappingSubmitting}
          serverError={mappingError}
          onClose={() => setMappingKey(null)}
          onSubmit={(v) => void handleMappingSubmit(v)}
        />
      )}
    </div>
  );
}
