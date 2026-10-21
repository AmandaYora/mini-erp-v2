package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Mapping keys for automatic postings. Unknown keys are rejected (typos must
// fail loudly, never post to a wrong account).
var mappingKeys = []string{
	"cash", "bank", "receivable", "inventory", "ppn_in", "payable",
	"ppn_out", "customer_advance", "supplier_advance", "opening_equity",
	"revenue", "sales_return", "cogs", "expense", "writeoff", "adjustment",
	"rounding_difference", "purchase_return_difference", "opening_correction",
}

// DefaultChart seeds the standard Indonesian CoA plus mappings, using the
// LEGACY account codes so accountants keep the codes they know and the data
// migration is an identity map on codes (F2b-8). Idempotent: re-runs keep
// codes, refresh names, and re-point mappings.
func DefaultChart() (accounts []accountSeed, mappings map[string]string) {
	accounts = []accountSeed{
		{"1000", "Kas Tunai", contracts.AccountAsset, true},
		{"1010", "Rekening Bank", contracts.AccountAsset, true},
		{"1100", "Piutang Usaha", contracts.AccountAsset, false},
		{"1200", "Persediaan Barang", contracts.AccountAsset, false},
		{"1300", "PPN Masukan", contracts.AccountAsset, false},
		{"2000", "Hutang Usaha", contracts.AccountLiability, false},
		{"2100", "PPN Keluaran", contracts.AccountLiability, false},
		{"2200", "Uang Muka Pelanggan", contracts.AccountLiability, false},
		{"2300", "Uang Muka Supplier", contracts.AccountAsset, false},
		{"3000", "Modal Awal / Saldo Awal", contracts.AccountEquity, false},
		{"3100", "Koreksi Saldo Awal", contracts.AccountEquity, false},
		{"4000", "Penjualan Barang", contracts.AccountIncome, false},
		{"4100", "Retur Penjualan", contracts.AccountIncome, false},
		{"5000", "Harga Pokok Penjualan", contracts.AccountExpense, false},
		{"5100", "Selisih Stok", contracts.AccountExpense, false},
		{"5200", "Pembulatan / Selisih Posting", contracts.AccountExpense, false},
		{"5300", "Selisih Retur Pembelian", contracts.AccountExpense, false},
		{"6000", "Beban Gaji", contracts.AccountExpense, false},
		{"6010", "Beban Listrik dan Air", contracts.AccountExpense, false},
		{"6020", "Beban Sewa", contracts.AccountExpense, false},
		{"6030", "Beban Internet dan Telepon", contracts.AccountExpense, false},
		{"6040", "Beban Transport dan BBM", contracts.AccountExpense, false},
		{"6050", "Beban Administrasi Bank", contracts.AccountExpense, false},
		{"6900", "Beban Lain-lain", contracts.AccountExpense, false},
	}
	mappings = map[string]string{
		"cash": "1000", "bank": "1010", "receivable": "1100",
		"inventory": "1200", "ppn_in": "1300", "payable": "2000",
		"ppn_out": "2100", "customer_advance": "2200", "supplier_advance": "2300",
		"opening_equity": "3000", "revenue": "4000", "sales_return": "4100",
		"cogs": "5000", "expense": "6900", "writeoff": "5100",
		"adjustment": "5100", "rounding_difference": "5200",
		"purchase_return_difference": "5300", "opening_correction": "3100",
	}
	return accounts, mappings
}

type accountSeed struct {
	code, name, accType string
	cash                bool
}

// Service implements contracts.FinanceClient plus finance administration.
// Finance is the top consumer: it reads every module through contracts and
// owns the only books (journals) and the HPP cost ledger.
type Service struct {
	repo            *infrastructure.Repository
	branches        branchcontracts.BranchClient
	products        productcontracts.ProductClient
	parties         partycontracts.PartyClient
	purchasing      purchasingcontracts.PurchaseOrderClient
	sales           salescontracts.SalesOrderClient
	salesReturns    salesreturncontracts.SalesReturnClient
	purchaseReturns purchasereturncontracts.PurchaseReturnClient
	goodsreceipt    goodsreceiptcontracts.GoodsReceiptClient
	delivery        deliverycontracts.DeliveryClient
	payment         paymentcontracts.PaymentClient
	stock           stockcontracts.StockClient
	audit           auditcontracts.AuditClient
	// salesOps/purchOps feed the readiness gate (missing invoice counts).
	// Injected via SetOps: nil means unwired, and readiness fails loud
	// rather than reporting a fake zero.
	salesOps salescontracts.SalesOpsClient
	purchOps purchasingcontracts.PurchasingOpsClient
	// media stores the attachments of fiscal corrections (FN-07).
	media mediacontracts.MediaClient
}

// SetMedia injects the media client (composition-root privilege).
func (s *Service) SetMedia(m mediacontracts.MediaClient) { s.media = m }

// NewService wires finance use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	parties partycontracts.PartyClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	salesReturns salesreturncontracts.SalesReturnClient,
	purchaseReturns purchasereturncontracts.PurchaseReturnClient,
	goodsreceipt goodsreceiptcontracts.GoodsReceiptClient,
	delivery deliverycontracts.DeliveryClient,
	payment paymentcontracts.PaymentClient,
	stock stockcontracts.StockClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, branches: branches, products: products, parties: parties,
		purchasing: purchasing, sales: sales, salesReturns: salesReturns,
		purchaseReturns: purchaseReturns, goodsreceipt: goodsreceipt,
		delivery: delivery, payment: payment, stock: stock, audit: audit}
}

