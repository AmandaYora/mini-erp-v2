import { useState } from "react";
import type { ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ActionRow,
  Badge,
  Button,
  ConfirmDialog,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { customerService, memberTypeService } from "@/modules/party/services/party.service";
import { PartyFormModal } from "@/modules/party/components/PartyFormModal";
import { AddressFormModal } from "@/modules/party/components/AddressFormModal";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import type { AddressFormValues } from "@/modules/party/schemas/address.schema";
import type { Address } from "@/modules/party/types";

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-muted text-[0.8rem] font-medium">{label}</dt>
      <dd className="mt-0.5 text-sm text-ink">{children}</dd>
    </div>
  );
}

export default function CustomerDetailPage() {
  const { id } = useParams();
  const customerId = Number(id);
  const validId = Number.isInteger(customerId) && customerId > 0;

  const can = useAuthStore((s) => s.can);
  const canUpdate = can("customers.update");
  const canArchive = can("customers.archive");

  const [editOpen, setEditOpen] = useState(false);
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [addrModal, setAddrModal] = useState<{ initial: Address | null } | null>(null);
  const [addrSubmitting, setAddrSubmitting] = useState(false);
  const [addrError, setAddrError] = useState<string | null>(null);
  const [addrToArchive, setAddrToArchive] = useState<Address | null>(null);

  const [confirmParty, setConfirmParty] = useState<"archive" | "restore" | null>(null);
  const [busy, setBusy] = useState(false);

  const { data: detail, loading, error, reload } = useAsyncData(
    () => (!validId ? Promise.resolve(null) : customerService.get(customerId)),
    [customerId],
  );

  const { data: memberData } = useAsyncData(
    async () => {
      if (!canUpdate) return { options: [], ok: false };
      try {
        const list = await memberTypeService.list("", "active");
        return {
          options: list.map((m) => ({ value: m.id, label: `${m.code} — ${m.name}` })),
          ok: true,
        };
      } catch {
        return { options: [], ok: false };
      }
    },
    [canUpdate],
  );
  const memberOptions: SearchSelectOption[] = memberData?.options ?? [];
  const memberLoadOk = memberData?.ok ?? false;

  if (!validId) {
    return <Notice tone="danger" title="ID customer tidak valid." />;
  }

  const isArchived = detail?.status === "archived";

  async function handleUpdate(values: PartyFormValues) {
    if (!detail) return;
    setEditSubmitting(true);
    setEditError(null);
    try {
      const prevMember = detail.member?.id ?? null;
      const nextMember = values.memberTypeId ?? null;
      const res = await customerService.update(detail.id, {
        name: values.name,
        phone: values.phone === "" ? undefined : values.phone,
        email: values.email === "" ? undefined : values.email,
        address: values.address === "" ? undefined : values.address,
        notes: values.notes === "" ? undefined : values.notes,
        memberTypeId: nextMember,
        // Menghilangkan pilihan = lepas membership eksplisit; backend tidak
        // pernah melepas diam-diam (KI-67).
        clearMember: prevMember !== null && nextMember === null,
      });
      toast.fromServer(res.message, "Customer disimpan");
      setEditOpen(false);
      reload();
    } catch (err) {
      setEditError(toApiError(err).message);
    } finally {
      setEditSubmitting(false);
    }
  }

  async function handlePartyStatus() {
    if (!detail || !confirmParty) return;
    setBusy(true);
    try {
      if (confirmParty === "archive") {
        const res = await customerService.archive(detail.id);
        toast.fromServer(res.message, "Customer diarsipkan");
      } else {
        const res = await customerService.restore(detail.id);
        toast.fromServer(res.message, "Customer dipulihkan");
      }
      setConfirmParty(null);
      reload();
    } catch (err) {
      toast.danger("Gagal", toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  function openAddrModal(initial: Address | null) {
    setAddrError(null);
    setAddrModal({ initial });
  }

  async function handleAddrSubmit(values: AddressFormValues) {
    if (!detail) return;
    setAddrSubmitting(true);
    setAddrError(null);
    try {
      const body = {
        label: values.label === "" ? undefined : values.label,
        recipient: values.recipient === "" ? undefined : values.recipient,
        phone: values.phone === "" ? undefined : values.phone,
        text: values.text,
        isPrimary: values.isPrimary,
        sortOrder: values.sortOrder,
      };
      if (addrModal?.initial) {
        const res = await customerService.updateAddress(detail.id, addrModal.initial.id, body);
        toast.fromServer(res.message, "Alamat disimpan");
      } else {
        const res = await customerService.createAddress(detail.id, body);
        toast.fromServer(res.message, "Alamat ditambahkan");
      }
      setAddrModal(null);
      reload();
    } catch (err) {
      setAddrError(toApiError(err).message);
    } finally {
      setAddrSubmitting(false);
    }
  }

  async function handleAddrArchive() {
    if (!detail || !addrToArchive) return;
    setBusy(true);
    try {
      const res = await customerService.archiveAddress(detail.id, addrToArchive.id);
        toast.fromServer(res.message, "Alamat diarsipkan");
        setAddrToArchive(null);
        reload();
    } catch (err) {
      toast.danger("Gagal", toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <Link to={ROUTE_PATHS.customers} className="text-sm font-medium text-brand hover:underline">
        ← Kembali ke daftar pelanggan
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
            description={`Customer #${detail.id}`}
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
                    <Button variant="secondary" onClick={() => setConfirmParty("restore")}>
                      Pulihkan
                    </Button>
                  ) : (
                    <Button variant="danger" onClick={() => setConfirmParty("archive")}>
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
              <InfoRow label="Tipe member">{detail.member?.code ?? "-"}</InfoRow>
              <InfoRow label="Alamat utama">{detail.address || "-"}</InfoRow>
              <InfoRow label="Catatan">{detail.notes || "-"}</InfoRow>
            </dl>
          </SectionCard>

          <SectionCard
            title="Buku alamat"
            description="Tepat satu alamat utama — backend menegakkannya otomatis."
            actions={
              canUpdate && !isArchived ? (
                <Button size="sm" onClick={() => openAddrModal(null)}>
                  Tambah Alamat
                </Button>
              ) : undefined
            }
          >
            {isArchived && (
              <div className="mb-4">
                <Notice tone="neutral" title="Customer diarsipkan — buku alamat terkunci." />
              </div>
            )}
            {detail.addresses.length === 0 ? (
              <EmptyState
                title="Belum ada alamat"
                description="Tambahkan alamat pengiriman customer di sini."
              />
            ) : (
              <ul className="space-y-3">
                {detail.addresses.map((a) => (
                  <li key={a.id} className="rounded-lg border border-hairline p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold text-ink">{a.label || "Alamat"}</span>
                        {a.isPrimary && <Badge tone="info">Utama</Badge>}
                      </div>
                      {canUpdate && !isArchived && (
                        <div className="flex gap-2">
                          <Button size="sm" variant="secondary" onClick={() => openAddrModal(a)}>
                            Ubah
                          </Button>
                          <Button size="sm" variant="secondary" onClick={() => setAddrToArchive(a)}>
                            Arsipkan
                          </Button>
                        </div>
                      )}
                    </div>
                    <p className="mt-2 text-sm text-ink">{a.text}</p>
                    <p className="text-muted mt-1 text-[0.82rem]">
                      {[a.recipient, a.phone].filter(Boolean).join(" · ") || "-"}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </SectionCard>

          {detail && (
            <ActionRow align="start">
              <span className="text-muted text-sm">
                Arsip tidak menghapus data — kode tetap dicadangkan dan dapat dipulihkan.
              </span>
            </ActionRow>
          )}
        </>
      ) : null}

      {editOpen && detail && (
        <PartyFormModal
          open
          title="Ubah Customer"
          initial={detail}
          withMember
          memberLoadOk={memberLoadOk}
          memberOptions={memberOptions}
          submitting={editSubmitting}
          serverError={editError}
          onClose={() => setEditOpen(false)}
          onSubmit={(v) => void handleUpdate(v)}
        />
      )}

      {addrModal && (
        <AddressFormModal
          open
          initial={addrModal.initial}
          submitting={addrSubmitting}
          serverError={addrError}
          onClose={() => setAddrModal(null)}
          onSubmit={(v) => void handleAddrSubmit(v)}
        />
      )}

      <ConfirmDialog
        open={confirmParty !== null}
        title={confirmParty === "archive" ? "Arsipkan customer?" : "Pulihkan customer?"}
        message={
          confirmParty === "archive"
            ? `Customer ${detail?.code} akan diarsipkan dan buku alamatnya dikunci.`
            : `Customer ${detail?.code} akan aktif kembali.`
        }
        confirmLabel={confirmParty === "archive" ? "Arsipkan" : "Pulihkan"}
        tone={confirmParty === "archive" ? "danger" : "primary"}
        onConfirm={() => {
          if (!busy) void handlePartyStatus();
        }}
        onCancel={() => setConfirmParty(null)}
      />

      <ConfirmDialog
        open={addrToArchive !== null}
        title="Arsipkan alamat?"
        message={`Alamat "${addrToArchive?.label || "ini"}" akan diarsipkan.${
          addrToArchive?.isPrimary ? " Alamat utama akan dialihkan ke alamat aktif tertua." : ""
        }`}
        confirmLabel="Arsipkan"
        onConfirm={() => {
          if (!busy) void handleAddrArchive();
        }}
        onCancel={() => setAddrToArchive(null)}
      />
    </div>
  );
}
