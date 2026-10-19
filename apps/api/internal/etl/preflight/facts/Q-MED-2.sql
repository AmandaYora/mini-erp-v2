-- id: Q-MED-2
-- severity: blocker
-- rule: stmtfirst:1>0;stmtrows:2>0
-- action: key >255 / pemilik yatim
SELECT COUNT(*) FROM {media_files} WHERE {co}=@c AND CHAR_LENGTH(object_key)>255;
SELECT m.id_media_file FROM {media_files} m WHERE m.{co}=@c AND m.owner_type='delivery_note' AND NOT EXISTS(SELECT 1 FROM {delivery_notes} d WHERE d.id_delivery_note=m.owner_id);
