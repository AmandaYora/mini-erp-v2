-- id: Q-MED-1
-- severity: info
-- rule: none
-- action: media per pemilik
SELECT owner_type,purpose,storage_driver,bucket,COUNT(*) n,SUM(size_bytes) bytes,SUM(archived_at IS NOT NULL) arch,MAX(CHAR_LENGTH(object_key)) mx_key FROM {media_files} WHERE {co}=@c GROUP BY 1,2,3,4;
