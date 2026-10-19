package contracts

import (
	"context"
	"io"
)

// MediaFile is one stored file, always tied to an owner
// (owner_type + owner_id primitive pair, e.g. "product"/123).
type MediaFile struct {
	ID           int64
	OwnerType    string
	OwnerID      int64
	Key          string
	OriginalName string
	MIME         string
	SizeBytes    int64
	IsPrimary    bool
	// Origin marks where the bytes came from (F1-9): "app" for files this
	// deployment uploaded, "legacy" for rows carried by the ETL whose bytes
	// still belong to the old system (never deleted by Archive).
	Origin string
	// Width/Height are the stored picture's pixels (0 for rows predating
	// image optimisation); OriginalSizeBytes is the size as uploaded.
	Width, Height     int
	OriginalSizeBytes int64
	// SortOrder places the file among its owner's gallery (ascending; the
	// primary file always lists first). Set from Upload.SortOrder.
	SortOrder int
}

// Origin values for MediaFile.Origin.
const (
	OriginApp    = "app"
	OriginLegacy = "legacy"
)

// Owner type constants used across modules.
const (
	OwnerProduct      = "product"
	OwnerPaymentProof = "payment_proof"
	// OwnerDeliveryProof is the optional delivery-location photo of a shipment;
	// OwnerDeliveryDocument is the scan/photo of the signed physical SJ (DL-01
	// purpose split: legacy delivery_signed_document vs delivery_location_photo).
	OwnerDeliveryProof    = "delivery_proof"
	OwnerDeliveryDocument = "delivery_document"
	// OwnerTaxAdjustment is the supporting scan/photo of a fiscal correction
	// journal (FN-07); owner_id is the journal entry id.
	OwnerTaxAdjustment = "tax_adjustment"
)

// Upload holds one incoming file.
type Upload struct {
	OriginalName string
	MIME         string
	SizeBytes    int64
	Content      io.Reader
	// Accept optionally restricts sniffed MIME prefixes (e.g. ["image/"]).
	// Empty means the driver extension whitelist decides alone.
	Accept []string
	// SortOrder is the gallery position recorded with the file (0 = default).
	SortOrder int
}

// MediaClient is the public surface of the media module.
type MediaClient interface {
	// Upload stores content under a server-generated key and records it.
	Upload(ctx context.Context, ownerType string, ownerID int64, up Upload, actorID int64) (*MediaFile, error)
	// List returns an owner's files, primary first.
	List(ctx context.Context, ownerType string, ownerID int64) ([]*MediaFile, error)
	// GetURL resolves a readable URL: a signed, expiring path under /media/
	// (local driver), or a presigned S3 URL (private bucket, legacy behavior).
	GetURL(ctx context.Context, f *MediaFile) (string, error)
	// SetPrimary marks one file primary, clearing the owner's others.
	SetPrimary(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error
	// PrimaryByOwners resolves the readable URL of each owner's primary file
	// in ONE read (list thumbnails). Owners without a file are absent.
	PrimaryByOwners(ctx context.Context, ownerType string, ownerIDs []int64) (map[int64]string, error)
	// Archive removes the row and best-effort deletes the stored bytes.
	Archive(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error
}
