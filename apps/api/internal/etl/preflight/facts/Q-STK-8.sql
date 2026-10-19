-- id: Q-STK-8
-- severity: info
-- rule: none
-- action: transfer dengan >1 pasangan lokasi dipecah
SELECT t.id_stock_transfer,t.transfer_number,COUNT(DISTINCT CONCAT(i.id_from_location,'>',i.id_to_location)) pairs
FROM {stock_transfers} t JOIN {stock_transfer_items} i ON i.id_stock_transfer=t.id_stock_transfer WHERE t.{co}=@c GROUP BY 1 HAVING pairs>1;
