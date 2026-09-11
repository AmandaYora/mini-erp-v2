package media

import (
	"database/sql"
	"time"

	"mini-erp/internal/config"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/media/application"
	"mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/media/infrastructure"
)

// Module wires the media bounded context. It exposes no HTTP routes of its
// own: uploads flow through each owner's endpoints (product photos, payment
// and delivery proofs). Bytes live on the configured driver (local disk or
// S3 object storage at the legacy address); reads resolve local paths or
// presigned URLs accordingly.
type Module struct {
	svc *application.Service
}

// NewModule builds media over its table plus the configured blob driver.
func NewModule(db *sql.DB, storage config.Storage, storageDir string, audit auditcontracts.AuditClient) (*Module, error) {
	var driver infrastructure.BlobDriver
	if storage.Driver == "s3" {
		driver = infrastructure.NewS3Driver(infrastructure.S3Config{
			Endpoint: storage.Endpoint, Bucket: storage.Bucket, Region: storage.Region,
			ForcePathStyle: storage.ForcePathStyle, AccessKeyID: storage.AccessKeyID,
			SecretAccessKey: storage.SecretAccessKey, PublicBaseURL: storage.PublicBaseURL,
			SignedURLTTLS: time.Duration(storage.SignedURLTTLSecs) * time.Second,
		})
	} else {
		local, err := infrastructure.NewLocalDriver(storageDir)
		if err != nil {
			return nil, err
		}
		driver = local
	}
	svc := application.NewService(infrastructure.NewRepository(db), driver, application.Prefixes{
		Product: storage.ProductPrefix, Transfer: storage.TransferPrefix, Delivery: storage.DeliveryPrefix,
	}, audit)
	return &Module{svc: svc}, nil
}

// Media exposes the public contract to other modules.
func (m *Module) Media() contracts.MediaClient {
	return m.svc
}
