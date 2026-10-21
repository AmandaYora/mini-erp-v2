import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  ToggleGroup,
  ToggleGroupItem,
} from "@/shared/components/ui";
import { DocNumber, Money, Page, SavedViews, Toolbar, useUrlTab } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { formatApiError, isAbort, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { OpenParty, Payment } from "@/modules/payment/types";
import {
  PAYMENT_LIST_DEFAULTS,
  paymentListFilterSchema,
  toPaymentFilterParams,
} from "@/modules/payment/schemas/payment-list-filter.schema";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { directionLabel, methodLabel, paymentKind, paymentKindLabel } from "@/modules/payment/types";
import type { PaymentFormValues } from "@/modules/payment/schemas/payment.schema";
import type { SettleCreditFormValues } from "@/modules/payment/schemas/payment.schema";
import { PaymentFormModal } from "@/modules/payment/components/PaymentFormModal";
import { SettleCreditModal } from "@/modules/payment/components/SettleCreditModal";
import { UseCreditModal } from "@/modules/payment/components/UseCreditModal";
import type { UseCreditPayload } from "@/modules/payment/types";
import { PartyBalanceSection } from "@/modules/payment/components/PartyBalanceSection";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const LIMIT = 20;

// Semua filter dikirim ke server (search, method, direction, status, partyId,
// orderType+orderId, from/to) — tidak ada penyaringan di klien.
const STATUS_OPTIONS = [
  { value: "", label: "Semua status" },
  { value: "active", label: "Aktif" },
  { value: "cancelled", label: "Dibatalkan" },
];

const METHOD_OPTIONS = [
  { value: "", label: "Semua metode" },
  { value: "cash", label: "Tunai" },
  { value: "transfer", label: "Transfer" },
  { value: "check", label: "Cek" },
  { value: "offset", label: "Offset" },
];

const TABS = ["daftar", "saldo"] as const;

export default function PaymentsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("payment.create");

  const [tab, setTab] = useUrlTab(TABS, "daftar");
  const { filters, setFilters, reset } = useUrlFilters(paymentListFilterSchema, PAYMENT_LIST_DEFAULTS);
  // Nama pihak hanya diketahui setelah dipilih di sesi ini; dari URL jatuh ke teks umum.
  const [partyLabel, setPartyLabel] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  // Pihak dari tombol "Catat bayar" di daftar pihak bersaldo (PY-01); null = form kosong.
  const [payParty, setPayParty] = useState<{ id: number; type: "customer" | "supplier" } | null>(null);
  const [balanceKey, setBalanceKey] = useState(0);
  const [settleOpen, setSettleOpen] = useState(false);
  const [useCreditOpen, setUseCreditOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsyncData(
    (signal) => {
      if (tab !== "daftar") return Promise.resolve(null);
      return paymentService
        .list({ ...toPaymentFilterParams(filters), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          // Request yang dibatalkan (filter berganti) bukan kegagalan.
          if (!isAbort(err)) toast.danger("Gagal memuat pembayaran", toApiError(err).message);
          throw err;
        });
    },
    [filters, tab],
  );
  const items: Payment[] | null = data?.items ?? null;
  const total = data?.total ?? 0;

  const hasFilters =
    filters.search !== "" ||
    filters.method !== "" ||
    filters.direction !== "" ||
    filters.status !== "" ||
    filters.partyId > 0 ||
    filters.orderType !== "" ||
    filters.from !== "" ||
    filters.to !== "";

  const active: ActiveFilter[] = [];
  if (filters.direction)
    active.push({
      key: "direction",
      label: `Arah: ${directionLabel(filters.direction)}`,
      onRemove: () => setFilters({ direction: "" }),
    });
  if (filters.partyId > 0)
    active.push({
      key: "party",
      label: `Pihak: ${partyLabel || "terpilih"}`,
      onRemove: () => setFilters({ partyId: 0 }),
    });
  if (filters.from || filters.to)
    active.push({
      key: "date",
      label: `Dibayar: ${filters.from ? formatDate(filters.from) : "…"} – ${filters.to ? formatDate(filters.to) : "…"}`,
      onRemove: () => setFilters({ from: "", to: "" }),
    });
  if (filters.method)
    active.push({
      key: "method",
      label: `Metode: ${methodLabel(filters.method)}`,
      onRemove: () => setFilters({ method: "" }),
    });
  if (filters.status)
    active.push({
      key: "status",
      label: `Status: ${STATUS_OPTIONS.find((o) => o.value === filters.status)?.label ?? filters.status}`,
      onRemove: () => setFilters({ status: "" }),
    });
  if (filters.orderType)
    active.push({
      key: "orderType",
      label: `Dokumen: ${filters.orderType === "sales" ? "Order jual" : "Order beli"}${filters.orderId > 0 ? " (dipilih)" : ""}`,
      onRemove: () => setFilters({ orderType: "", orderId: 0 }),
    });
  const moreActiveCount = [filters.method, filters.status, filters.orderType].filter((v) => v !== "").length;

  // Ringkasan satu baris untuk halaman yang sedang tampil (pembayaran aktif saja).
  const pageIn = (items ?? [])
    .filter((p) => p.status === "active" && p.direction === "in")
    .reduce((s, p) => s + p.amount, 0);
  const pageOut = (items ?? [])
    .filter((p) => p.status === "active" && p.direction === "out")
    .reduce((s, p) => s + p.amount, 0);

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
        reference: values.reference,
        amountTendered: values.amountTendered,
        clientRef: values.clientRef,
        allocations: values.allocations.length > 0 ? values.allocations : undefined,
      });
      toast.fromServer(res.message, "Pembayaran dicatat", res.data.number);
      setCreateOpen(false);
      setPayParty(null);
      setBalanceKey((k) => k + 1);
      setFilters({ page: 1 });
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
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
      setFilters({ page: 1 });
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUseCredit(values: UseCreditPayload) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await paymentService.useCredit(values);
      toast.fromServer(res.message, "Saldo uang muka dipakai", res.data.number);
      setUseCreditOpen(false);
      setFilters({ page: 1 });
      setBalanceKey((k) => k + 1);
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  function openCreate() {
    setServerError(null);
    setPayParty(null);
    setCreateOpen(true);
  }

  function openPayFor(p: OpenParty) {
    setServerError(null);
    setPayParty({ id: p.partyId, type: p.side === "payable" ? "supplier" : "customer" });
    setCreateOpen(true);
  }

  return (
    <Page>
      <PageHeader
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
                Offset kredit
              </Button>
              <Button
                variant="secondary"
                onClick={() => {
                  setServerError(null);
                  setUseCreditOpen(true);
                }}
              >
                Pakai saldo
              </Button>
              <Button onClick={openCreate}>Catat pembayaran</Button>
            </>
          ) : undefined
        }
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-6">
        <TabsList variant="line">
          <TabsTrigger value="daftar">Daftar pembayaran</TabsTrigger>
          <TabsTrigger value="saldo">Saldo pihak</TabsTrigger>
        </TabsList>

        <TabsContent value="daftar" className="flex flex-col gap-6">
          <Toolbar
            right={<SavedViews listKey="payments" columnsKey="payments" />}
            search={
              <SearchInput
                aria-label="Cari pembayaran"
                value={filters.search}
                onSearch={(v) => setFilters({ search: v })}
                placeholder="Nomor, pihak, catatan, referensi…"
              />
            }
            primary={
              <>
                <FormField label="Arah">
                  <ToggleGroup
                    type="single"
                    variant="outline"
                    aria-label="Arah pembayaran"
                    value={filters.direction}
                    onValueChange={(v) => setFilters({ direction: v as typeof filters.direction })}
                  >
                    <ToggleGroupItem value="" aria-label="Semua arah">
                      Semua
                    </ToggleGroupItem>
                    <ToggleGroupItem value="in">Masuk</ToggleGroupItem>
                    <ToggleGroupItem value="out">Keluar</ToggleGroupItem>
                  </ToggleGroup>
                </FormField>
                <div className="min-w-56">
                  <FormField label="Pihak">
                    <PartySearchSelect
                      partyKind="any"
                      value={filters.partyId || null}
                      placeholder="Semua pihak"
                      onChange={(id, party) => {
                        setPartyLabel(party?.name ?? "");
                        setFilters({ partyId: id ?? 0 });
                      }}
                    />
                  </FormField>
                </div>
                <DateRangeFields
                  from={filters.from}
                  to={filters.to}
                  fromLabel="Dibayar dari"
                  toLabel="Dibayar sampai"
                  onChange={({ from, to }) => setFilters({ from, to })}
                />
              </>
            }
            more={
              <>
                <FormField label="Metode">
                  <SelectInput
                    aria-label="Metode"
                    value={filters.method}
                    onChange={(e) => setFilters({ method: e.target.value as typeof filters.method })}
                  >
                    {METHOD_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
                <FormField label="Status">
                  <SelectInput
                    aria-label="Status"
                    value={filters.status}
                    onChange={(e) => setFilters({ status: e.target.value as typeof filters.status })}
                  >
                    {STATUS_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
                <FormField label="Jenis dokumen">
                  <SelectInput
                    aria-label="Jenis dokumen"
                    value={filters.orderType}
                    onChange={(e) =>
                      setFilters({ orderType: e.target.value as typeof filters.orderType, orderId: 0 })
                    }
                  >
                    <option value="">Semua dokumen</option>
                    <option value="sales">Order jual</option>
                    <option value="purchase">Order beli</option>
                  </SelectInput>
                </FormField>
                {filters.orderType === "sales" && (
                  <FormField label="Order jual">
                    <SalesOrderSearchSelect
                      value={filters.orderId || null}
                      allowClear
                      placeholder="Semua order jual"
                      onChange={(id) => setFilters({ orderId: id ?? 0 })}
                    />
                  </FormField>
                )}
                {filters.orderType === "purchase" && (
                  <FormField label="Order beli">
                    <PurchaseOrderSearchSelect
                      value={filters.orderId || null}
                      allowClear
                      placeholder="Semua order beli"
                      onChange={(id) => setFilters({ orderId: id ?? 0 })}
                    />
                  </FormField>
                )}
              </>
            }
            moreActiveCount={moreActiveCount}
            active={active}
            onClearAll={() => {
              setPartyLabel("");
              reset();
            }}
          />

          <SectionCard flush>
            {items && items.length > 0 && (
              <p className="border-b border-border px-5 py-3 text-sm text-muted-foreground">
                <span className="tabular">{total}</span> pembayaran · di halaman ini masuk <Money value={pageIn} />{" "}
                · keluar <Money value={pageOut} />
              </p>
            )}
            <DataTable<Payment>
              columnsKey="payments"
              rows={items}
              rowKey={(r) => r.id}
              loading={loading}
              error={error}
              onRetry={reload}
              onRowClick={(r) => void navigate(`${ROUTE_PATHS.payments}/${r.id}`)}
              emptyTitle={hasFilters ? "Tidak ada pembayaran yang cocok" : "Belum ada pembayaran"}
              emptyDescription={
                hasFilters
                  ? "Ubah atau hapus filter untuk melihat pembayaran lain."
                  : "Pembayaran yang dicatat akan muncul di sini."
              }
              emptyAction={
                !hasFilters && canCreate ? <Button onClick={openCreate}>Catat pembayaran pertama</Button> : undefined
              }
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link to={`${ROUTE_PATHS.payments}/${r.id}`} className="font-medium text-primary hover:underline">
                      <DocNumber value={r.number} />
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.paidAt) },
                { header: "Pihak", render: (r) => r.partyName || "-" },
                {
                  header: "Metode",
                  render: (r) => (
                    <div className="flex flex-col items-start gap-0.5">
                      <Badge tone="neutral">{methodLabel(r.method)}</Badge>
                      {paymentKind(r) !== "bill" && (
                        <span className="text-xs text-muted-foreground">{paymentKindLabel(r)}</span>
                      )}
                    </div>
                  ),
                },
                {
                  header: "Jumlah",
                  align: "right",
                  render: (r) => <Money value={r.amount} className="font-medium" />,
                },
                { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
              ]}
            />
            <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
          </SectionCard>
        </TabsContent>

        <TabsContent value="saldo">
          <PartyBalanceSection canPay={canCreate} refreshKey={balanceKey} onPay={openPayFor} />
        </TabsContent>
      </Tabs>

      {createOpen && (
        <PaymentFormModal
          open
          submitting={submitting}
          serverError={serverError}
          initialParty={payParty ?? undefined}
          onClose={() => setCreateOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}
      {useCreditOpen && (
        <UseCreditModal
          open
          submitting={submitting}
          serverError={serverError}
          onClose={() => setUseCreditOpen(false)}
          onSubmit={(v) => void handleUseCredit(v)}
        />
      )}
      {settleOpen && (
        <SettleCreditModal
          open
          submitting={submitting}
          serverError={serverError}
          onClose={() => setSettleOpen(false)}
          onSubmit={(v) => void handleSettle(v)}
        />
      )}
    </Page>
  );
}
