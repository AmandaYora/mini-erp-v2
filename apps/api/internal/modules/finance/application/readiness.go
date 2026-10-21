package application

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// Safe period close without a close-checklist table (H-revisi): readiness
// derives every gate from live books and documents, and safe-close refuses
// to seal a month that fails any of them. The plain ClosePeriod stays as
// the explicit override (audited), so an emergency close never strands the
// accountant.

// Check statuses.
const (
	CheckOK      = "ok"
	CheckWarning = "warning"
	CheckFailed  = "failed"
)

// ReadinessCheck is one gate of the pre-close checklist.
type ReadinessCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | warning | failed
	Detail string `json:"detail"`
	Count  int64  `json:"count"`
}

// Readiness is the pre-close checklist for one accounting month. The flat
// fields are kept for existing clients; Checks is the structured form.
type Readiness struct {
	Year     int  `json:"year"`
	Month    int  `json:"month"`
	Closed   bool `json:"closed"`
	Unposted int  `json:"unposted"`
	// Unbalanced counts journal entries whose legs do not net to zero.
	// Inserts enforce balance, so nonzero means migrated/hand-touched data.
	Unbalanced             int64            `json:"unbalanced"`
	MissingTaxInvoice      int64            `json:"missingTaxInvoice"`
	MissingSupplierInvoice int64            `json:"missingSupplierInvoice"`
	LatePeriodDocs         int              `json:"latePeriodDocs"`
	Ignored                int              `json:"ignored"`
	Orphans                int              `json:"orphans"`
	Checks                 []ReadinessCheck `json:"checks"`
	Ready                  bool             `json:"ready"`
	Blockers               []string         `json:"blockers"`
	Warnings               []string         `json:"warnings"`
}

