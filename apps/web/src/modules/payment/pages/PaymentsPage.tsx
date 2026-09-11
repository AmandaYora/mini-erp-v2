import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  Segmented,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import {
  paymentLookupService,
  paymentService,
} from "@/modules/payment/services/payment.service";
import type {
  PartyLookup,
  Payment,
} from "@/modules/payment/types";
import { methodLabel } from "@/modules/payment/types";
import type { PaymentFormValues } from "@/modules/payment/schemas/payment.schema";
import type { SettleCreditFormValues } from "@/modules/payment/schemas/payment.schema";
import { PaymentFormModal } from "@/modules/payment/components/PaymentFormModal";
import { SettleCreditModal } from "@/modules/payment/components/SettleCreditModal";
import { PartyBalanceSection } from "@/modules/payment/components/PartyBalanceSection";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const LIMIT = 20;

// Backend List hanya mengenal status "active" | "cancelled" (repository.go)
// dan TIDAK punya param search/method — keduanya difilter di klien pada
// halaman yang sedang dimuat.
const STATUS_OPTIONS = [
  { value: "", label: "Semua status" },
  { value: "active", label: "Aktif" },
  { value: "cancelled", label: "Dibatalkan" },
];

const METHOD_OPTIONS = [
  { value: "", label: "Semua metode" },
  { value: "cash", label: "Tunai" },
  { value: "transfer", label: "Transfer" },
  { value: "offset", label: "Offset" },
];

type Tab = "daftar" | "saldo";

export default function PaymentsPage() {
  const can = useAuthStore((s) => s.can);
  const canCreate = can("payment.create");

  const [tab, setTab] = useState<Tab>("daftar");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [method, setMethod] = useState("");
  const [page, setPage] = useState(1);

  const [parties, setParties] = useState<PartyLookup[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [settleOpen, setSettleOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    paymentLookupService
      .partyOptions()
      .then((opts) => {
        if (!cancelled) setParties(opts);
      })
      .catch((err) => {
        if (!cancelled)
          toast.danger("Gagal memuat daftar pihak", toApiError(err).message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      paymentService
        .list({
          status,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat pembayaran", apiErr.message);
          throw err;
        }),
    [status, page],
  );
  const items: Payment[] = useMemo(() => data?.items ?? [], [data]);
  const total = data?.total ?? 0;

  // Filter klien (backend tidak mendukung search/method): nomor, pihak, catatan.
  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter((r) => {
      if (method && r.method !== method) return false;
      if (!q) return true;
      return (
        r.number.toLowerCase().includes(q) ||
        (r.partyName ?? "").toLowerCase().includes(q) ||
        (r.notes ?? "").toLowerCase().includes(q)
      );
    });
  }, [items, search, method]);

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput);
  }

  async function handleCreate(values: PaymentFormValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await paymentService.create({
        partyId: values.partyId,
        amount: values.amount,
        method: values.method,
        paidAt: values.paidAt,
        notes: values.notes,
        allocations:
          values.allocations.length > 0 ? values.allocations : undefined,
      });
      toast.fromServer(res.message, "Pembayaran dicatat", res.data.number);
      setCreateOpen(false);
      setPage(1);
      reload();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleSettle(values: SettleCreditFormValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await paymentService.settleCredit({
        partyId: values.partyId,
        returnType: values.returnType,
        returnId: values.returnId,
        orderType: values.orderType,
        orderId: values.orderId,
        amount: values.amount,
        notes: values.notes,
      });
      toast.fromServer(res.message, "Kredit di-offset", res.data.number);
      setSettleOpen(false);
      setPage(1);
      reload();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title="Pembayaran"
        description="Catat pembayaran, offset kredit retur, dan pantau saldo pihak."
        actions={
          canCreate && tab === "daftar" ? (
            <>
              <Button
                variant="secondary"
                onClick={() => {
                  setServerError(null);
                  setSettleOpen(true);
                }}
              >
                Offset Kredit
              </Button>
              <Button
                onClick={() => {
                  setServerError(null);
                  setCreateOpen(true);
                }}
              >
                Catat Pembayaran
              </Button>
            </>
          ) : undefined
        }
      />

      <div className="mb-4">
        <Segmented
          options={[
            { value: "daftar", label: "Daftar Pembayaran" },
            { value: "saldo", label: "Saldo Pihak" },
          ]}
          value={tab}
          onChange={(v) => setTab(v as Tab)}
        />
      </div>

      {tab === "saldo" ? (
        <PartyBalanceSection parties={parties} />
      ) : (
        <>
          <FilterBar>
            <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
              <div className="min-w-52 flex-1">
                <FormField label="Cari">
                  <TextInput
                    value={searchInput}
                    onChange={(e) => setSearchInput(e.target.value)}
                    placeholder="Nomor / pihak / catatan…"
                  />
                </FormField>
              </div>
              <div className="min-w-44">
                <FormField label="Status">
                  <SelectInput
                    value={status}
                    onChange={(e) => {
                      setStatus(e.target.value);
                      setPage(1);
                    }}
                  >
                    {STATUS_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
              </div>
              <div className="min-w-44">
                <FormField label="Metode">
                  <SelectInput
                    value={method}
                    onChange={(e) => setMethod(e.target.value)}
                  >
                    {METHOD_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
              </div>
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
              <>
                <DataTable<Payment>
                  rows={visible}
                  rowKey={(r) => r.id}
                  emptyTitle="Belum ada pembayaran"
                  emptyDescription="Catat pembayaran baru atau ubah kata kunci pencarian."
                  columns={[
                    {
                      header: "Nomor",
                      render: (r) => (
                        <Link
                          to={`${ROUTE_PATHS.payments}/${r.id}`}
                          className="font-medium text-brand hover:underline"
                        >
                          {r.number}
                        </Link>
                      ),
                    },
                    { header: "Tanggal", render: (r) => formatDate(r.paidAt) },
                    { header: "Pihak", render: (r) => r.partyName || "-" },
                    {
                      header: "Metode",
                      render: (r) => (
                        <Badge tone="neutral">{methodLabel(r.method)}</Badge>
                      ),
                    },
                    {
                      header: "Jumlah",
                      align: "right",
                      render: (r) => (
                        <span className="font-medium">
                          {formatIDR(r.amount)}
                        </span>
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
                      render: (r) => (
                        <Link
                          to={`${ROUTE_PATHS.payments}/${r.id}`}
                          className="font-medium text-brand hover:underline"
                        >
                          Detail
                        </Link>
                      ),
                    },
                  ]}
                />
                <Pagination
                  page={page}
                  limit={LIMIT}
                  total={total}
                  onPageChange={(p) => setPage(p)}
                />
              </>
            )}
          </SectionCard>
        </>
      )}

      {createOpen && (
        <PaymentFormModal
          open
          parties={parties}
          submitting={submitting}
          serverError={serverError}
          onClose={() => setCreateOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}
      {settleOpen && (
        <SettleCreditModal
          open
          parties={parties}
          submitting={submitting}
          serverError={serverError}
          onClose={() => setSettleOpen(false)}
          onSubmit={(v) => void handleSettle(v)}
        />
      )}
    </div>
  );
}