// SetOps injects the operational order surfaces (composition-root
// privilege). Required before Readiness/SafeClose; without it the gate
// refuses to run instead of inventing a clean bill of health.
func (s *Service) SetOps(sales salescontracts.SalesOpsClient, purch purchasingcontracts.PurchasingOpsClient) {
	s.salesOps, s.purchOps = sales, purch
}

func (s *Service) salesOpsCountMissingTaxInvoice(ctx context.Context, branchID int64, from, to string) (int64, error) {
	if s.salesOps == nil {
		return 0, apperror.Internal(errOpsUnwired)
	}
	n, err := s.salesOps.CountMissingTaxInvoiceBetween(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

func (s *Service) purchOpsCountMissingSupplierInvoice(ctx context.Context, branchID int64, from, to string) (int64, error) {
	if s.purchOps == nil {
		return 0, apperror.Internal(errOpsUnwired)
	}
	n, err := s.purchOps.CountMissingSupplierInvoiceBetween(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// EnsureChart seeds the default CoA + mappings (idempotent).
func (s *Service) EnsureChart(ctx context.Context, actorID int64) error {
	accounts, mappings := DefaultChart()
	for _, a := range accounts {
		// An existing code of a different type would silently change what the
		// books mean (the legacy-code realignment, F2b-8): refuse when journals
		// already use it; otherwise the seed is safe to refresh.
		if existing, err := s.repo.AccountByCode(ctx, a.code); err != nil {
			return apperror.Internal(err)
		} else if existing != nil && existing.Type != a.accType {
			if used, uerr := s.repo.AccountInUse(ctx, existing.ID); uerr != nil {
				return apperror.Internal(uerr)
			} else if used {
				return apperror.Conflict("Akun " + a.code + " sudah dipakai jurnal dengan tipe '" + existing.Type +
					"'; tipe standar '" + a.accType + "' tidak dapat diterapkan")
			}
		}
		if err := s.repo.EnsureAccount(ctx, a.code, a.name, a.accType, a.cash); err != nil {
			return apperror.Internal(err)
		}
	}
	for key, code := range mappings {
		acc, err := s.repo.AccountByCode(ctx, code)
		if err != nil {
			return apperror.Internal(err)
		}
		if acc == nil {
			return apperror.Internal(errMissingAccount(code))
		}
		if err := s.repo.SetMapping(ctx, key, acc.ID, actorID); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

func errMissingAccount(code string) error {
	return fmt.Errorf("default account %s missing after seed", code)
}

// errOpsUnwired fires when the readiness gate runs before the composition
// root injected the operational order surfaces.
var errOpsUnwired = fmt.Errorf("operational order surfaces not wired (SetOps)")

// --- accounts ---------------------------------------------------------------

var validAccountTypes = map[string]bool{
	contracts.AccountAsset: true, contracts.AccountLiability: true,
	contracts.AccountEquity: true, contracts.AccountIncome: true,
	contracts.AccountExpense: true,
}

// CreateAccount inserts a custom account (code immutable once issued).
func (s *Service) CreateAccount(ctx context.Context, actorID int64, code, name, accType string, isCash bool) (*contracts.Account, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if !validAccountTypes[accType] {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "tipe tidak dikenal"}})
	}
	if dup, err := s.repo.AccountByCode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if dup != nil {
		return nil, apperror.Conflict("Kode akun '" + code + "' sudah digunakan")
	}
	id, err := s.repo.CreateAccount(ctx, &contracts.Account{Code: code, Name: strings.TrimSpace(name), Type: accType, IsCash: isCash})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "accounts.create", Entity: "account", EntityID: id, ActorID: actorID, Note: code})
	return s.repo.AccountByID(ctx, id)
}

// UpdateAccount rewrites name/type/cash flag.
func (s *Service) UpdateAccount(ctx context.Context, actorID, id int64, name, accType string, isCash bool) (*contracts.Account, error) {
	a, err := s.repo.AccountByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if a == nil {
		return nil, apperror.NotFound("Akun")
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if !validAccountTypes[accType] {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "tipe tidak dikenal"}})
	}
	if a.Type != accType {
		// KI-147 class: flipping a type rewrites every historical report.
		if used, err := s.repo.AccountInUse(ctx, id); err != nil {
			return nil, apperror.Internal(err)
		} else if used {
			return nil, apperror.Conflict("Tipe akun berjurnal tidak dapat diubah")
		}
	}
	a.Name = strings.TrimSpace(name)
	a.Type = accType
	a.IsCash = isCash
	if err := s.repo.UpdateAccount(ctx, a); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.repo.AccountByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "accounts.update", Entity: "account", EntityID: id, ActorID: actorID, Note: a.Code})
	return updated, nil
}

