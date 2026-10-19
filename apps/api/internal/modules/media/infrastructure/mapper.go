package infrastructure

import (
	"database/sql"

	"mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/media/infrastructure/sqlc"
)

// toMediaFile converts a generated media_files projection to the public type.
// NULL width/height/original size read as 0 (rows predating image optimisation).
func toMediaFile(r sqlc.ListMediaFilesRow) *contracts.MediaFile {
	return &contracts.MediaFile{
		ID: r.ID, OwnerType: r.OwnerType, OwnerID: r.OwnerID, Key: r.FileKey,
		OriginalName: r.OriginalName, MIME: r.Mime, SizeBytes: r.SizeBytes,
		IsPrimary: r.IsPrimary, Origin: string(r.Origin),
		Width: int(r.Width.Int32), Height: int(r.Height.Int32), OriginalSizeBytes: r.OriginalSizeBytes.Int64,
	}
}

// nullPositive32 stores non-positive dimensions as NULL.
func nullPositive32(n int) sql.NullInt32 {
	if n <= 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(n), Valid: true}
}

func nullPositive64(n int64) sql.NullInt64 {
	if n <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}
