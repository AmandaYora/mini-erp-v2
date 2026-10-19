import { z } from "zod";
import { enumOrEmpty, pageFilter } from "@/shared/lib/list-schema";

// Filter daftar Transfer (GET /stock/transfers): backend hanya mengenal status
// tunggal + paginasi. State hidup di URL.
export const TRANSFER_STATUSES = ["draft", "dispatched", "received", "cancelled"] as const;

// Arah relatif cabang aktif (ST-06): keluar = ke cabang lain, masuk = dari cabang lain, dalam = satu cabang.
export const TRANSFER_DIRECTIONS = ["out", "in", "internal"] as const;

export const transferListFilterSchema = z.object({
  status: enumOrEmpty(TRANSFER_STATUSES),
  direction: enumOrEmpty(TRANSFER_DIRECTIONS),
  page: pageFilter,
});

export type TransferListFilter = z.infer<typeof transferListFilterSchema>;

export const TRANSFER_LIST_DEFAULTS: TransferListFilter = { status: "", direction: "", page: 1 };
