package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
)

// Mapping keys for automatic postings. Unknown keys are rejected (typos must
// fail loudly, never post to a wrong account).
var mappingKeys = []string{
	"cash", "bank", "receivable", "inventory", "ppn_in", "payable",
	"ppn_out", "revenue", "sales_return", "cogs", "expense",
	"writeoff", "adjustment",
}

// DefaultChart seeds the standard Indonesian CoA plus mappings. Idempotent:
// re-runs keep codes, refresh names, and re-point mappings.
func DefaultChart() (accounts []accountSeed, mappings map[string]string) {
	accounts = []accountSeed{
		{"1100", "Kas Tunai", contracts.AccountAsset, true},
		{"1110", "Bank Transfer", contracts.AccountAsset, true},
		{"1200", "Piutang Usaha", contracts.AccountAsset, false},
		{"1300", "Persediaan Barang", contracts.AccountAsset, false},
		{"1400", "PPN Masukan", contracts.AccountAsset, false},
		{"2100", "Hutang Usaha", contracts.AccountLiability, false},
		{"2200", "PPN Keluaran", contracts.AccountLiability, false},
		{"3000", "Modal Awal", contracts.AccountEquity, false},
		{"4000", "Penjualan", contracts.AccountIncome, false},
		{"4100", "Retur Penjualan", contracts.AccountIncome, false},
		{"5000", "Harga Pokok Penjualan", contracts.AccountExpense, false},
		{"6000", "Beban Operasional", contracts.AccountExpense, false},
		{"6100", "Beban Hapus Stok", contracts.AccountExpense, false},
		{"6200", "Penyesuaian Persediaan", contracts.AccountExpense, false},
	}
	mappings = map[string]string{
		"cash": "1100", "bank": "1110", "receivable": "1200",
		"inventory": "1300", "ppn_in": "1400", "payable": "2100",
		"ppn_out": "2200", "revenue": "4000", "sales_return": "4100",
		"cogs": "5000", "expense": "6000", "writeoff": "6100",
		"adjustment": "6200",
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
}

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

func (s *Service) salesOpsCountMissingTaxInvoice(ctx context.Context, branchID int64) (int64, error) {
	if s.salesOps == nil {
		return 0, apperror.Internal(errOpsUnwired)
	}
	n, err := s.salesOps.CountMissingTaxInvoice(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

func (s *Service) purchOpsCountMissingSupplierInvoice(ctx context.Context, branchID int64) (int64, error) {
	if s.purchOps == nil {
		return 0, apperror.Internal(errOpsUnwired)
	}
	n, err := s.purchOps.CountMissingSupplierInvoice(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// EnsureChart seeds the default CoA + mappings (idempotent).
func (s *Service) EnsureChart(ctx context.Context, actorID int64) error {
	accounts, mappings := DefaultChart()
	for _, a := range accounts {
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
	if err := s.repo.SetPeriodStatus(ctx, branchID, year, month, "closed"); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "fiscal_periods.close", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID, Note: fmt.Sprintf("%04d-%02d", year, month)})
	return nil
}

// ReopenPeriod unlocks a month.
func (s *Service) ReopenPeriod(ctx context.Context, actorID, branchID int64, year, month int) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if err := s.repo.SetPeriodStatus(ctx, branchID, year, month, "open"); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "fiscal_periods.reopen", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID, Note: fmt.Sprintf("%04d-%02d", year, month)})
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

// nextNumber allocates JV-/EXP- numbers (yearly counters).
func (s *Service) nextNumber(ctx context.Context, branchID int64, prefix, year string) (string, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return "", apperror.Internal(err)
	}
	if b == nil {
		return "", apperror.NotFound("Cabang")
	}
	n, err := s.repo.NextSequence(ctx, branchID, prefix+":"+year)
	if err != nil {
		return "", apperror.Internal(err)
	}
	return fmt.Sprintf("%s-%s/%s/%05d", prefix, b.Code, year, n), nil
}

// --- FinanceClient ----------------------------------------------------------

// AccountBalance returns lifetime debit/credit footing for one account.
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

// NetProfit returns revenue/expense/profit for an inclusive date range.
func (s *Service) NetProfit(ctx context.Context, branchID int64, from, to string) (int64, int64, int64, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return 0, 0, 0, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end)
	if err != nil {
		return 0, 0, 0, err
	}
	return rep.Revenue, rep.Expense, rep.Profit, nil
}
