package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/media/infrastructure/sqlc"
)

// Repository owns the media_files table. Bytes live on disk (see driver.go);
// this package only tracks metadata.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// Insert records one stored file. The first file of an owner becomes primary:
// the count is taken under a row lock inside the transaction, so two uploads
// racing on an empty owner cannot both claim (or both miss) the primary flag.
func (r *Repository) Insert(ctx context.Context, f *contracts.MediaFile, actorID int64) (int64, error) {
	origin := f.Origin
	if origin == "" {
		origin = contracts.OriginApp
	}
	// FOR UPDATE on an owner with no rows locks nothing under READ COMMITTED
	// (no gap locks), so two first uploads would both see "empty" and both
	// claim primary. A named lock per owner serialises them; it is taken on a
	// dedicated connection and released only after the commit.
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	lockName := "media:" + f.OwnerType + ":" + strconv.FormatInt(f.OwnerID, 10)
	cq := sqlc.New(conn)
	got, err := cq.AcquireNamedLock(ctx, sqlc.AcquireNamedLockParams{LockName: lockName, TimeoutSeconds: 10})
	if err != nil {
		return 0, err
	}
	if got != 1 {
		return 0, errors.New("gagal mengunci unggahan, coba lagi")
	}
	defer func() { _, _ = cq.ReleaseNamedLock(context.Background(), lockName) }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	tq := cq.WithTx(tx)
	existing, err := tq.LockOwnerFileIDs(ctx, sqlc.LockOwnerFileIDsParams{OwnerType: f.OwnerType, OwnerID: f.OwnerID})
	if err != nil {
		return 0, err
	}
	primary := len(existing) == 0
	res, err := tq.InsertMediaFile(ctx, sqlc.InsertMediaFileParams{
		OwnerType: f.OwnerType, OwnerID: f.OwnerID, FileKey: f.Key, OriginalName: f.OriginalName,
		Mime: f.MIME, SizeBytes: f.SizeBytes, IsPrimary: primary, Origin: sqlc.MediaFilesOrigin(origin),
		Width: nullPositive32(f.Width), Height: nullPositive32(f.Height),
		OriginalSizeBytes: nullPositive64(f.OriginalSizeBytes),
		CreatedBy:         sql.NullInt64{Int64: actorID, Valid: true},
		SortOrder:         int32(f.SortOrder),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	f.IsPrimary = primary
	return id, tx.Commit()
}

// PrimaryKeysByOwners returns each owner's primary file key in ONE read.
func (r *Repository) PrimaryKeysByOwners(ctx context.Context, ownerType string, ownerIDs []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ownerIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.PrimaryKeysByOwners(ctx, sqlc.PrimaryKeysByOwnersParams{OwnerType: ownerType, OwnerIds: ownerIDs})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OwnerID] = row.FileKey
	}
	return out, nil
}

// List returns an owner's files, primary first.
func (r *Repository) List(ctx context.Context, ownerType string, ownerID int64) ([]*contracts.MediaFile, error) {
	rows, err := r.q.ListMediaFiles(ctx, sqlc.ListMediaFilesParams{OwnerType: ownerType, OwnerID: ownerID})
	if err != nil {
		return nil, err
	}
	var out []*contracts.MediaFile
	for _, row := range rows {
		out = append(out, toMediaFile(row))
	}
	return out, nil
}

// GetByID loads one file row, or nil when missing/foreign-owned.
func (r *Repository) GetByID(ctx context.Context, ownerType string, ownerID, fileID int64) (*contracts.MediaFile, error) {
	row, err := r.q.GetMediaFile(ctx, sqlc.GetMediaFileParams{ID: fileID, OwnerType: ownerType, OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toMediaFile(sqlc.ListMediaFilesRow(row)), nil
}

// SetPrimary marks one file primary and clears the owner's others atomically.
func (r *Repository) SetPrimary(ctx context.Context, ownerType string, ownerID, fileID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	tq := r.q.WithTx(tx)
	if err := tq.ClearOwnerPrimary(ctx, sqlc.ClearOwnerPrimaryParams{OwnerType: ownerType, OwnerID: ownerID}); err != nil {
		return err
	}
	res, err := tq.MarkMediaFilePrimary(ctx, sqlc.MarkMediaFilePrimaryParams{ID: fileID, OwnerType: ownerType, OwnerID: ownerID})
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

// Delete removes the metadata row, returning the key and origin for byte
// cleanup. Origin "legacy" means the bytes still belong to the old system:
// the caller must keep the object and delete the row only (F1-9).
func (r *Repository) Delete(ctx context.Context, ownerType string, ownerID, fileID int64) (key, origin string, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	tq := r.q.WithTx(tx)
	row, err := tq.GetMediaFileForUpdate(ctx, sqlc.GetMediaFileForUpdateParams{ID: fileID, OwnerType: ownerType, OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	f := toMediaFile(sqlc.ListMediaFilesRow(row))
	if err := tq.DeleteMediaFile(ctx, fileID); err != nil {
		return "", "", err
	}
	// The primary photo is gone: promote the oldest remaining one so the
	// product keeps a thumbnail instead of silently showing none.
	if f.IsPrimary {
		if err := tq.PromoteOldestMediaFile(ctx, sqlc.PromoteOldestMediaFileParams{OwnerType: ownerType, OwnerID: ownerID}); err != nil {
			return "", "", err
		}
	}
	return f.Key, f.Origin, tx.Commit()
}
