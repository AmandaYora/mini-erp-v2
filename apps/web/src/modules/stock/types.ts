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
  status: string;
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
}

export interface TransferItem {
  productId: number;
  variantId: number;
  qty: number;
  notes: string;
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
  errors: OpeningRowError[];
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
  items: { productId: number; variantId: number; qty: number; notes: string }[];
}

/** Pindah lokasi satu langkah (J4) — tanpa cabang tujuan, selalu cabang aktif. */
export interface MoveLocationPayload {
  fromLocationId: number;
  toLocationId: number;
  notes: string;
  items: { productId: number; variantId: number; qty: number; notes: string }[];
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
