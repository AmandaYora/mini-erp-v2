-- id: Q-RET-3
-- severity: warn
-- rule: none
-- action: item retur tanpa lokasi/rusak
SELECT SUM(id_stock_location IS NULL) no_loc,SUM(return_condition='damaged') damaged,SUM(return_condition='not_returned') not_returned FROM {sales_return_items} WHERE id_sales_return IN (SELECT id_sales_return FROM {sales_returns} WHERE {co}=@c);
