-- name: LockOwnerFileIDs :many
SELECT id FROM media_files WHERE owner_type = ? AND owner_id = ? FOR UPDATE;

-- name: InsertMediaFile :execresult
INSERT INTO media_files
  (owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary, origin,
   width, height, original_size_bytes, created_by, sort_order)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: PrimaryKeysByOwners :many
SELECT owner_id, file_key FROM media_files
WHERE owner_type = sqlc.arg(owner_type) AND is_primary = 1 AND owner_id IN (sqlc.slice(owner_ids));

-- name: ListMediaFiles :many
SELECT id, owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary, origin, width, height, original_size_bytes
FROM media_files
WHERE owner_type = ? AND owner_id = ?
ORDER BY is_primary DESC, sort_order ASC, id ASC;

-- name: GetMediaFile :one
SELECT id, owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary, origin, width, height, original_size_bytes
FROM media_files
WHERE id = ? AND owner_type = ? AND owner_id = ?;

-- name: GetMediaFileForUpdate :one
SELECT id, owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary, origin, width, height, original_size_bytes
FROM media_files
WHERE id = ? AND owner_type = ? AND owner_id = ? FOR UPDATE;

-- name: ClearOwnerPrimary :exec
UPDATE media_files SET is_primary = 0 WHERE owner_type = ? AND owner_id = ?;

-- name: MarkMediaFilePrimary :execresult
UPDATE media_files SET is_primary = 1 WHERE id = ? AND owner_type = ? AND owner_id = ?;

-- name: DeleteMediaFile :exec
DELETE FROM media_files WHERE id = ?;

-- name: PromoteOldestMediaFile :exec
UPDATE media_files SET is_primary = 1 WHERE id = (
  SELECT id FROM (SELECT mf.id FROM media_files mf WHERE mf.owner_type = ? AND mf.owner_id = ?
                  ORDER BY mf.sort_order ASC, mf.id ASC LIMIT 1) t);

-- name: AcquireNamedLock :one
SELECT CAST(COALESCE(GET_LOCK(sqlc.arg(lock_name), sqlc.arg(timeout_seconds)), 0) AS SIGNED) AS got;

-- name: ReleaseNamedLock :one
SELECT CAST(COALESCE(RELEASE_LOCK(sqlc.arg(lock_name)), 0) AS SIGNED) AS released;
