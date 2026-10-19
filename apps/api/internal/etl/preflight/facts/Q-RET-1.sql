-- id: Q-RET-1
-- severity: warn
-- rule: none
-- action: sebaran retur per mode/status/penyelesaian (retur dimigrasi; refund/kolek menjadi pembayaran sintetis, sisa tagihan menjadi offset)
SELECT return_mode,status,settlement_type,COUNT(*) n,SUM(replacement_total_amount) repl,SUM(difference_amount) diff FROM {sales_returns} WHERE {co}=@c GROUP BY 1,2,3;
SELECT status,settlement_type,COUNT(*),SUM(returned_total_amount) FROM {purchase_returns} WHERE {co}=@c GROUP BY 1,2;
SELECT srs.settlement_type,COUNT(*),SUM(srs.amount) FROM {sales_return_settlements} srs JOIN {sales_returns} sr ON sr.id_sales_return=srs.id_sales_return WHERE sr.{co}=@c AND srs.archived_at IS NULL GROUP BY 1;
SELECT prs.settlement_type,COUNT(*),SUM(prs.amount) FROM {purchase_return_settlements} prs JOIN {purchase_returns} pr ON pr.id_purchase_return=prs.id_purchase_return WHERE pr.{co}=@c AND prs.archived_at IS NULL GROUP BY 1;
