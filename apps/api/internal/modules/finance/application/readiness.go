package application

import (
	"context"
	"fmt"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/shared/apperror"
)

// Safe period close without a close-checklist table (H-revisi): readiness
// derives every gate from live books and documents, and safe-close refuses
// to seal a month that fails any of them. The plain ClosePeriod stays as
// the explicit override (audited), so an emergency close never strands the
// accountant.

// Readiness is the pre-close checklist for one accounting month.
type Readiness struct {
	Year     int `json:"year"`
	Month    int `json:"month"`
	Closed   bool `json:"closed"`
	Unposted int  `json:"unposted"`
	// Unbalanced counts journal entries whose legs do not net to zero.
	// Inserts enforce balance, so nonzero means migrated/hand-touched data.
	Unbalanced            int64 `json:"unbalanced"`
	MissingTaxInvoice     int64 `json:"missingTaxInvoice"`
	MissingSupplierInvoice int64 `json:"missingSupplierInvoice"`
	Ready                 bool  `json:"ready"`
	Blockers              []string `json:"blockers"`
}

// Readiness computes the checklist. Query count is constant: 1 derived
// queue + 1 unbalanced count + 2 invoice counts + 1 period read (P3).
func (s *Service) Readiness(ctx context.Context, branchID int64, year, month int) (*Readiness, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	res := &Readiness{Year: year, Month: month}
	if p, err := s.repo.GetPeriod(ctx, branchID, year, month); err != nil {
		return nil, apperror.Internal(err)
	} else if p != nil && p.Status == "closed" {
		res.Closed = true
	}
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	res.Unposted = len(queue)
	if n, err := s.repo.CountUnbalanced(ctx, branchID, monthStart(year, month), monthEnd(year, month)); err != nil {
		return nil, apperror.Internal(err)
	} else {
		res.Unbalanced = n
	}
	if n, err := s.salesOpsCountMissingTaxInvoice(ctx, branchID); err != nil {
		return nil, err
	} else {
		res.MissingTaxInvoice = n
	}
	if n, err := s.purchOpsCountMissingSupplierInvoice(ctx, branchID); err != nil {
		return nil, err
	} else {
		res.MissingSupplierInvoice = n
	}
	if res.Closed {
		res.Blockers = append(res.Blockers, "periode sudah ditutup")
	}
	if res.Unposted > 0 {
		res.Blockers = append(res.Blockers, fmt.Sprintf("%d dokumen belum diposting", res.Unposted))
	}
	if res.Unbalanced > 0 {
		res.Blockers = append(res.Blockers, fmt.Sprintf("%d jurnal tidak seimbang", res.Unbalanced))
	}
	if res.MissingTaxInvoice > 0 {
		res.Blockers = append(res.Blockers, fmt.Sprintf("%d order jual kena pajak tanpa nomor faktur", res.MissingTaxInvoice))
	}
	if res.MissingSupplierInvoice > 0 {
		res.Blockers = append(res.Blockers, fmt.Sprintf("%d order beli kena pajak tanpa nomor faktur supplier", res.MissingSupplierInvoice))
	}
	res.Ready = len(res.Blockers) == 0
	return res, nil
}

// SafeClose seals the month only when every gate passes. Use ClosePeriod
// directly (audited) only as a deliberate override.
func (s *Service) SafeClose(ctx context.Context, actorID, branchID int64, year, month int) error {
	check, err := s.Readiness(ctx, branchID, year, month)
	if err != nil {
		return err
	}
	if !check.Ready {
		return apperror.Conflict("Periode belum siap ditutup: " + joinBlockers(check.Blockers))
	}
	if err := s.repo.SetPeriodStatus(ctx, branchID, year, month, "closed"); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "fiscal_periods.safe_close", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID, Note: fmt.Sprintf("%04d-%02d", year, month)})
	return nil
}

func joinBlockers(b []string) string {
	out := ""
	for i, s := range b {
		if i > 0 {
			out += "; "
		}
		out += s
	}
	return out
}
