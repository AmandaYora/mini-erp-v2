-- id: Q-PRD-2
-- severity: blocker
-- rule: stmtrows:2>0
-- action: barcode ganda
SELECT SUM(barcode='') empty_bc, COUNT(*)-COUNT(DISTINCT NULLIF(barcode,'')) dup_or_null_bc, MAX(CHAR_LENGTH(variant_code)) mx_vcode FROM {product_variants} WHERE {co}=@c;
SELECT NULLIF(barcode,'') bc,COUNT(*) FROM {product_variants} WHERE {co}=@c AND barcode<>'' GROUP BY 1 HAVING COUNT(*)>1;
