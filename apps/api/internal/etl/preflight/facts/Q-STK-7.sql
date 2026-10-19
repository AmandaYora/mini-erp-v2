-- id: Q-STK-7
-- severity: warn
-- rule: none
-- action: lokasi default/rusak/kode >30
SELECT b.id_branch,SUM(l.is_default) defaults,SUM(l.status='damaged') damaged,MAX(CHAR_LENGTH(l.code)) mx_code
FROM {branches} b JOIN {stock_locations} l ON l.id_branch=b.id_branch WHERE b.{co}=@c GROUP BY 1;
