// Kunci JSON camelCase terverifikasi dari backend:
// - stock/presentation/handler.go: locationView {id,branchId,code,name,parentId,isSystem,status},
//   balanceView/contractBalanceView {branchId,productId,variantId,locationId,onHand,reserved,available},
//   movement inline {id,productId,variantId,locationId,direction,type,qty,refType,refId,notes,createdBy,createdAt},
//   transferView {id,number,fromBranchId,toBranchId,fromLocationId,toLocationId,status,items[]},
//   Adjust {productId,variantId,locationId,mode,qtyAfter,qtyDelta,reason,approver,approverPassword},
//   CreateTransfer {toBranchId,fromLocationId,toLocationId,notes,items[{productId,variantId,qty,notes}]},
//   damagedInput {productId,variantId,locationId,qty,reason},
//   write-off {productId,variantId,qty,reason} (tanpa location).
// - stock/application/opening.go: OpeningPreview {validRows,errors[]},
//   OpeningResult {posted,skipped,failures[]}, OpeningRowError {row,column,value,reason}.
// - product/presentation/handler.go: SearchOptions {id,code,name,sellingPrice},
//   productView variants {id,code,name,barcode,isDefault,status}.
// - branch/presentation/handler.go: branchView {id,code,name,address,city,phone,status,isHead}.

export interface StockLocation {
  id: number;
  branchId: number;
  code: string;
  name: string;
  parentId: number | null;
  isSystem: boolean;
  /** "normal" | "damaged" — ditentukan server, tidak dapat diubah dari form. */
  kind: string;
  status: string;
  /** Atribut alokasi (ST-01): area picking dan lokasi utama diprioritaskan; sortOrder = urutan. */
  isPickingArea: boolean;
  isPrimary: boolean;
  sortOrder: number;
}

export interface StockBalance {
  branchId: number;
  productId: number;
  variantId: number;
  locationId: number;
  onHand: number;
  reserved: number;
  available: number;
}

/** Baris GET /stock/balances (tanpa productId): membawa kode+nama produk. */
export interface StockBalanceRow extends StockBalance {
  productCode: string;
  productName: string;
}

export interface StockMovement {
  id: number;
  productId: number;
  variantId: number;
  locationId: number;
  direction: string;
  type: string;
  qty: number;
  refType: string;
  refId: number;
  notes: string;
  createdBy: number;
  createdAt: string;
  /** Saldo posisi sebelum/sesudah mutasi (ST-04); null untuk baris lama tanpa snapshot. */
  balanceBefore: number | null;
  balanceAfter: number | null;
  movedAt: string;
}

export interface TransferItem {
  productId: number;
  variantId: number;
  qty: number;
  notes: string;
  /** Tujuan per baris; 0 = pakai tujuan dokumen. */
  toLocationId?: number;
}

export interface StockTransfer {
  id: number;
  number: string;
  fromBranchId: number;
  toBranchId: number;
  fromLocationId: number;
  toLocationId: number;
  status: string;
  items: TransferItem[];
  /** Muka dokumen transfer (ST-02); kosong pada transfer lama. */
  notes?: string;
  driverName?: string;
  vehiclePlate?: string;
  /** YYYY-MM-DD (WIB). */
  transferDate?: string;
}

export interface ProductOption {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
}

export interface ProductVariantOption {
  id: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: string;
}

export interface ProductDetail {
  id: number;
  code: string;
  name: string;
  status: string;
  variants: ProductVariantOption[];
  /** Satuan dasar/beli/jual + faktornya ke satuan dasar (label dual-satuan, ST-06). */
  baseUom?: string;
  purchaseUom?: string;
  salesUom?: string;
  purchaseFactor?: number;
  salesFactor?: number;
}

/** Hasil POST /stock/scan-product (status ok | inactive | variant_required). */
export interface ScanResult {
  status: string;
  via?: string;
  product: { id: number; code: string; name: string };
  variant: { id: number; code: string; name: string } | null;
  variants?: { id: number; code: string; name: string; barcode?: string }[];
}

export interface BranchOption {
  id: number;
  code: string;
  name: string;
  status: string;
}

export interface OpeningRowError {
  row: number;
  column: string;
  value: string;
  reason: string;
}

export interface OpeningPreview {
  validRows: number;
  /** Baris yang dilewati (posisi sudah punya mutasi) — tidak diposting ulang. */
  skipped: number;
  errors: OpeningRowError[];
  /** Rincian per baris (maks 500 pertama), total, dan peringatan (ST-05). */
  rows?: OpeningPreviewRow[];
  rowsTruncated?: boolean;
  totalQty?: number;
  totalValue?: number;
  warnings?: string[];
}

export interface OpeningPreviewRow {
  row: number;
  productCode: string;
  productName: string;
  variantCode: string;
  locationCode: string;
  qty: number;
  unitCost: number;
  value: number;
  /** "post" = akan dibukukan, "skip" = dilewati. */
  status: "post" | "skip";
  note: string;
}

export interface OpeningResult {
  posted: number;
  skipped: number;
  failures: OpeningRowError[];
}

export interface LocationPayload {
  code: string;
  name: string;
  parentId: number | null;
  forceTransfer?: boolean;
  isPickingArea?: boolean;
  isPrimary?: boolean;
  sortOrder?: number;
}

export interface AdjustmentPayload {
  productId: number;
  variantId: number;
  locationId: number;
  mode: string;
  qtyAfter: number;
  qtyDelta: number;
  reason: string;
  approver: string;
  approverPassword: string;
}

export interface TransferPayload {
  toBranchId: number;
  fromLocationId: number;
  toLocationId: number;
  notes: string;
  driverName?: string;
  vehiclePlate?: string;
  transferDate?: string;
  items: TransferItemPayload[];
}

export interface TransferItemPayload {
  productId: number;
  variantId: number;
  qty: number;
  notes: string;
  /** Opsional: tujuan khusus baris ini (cabang yang sama dengan tujuan dokumen). */
  toLocationId?: number;
}

/** Pindah lokasi satu langkah (J4) — tanpa cabang tujuan, selalu cabang aktif. */
export interface MoveLocationPayload {
  fromLocationId: number;
  toLocationId: number;
  notes: string;
  items: TransferItemPayload[];
}

/** Satu posisi stok rusak dengan estimasi kerugian (ST-03). */
export interface DamagedBalance extends StockBalance {
  /** Harga modal rata-rata per satuan dasar (rupiah). */
  estimatedUnitCost: number;
  estimatedLoss: number;
}

export interface DamagedPage {
  items: DamagedBalance[];
  /** Jumlah baris himpunan terpilih (ada stok / sudah nol). */
  total: number;
  withStock: number;
  empty: number;
  /** Potensi kerugian seluruh stok rusak, bukan hanya halaman ini. */
  totalEstimatedLoss: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface DamagedMovePayload {  productId: number;
  variantId: number;
  locationId: number;
  qty: number;
  reason: string;
}

export interface DamagedWriteOffPayload {
  productId: number;
  variantId: number;
  qty: number;
  reason: string;
}