// ArchiveAccount archives an unused, unmapped account.
func (s *Service) ArchiveAccount(ctx context.Context, actorID, id int64) error {
	a, err := s.repo.AccountByID(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if a == nil {
		return apperror.NotFound("Akun")
	}
	if used, err := s.repo.AccountInUse(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if used {
		return apperror.Conflict("Akun sudah dipakai jurnal dan tidak dapat diarsipkan")
	}
	mappings, err := s.repo.ListMappings(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	for key, mapped := range mappings {
		if mapped.ID == id {
			return apperror.Conflict("Akun terpetakan sebagai '" + key + "', lepas dulu")
		}
	}
	if err := s.repo.SetAccountStatus(ctx, id, "archived"); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "accounts.archive", Entity: "account", EntityID: id, ActorID: actorID, Note: a.Code})
	return nil
}

// ListAccounts returns accounts (optionally filtered by status).
func (s *Service) ListAccounts(ctx context.Context, status string) ([]*contracts.Account, error) {
	accounts, err := s.repo.ListAccounts(ctx, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if accounts == nil {
		accounts = []*contracts.Account{}
	}
	return accounts, nil
}

// --- mappings ---------------------------------------------------------------

// GetMappings returns key → account rows.
func (s *Service) GetMappings(ctx context.Context) (map[string]*contracts.Account, error) {
	mappings, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if mappings == nil {
		mappings = map[string]*contracts.Account{}
	}
	return mappings, nil
}

// SetMapping points a known key at an active account.
func (s *Service) SetMapping(ctx context.Context, actorID int64, key string, accountID int64) error {
	known := false
	for _, k := range mappingKeys {
		if k == key {
			known = true
			break
		}
	}
	if !known {
		return apperror.Validation("", []apperror.FieldError{{Field: "key", Message: "kunci pemetaan tidak dikenal"}})
	}
	a, err := s.repo.AccountByID(ctx, accountID)
	if err != nil {
		return apperror.Internal(err)
	}
	if a == nil || a.Status != "active" {
		return apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "akun tidak ditemukan"}})
	}
	if !mappingTypes[key][a.Type] {
		return apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "tipe akun tidak sesuai untuk '" + key + "'"}})
	}
	// Re-pointing a mapping whose old account already carries journals would
	// split one meaning across two accounts (legacy rule, F2b-7).
	if old, err := s.repo.MappingAccount(ctx, key); err != nil {
		return apperror.Internal(err)
	} else if old != nil && old.ID != accountID {
		if used, uerr := s.repo.AccountInUse(ctx, old.ID); uerr != nil {
			return apperror.Internal(uerr)
		} else if used {
			return apperror.Conflict("Pemetaan '" + key + "' tidak dapat diubah: akun lama " + old.Code +
				" sudah dipakai jurnal. Buat akun baru untuk transaksi ke depan")
		}
	}
	if err := s.repo.SetMapping(ctx, key, accountID, actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "account_mappings.set", Entity: "account_mapping", ActorID: actorID, Note: key})
	return nil
}

