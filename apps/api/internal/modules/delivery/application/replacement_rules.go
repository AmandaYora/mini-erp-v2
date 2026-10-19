package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/shared/apperror"
)

// evidenceDisabled reports the company's switch that turns the shipment
// paperwork rules off (EvidenceSetting = "false").
func (s *Service) evidenceDisabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	cfg, err := s.settings.GetSettings(ctx)
	return err == nil && strings.EqualFold(strings.TrimSpace(cfg[EvidenceSetting]), "false")
}

// requireReplacementCrew makes a replacement shipment carry the same crew as an
// ordinary one (RT-03, legacy "Nama supir / petugas gudang wajib diisi"): driver
// and warehouse staff, unless the company switched the evidence rule off. A
// replacement handed over at the POS counter has no crew and is exempt.
func (s *Service) requireReplacementCrew(ctx context.Context, in contracts.ReplacementDraft) error {
	if in.AtCounter || s.evidenceDisabled(ctx) {
		return nil
	}
	var fields []apperror.FieldError
	if strings.TrimSpace(in.DriverName) == "" {
		fields = append(fields, apperror.FieldError{Field: "driverName", Message: "nama sopir wajib diisi"})
	}
	if strings.TrimSpace(in.WarehouseStaff) == "" {
		fields = append(fields, apperror.FieldError{Field: "warehouseStaffName", Message: "nama petugas gudang wajib diisi"})
	}
	if len(fields) > 0 {
		return apperror.Validation("Surat jalan pengganti belum lengkap", fields)
	}
	return nil
}
