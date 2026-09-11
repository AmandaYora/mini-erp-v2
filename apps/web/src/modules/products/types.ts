// Kunci JSON camelCase terverifikasi dari backend:
// handler.go: productView, categoryView — media.go: mediaView.
// application/import.go: ImportPreview {validProducts, validVariants, errors},
// ImportResult {inserted, skipped, failures}.

export interface ProductVariant {
  id: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: string;
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
  purchasePrice: number;
  sellingPrice: number;
  minSellingPrice: number;
  minStock: number;
  status: string;
  variants: ProductVariant[];
}

export interface ProductCategory {
  id: number;
  code: string;
  name: string;
  parentId: number | null;
  status: string;
}

export interface ProductMedia {
  id: number;
  originalName: string;
  mime: string;
  sizeBytes: number;
  isPrimary: boolean;
  url: string;
}

export interface ImportRowError {
  sheet: string;
  row: number;
  column: string;
  value: string;
  reason: string;
}

export interface ImportPreview {
  validProducts: number;
  validVariants: number;
  errors: ImportRowError[];
}

export interface ImportResult {
  inserted: number;
  skipped: number;
  failures: ImportRowError[];
}

export interface ProductPayload {
  code: string;
  name: string;
  categoryId: number | null;
  type: string;
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
  variants: {
    code: string;
    name: string;
    barcode: string;
    isDefault: boolean;
  }[];
}