// Readiness computes the checklist.
//
// Gates (F2b-10): documents not yet journalled in or before this month (exact
// count; documents of already-closed months are a warning, not a blocker),
// unbalanced journals, incomplete account mappings, negative inventory,
// journals of cancelled documents not yet reversed, and — as warnings, legacy
// parity — taxable orders without an invoice number.
func (s *Service) Readiness(ctx context.Context, branchID int64, year, month int) (*Readiness, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	res := &Readiness{Year: year, Month: month, Blockers: []string{}, Warnings: []string{}}
	if p, err := s.repo.GetPeriod(ctx, branchID, year, month); err != nil {
		return nil, apperror.Internal(err)
	} else if p != nil && p.Status == "closed" {
		res.Closed = true
	}
	monthKey := fmt.Sprintf("%04d-%02d", year, month)
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	for _, q := range queue {
		switch {
		case q.Ignored:
			// Taken out of the queue on purpose (FN-01): a warning, never a blocker.
			res.Ignored++
		case q.PeriodClosed:
			res.LatePeriodDocs++
		case q.Date == "" || q.Date[:7] <= monthKey:
			res.Unposted++
		}
	}
	if n, err := s.repo.CountUnbalanced(ctx, branchID, monthStart(year, month), monthEnd(year, month)); err != nil {
		return nil, apperror.Internal(err)
	} else {
		res.Unbalanced = n
	}
	if n, err := s.salesOpsCountMissingTaxInvoice(ctx, branchID, monthStart(year, month), monthEnd(year, month)); err != nil {
		return nil, err
	} else {
		res.MissingTaxInvoice = n
	}
	if n, err := s.purchOpsCountMissingSupplierInvoice(ctx, branchID, monthStart(year, month), monthEnd(year, month)); err != nil {
		return nil, err
	} else {
		res.MissingSupplierInvoice = n
	}
	missingMaps, err := s.MissingMappings(ctx)
	if err != nil {
		return nil, err
	}
	negative, err := s.negativeInventory(ctx, branchID)
	if err != nil {
		return nil, err
	}
	orphans, err := s.Orphans(ctx, branchID)
	if err != nil {
		return nil, err
	}
	res.Orphans = len(orphans)
	recon, err := s.ReconcileInventory(ctx, branchID)
	if err != nil {
		return nil, err
	}

	add := func(key, label, status, detail string, count int64) {
		res.Checks = append(res.Checks, ReadinessCheck{Key: key, Label: label, Status: status, Detail: detail, Count: count})
		switch status {
		case CheckFailed:
			res.Blockers = append(res.Blockers, detail)
		case CheckWarning:
			res.Warnings = append(res.Warnings, detail)
		}
	}
	state := func(n int64, bad string) string {
		if n > 0 {
			return bad
		}
		return CheckOK
	}
	if res.Closed {
		add("period_open", "Periode terbuka", CheckFailed, "periode sudah ditutup", 1)
	} else {
		add("period_open", "Periode terbuka", CheckOK, "", 0)
	}
	add("unposted", "Dokumen sudah diposting", state(int64(res.Unposted), CheckFailed),
		fmt.Sprintf("%d dokumen belum diposting", res.Unposted), int64(res.Unposted))
	add("late_period_docs", "Dokumen di periode tertutup", state(int64(res.LatePeriodDocs), CheckWarning),
		fmt.Sprintf("%d dokumen jatuh di periode yang sudah ditutup dan belum dijurnal", res.LatePeriodDocs), int64(res.LatePeriodDocs))
	if res.Ignored > 0 {
		add("ignored_docs", "Dokumen diabaikan", CheckWarning,
			fmt.Sprintf("%d dokumen sengaja diabaikan dari antrian posting (tidak masuk pembukuan)", res.Ignored), int64(res.Ignored))
	}
	add("balanced", "Jurnal seimbang", state(res.Unbalanced, CheckFailed),
		fmt.Sprintf("%d jurnal tidak seimbang", res.Unbalanced), res.Unbalanced)
	add("mappings", "Pemetaan akun lengkap", state(int64(len(missingMaps)), CheckFailed),
		fmt.Sprintf("%d pemetaan akun belum diatur", len(missingMaps)), int64(len(missingMaps)))
	add("negative_inventory", "Persediaan tidak negatif", state(negative, CheckFailed),
		fmt.Sprintf("%d posisi persediaan bernilai negatif", negative), negative)
	// The gap is netted by the ledger value of the documents still waiting
	// for a journal (and of those ignored on purpose); what remains has no
	// known cause and blocks the close.
	reconStatus, reconDetail := CheckOK, ""
	switch {
	case !recon.Consistent:
		reconStatus = CheckFailed
		reconDetail = fmt.Sprintf("akun Persediaan buku %d berbeda %d dari nilai ledger biaya %d; "+
			"setelah dokumen belum dijurnal (%d) dan diabaikan (%d) masih tersisa selisih tak terjelaskan %d",
			recon.Book, recon.Diff, recon.Ledger, recon.PendingValue, recon.IgnoredValue, recon.Unexplained)
	case recon.Diff != 0 && (recon.PendingValue != 0 || recon.IgnoredValue != 0):
		reconStatus = CheckWarning
		reconDetail = fmt.Sprintf("selisih buku−ledger %d dijelaskan oleh dokumen belum dijurnal (%d) dan dokumen diabaikan (%d)",
			recon.Diff, recon.PendingValue, recon.IgnoredValue)
	}
	add("inventory_recon", "Persediaan buku = ledger biaya", reconStatus, reconDetail, absInt64(recon.Unexplained))
	if recon.OpeningUnposted != 0 {
		add("opening_unposted", "Stok awal belum dijurnal", CheckWarning,
			fmt.Sprintf("stok awal senilai %d belum dijurnal: saldo awal keuangan belum diposting", recon.OpeningUnposted), 1)
	}
	if recon.TransferNet != 0 {
		add("transfer_value", "Nilai transfer antar-cabang", CheckWarning,
			fmt.Sprintf("transfer antar-cabang menggeser nilai persediaan ledger %d tanpa jurnal (dikeluarkan dari rekonsiliasi)", recon.TransferNet), 1)
	}
	if recon.MissingCost > 0 {
		add("missing_cost", "Produk berstok tanpa harga modal", CheckWarning,
			fmt.Sprintf("%d produk berstok belum punya harga modal (nilai persediaan nol)", recon.MissingCost), int64(recon.MissingCost))
	}
	add("orphans", "Jurnal dokumen batal sudah dibalik", state(int64(len(orphans)), CheckFailed),
		fmt.Sprintf("%d jurnal dokumen batal belum dibalik", len(orphans)), int64(len(orphans)))
	add("tax_invoice", "Nomor faktur penjualan", state(res.MissingTaxInvoice, CheckWarning),
		fmt.Sprintf("%d order jual kena pajak tanpa nomor faktur", res.MissingTaxInvoice), res.MissingTaxInvoice)
	add("supplier_invoice", "Nomor faktur supplier", state(res.MissingSupplierInvoice, CheckWarning),
		fmt.Sprintf("%d order beli kena pajak tanpa nomor faktur supplier", res.MissingSupplierInvoice), res.MissingSupplierInvoice)
	res.Ready = len(res.Blockers) == 0
	return res, nil
}

