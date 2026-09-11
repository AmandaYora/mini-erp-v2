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
	sniffed := http.DetectContentType(raw)
	for _, prefix := range up.Accept {
		if !strings.HasPrefix(sniffed, prefix) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "tipe file tidak didukung"}})
		}
	}
	ext := strings.ToLower(filepath.Ext(up.OriginalName))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".pdf":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "tipe file tidak didukung"}})
	}
	key := s.buildKey(ownerType, ext)
	if err := s.driver.Store(ctx, key, bytes.NewReader(raw), sniffed, up.OriginalName); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	existing, err := s.repo.List(ctx, ownerType, ownerID)
	if err != nil {
		s.driver.Remove(ctx, key)
		return nil, apperror.Internal(err)
	}
	f := &contracts.MediaFile{
		OwnerType: ownerType, OwnerID: ownerID, Key: key,
		OriginalName: up.OriginalName, MIME: sniffed,
		SizeBytes: int64(len(raw)), IsPrimary: len(existing) == 0,
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
	case contracts.OwnerPaymentProof:
		prefix, dated = s.prefixes.Transfer, true
	case contracts.OwnerDeliveryProof:
		prefix, dated = s.prefixes.Delivery, true
	}
	name := uuid.NewString() + ext
	if dated {
		return prefix + "/" + timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02") + "/" + name
	}
	return prefix + "/" + name
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

// GetURL resolves a readable URL: local path under /uploads/, or a
// presigned S3 URL (private bucket, legacy behavior).
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

// Archive removes the row and best-effort deletes the bytes.
func (s *Service) Archive(ctx context.Context, ownerType string, ownerID, fileID, actorID int64) error {
	key, err := s.repo.Delete(ctx, ownerType, ownerID, fileID)
	if err != nil {
		return apperror.Internal(err)
	}
	if key == "" {
		return apperror.NotFound("File")
	}
	s.driver.Remove(ctx, key)
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "media.archive", Entity: "media",
		EntityID: fileID, ActorID: actorID, Note: ownerType + ":" + strconv.FormatInt(ownerID, 10)})
	return nil
}
