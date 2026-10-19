// Kelengkapan dokumen surat jalan reguler sebelum dikonfirmasi — cermin
// requireEvidence backend (delivery/application/service.go): sopir + petugas
// gudang pada dokumen, nama penerima + status tanda tangan saat konfirmasi,
// dan minimal satu foto/scan SJ fisik bertanda tangan (DL-01; foto lokasi
// bersifat opsional dan tidak memenuhi syarat ini). Pengiriman POS dan SJ pengganti retur
// dikecualikan; perusahaan boleh mematikan aturan (setting
// delivery_require_evidence=false) — klien tidak bisa membacanya, jadi hasil
// di sini PETUNJUK; server tetap penentu dan galat per kolomnya ditampilkan.

export type EvidenceKey = "driverName" | "warehouseStaffName" | "recipientName" | "signedDocument";

export interface EvidenceCheck {
  key: EvidenceKey;
  label: string;
  ok: boolean;
  /** Cara memenuhinya bila belum. */
  hint: string;
}

export function evidenceRequired(channel: string | null | undefined, documentKind: string | null | undefined): boolean {
  if (channel === "pos") return false;
  if (documentKind === "replacement") return false;
  return true;
}

export interface EvidenceInput {
  driverName: string;
  warehouseStaffName: string;
  /** Isian form konfirmasi saat ini. */
  recipientName: string;
  signatureStatus: string;
  /** Jumlah scan/foto SJ bertanda tangan yang sudah diunggah. */
  signedDocumentCount: number;
}

export function evidenceChecklist(i: EvidenceInput): EvidenceCheck[] {
  return [
    {
      key: "driverName",
      label: "Nama sopir pada surat jalan",
      ok: i.driverName.trim() !== "",
      hint: "Sopir diisi saat surat jalan dibuat dan tidak bisa diubah. Batalkan draf ini dan buat ulang dengan sopir terisi.",
    },
    {
      key: "warehouseStaffName",
      label: "Nama petugas gudang pada surat jalan",
      ok: i.warehouseStaffName.trim() !== "",
      hint: "Petugas gudang diisi saat surat jalan dibuat dan tidak bisa diubah. Batalkan draf ini dan buat ulang dengan petugas terisi.",
    },
    {
      key: "recipientName",
      label: "Nama penerima dan status tanda tangan",
      ok: i.recipientName.trim() !== "" && i.signatureStatus.trim() !== "",
      hint: "Isi nama penerima dan pilih status tanda tangan pada form konfirmasi.",
    },
    {
      key: "signedDocument",
      label: "Foto/scan surat jalan fisik bertanda tangan (minimal satu)",
      ok: i.signedDocumentCount > 0,
      hint: "Unggah foto atau scan surat jalan fisik yang sudah ditandatangani penerima sebelum konfirmasi.",
    },
  ];
}

export function missingEvidence(checks: EvidenceCheck[]): EvidenceCheck[] {
  return checks.filter((c) => !c.ok);
}
