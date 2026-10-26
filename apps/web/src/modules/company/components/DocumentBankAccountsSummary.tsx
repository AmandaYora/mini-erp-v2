import { Link } from "react-router-dom";
import { Badge, buttonClass } from "@/shared/components/ui";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import { bankAccountLabel } from "@/modules/finance/lib/bank-accounts";
import type { DocumentBankAccount } from "@/modules/company/document-profile";

// Rekening di dokumen, baca saja (analysis/004): rekening perusahaan hanya diatur di
// Keuangan › Akun › Rekening bank. Server memindahkan daftar teks lama di profil dokumen ke
// master sekali saat start; selama master masih kosong teks lama itu yang tampil dan dicetak.
export default function DocumentBankAccountsSummary({ legacy }: { legacy: DocumentBankAccount[] }) {
  const canManageFinance = useAuthStore((s) => s.can("finance.manage"));
  const bank = useBankAccounts();
  const printed = bank.active.filter((b) => b.showOnDocuments);
  const legacyRows = legacy.filter((b) => b.number.trim() !== "");
  const fromMaster = bank.all.length > 0;

  return (
    <div>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="text-sm font-semibold text-foreground">Rekening di dokumen</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Rekening perusahaan diatur di Keuangan › Akun › Rekening bank. Daftar di sini hanya tampilan.
          </p>
        </div>
        {canManageFinance && (
          <Link
            to={`${ROUTE_PATHS.financeAccounts}?tab=rekening`}
            className={`${buttonClass("secondary", "sm")} hover:no-underline`}
          >
            Kelola rekening
          </Link>
        )}
      </div>
      {bank.loading ? (
        <p className="mt-2 text-sm text-muted-foreground">Memuat rekening…</p>
      ) : fromMaster ? (
        printed.length > 0 ? (
          <ul aria-label="Rekening di dokumen" className="mt-2 flex flex-col gap-1.5 text-sm text-foreground">
            {printed.map((b) => (
              <li key={b.id} className="flex flex-wrap items-center gap-2">
                {bankAccountLabel(b)}
                {b.isDefault && <Badge tone="info">Default</Badge>}
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">Tidak ada rekening yang ditandai tampil di dokumen.</p>
        )
      ) : legacyRows.length > 0 ? (
        <>
          <ul aria-label="Rekening di dokumen" className="mt-2 flex flex-col gap-1.5 text-sm text-foreground">
            {legacyRows.map((b, i) => (
              <li key={`${b.number}-${i}`}>
                {bankAccountLabel({ bankName: b.bank, accountNumber: b.number, accountHolder: b.holder })}
              </li>
            ))}
          </ul>
          <p className="mt-2 text-xs text-muted-foreground">
            Dari data sebelumnya. Dipindahkan otomatis ke Keuangan › Akun › Rekening bank saat aplikasi dimulai ulang.
          </p>
        </>
      ) : (
        <p className="mt-2 text-sm text-muted-foreground">Belum ada rekening.</p>
      )}
    </div>
  );
}
