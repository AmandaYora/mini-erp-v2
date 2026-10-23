import { z } from "zod";
import { enumOrEmpty, pageFilter, searchFilter } from "@/shared/lib/list-schema";

// Filter daftar customer/supplier/tipe member (GET /customers, /suppliers,
// /member-types). State hidup di URL. Default "Aktif" (arsip tidak bercampur);
// "all" = pilihan eksplisit "Semua status" — nilai terpisah agar tetap tahan
// di URL walau default-nya "active" (nilai default tidak ditulis ke URL).
export const partyListFilterSchema = z.object({
  search: searchFilter,
  status: z.enum(["active", "archived", "all"]),
  // Nama ganda (alert kesehatan data A10, hanya halaman pelanggan memakainya).
  duplicateName: enumOrEmpty(["1"] as const),
  page: pageFilter,
});

export type PartyListFilter = z.infer<typeof partyListFilterSchema>;

export const PARTY_LIST_DEFAULTS: PartyListFilter = {
  search: "",
  status: "active",
  duplicateName: "",
  page: 1,
};

/** Label chip/penjelas untuk status non-default. */
export function partyStatusLabel(status: PartyListFilter["status"]): string {
  if (status === "archived") return "Diarsipkan";
  if (status === "all") return "Semua status";
  return "Aktif";
}

/** Nilai param API: "all" berarti tanpa filter status. */
export function partyStatusParam(status: string): string {
  return status === "all" ? "" : status;
}