// mappingTypes pins each mapping key to the account types that make sense.
// A revenue key pointing at an asset account would silently corrupt every
// report that groups by type.
var mappingTypes = map[string]map[string]bool{
	"cash":         {contracts.AccountAsset: true},
	"bank":         {contracts.AccountAsset: true},
	"receivable":   {contracts.AccountAsset: true},
	"inventory":    {contracts.AccountAsset: true},
	"ppn_in":       {contracts.AccountAsset: true},
	"payable":      {contracts.AccountLiability: true},
	"ppn_out":      {contracts.AccountLiability: true},
	"revenue":      {contracts.AccountIncome: true},
	"sales_return": {contracts.AccountIncome: true},
	"cogs":         {contracts.AccountExpense: true},
	"expense":      {contracts.AccountExpense: true},
	"writeoff":     {contracts.AccountExpense: true},
	"adjustment":   {contracts.AccountExpense: true},

	"customer_advance":           {contracts.AccountLiability: true},
	"supplier_advance":           {contracts.AccountAsset: true},
	"opening_equity":             {contracts.AccountEquity: true},
	"opening_correction":         {contracts.AccountEquity: true},
	"rounding_difference":        {contracts.AccountExpense: true},
	"purchase_return_difference": {contracts.AccountExpense: true},
}

// mustMap resolves a mapping key or fails with an actionable message.
func (s *Service) mustMap(ctx context.Context, key string) (*contracts.Account, error) {
	a, err := s.repo.MappingAccount(ctx, key)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if a == nil || a.Status != "active" {
		return nil, apperror.Conflict("Pemetaan akun '" + key + "' belum diatur")
	}
	return a, nil
}

// --- periods ----------------------------------------------------------------

// ListPeriods returns a branch's touched months.
func (s *Service) ListPeriods(ctx context.Context, branchID int64) ([]*infrastructure.Period, error) {
	periods, err := s.repo.ListPeriods(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if periods == nil {
		periods = []*infrastructure.Period{}
	}
	return periods, nil
}

func validYearMonth(year, month int) bool {
	return year >= 2000 && year <= 2100 && month >= 1 && month <= 12
}

// ClosePeriod locks a month against new entries.
func (s *Service) ClosePeriod(ctx context.Context, actorID, branchID int64, year, month int) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if err := s.repo.SetPeriodStatus(ctx, branchID, year, month, "closed",
		s.periodAudit("fiscal_periods.close", "", "closed", branchID, actorID, year, month, "")); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// periodAudit builds the in-transaction trail hook for a period status
// change, with before/after status (F3b-8, ADR-0011).
//
// reason is recorded only for a reopen (FN-05): it lands in the audit diff
// (After.reason + Metadata.reason) and in the note, so the trail answers "why
// was this month opened again".
func (s *Service) periodAudit(action, notePrefix, after string, branchID, actorID int64, year, month int, reason string) func(ctx context.Context, tx *sql.Tx, before string) error {
	return func(ctx context.Context, tx *sql.Tx, before string) error {
		afterMap := map[string]any{"status": after}
		meta := map[string]any{"year": year, "month": month}
		note := notePrefix + fmt.Sprintf("%04d-%02d", year, month)
		if reason != "" {
			afterMap["reason"] = reason
			meta["reason"] = reason
			note += ": " + reason
		}
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: action, Entity: "fiscal_period", BranchID: branchID, ActorID: actorID,
			Note:   note,
			Before: map[string]any{"status": before}, After: afterMap,
			Metadata: meta,
		})
	}
}

// reopenReason validates the mandatory reason of a reopen (FN-05): a closed
// month is a control, so opening it again must say why.
func reopenReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan membuka kembali periode wajib diisi"}})
	}
	if len([]rune(reason)) > 255 {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan maksimal 255 karakter"}})
	}
	return reason, nil
}

// ReopenPeriod unlocks a month. A reason is mandatory and lands in the audit
// diff (FN-05).
func (s *Service) ReopenPeriod(ctx context.Context, actorID, branchID int64, year, month int, reason string) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	reason, rerr := reopenReason(reason)
	if rerr != nil {
		return rerr
	}
	if err := s.repo.SetPeriodStatus(ctx, branchID, year, month, "open",
		s.periodAudit("fiscal_periods.reopen", "", "open", branchID, actorID, year, month, reason)); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// requireOpenPeriod rejects dates inside a closed accounting month.
