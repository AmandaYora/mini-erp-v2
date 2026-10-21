import { useState } from "react";
import { useParams } from "react-router-dom";
import { Archive, SquarePen } from "lucide-react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DetailSkeleton,
  PageHeader,
  SectionCard,
  StatusBadge,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { ActionMenu, DescriptionList, DocNumber, Page, useUrlTab } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { customerService, memberTypeService } from "@/modules/party/services/party.service";
import { PartyFormModal } from "@/modules/party/components/PartyFormModal";
import { AddressFormModal } from "@/modules/party/components/AddressFormModal";
import { PartyBalancePanel, PartyTransactionsPanel } from "@/modules/party/components/PartyActivityPanels";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import { rowsToAttrs } from "@/shared/lib/key-value";
import { PartyAttributes } from "@/modules/party/components/PartyAttributes";
import type { AddressFormValues } from "@/modules/party/schemas/address.schema";
import type { Address } from "@/modules/party/types";

const TABS = ["ringkasan", "alamat", "transaksi", "saldo"] as const;

export default function CustomerDetailPage() {
  const { id } = useParams();
  const customerId = Number(id);
  const validId = Number.isInteger(customerId) && customerId > 0;

  const can = useAuthStore((s) => s.can);
  const canUpdate = can("customers.update");
  const canArchive = can("customers.archive");
  const canOrders = can("sales.view");
  const canBalance = can("payment.view");

  const [rawTab, setTab] = useUrlTab(TABS, "ringkasan");
  const tab = (rawTab === "transaksi" && !canOrders) || (rawTab === "saldo" && !canBalance) ? "ringkasan" : rawTab;

  const [editOpen, setEditOpen] = useState(false);
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [addrModal, setAddrModal] = useState<{ initial: Address | null } | null>(null);
  const [addrSubmitting, setAddrSubmitting] = useState(false);
  const [addrError, setAddrError] = useState<string | null>(null);
  const [addrToArchive, setAddrToArchive] = useState<Address | null>(null);

  const [confirmParty, setConfirmParty] = useState<"archive" | "restore" | null>(null);
  const [busy, setBusy] = useState(false);

  const { data: detail, loading, error, errorStatus, reload } = useAsyncData(
    () => (!validId ? Promise.resolve(null) : customerService.get(customerId)),
    [customerId],
  );
  usePageCrumb(detail?.name);

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

  if (!validId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Customer tidak ditemukan"
        message={!validId ? "ID customer tidak valid." : "Customer ini tidak ada atau sudah dihapus."}
        backTo={ROUTE_PATHS.customers}
        backLabel="Kembali ke daftar pelanggan"
      />
    );
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
        // Selalu dikirim: editor memuat isi lama, {} = dikosongkan.
        attributes: rowsToAttrs(values.attrs),
      });
      toast.fromServer(res.message, "Customer disimpan");
      setEditOpen(false);
      reload();
    } catch (err) {
      setEditError(formatApiError(err));
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
      toast.danger("Gagal", formatApiError(err));
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
      setAddrError(formatApiError(err));
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
      toast.danger("Gagal", formatApiError(err));
    } finally {
      setBusy(false);
    }
  }

  if (loading && !detail) return <DetailSkeleton />;

  if (error || !detail) {
    return (
      <Page>
        <ErrorState title="Gagal memuat customer" message={error ?? "Customer tidak ditemukan."} onRetry={reload} />
      </Page>
    );
  }

  const addresses = detail.addresses;

  return (
    <Page>
      <PageHeader
        title={detail.name}
        meta={<StatusBadge status={detail.status} />}
        subtitle={
          <>
            <DocNumber value={detail.code} copyable /> · Customer
          </>
        }
        actions={
          <>
            {canArchive &&
              (isArchived ? (
                <Button variant="secondary" onClick={() => setConfirmParty("restore")}>
                  Pulihkan
                </Button>
              ) : (
                <Button variant="danger" onClick={() => setConfirmParty("archive")}>
                  <Archive aria-hidden /> Arsipkan
                </Button>
              ))}
            {canUpdate && !isArchived && (
              <Button
                onClick={() => {
                  setEditError(null);
                  setEditOpen(true);
                }}
              >
                <SquarePen aria-hidden /> Ubah
              </Button>
            )}
          </>
        }
      />

      {isArchived && (
        <Notice tone="muted" title="Customer diarsipkan">
          Buku alamat terkunci. Arsip tidak menghapus data — kode tetap dicadangkan dan dapat dipulihkan.
        </Notice>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-5">
        <TabsList variant="line" className="w-full justify-start overflow-x-auto border-b border-border">
          <TabsTrigger value="ringkasan" className="flex-none px-3">
            Ringkasan
          </TabsTrigger>
          <TabsTrigger value="alamat" className="flex-none px-3">
            Alamat ({addresses.length})
          </TabsTrigger>
          {canOrders && (
            <TabsTrigger value="transaksi" className="flex-none px-3">
              Transaksi
            </TabsTrigger>
          )}
          {canBalance && (
            <TabsTrigger value="saldo" className="flex-none px-3">
              Saldo
            </TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="ringkasan" className="flex flex-col gap-6">
          <SectionCard title="Informasi">
            <DescriptionList
              items={[
                { label: "Kode", value: <DocNumber value={detail.code} /> },
                { label: "Nama", value: detail.name },
                { label: "Status", value: <StatusBadge status={detail.status} /> },
                { label: "Telepon", value: detail.phone },
                { label: "Email", value: detail.email },
                { label: "Tipe member", value: detail.member?.code },
                { label: "Alamat utama", value: detail.address, wide: true },
                { label: "Catatan", value: detail.notes, wide: true },
              ]}
            />
          </SectionCard>
          <PartyAttributes attributes={detail.attributes} />
        </TabsContent>

        <TabsContent value="alamat">
          <SectionCard
            title="Buku alamat"
            description="Tepat satu alamat utama — backend menegakkannya otomatis."
            actions={
              canUpdate && !isArchived ? (
                <Button size="sm" onClick={() => openAddrModal(null)}>
                  Tambah alamat
                </Button>
              ) : undefined
            }
          >
            {addresses.length === 0 ? (
              <EmptyState
                title="Belum ada alamat"
                description="Tambahkan alamat pengiriman customer di sini."
                action={
                  canUpdate && !isArchived ? (
                    <Button variant="secondary" onClick={() => openAddrModal(null)}>
                      Tambah alamat
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <ul className="space-y-3">
                {addresses.map((a) => (
                  <li key={a.id} className="rounded-lg border border-border p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold text-foreground">{a.label || "Alamat"}</span>
                        {a.isPrimary && <Badge tone="info">Utama</Badge>}
                        {a.status === "archived" && <StatusBadge status="archived" />}
                      </div>
                      {canUpdate && !isArchived && a.status !== "archived" && (
                        <ActionMenu
                          variant="ghost"
                          label={`Aksi untuk alamat ${a.label || ""}`.trim()}
                          items={[
                            { key: "edit", label: "Ubah alamat", onSelect: () => openAddrModal(a) },
                            {
                              key: "archive",
                              label: "Arsipkan alamat",
                              destructive: true,
                              onSelect: () => setAddrToArchive(a),
                            },
                          ]}
                        />
                      )}
                    </div>
                    <p className="mt-2 text-sm text-foreground">{a.text}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {[a.recipient, a.phone].filter(Boolean).join(" · ") || "–"}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </SectionCard>
        </TabsContent>

        {canOrders && (
          <TabsContent value="transaksi">
            <PartyTransactionsPanel kind="customer" partyId={detail.id} />
          </TabsContent>
        )}

        {canBalance && (
          <TabsContent value="saldo">
            <PartyBalancePanel partyId={detail.id} />
          </TabsContent>
        )}
      </Tabs>

      {editOpen && (
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
            ? `Customer ${detail.code} akan diarsipkan dan buku alamatnya dikunci.`
            : `Customer ${detail.code} akan aktif kembali.`
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
    </Page>
  );
}
