import { useState } from "react";
import { useParams } from "react-router-dom";
import { Archive, SquarePen } from "lucide-react";
import {
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
import { DescriptionList, DocNumber, Page, useUrlTab } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { supplierService } from "@/modules/party/services/party.service";
import { PartyFormModal } from "@/modules/party/components/PartyFormModal";
import { PartyBalancePanel, PartyTransactionsPanel } from "@/modules/party/components/PartyActivityPanels";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import { rowsToAttrs } from "@/shared/lib/key-value";
import { PartyAttributes } from "@/modules/party/components/PartyAttributes";

const TABS = ["ringkasan", "transaksi", "saldo"] as const;

export default function SupplierDetailPage() {
  const { id } = useParams();
  const supplierId = Number(id);
  const validId = Number.isInteger(supplierId) && supplierId > 0;

  const can = useAuthStore((s) => s.can);
  const canUpdate = can("suppliers.update");
  const canArchive = can("suppliers.archive");
  const canOrders = can("purchasing.view");
  const canBalance = can("payment.view");

  const [rawTab, setTab] = useUrlTab(TABS, "ringkasan");
  const tab = (rawTab === "transaksi" && !canOrders) || (rawTab === "saldo" && !canBalance) ? "ringkasan" : rawTab;

  const [confirm, setConfirm] = useState<"archive" | "restore" | null>(null);
  const [busy, setBusy] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const { data: detail, loading, error, errorStatus, reload } = useAsyncData(
    () => (!validId ? Promise.resolve(null) : supplierService.get(supplierId)),
    [supplierId],
  );
  usePageCrumb(detail?.name);

  if (!validId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Supplier tidak ditemukan"
        message={!validId ? "ID supplier tidak valid." : "Supplier ini tidak ada atau sudah dihapus."}
        backTo={ROUTE_PATHS.suppliers}
        backLabel="Kembali ke daftar pemasok"
      />
    );
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
        // Selalu dikirim: editor memuat isi lama, {} = dikosongkan.
        attributes: rowsToAttrs(values.attrs),
      });
      toast.fromServer(res.message, "Supplier disimpan");
      setEditOpen(false);
      reload();
    } catch (err) {
      setEditError(formatApiError(err));
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
      toast.danger("Gagal", formatApiError(err));
    } finally {
      setBusy(false);
    }
  }

  if (loading && !detail) return <DetailSkeleton />;

  if (error || !detail) {
    return (
      <Page>
        <ErrorState title="Gagal memuat supplier" message={error ?? "Supplier tidak ditemukan."} onRetry={reload} />
      </Page>
    );
  }

  return (
    <Page>
      <PageHeader
        title={detail.name}
        meta={<StatusBadge status={detail.status} />}
        subtitle={
          <>
            <DocNumber value={detail.code} copyable /> · Supplier
          </>
        }
        actions={
          <>
            {canArchive &&
              (isArchived ? (
                <Button variant="secondary" onClick={() => setConfirm("restore")}>
                  Pulihkan
                </Button>
              ) : (
                <Button variant="danger" onClick={() => setConfirm("archive")}>
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
        <Notice tone="muted" title="Supplier diarsipkan">
          Arsip tidak menghapus data — kode tetap dicadangkan dan dapat dipulihkan.
        </Notice>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-5">
        <TabsList variant="line" className="w-full justify-start overflow-x-auto border-b border-border">
          <TabsTrigger value="ringkasan" className="flex-none px-3">
            Ringkasan
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
                { label: "Alamat", value: detail.address, wide: true },
                { label: "Catatan", value: detail.notes, wide: true },
              ]}
            />
          </SectionCard>
          <PartyAttributes attributes={detail.attributes} />
        </TabsContent>

        {canOrders && (
          <TabsContent value="transaksi">
            <PartyTransactionsPanel kind="supplier" partyId={detail.id} />
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
            ? `Supplier ${detail.code} akan diarsipkan.`
            : `Supplier ${detail.code} akan aktif kembali.`
        }
        confirmLabel={confirm === "archive" ? "Arsipkan" : "Pulihkan"}
        tone={confirm === "archive" ? "danger" : "primary"}
        onConfirm={() => {
          if (!busy) void handleStatus();
        }}
        onCancel={() => setConfirm(null)}
      />
    </Page>
  );
}