func (s *Service) requireOpenPeriod(ctx context.Context, branchID int64, date string) error {
	y, m := parseYearMonth(date)
	p, err := s.repo.GetPeriod(ctx, branchID, y, m)
	if err != nil {
		return apperror.Internal(err)
	}
	if p != nil && p.Status == "closed" {
		return apperror.Conflict("Periode sudah ditutup")
	}
	if p == nil {
		before, err := s.repo.BeforeBooksStart(ctx, branchID, y, m)
		if err != nil {
			return apperror.Internal(err)
		}
		if before {
			return apperror.Conflict(beforeBooksStartMessage)
		}
	}
	return nil
}

const beforeBooksStartMessage = "Tanggal sebelum awal pembukuan — bulan itu tidak tercakup buku (saldo awal sudah memuatnya)"

// periodError maps the in-transaction period guards to the API error; nil when
// err is neither.
func periodError(err error) error {
	switch {
	case errors.Is(err, infrastructure.ErrPeriodClosed):
		return apperror.Conflict("Periode sudah ditutup")
	case errors.Is(err, infrastructure.ErrBeforeBooksStart):
		return apperror.Conflict(beforeBooksStartMessage)
	}
	return nil
}

func parseYearMonth(date string) (int, int) {
	if len(date) < 7 {
		return 0, 0
	}
	y, err1 := strconv.Atoi(date[:4])
	m, err2 := strconv.Atoi(date[5:7])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return y, m
}

// --- numbers ----------------------------------------------------------------

// --- FinanceClient ----------------------------------------------------------

// AccountBalance returns lifetime debit/credit footing for one account.
//
// AccountFootings aggregates the branch's whole posting history, so asking
// for N accounts this way pays for that history N times. Readers wanting
// several key figures must call AccountBalances instead.
func (s *Service) AccountBalance(ctx context.Context, branchID int64, code string) (int64, int64, error) {
	a, err := s.repo.AccountByCode(ctx, code)
	if err != nil {
		return 0, 0, apperror.Internal(err)
	}
	if a == nil {
		return 0, 0, apperror.NotFound("Akun")
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", "")
	if err != nil {
		return 0, 0, apperror.Internal(err)
	}
	f := footings[a.ID]
	return f[0], f[1], nil
}

// AccountBalances returns lifetime net balances (debit − credit) for several
// account codes in one grouped read, keyed by code.
//
// Two queries total, whatever the number of codes: the chart (small, fully
// indexed) and one footing over the branch's history. Codes with no account
// read as zero — an unseeded chart is not an error here, matching what
// AccountBalance's NotFound meant to its dashboard callers.
func (s *Service) BalancesByKey(ctx context.Context, branchID int64, keys []string, asOf string) (map[string]int64, error) {
	out := make(map[string]int64, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	to, err := asOfUpperBound(asOf)
	if err != nil {
		return nil, err
	}
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, k := range keys {
		out[k] = 0
		if a := maps[k]; a != nil {
			f := footings[a.ID]
			out[k] = f[0] - f[1]
		}
	}
	return out, nil
}

// asOfUpperBound turns an inclusive WIB calendar day into the footing's upper
// bound — the same "<day> 23:59:59" BalanceSheet and TrialBalanceAsOf use, so a
// dashboard capped at today equals the balance sheet of today. Empty = no bound.
func asOfUpperBound(asOf string) (string, error) {
	asOf = strings.TrimSpace(asOf)
	if asOf == "" {
		return "", nil
	}
	day, err := timeutil.ParseDateInput(asOf)
	if err != nil {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "asOf", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	return day.Format("2006-01-02") + " 23:59:59", nil
}

// AccountBalances returns lifetime net balances for several chart codes in one read.
func (s *Service) AccountBalances(ctx context.Context, branchID int64, codes []string) (map[string]int64, error) {
	out := make(map[string]int64, len(codes))
	for _, c := range codes {
		out[c] = 0
	}
	if len(codes) == 0 {
		return out, nil
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Only the requested codes: the chart is small, the intersection smaller.
	idToCode := make(map[int64]string, len(codes))
	for _, a := range accounts {
		if _, want := out[a.Code]; want {
			idToCode[a.ID] = a.Code
		}
	}
	if len(idToCode) == 0 {
		return out, nil
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for id, code := range idToCode {
		f := footings[id]
		out[code] = f[0] - f[1]
	}
	return out, nil
}

// NetProfit returns revenue/expense/profit for an inclusive date range.
func (s *Service) NetProfit(ctx context.Context, branchID int64, from, to string) (int64, int64, int64, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return 0, 0, 0, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end, fullBooks)
	if err != nil {
		return 0, 0, 0, err
	}
	return rep.Revenue, rep.Expense, rep.Profit, nil
}
