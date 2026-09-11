// Identitas & format dokumen cetak (Tahap E).
//
// Port dari print-shared.tsx sistem lama: pengelompokan telepon per label,
// resolusi kontak ship-to, dan kop surat. Uang/tanggal memakai helper
// terpusat shared (formatIDR/formatDate) — modul ini hanya menambah
// terbilang rupiah yang belum ada di shared.

import type {
  DocumentBankAccount,
  DocumentPhone,
  DocumentProfile,
} from "@/modules/print/paper";

export type { DocumentBankAccount, DocumentPhone };

export interface PrintIdentity {
  name: string;
  addressLine: string;
  cityLine: string;
  tagline: string;
  phones: DocumentPhone[];
  bankAccounts: DocumentBankAccount[];
  sellingPoints: string[];
  returnNote: string;
  thanksNote: string;
}

function clean(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

export interface IdentityFallback {
  companyName?: string | null;
  companyCity?: string | null;
}

/** Bentuk identitas cetak dari profil + fallback perusahaan sesi. */
export function resolveIdentity(
  profile: DocumentProfile,
  fallback: IdentityFallback = {},
): PrintIdentity {
  return {
    name:
      clean(profile.headerName) || clean(fallback.companyName) || "Mini ERP",
    addressLine: clean(profile.addressLine),
    cityLine: clean(profile.cityLine) || clean(fallback.companyCity),
    tagline: clean(profile.tagline),
    phones: (profile.phones ?? []).filter((p) => clean(p?.number) !== ""),
    bankAccounts: (profile.bankAccounts ?? []).filter(
      (a) => clean(a?.number) !== "",
    ),
    sellingPoints: (profile.sellingPoints ?? [])
      .map((s) => clean(s))
      .filter((s) => s !== ""),
    returnNote: clean(profile.returnNote),
    thanksNote: clean(profile.thanksNote),
  };
}

/** Satu baris telepon: "TOKO (0813-...)" atau nomor polos bila tanpa label. */
export function formatPhoneLine(phone: DocumentPhone): string {
  const label = clean(phone.label);
  const number = clean(phone.number);
  return label !== "" ? `${label} (${number})` : number;
}

export interface PhoneGroup {
  label: string;
  numbers: string[];
}

/**
 * Kelompokkan nomor telepon per label — beberapa baris berlabel sama tampil
 * dalam SATU baris "HANDOKO (nomor1) / (nomor2)". Urutan grup mengikuti
 * kemunculan pertama tiap label.
 */
export function groupPhonesByLabel(phones: DocumentPhone[]): PhoneGroup[] {
  const groups: PhoneGroup[] = [];
  const indexByLabel = new Map<string, number>();
  for (const phone of phones) {
    const label = clean(phone.label);
    const number = clean(phone.number);
    if (number === "") continue;
    const existing = indexByLabel.get(label);
    if (existing === undefined) {
      indexByLabel.set(label, groups.length);
      groups.push({ label, numbers: [number] });
    } else {
      groups[existing].numbers.push(number);
    }
  }
  return groups;
}

/** "HANDOKO (nomor1) / (nomor2)", atau nomor polos bila label kosong. */
export function formatPhoneGroup(group: PhoneGroup): string {
  if (group.label === "") return group.numbers.join(" / ");
  return `${group.label} ${group.numbers.map((n) => `(${n})`).join(" / ")}`;
}

/**
 * Alamat & telepon yang dicetak: bila order memilih alamat kirim (ship-to),
 * keduanya mengikuti ship-to (telepon ship-to atau "-", JANGAN jatuh ke
 * telepon pelanggan agar kontak tidak berbeda lokasi dari alamat).
 */
export function resolveShipToContact(input: {
  shipToAddress?: string | null;
  shipToPhone?: string | null;
  fallbackAddress?: string | null;
  fallbackPhone?: string | null;
}): { address: string; phone: string } {
  const shipToAddress = clean(input.shipToAddress);
  if (shipToAddress !== "") {
    return {
      address: shipToAddress,
      phone: clean(input.shipToPhone) || "-",
    };
  }
  return {
    address: clean(input.fallbackAddress) || "-",
    phone: clean(input.fallbackPhone) || "-",
  };
}

// --- terbilang rupiah (bilangan bulat, untuk kwitansi/faktur) ---

const UNITS = [
  "",
  "satu",
  "dua",
  "tiga",
  "empat",
  "lima",
  "enam",
  "tujuh",
  "delapan",
  "sembilan",
  "sepuluh",
  "sebelas",
];

function spellBelowThousand(n: number): string {
  if (n < 12) return UNITS[n];
  if (n < 20) return `${spellBelowThousand(n - 10)} belas`;
  if (n < 100) {
    const rest = n % 10;
    return `${spellBelowThousand(Math.floor(n / 10))} puluh${rest > 0 ? ` ${spellBelowThousand(rest)}` : ""}`;
  }
  if (n < 200) {
    const rest = n - 100;
    return `seratus${rest > 0 ? ` ${spellBelowThousand(rest)}` : ""}`;
  }
  const rest = n % 100;
  return `${spellBelowThousand(Math.floor(n / 100))} ratus${rest > 0 ? ` ${spellBelowThousand(rest)}` : ""}`;
}

function spellPositive(n: number): string {
  if (n < 1000) return spellBelowThousand(n);
  if (n < 2000) {
    const rest = n - 1000;
    return `seribu${rest > 0 ? ` ${spellPositive(rest)}` : ""}`;
  }
  if (n < 1_000_000) {
    const rest = n % 1000;
    return `${spellPositive(Math.floor(n / 1000))} ribu${rest > 0 ? ` ${spellPositive(rest)}` : ""}`;
  }
  if (n < 1_000_000_000) {
    const rest = n % 1_000_000;
    return `${spellPositive(Math.floor(n / 1_000_000))} juta${rest > 0 ? ` ${spellPositive(rest)}` : ""}`;
  }
  if (n < 1_000_000_000_000) {
    const rest = n % 1_000_000_000;
    return `${spellPositive(Math.floor(n / 1_000_000_000))} milyar${rest > 0 ? ` ${spellPositive(rest)}` : ""}`;
  }
  const rest = n % 1_000_000_000_000;
  return `${spellPositive(Math.floor(n / 1_000_000_000_000))} trilyun${rest > 0 ? ` ${spellPositive(rest)}` : ""}`;
}

/** "125000" -> "seratus dua puluh lima ribu rupiah". Nol/negatif ditolak. */
export function terbilangRupiah(amount: number): string {
  if (!Number.isFinite(amount) || amount <= 0) return "-";
  return `${spellPositive(Math.floor(amount))} rupiah`;
}
