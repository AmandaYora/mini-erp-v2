import { useState } from "react";
import { ChevronDown, ChevronRight, Archive, Pencil } from "lucide-react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  PageHeader,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { ActionMenu, Page, Toolbar, useUrlTab } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { financeService } from "@/modules/finance/services/finance.service";
import type { AccountCreateValues, AccountEditValues } from "@/modules/finance/schemas/finance.schema";
import { buildAccountTree, filterAccounts } from "@/modules/finance/lib/account-tree";
import type { AccountTreeRow } from "@/modules/finance/lib/account-tree";
import { MAPPING_KEYS, mappingKeyLabel, missingMappingKeys } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";
import { AccountFormSheet } from "@/modules/finance/components/AccountFormSheet";
import { MappingSheet } from "@/modules/finance/components/MappingSheet";

const TABS = ["bagan", "pemetaan"] as const;

// Akun + pemetaan (T4). Bagan akun = pohon golongan (tipe) → akun; Ubah/Arsipkan lewat menu baris.
// Gate finance.manage (halaman di balik menu "Akun"; tombol juga dicek via can()).
export default function AccountsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data, loading, error, reload } = useAsyncData(
    () => Promise.all([financeService.accounts(), financeService.mappings()]).then(([accs, maps]) => ({ accounts: accs ?? [], mappings: maps ?? {} })),
    [],
  );
  const accounts = data?.accounts ?? [];
  const mappings = data?.mappings ?? {};

  const [tab, setTab] = useUrlTab(TABS, "bagan");
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "active" | "archived">("");
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const [modalMode, setModalMode] = useState<"create" | "edit" | null>(null);
  const [editing, setEditing] = useState<Account | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<Account | null>(null);

  const [mappingKey, setMappingKey] = useState<string | null>(null);
  const [mappingSubmitting, setMappingSubmitting] = useState(false);
  const [mappingError, setMappingError] = useState<string | null>(null);

  const filtered = filterAccounts(accounts, query, statusFilter);
  const treeRows = buildAccountTree(filtered, collapsed, query.trim() !== "");
  const hasFilter = query.trim() !== "" || statusFilter !== "";

  function toggleGroup(type: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
  }

  async function handleAccountSubmit(values: AccountCreateValues | AccountEditValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      if (modalMode === "create") {
        const v = values as AccountCreateValues;
        const res = await financeService.createAccount({ code: v.code, name: v.name, type: v.type, isCash: v.isCash });
        toast.fromServer(res.message, "Akun dibuat", res.data.code);
      } else if (editing) {
        const v = values as AccountEditValues;
        const res = await financeService.updateAccount(editing.id, { name: v.name, type: v.type, isCash: v.isCash });
        toast.fromServer(res.message, "Akun disimpan", res.data.code);
      }
      setModalMode(null);
      setEditing(null);
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
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
      toast.danger("Gagal mengarsipkan akun", formatApiError(err));
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
      setMappingError(formatApiError(err));
    } finally {
      setMappingSubmitting(false);
    }
  }

  const missing = missingMappingKeys(mappings);

  return (
    <Page>
      <PageHeader
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
              Tambah akun
            </Button>
          ) : undefined
        }
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-6">
        <TabsList variant="line">
          <TabsTrigger value="bagan">Bagan akun</TabsTrigger>
          <TabsTrigger value="pemetaan">
            Pemetaan {missing.length > 0 && <Badge tone="warning">{missing.length}</Badge>}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="bagan" className="flex flex-col gap-6">
          <Toolbar
            search={<SearchInput aria-label="Cari akun" value={query} onSearch={setQuery} placeholder="Kode atau nama akun…" />}
            primary={
              <FormField label="Status">
                <SelectInput aria-label="Status akun" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}>
                  <option value="">Semua</option>
                  <option value="active">Aktif</option>
                  <option value="archived">Diarsipkan</option>
                </SelectInput>
              </FormField>
            }
            active={statusFilter ? [{ key: "status", label: `Status: ${statusFilter === "active" ? "Aktif" : "Diarsipkan"}`, onRemove: () => setStatusFilter("") }] : []}
            onClearAll={() => {
              setQuery("");
              setStatusFilter("");
            }}
          />
          <SectionCard flush title="Bagan akun" description="Dikelompokkan menurut tipe akun; klik golongan untuk melipat.">
            <DataTable<AccountTreeRow>
              caption="Bagan akun"
              rows={treeRows}
              rowKey={(r) => r.id}
              loading={loading}
              error={error}
              onRetry={reload}
              onRowClick={(r) => {
                if (r.kind === "group") toggleGroup(r.type);
              }}
              emptyTitle={hasFilter ? "Tidak ada akun yang cocok" : "Belum ada akun"}
              emptyDescription={hasFilter ? "Ubah kata kunci atau filter status." : "Tambah akun pertama untuk menyusun bagan akun."}
              columns={[
                {
                  header: "Akun",
                  render: (r) =>
                    r.kind === "group" ? (
                      <button
                        type="button"
                        onClick={() => toggleGroup(r.type)}
                        aria-expanded={r.expanded}
                        className="inline-flex items-center gap-2 font-semibold text-foreground"
                      >
                        {r.expanded ? <ChevronDown size={16} aria-hidden /> : <ChevronRight size={16} aria-hidden />}
                        {r.label}
                        <span className="tabular text-xs font-normal text-muted-foreground">({r.count})</span>
                      </button>
                    ) : (
                      <span className="pl-6">
                        <span className="font-mono text-xs">{r.account.code}</span> {r.account.name}
                      </span>
                    ),
                },
                { header: "Kas", render: (r) => (r.kind === "account" && r.account.isCash ? <Badge tone="info">Kas</Badge> : null) },
                { header: "Status", render: (r) => (r.kind === "account" ? <StatusBadge status={r.account.status} /> : null) },
                {
                  header: "",
                  align: "right",
                  render: (r) =>
                    r.kind === "account" && canManage && r.account.status === "active" ? (
                      <ActionMenu
                        label={`Aksi untuk akun ${r.account.code}`}
                        variant="ghost"
                        items={[
                          {
                            key: "edit",
                            label: "Ubah",
                            icon: <Pencil aria-hidden />,
                            onSelect: () => {
                              setEditing(r.account);
                              setServerError(null);
                              setModalMode("edit");
                            },
                          },
                          { key: "archive", label: "Arsipkan", icon: <Archive aria-hidden />, destructive: true, onSelect: () => setArchiveTarget(r.account) },
                        ]}
                      />
                    ) : null,
                },
              ]}
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="pemetaan" className="flex flex-col gap-6">
          {missing.length > 0 && (
            <Notice tone="warning" title={`${missing.length} pemetaan belum diatur`}>
              Pemetaan lengkap wajib sebelum periode bisa ditutup aman: {missing.map(mappingKeyLabel).join(", ")}.
            </Notice>
          )}
          <SectionCard flush title="Pemetaan akun" description="Akun otomatis yang dipakai builder posting per kejadian (18 kunci).">
            <DataTable<string>
              caption="Pemetaan akun"
              rows={[...MAPPING_KEYS]}
              rowKey={(k) => k}
              loading={loading}
              error={error}
              onRetry={reload}
              emptyTitle="Tidak ada kunci pemetaan"
              columns={[
                { header: "Kunci", render: (k) => mappingKeyLabel(k) },
                {
                  header: "Akun",
                  render: (k) => {
                    const a = mappings[k];
                    return a ? `${a.code} — ${a.name}` : <Badge tone="warning">Belum diatur</Badge>;
                  },
                },
                {
                  header: "",
                  align: "right",
                  render: (k) =>
                    canManage ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Ubah pemetaan ${mappingKeyLabel(k)}`}
                        onClick={() => {
                          setMappingError(null);
                          setMappingKey(k);
                        }}
                      >
                        Ubah
                      </Button>
                    ) : null,
                },
              ]}
            />
          </SectionCard>
        </TabsContent>
      </Tabs>

      {modalMode && (
        <AccountFormSheet
          key={modalMode === "edit" ? editing?.id : "create"}
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
        title="Arsipkan akun?"
        message={`Akun ${archiveTarget?.code ?? ""} (${archiveTarget?.name ?? ""}) akan diarsipkan. Akun berjurnal atau terpetakan ditolak backend. Lanjutkan?`}
        confirmLabel="Arsipkan"
        onConfirm={() => void handleArchive()}
        onCancel={() => setArchiveTarget(null)}
      />

      {mappingKey && (
        <MappingSheet
          mappingKey={mappingKey}
          currentAccountId={mappings[mappingKey]?.id ?? null}
          accounts={accounts}
          submitting={mappingSubmitting}
          serverError={mappingError}
          onClose={() => setMappingKey(null)}
          onSubmit={(v) => void handleMappingSubmit(v)}
        />
      )}
    </Page>
  );
}
