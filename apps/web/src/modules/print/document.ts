// Identitas & format dokumen cetak (Tahap E).
//
// Port dari print-shared.tsx & utils.ts sistem lama: pengelompokan telepon
// per label, resolusi kontak ship-to, terbilang, format tanggal dokumen, dan
// nama barang. Dokumen yang dipegang pelanggan harus tercetak sama dengan
// sistem lama (identitas perusahaan), jadi format di sini sengaja mengikuti
// legacy, bukan helper tampilan layar di shared/lib/format.

import { methodLabel } from "@/modules/payment/types";
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
  /** Cabang sesi: alamat & kota kop bila profil dokumen belum diisi (paritas legacy). */
  branchAddress?: string | null;
  branchCity?: string | null;
}

/** Bentuk identitas cetak dari profil + fallback perusahaan sesi. */
export function resolveIdentity(
  profile: DocumentProfile,
  fallback: IdentityFallback = {},
): PrintIdentity {
  return {
    name:
      clean(profile.headerName) || clean(fallback.companyName) || "Mini ERP",
    addressLine: clean(profile.addressLine) || clean(fallback.branchAddress),
    cityLine:
      clean(profile.cityLine) ||
      clean(fallback.branchCity) ||
      clean(fallback.companyCity),
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

// --- terbilang rupiah (sama persis dengan utils.terbilangRupiah sistem lama) ---

const UNITS = [
  "nol",
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

const SCALES: [number, string][] = [
  [1_000_000_000_000, "triliun"],
  [1_000_000_000, "miliar"],
  [1_000_000, "juta"],
  [1000, "ribu"],
];

function spell(n: number): string {
  if (n < 12) return UNITS[n];
  if (n < 20) return `${spell(n - 10)} belas`;
  if (n < 100) {
    const rest = n % 10;
    return `${spell(Math.floor(n / 10))} puluh${rest > 0 ? ` ${spell(rest)}` : ""}`;
  }
  if (n < 200) {
    const rest = n - 100;
    return `seratus${rest > 0 ? ` ${spell(rest)}` : ""}`;
  }
  if (n < 1000) {
    const rest = n % 100;
    return `${spell(Math.floor(n / 100))} ratus${rest > 0 ? ` ${spell(rest)}` : ""}`;
  }
  if (n < 2000) {
    const rest = n - 1000;
    return `seribu${rest > 0 ? ` ${spell(rest)}` : ""}`;
  }
  for (const [value, name] of SCALES) {
    if (n >= value) {
      const rest = n % value;
      return `${spell(Math.floor(n / value))} ${name}${rest > 0 ? ` ${spell(rest)}` : ""}`;
    }
  }
  return UNITS[0];
}

/**
 * "125000" -> "seratus dua puluh lima ribu rupiah". Sama dengan sistem lama:
 * nol -> "nol rupiah", negatif diawali "minus", pecahan dibulatkan.
 */
export function terbilangRupiah(amount: number): string {
  const safe = Number.isFinite(amount) ? amount : 0;
  const value = Math.round(Math.abs(safe));
  if (value === 0) return "nol rupiah";
  return `${safe < 0 ? "minus " : ""}${spell(value)} rupiah`;
}

// --- format tanggal dokumen cetak (sama dengan utils sistem lama) ---

function toDate(v: string | null | undefined): Date | null {
  if (!v) return null;
  const d = new Date(v.length <= 10 ? `${v}T00:00:00` : v.includes(" ") && !v.includes("T") ? v.replace(" ", "T") : v);
  return Number.isNaN(d.getTime()) ? null : d;
}

function formatWith(v: string | null | undefined, opts: Intl.DateTimeFormatOptions): string {
  const d = toDate(v);
  return d === null ? "-" : new Intl.DateTimeFormat("id-ID", opts).format(d);
}

/** "24 Juni 2026" — tanggal dokumen (nota, surat jalan). */
export function formatLongDate(v: string | null | undefined): string {
  return formatWith(v, { day: "2-digit", month: "long", year: "numeric" });
}

/** "24 Jun 2026" — tanggal ringkas (kepala kwitansi). */
export function formatDocDate(v: string | null | undefined): string {
  return formatWith(v, { day: "2-digit", month: "short", year: "numeric" });
}

/** "24 Jun 2026 14.30" — cap waktu cetak & tanggal bayar. */
export function formatDocDateTime(v: string | null | undefined): string {
  return formatWith(v, { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

/** "24/06/26 14.30" — tanggal struk termal. */
export function formatReceiptDateTime(v: string | null | undefined): string {
  return formatWith(v, { day: "2-digit", month: "2-digit", year: "2-digit", hour: "2-digit", minute: "2-digit" });
}

/** Kuantitas struk: maksimal 3 desimal, format id-ID. */
export function formatReceiptQuantity(qty: number): string {
  return new Intl.NumberFormat("id-ID", { maximumFractionDigits: 3 }).format(qty);
}

/**
 * Nama barang tercetak: "Produk - Varian", varian hanya bila memang varian
 * nyata. Varian bawaan tersembunyi menyalin nama & kode produknya, jadi tidak
 * pernah tercetak "PAKU - PAKU".
 */
export function printItemName(line: {
  productName: string;
  productCode?: string;
  variantName?: string;
  variantCode?: string;
}): string {
  const name = line.productName.trim();
  const variant = (line.variantName ?? "").trim();
  const isDefault =
    variant === "" ||
    variant === name ||
    ((line.variantCode ?? "").trim() !== "" && (line.variantCode ?? "").trim() === (line.productCode ?? "").trim());
  return isDefault ? name : `${name} - ${variant}`;
}

/** Label metode bayar pada dokumen cetak (istilah sistem lama). */
export function printMethodLabel(method: string | null | undefined): string {
  switch ((method ?? "").toLowerCase()) {
    case "cash":
      return "Tunai";
    case "transfer":
      return "Transfer Bank";
    case "check":
      return "Cek / Giro";
    default:
      // Metode buatan v2 (offset, saldo uang muka) memakai label aplikasi.
      return methodLabel(method);
  }
}
