// Kunci JSON camelCase terverifikasi dari backend:
// product/presentation/handler.go: productView, variantView, categoryView,
// SearchOptions — media.go: mediaView — import.go + application/import_plan.go:
// ImportPreview {okToCommit, summary, issues, categories, products} (+ field
// lama validProducts/validVariants/errors), ImportResult {imported, failures}.

export type ProductType = "barang" | "jasa" | "paket" | "non_stok";
export type ProductStatus = "active" | "inactive" | "archived";
export type VariantStatus = "active" | "inactive" | "archived";

export interface ProductVariant {
  id: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: string;
  /** Varian default bawaan (bukan pilihan pengguna) — tak ditampilkan di form. */
  isHidden: boolean;
  sortOrder: number;
  attributes: Record<string, string>;
}

export interface Product {
  id: number;
  code: string;
  name: string;
  categoryId: number | null;
  categoryName: string;
  type: string;
  tracked: boolean;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  purchaseFactor: number;
  salesFactor: number;
  /** Daftar produk menyembunyikannya dari peran tanpa izin biaya (products.update / purchasing.view). */
  purchasePrice: number;
  sellingPrice: number;
  minSellingPrice: number;
  minStock: number;
  status: string;
  summary: string;
  attributes: Record<string, string>;
  variants: ProductVariant[];
  /** Hanya pada daftar (GET /products): URL foto utama atau null. */
  primaryImageUrl?: string | null;
  /** Hanya daftar, bila ada izin stock.view + cabang aktif: stok tersedia dalam satuan jual. */
  stockAvailable?: number;
  stockUom?: string;
  /** Stok tersedia ≤ stok minimum (hanya produk terpantau ber-minimum). */
  belowMinStock?: boolean;
  /** Hanya daftar: waktu produk diarsipkan (produk berstatus arsip). */
  archivedAt?: string | null;
}

/** Baris GET /products/search-options (lebih kaya dari daftar; tanpa kategori). */
export interface ProductOptionRow {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  purchaseFactor: number;
  salesFactor: number;
  purchasePrice: number;
  minSellingPrice: number;
  type: string;
  tracked: boolean;
  status: string;
  variants: ProductVariant[];
}

/** GET /products/scan?code= */
export interface ProductScanLookup {
  id: number;
  code: string;
  name: string;
  status: string;
  via: string;
  /** Kode produk dengan beberapa varian terlihat (pilih varian di detail). */
  variantRequired: boolean;
  variantId?: number;
}

export interface ProductCategory {
  id: number;
  code: string;
  name: string;
  parentId: number | null;
  /** Urutan tampil antar-kategori (kecil dulu, lalu nama). */
  sortOrder: number;
  status: string;
}

export interface ProductMedia {
  id: number;
  originalName: string;
  mime: string;
  sizeBytes: number;
  isPrimary: boolean;
  url: string;
  /** Piksel gambar tersimpan (0 / tak ada untuk baris lama). */
  width?: number;
  height?: number;
  originalSizeBytes?: number;
}

/** Baris GET /stock/balances?productId= (saldo per varian per lokasi). */
export interface ProductStockBalance {
  variantId: number;
  locationId: number;
  onHand: number;
  reserved: number;
  available: number;
}

// --- impor ---

export interface ImportIssue {
  severity: "error" | "warning";
  sheet: string;
  row: number;
  column: string;
  message: string;
}

export interface ImportRowError {
  sheet: string;
  row: number;
  column: string;
  value: string;
  reason: string;
}

export interface ImportSummary {
  categoriesTotal: number;
  categoriesToCreate: number;
  categoriesToUpdate: number;
  productsTotal: number;
  productsToCreate: number;
  productsToUpdate: number;
  variantsTotal: number;
  attributesTotal: number;
  errors: number;
  warnings: number;
}

export interface ImportPlannedRow {
  row: number;
  code: string;
  name: string;
  action: "create" | "update";
}

export interface ImportPreview {
  okToCommit: boolean;
  summary: ImportSummary;
  issues: ImportIssue[];
  categories: ImportPlannedRow[];
  products: ImportPlannedRow[];
  // Field pra-paritas (tetap dikirim backend).
  validProducts: number;
  validVariants: number;
  errors: ImportRowError[];
}

export interface ImportResult {
  imported: {
    categoriesCreated: number;
    categoriesUpdated: number;
    productsCreated: number;
    productsUpdated: number;
  };
  inserted: number;
  skipped: number;
  failures: ImportRowError[];
  notes: string[];
}

export interface VariantPayload {
  id?: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: VariantStatus;
  sortOrder: number;
  attributes: Record<string, string>;
  isHidden: false;
}

export interface ProductPayload {
  code: string;
  name: string;
  categoryId: number | null;
  type: ProductType;
  tracked: boolean;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  purchaseFactor: number;
  salesFactor: number;
  purchasePrice: number;
  sellingPrice: number;
  minSellingPrice: number;
  minStock: number;
  status: "active" | "inactive";
  summary: string;
  attributes: Record<string, string>;
  /** Tidak ada = set varian tersimpan dipertahankan; [] = kembali ke varian bawaan. */
  variants?: VariantPayload[];
}
