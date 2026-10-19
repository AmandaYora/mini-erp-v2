import { z } from "zod";
import { idFilter, pageFilter } from "@/shared/lib/list-schema";

// Filter daftar Penerimaan Barang (GET /goods-receipts): backend hanya
// mengenal purchaseOrderId + paginasi. State hidup di URL.
export const goodsreceiptListFilterSchema = z.object({
  purchaseOrderId: idFilter,
  page: pageFilter,
});

export type GoodsReceiptListFilter = z.infer<typeof goodsreceiptListFilterSchema>;

export const GOODSRECEIPT_LIST_DEFAULTS: GoodsReceiptListFilter = {
  purchaseOrderId: 0,
  page: 1,
};
