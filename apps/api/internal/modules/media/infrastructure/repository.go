package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/modules/media/contracts"
)

// Repository owns the media_files table. Bytes live on disk (see driver.go);
// this package only tracks metadata.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func scanFile(row *sql.Row, f *contracts.MediaFile) error {
	var primary int
	err := row.Scan(&f.ID, &f.OwnerType, &f.OwnerID, &f.Key, &f.OriginalName,
		&f.MIME, &f.SizeBytes, &primary)
	f.IsPrimary = primary == 1
	return err
}

const fileColumns = "id, owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary"

// Insert records one stored file.
func (r *Repository) Insert(ctx context.Context, f *contracts.MediaFile, actorID int64) (int64, error) {
	primary := 0
	if f.IsPrimary {
		primary = 1
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO media_files
		 (owner_type, owner_id, file_key, original_name, mime, size_bytes, is_primary, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.OwnerType, f.OwnerID, f.Key, f.OriginalName, f.MIME, f.SizeBytes, primary, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List returns an owner's files, primary first.
func (r *Repository) List(ctx context.Context, ownerType string, ownerID int64) ([]*contracts.MediaFile, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM media_files WHERE owner_type = ? AND owner_id = ? ORDER BY is_primary DESC, id ASC",
		ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.MediaFile
	for rows.Next() {
		f := &contracts.MediaFile{}
		var primary int
		if err := rows.Scan(&f.ID, &f.OwnerType, &f.OwnerID, &f.Key, &f.OriginalName,
			&f.MIME, &f.SizeBytes, &primary); err != nil {
			return nil, err
		}
		f.IsPrimary = primary == 1
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetByID loads one file row, or nil when missing/foreign-owned.
func (r *Repository) GetByID(ctx context.Context, ownerType string, ownerID, fileID int64) (*contracts.MediaFile, error) {
	f := &contracts.MediaFile{}
	err := scanFile(r.db.QueryRowContext(ctx,
		"SELECT "+fileColumns+" FROM media_files WHERE id = ? AND owner_type = ? AND owner_id = ?",
		fileID, ownerType, ownerID), f)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return f, err
}

// SetPrimary marks one file primary and clears the owner's others atomically.
func (r *Repository) SetPrimary(ctx context.Context, ownerType string, ownerID, fileID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET is_primary = 0 WHERE owner_type = ? AND owner_id = ?",
		ownerType, ownerID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE media_files SET is_primary = 1 WHERE id = ? AND owner_type = ? AND owner_id = ?",
		fileID, ownerType, ownerID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		if err == nil {
			return sql.ErrNoRows
		}
		return err
	}
	return tx.Commit()
}

// Delete removes the metadata row, returning the key for byte cleanup.
func (r *Repository) Delete(ctx context.Context, ownerType string, ownerID, fileID int64) (string, error) {
	f, err := r.GetByID(ctx, ownerType, ownerID, fileID)
	if err != nil || f == nil {
		return "", err
	}
	if _, err := r.db.ExecContext(ctx, "DELETE FROM media_files WHERE id = ?", fileID); err != nil {
		return "", err
	}
	return f.Key, nil
}
