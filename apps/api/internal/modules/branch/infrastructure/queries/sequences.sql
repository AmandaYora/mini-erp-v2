-- name: SumSequenceValues :one
SELECT CAST(COALESCE(SUM(current_value), 0) AS SIGNED) AS total
FROM branch_document_sequences WHERE branch_id = ?;

-- name: InsertSequenceIgnore :exec
INSERT IGNORE INTO branch_document_sequences (branch_id, doc_kind, current_value) VALUES (?, ?, 0);

-- name: BumpSequence :execresult
UPDATE branch_document_sequences SET current_value = LAST_INSERT_ID(current_value + 1)
WHERE branch_id = ? AND doc_kind = ?;

-- name: LastInsertID :one
SELECT CAST(LAST_INSERT_ID() AS SIGNED) AS id;
