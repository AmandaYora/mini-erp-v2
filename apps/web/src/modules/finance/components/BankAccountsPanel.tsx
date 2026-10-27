import { useState } from "react";
import { Archive, Pencil, RotateCcw } from "lucide-react";
import { Badge, Button, ConfirmDialog, DataTable, SectionCard, StatusBadge } from "@/shared/components/ui";
import { ActionMenu } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import type { BankAccountPayload } from "@/modules/finance/services/finance.service";
import type { Account, BankAccount } from "@/modules/finance/types";
import { BankAccountFormSheet } from "@/modules/finance/components/BankAccountFormSheet";
import { invalidateBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";

interface BankAccountsPanelProps {
  accounts: Account[];
  cashAccountId: number | null;
  canManage: boolean;
  /** Bagan akun dimuat ulang setelah rekening membuat akun baru. */
  onChartChanged: () => void;
}

// Rekening perusahaan (ADR-0026): daftar + tambah/ubah/arsipkan.
export function BankAccountsPanel({ accounts, cashAccountId, canManage, onChartChanged }: BankAccountsPanelProps) {
  const { data, loading, error, reload } = useAsyncData(() => financeService.bankAccounts(), []);
  const rows = data ?? [];
  const [mode, setMode] = useState<"create" | "edit" | null>(null);
  const [editing, setEditing] = useState<BankAccount | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<BankAccount | null>(null);

  function refresh() {
    invalidateBankAccounts();
    reload();
  }

  async function handleSubmit(values: BankAccountPayload) {
    setSubmitting(true);
    setServerError(null);
    try {
      if (mode === "create") {
        const res = await financeService.createBankAccount(values);
        toast.fromServer(res.message, "Rekening ditambahkan", `${res.data.bankName} ${res.data.accountNumber}`);
        onChartChanged();
      } else if (editing) {
        const res = await financeService.updateBankAccount(editing.id, values);
        toast.fromServer(res.message, "Rekening disimpan", `${res.data.bankName} ${res.data.accountNumber}`);
      }
      setMode(null);
      setEditing(null);
      refresh();
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  async function setStatus(b: BankAccount, active: boolean) {
    try {
      const res = active ? await financeService.restoreBankAccount(b.id) : await financeService.archiveBankAccount(b.id);
      toast.fromServer(res.message, active ? "Rekening diaktifkan" : "Rekening diarsipkan", `${b.bankName} ${b.accountNumber}`);
      setArchiveTarget(null);
      refresh();
    } catch (err) {
      toast.danger(active ? "Gagal mengaktifkan rekening" : "Gagal mengarsipkan rekening", formatApiError(err));
    }
  }

  const activeCount = rows.filter((b) => b.status === "active").length;

  return (
    <div className="flex flex-col gap-6">
      {activeCount > 0 ? (
        <Notice tone="info" title="Pembayaran transfer/cek wajib memilih rekening">
          Rekening default terisi otomatis di order baru dan pembayaran transfer. Saldo tiap rekening ada di laporan Kas &amp; Rekening.
        </Notice>
      ) : (
        <Notice tone="info" title="Belum ada rekening">
          Selama daftar ini kosong, pembayaran transfer/cek tetap dicatat ke akun pemetaan "bank" seperti sebelumnya.
        </Notice>
      )}
      <SectionCard
        flush
        title="Rekening bank perusahaan"
        description="Dipilih saat pembayaran transfer/cek dicatat."
        actions={
          canManage ? (
            <Button
              size="sm"
              onClick={() => {
                setEditing(null);
                setServerError(null);
                setMode("create");
              }}
            >
              Tambah rekening
            </Button>
          ) : undefined
        }
      >
        <DataTable<BankAccount>
          caption="Rekening bank perusahaan"
          rows={rows}
          rowKey={(b) => b.id}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada rekening"
          emptyDescription="Tambah rekening agar kasir bisa memilih tujuan transfer."
          columns={[
            {
              header: "Rekening",
              render: (b) => (
                <div>
                  <p className="font-medium text-foreground">
                    {b.bankName} {b.accountNumber}
                  </p>
                  <p className="text-xs text-muted-foreground">a.n. {b.accountHolder}</p>
                </div>
              ),
            },
            {
              header: "Akun buku besar",
              render: (b) => (
                <span>
                  <span className="font-mono text-xs">{b.accountCode}</span> {b.accountName}
                </span>
              ),
            },
            {
              header: "Penanda",
              render: (b) => (
                <div className="flex flex-wrap gap-1">
                  {b.isDefault && <Badge tone="info">Default</Badge>}
                  {b.showOnDocuments && <Badge tone="neutral">Di dokumen</Badge>}
                </div>
              ),
            },
            { header: "Status", render: (b) => <StatusBadge status={b.status} /> },
            {
              header: "",
              align: "right",
              render: (b) =>
                canManage ? (
                  <ActionMenu
                    label={`Aksi untuk rekening ${b.bankName} ${b.accountNumber}`}
                    variant="ghost"
                    items={
                      b.status === "active"
                        ? [
                            {
                              key: "edit",
                              label: "Ubah",
                              icon: <Pencil aria-hidden />,
                              onSelect: () => {
                                setEditing(b);
                                setServerError(null);
                                setMode("edit");
                              },
                            },
                            { key: "archive", label: "Arsipkan", icon: <Archive aria-hidden />, destructive: true, onSelect: () => setArchiveTarget(b) },
                          ]
                        : [{ key: "restore", label: "Aktifkan lagi", icon: <RotateCcw aria-hidden />, onSelect: () => void setStatus(b, true) }]
                    }
                  />
                ) : null,
            },
          ]}
        />
      </SectionCard>

      {mode && (
        <BankAccountFormSheet
          key={mode === "edit" ? editing?.id : "create"}
          mode={mode}
          initial={editing}
          accounts={accounts}
          existing={rows}
          cashAccountId={cashAccountId}
          submitting={submitting}
          serverError={serverError}
          onClose={() => {
            setMode(null);
            setEditing(null);
          }}
          onSubmit={(v) => void handleSubmit(v)}
        />
      )}

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan rekening?"
        message={`Rekening ${archiveTarget?.bankName ?? ""} ${archiveTarget?.accountNumber ?? ""} tidak bisa dipilih lagi untuk tagihan dan pembayaran baru. Riwayat dan saldonya tetap. Lanjutkan?`}
        confirmLabel="Arsipkan"
        onConfirm={() => archiveTarget && void setStatus(archiveTarget, false)}
        onCancel={() => setArchiveTarget(null)}
      />
    </div>
  );
}
