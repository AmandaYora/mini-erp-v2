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
}

// Owner type constants used across modules.
const (
	OwnerProduct       = "product"
	OwnerPaymentProof  = "payment_proof"
	OwnerDeliveryProof = "delivery_proof"
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
}

// MediaClient is the public surface of the media module.
type MediaClient interface {
	// Upload stores content under a server-generated key and records it.
	Upload(ctx context.Context, ownerType string, ownerID int64, up Upload, actorID int64) (*MediaFile, error)
	// List returns an owner's files, primary first.
	List(ctx context.Context, ownerType string, ownerID int64) ([]*MediaFile, error)
	// GetURL resolves a readable URL: local path under /uploads/, or a
	// presigned S3 URL (private bucket, legacy behavior).
	GetURL(ctx context.Context, f *MediaFile) (string, error)
	// SetPrimary marks one file primary, clearing the owner's others.
	SetPrimary(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error
	// Archive removes the row and best-effort deletes the stored bytes.
	Archive(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error
}
