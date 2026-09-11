import { useState } from "react";
import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { supplierService } from "@/modules/party/services/party.service";
import { PartyFormModal } from "@/modules/party/components/PartyFormModal";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-muted text-[0.8rem] font-medium">{label}</dt>
      <dd className="mt-0.5 text-sm text-ink">{children}</dd>
    </div>
  );
}

export default function SupplierDetailPage() {
  const { id } = useParams();
  const supplierId = Number(id);
  const validId = Number.isInteger(supplierId) && supplierId > 0;

  const can = useAuthStore((s) => s.can);
  const canUpdate = can("suppliers.update");
  const canArchive = can("suppliers.archive");

  const [confirm, setConfirm] = useState<"archive" | "restore" | null>(null);
  const [busy, setBusy] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const { data: detail, loading, error, reload } = useAsyncData(
    () => (!validId ? Promise.resolve(null) : supplierService.get(supplierId)),
    [supplierId],
  );

  if (!validId) {
    return <Notice tone="danger" title="ID supplier tidak valid." />;
  }

  const isArchived = detail?.status === "archived";

  async function handleUpdate(values: PartyFormValues) {
    if (!detail) return;
    setEditSubmitting(true);
    setEditError(null);
    try {
      const res = await supplierService.update(detail.id, {
        name: values.name,
        phone: values.phone === "" ? undefined : values.phone,
        email: values.email === "" ? undefined : values.email,
        address: values.address === "" ? undefined : values.address,
        notes: values.notes === "" ? undefined : values.notes,
      });
      toast.fromServer(res.message, "Supplier disimpan");
      setEditOpen(false);
      reload();
    } catch (err) {
      setEditError(toApiError(err).message);
    } finally {
      setEditSubmitting(false);
    }
  }

  async function handleStatus() {
    if (!detail || !confirm) return;
    setBusy(true);
    try {
      if (confirm === "archive") {
        const res = await supplierService.archive(detail.id);
        toast.fromServer(res.message, "Supplier diarsipkan");
      } else {
        const res = await supplierService.restore(detail.id);
        toast.fromServer(res.message, "Supplier dipulihkan");
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
    <div className="space-y-6">
      <Link to={ROUTE_PATHS.suppliers} className="text-sm font-medium text-brand hover:underline">
        ← Kembali ke daftar pemasok
      </Link>

      {loading && !detail ? (
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      ) : error ? (
        <Notice tone="danger" title={error}>
          <button
            type="button"
            onClick={() => reload()}
            className="cursor-pointer font-semibold text-brand hover:underline"
          >
            Coba lagi
          </button>
        </Notice>
      ) : detail ? (
        <>
          <PageHeader
            eyebrow={detail.code}
            title={detail.name}
            description={`Supplier #${detail.id}`}
            actions={
              <>
                {canUpdate && !isArchived && (
                  <Button
                    variant="secondary"
                    onClick={() => {
                      setEditError(null);
                      setEditOpen(true);
                    }}
                  >
                    Ubah
                  </Button>
                )}
                {canArchive &&
                  (isArchived ? (
                    <Button variant="secondary" onClick={() => setConfirm("restore")}>
                      Pulihkan
                    </Button>
                  ) : (
                    <Button variant="danger" onClick={() => setConfirm("archive")}>
                      Arsipkan
                    </Button>
                  ))}
              </>
            }
          />

          <SectionCard title="Informasi">
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <InfoRow label="Kode">{detail.code}</InfoRow>
              <InfoRow label="Nama">{detail.name}</InfoRow>
              <InfoRow label="Status">
                <Badge tone={statusTone(detail.status)}>{statusLabel(detail.status)}</Badge>
              </InfoRow>
              <InfoRow label="Telepon">{detail.phone || "-"}</InfoRow>
              <InfoRow label="Email">{detail.email || "-"}</InfoRow>
              <InfoRow label="Alamat">{detail.address || "-"}</InfoRow>
              <InfoRow label="Catatan">{detail.notes || "-"}</InfoRow>
            </dl>
          </SectionCard>
        </>
      ) : null}

      {editOpen && detail && (
        <PartyFormModal
          open
          title="Ubah Supplier"
          initial={detail}
          withMember={false}
          memberLoadOk={false}
          memberOptions={[]}
          submitting={editSubmitting}
          serverError={editError}
          onClose={() => setEditOpen(false)}
          onSubmit={(v) => void handleUpdate(v)}
        />
      )}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm === "archive" ? "Arsipkan supplier?" : "Pulihkan supplier?"}
        message={
          confirm === "archive"
            ? `Supplier ${detail?.code} akan diarsipkan.`
            : `Supplier ${detail?.code} akan aktif kembali.`
        }
        confirmLabel={confirm === "archive" ? "Arsipkan" : "Pulihkan"}
        tone={confirm === "archive" ? "danger" : "primary"}
        onConfirm={() => {
          if (!busy) void handleStatus();
        }}
        onCancel={() => setConfirm(null)}
      />
    </div>
  );
}
