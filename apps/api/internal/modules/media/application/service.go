package application

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/media/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// MaxFileBytes caps uploads at 5 MB (legacy Multer limit, kept).
const MaxFileBytes = 5 << 20

// Prefixes maps owner types to key prefixes (legacy STORAGE_*_PREFIX layout).
type Prefixes struct {
	Product  string
	Transfer string
	Delivery string
}

// Service implements contracts.MediaClient over a blob driver (local/S3).
type Service struct {
	repo     *infrastructure.Repository
	driver   infrastructure.BlobDriver
	prefixes Prefixes
	audit    auditcontracts.AuditClient
}

// NewService wires media use cases.
func NewService(repo *infrastructure.Repository, driver infrastructure.BlobDriver, prefixes Prefixes, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, driver: driver, prefixes: prefixes, audit: audit}
}

// Upload validates, stores, and records one file. The first file of an owner
// becomes primary automatically. Content is read fully (capped at 5 MB) so
// the recorded size is exact and the MIME is sniffed, never trusted.
func (s *Service) Upload(ctx context.Context, ownerType string, ownerID int64, up contracts.Upload, actorID int64) (*contracts.MediaFile, error) {
	if up.OriginalName == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "wajib diisi"}})
	}
	raw, err := io.ReadAll(io.LimitReader(up.Content, MaxFileBytes+1))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(raw) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "file kosong"}})
	}
	if len(raw) > MaxFileBytes {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "maksimal 5MB"}})
	}
	// Only pictures are stored: the sniffed type (never the client's claim)
	// must be exactly JPEG, PNG or WebP, the extension must agree with that
	// family, and the bytes must decode (a .png holding a GIF is refused).
	sniffed := http.DetectContentType(raw)
	switch sniffed {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "tipe file tidak didukung (hanya JPG, PNG, WebP)"}})
	}
	switch strings.ToLower(filepath.Ext(up.OriginalName)) {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "tipe file tidak didukung"}})
	}
	maxDim := MaxDimProduct
	if ownerType == contracts.OwnerPaymentProof || ownerType == contracts.OwnerDeliveryProof || ownerType == contracts.OwnerDeliveryDocument || ownerType == contracts.OwnerTaxAdjustment {
		maxDim = MaxDimProof
	}
	img, err := processImage(raw, maxDim)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	originalSize := int64(len(raw))
	raw, sniffed = img.Data, img.MIME
	key := s.buildKey(ownerType, img.Ext)
	if err := s.driver.Store(ctx, key, bytes.NewReader(raw), sniffed, up.OriginalName); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	f := &contracts.MediaFile{
		OwnerType: ownerType, OwnerID: ownerID, Key: key,
		OriginalName: up.OriginalName, MIME: sniffed,
		SizeBytes: int64(len(raw)), Width: img.Width, Height: img.Height,
		OriginalSizeBytes: originalSize, SortOrder: up.SortOrder,
	}
	id, err := s.repo.Insert(ctx, f, actorID)
	if err != nil {
		s.driver.Remove(ctx, key)
		return nil, apperror.Internal(err)
	}
	f.ID = id
	// The upload itself is the outermost user action here (called directly
	// from product/delivery/payment proof endpoints, never as a downstream
	// effect of another audited action), so it is trailed at the source.
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "media.upload", Entity: "media",
		EntityID: id, ActorID: actorID, Note: ownerType + ":" + strconv.FormatInt(ownerID, 10)})
	return f, nil
}

// buildKey lays out keys like legacy: {prefix}/{uuid}.{ext} for products,
// {prefix}/{YYYY-MM-DD}/{uuid}.{ext} for proofs and transfers.
func (s *Service) buildKey(ownerType, ext string) string {
	prefix := s.prefixes.Product
	dated := false
	switch ownerType {
	case contracts.OwnerPaymentProof, contracts.OwnerTaxAdjustment:
		prefix, dated = s.prefixes.Transfer, true
	case contracts.OwnerDeliveryProof, contracts.OwnerDeliveryDocument:
		prefix, dated = s.prefixes.Delivery, true
	}
	name := uuid.NewString() + ext
	parts := []string{strings.Trim(prefix, "/")}
	if dated {
		parts = append(parts, timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02"))
	}
	parts = append(parts, name)
	// Empty segments are dropped so a missing prefix can never yield a key that
	// starts with "/" (which would become a "//" URL and escape the signature).
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}

// List returns an owner's files, primary first.
func (s *Service) List(ctx context.Context, ownerType string, ownerID int64) ([]*contracts.MediaFile, error) {
	files, err := s.repo.List(ctx, ownerType, ownerID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if files == nil {
		files = []*contracts.MediaFile{}
	}
	return files, nil
}

// PrimaryByOwners resolves each owner's primary file URL in ONE read.
func (s *Service) PrimaryByOwners(ctx context.Context, ownerType string, ownerIDs []int64) (map[int64]string, error) {
	keys, err := s.repo.PrimaryKeysByOwners(ctx, ownerType, ownerIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make(map[int64]string, len(keys))
	for id, key := range keys {
		if url, err := s.driver.URLFor(ctx, key); err == nil {
			out[id] = url
		}
	}
	return out, nil
}

// GetURL resolves a readable URL: a signed, expiring path under /media/
// (local driver), or a presigned S3 URL (private bucket, legacy behavior).
func (s *Service) GetURL(ctx context.Context, f *contracts.MediaFile) (string, error) {
	return s.driver.URLFor(ctx, f.Key)
}

// SetPrimary marks one owned file primary. Unknown or foreign-owned files
// read as missing, never as errors.
func (s *Service) SetPrimary(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error {
	if err := s.repo.SetPrimary(ctx, ownerType, ownerID, fileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.NotFound("File")
		}
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "media.primary", Entity: "media",
		EntityID: fileID, ActorID: actorID, Note: ownerType + ":" + strconv.FormatInt(ownerID, 10)})
	return nil
}

// Archive removes the row and best-effort deletes the bytes — except for
// legacy rows carried by the ETL (origin "legacy"): their bytes still belong
// to the old system (rollback path), so only the row is removed and the
// shared object is kept (F1-9).
func (s *Service) Archive(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error {
	key, origin, err := s.repo.Delete(ctx, ownerType, ownerID, fileID)
	if err != nil {
		return apperror.Internal(err)
	}
	if key == "" {
		return apperror.NotFound("File")
	}
	note := ownerType + ":" + strconv.FormatInt(ownerID, 10)
	if origin == contracts.OriginLegacy {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "media.archive", Entity: "media",
			EntityID: fileID, ActorID: actorID, Note: note + " (objek bersama legacy dipertahankan)"})
		return nil
	}
	s.driver.Remove(ctx, key)
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "media.archive", Entity: "media",
		EntityID: fileID, ActorID: actorID, Note: note})
	return nil
}
