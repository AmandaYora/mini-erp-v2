package integration

import (
	"context"
	"io"
	"testing"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	mediaapp "mini-erp/internal/modules/media/application"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	mediainfra "mini-erp/internal/modules/media/infrastructure"
)

// recordingDriver is a BlobDriver fake: Store/URLFor succeed trivially while
// Remove calls are recorded for the Archive assertions below.
type recordingDriver struct {
	removed []string
}

func (d *recordingDriver) Store(_ context.Context, _ string, _ io.Reader, _, _ string) error {
	return nil
}

func (d *recordingDriver) Remove(_ context.Context, key string) {
	d.removed = append(d.removed, key)
}

func (d *recordingDriver) URLFor(_ context.Context, key string) (string, error) {
	return "/media/" + key, nil
}

type noopAudit struct{}

func (noopAudit) Log(_ context.Context, _ auditcontracts.Entry) error { return nil }

func (noopAudit) LogTx(_ context.Context, _ auditcontracts.AuditTx, _ auditcontracts.Entry) error {
	return nil
}

func (noopAudit) List(_ context.Context, _ int64, _, _ string, _, _ int) ([]auditcontracts.Record, int64, error) {
	return nil, 0, nil
}

// TestArchiveKeepsLegacyObjects (F1-9): archiving a legacy-origin row removes
// the metadata row but must NOT delete the shared object; archiving an
// app-origin row deletes both.
func TestArchiveKeepsLegacyObjects(t *testing.T) {
	ctx := context.Background()
	freshDB(t)

	drv := &recordingDriver{}
	svc := mediaapp.NewService(mediainfra.NewRepository(testMods.db), drv,
		mediaapp.Prefixes{Product: "t", Transfer: "t", Delivery: "t"}, noopAudit{})

	legacy := &mediacontracts.MediaFile{OwnerType: "product", OwnerID: 7, Key: "vioni/production/upload/lama.jpg",
		OriginalName: "lama.jpg", MIME: "image/jpeg", SizeBytes: 10, Origin: mediacontracts.OriginLegacy}
	repo := mediainfra.NewRepository(testMods.db)
	legacyID, err := repo.Insert(ctx, legacy, 0)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	app := &mediacontracts.MediaFile{OwnerType: "product", OwnerID: 7, Key: "vioni/production-v2/upload/baru.jpg",
		OriginalName: "baru.jpg", MIME: "image/jpeg", SizeBytes: 10, Origin: mediacontracts.OriginApp}
	appID, err := repo.Insert(ctx, app, 0)
	if err != nil {
		t.Fatalf("insert app: %v", err)
	}

	if err := svc.Archive(ctx, "product", 7, legacyID, 0); err != nil {
		t.Fatalf("archive legacy: %v", err)
	}
	if len(drv.removed) != 0 {
		t.Fatalf("objek legacy ikut dihapus: %v", drv.removed)
	}
	if got, err := repo.GetByID(ctx, "product", 7, legacyID); err != nil || got != nil {
		t.Fatalf("baris legacy harus hilang: got=%v err=%v", got, err)
	}

	if err := svc.Archive(ctx, "product", 7, appID, 0); err != nil {
		t.Fatalf("archive app: %v", err)
	}
	if len(drv.removed) != 1 || drv.removed[0] != "vioni/production-v2/upload/baru.jpg" {
		t.Fatalf("objek app harus dihapus: %v", drv.removed)
	}
}
