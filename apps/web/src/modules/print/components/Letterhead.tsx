import {
  formatPhoneGroup,
  groupPhonesByLabel,
  type DocumentBankAccount,
  type PrintIdentity,
} from "@/modules/print/document";
import { PRINT_LOGO_SRC } from "@/modules/print/components/print-assets";

/**
 * Kop surat (logo + nama + alamat + telepon). Ukuran font lewat
 * var(--print-*-size) agar bisa dipadatkan per mode kertas tanpa prop baru:
 * custom property ikut cascade biasa walau font-size dipasang inline.
 */
export function Letterhead({ identity }: { identity: PrintIdentity }) {
  const phoneGroups = groupPhonesByLabel(identity.phones);
  return (
    <div
      style={{
        display: "flex",
        gap: "12px",
        alignItems: "flex-start",
        minWidth: 0,
      }}
    >
      <img
        alt="Logo"
        src={PRINT_LOGO_SRC}
        style={{ width: "62px", height: "auto", flex: "0 0 auto" }}
      />
      <div style={{ minWidth: 0 }}>
        <div
          style={{
            fontSize: "var(--print-name-size, 15pt)",
            fontWeight: 800,
            lineHeight: 1.15,
          }}
        >
          {identity.name}
        </div>
        {identity.addressLine !== "" ? (
          <div
            style={{ fontSize: "var(--print-detail-size, 8.5pt)", marginTop: "2px" }}
          >
            {identity.addressLine}
          </div>
        ) : null}
        {phoneGroups.map((group, index) => (
          <div
            key={`${group.label}-${index}`}
            style={{ fontSize: "var(--print-detail-size, 8.5pt)" }}
          >
            HP : {formatPhoneGroup(group)}
          </div>
        ))}
      </div>
    </div>
  );
}

/** Daftar rekening transfer dalam kotak berlabel (nota & surat jalan). */
export function BankAccountsBlock({
  accounts,
  title = "REKENING TRANSFER MELALUI:",
}: {
  accounts: DocumentBankAccount[];
  title?: string;
}) {
  if (accounts.length === 0) return null;
  return (
    <div style={{ fontSize: "var(--print-detail-size, 9pt)" }}>
      <div style={{ fontWeight: 700, marginBottom: "4px" }}>{title}</div>
      <div style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
        {accounts.map((account, index) => (
          <div
            key={`${account.bank}-${account.number}-${index}`}
            style={{ display: "flex", alignItems: "center", gap: "8px" }}
          >
            <span
              style={{
                border: "1px solid #111",
                fontWeight: 800,
                padding: "2px 6px",
                minWidth: "34px",
                textAlign: "center",
              }}
            >
              {account.bank}
            </span>
            <span>
              <strong>{account.holder}</strong>
              <br />
              {account.number}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