// MissingMappings lists the mapping keys that point at no active account.
func (s *Service) MissingMappings(ctx context.Context) ([]string, error) {
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var missing []string
	for _, k := range mappingKeys {
		if a := maps[k]; a == nil || a.Status != "active" {
			missing = append(missing, k)
		}
	}
	return missing, nil
}

// negativeInventory counts cost positions whose quantity or value is negative.
func (s *Service) negativeInventory(ctx context.Context, branchID int64) (int64, error) {
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	var n int64
	for _, p := range positions {
		if p.Qty < -1e-9 || p.Total < -0.5 {
			n++
		}
	}
	return n, nil
}

// Orphans lists live journals whose source document was cancelled afterwards
// (payments and returns can be cancelled after posting). The automatic reversal
// normally prevents them; this is the safety net for when it failed.
func (s *Service) Orphans(ctx context.Context, branchID int64) ([]QueueItem, error) {
	type owner struct {
		docType   string
		cancelled func(ctx context.Context, ids []int64) ([]int64, error)
	}
	owners := []owner{
		{SourcePayment, func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.payment.CancelledAmong(ctx, branchID, ids)
		}},
		{SourceSalesReturn, func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.salesReturns.CancelledAmong(ctx, branchID, ids)
		}},
		{SourcePurchaseReturn, func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.purchaseReturns.CancelledAmong(ctx, branchID, ids)
		}},
		// Balance reclassifications of a cancelled return (K-04a).
		{SourceSalesReturnSettlement, func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.salesReturns.CancelledSettlementsAmong(ctx, branchID, ids)
		}},
		{SourcePurchaseReturnSettlement, func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.purchaseReturns.CancelledSettlementsAmong(ctx, branchID, ids)
		}},
	}
	out := []QueueItem{}
	for _, o := range owners {
		var after int64
		for {
			ids, err := s.repo.PostedSourceIDsAfter(ctx, branchID, o.docType, after, QueuePage)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			if len(ids) == 0 {
				break
			}
			gone, err := o.cancelled(ctx, ids)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			for _, id := range gone {
				out = append(out, QueueItem{DocType: o.docType, DocID: id, Number: "Jurnal " + o.docType + " #" + strconv.FormatInt(id, 10)})
			}
			after = ids[len(ids)-1]
			if len(ids) < QueuePage {
				break
			}
		}
	}
	return out, nil
}

// SafeClose seals the month only when every gate passes. The period row is
// locked FOR UPDATE while the gate is evaluated, and every posting takes the
// same row FOR SHARE: a document journalled concurrently either lands before
// the gate (and is seen) or is refused after the close — it cannot slip into a
// month that was just sealed. Use ClosePeriod only as a deliberate override.
func (s *Service) SafeClose(ctx context.Context, actorID, branchID int64, year, month int) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	err := s.repo.CloseWithGate(ctx, branchID, year, month, func() error {
		check, err := s.Readiness(ctx, branchID, year, month)
		if err != nil {
			return err
		}
		if !check.Ready {
			return apperror.Conflict("Periode belum siap ditutup: " + joinBlockers(check.Blockers))
		}
		return nil
	}, func(ctx context.Context, tx *sql.Tx) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "fiscal_periods.safe_close", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID,
			Note:   fmt.Sprintf("%04d-%02d", year, month),
			Before: map[string]any{"status": "open"}, After: map[string]any{"status": "closed"},
			Metadata: map[string]any{"year": year, "month": month},
		})
	})
	if err != nil {
		if err == infrastructure.ErrPeriodClosed {
			return apperror.Conflict("Periode sudah ditutup")
		}
		if _, ok := err.(*apperror.AppError); ok {
			return err
		}
		return apperror.Internal(err)
	}
	return nil
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
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
