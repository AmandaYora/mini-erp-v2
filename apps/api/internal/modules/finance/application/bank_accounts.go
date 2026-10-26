package application

import (
	"context"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// --- bank accounts (rekening perusahaan, ADR-0026) ---------------------------
//
// A rekening is bank identity on top of one cash/bank GL account (aset,
// is_cash). Sales stores the GL account id as the bill's rekening tujuan,
// payment stores it as the account the money moved through, and the payment
// journal posts to it. The balance lives on the GL account only.

// BankAccountInput creates or edits a rekening. AccountID names an existing
// cash/bank account; 0 with AccountCode creates that account first.
type BankAccountInput struct {
	AccountID       int64
	AccountCode     string
	AccountName     string
	BankName        string
	AccountNumber   string
	AccountHolder   string
	ShowOnDocuments bool
	IsDefault       bool
	SortOrder       int
}

func (in *BankAccountInput) normalize() []apperror.FieldError {
	in.BankName = strings.TrimSpace(in.BankName)
	in.AccountNumber = strings.TrimSpace(in.AccountNumber)
	in.AccountHolder = strings.TrimSpace(in.AccountHolder)
	in.AccountCode = strings.ToUpper(strings.TrimSpace(in.AccountCode))
	in.AccountName = strings.TrimSpace(in.AccountName)
	var errs []apperror.FieldError
	req := func(field, v string, max int) {
		switch {
		case v == "":
			errs = append(errs, apperror.FieldError{Field: field, Message: "wajib diisi"})
		case utf8.RuneCountInString(v) > max:
			errs = append(errs, apperror.FieldError{Field: field, Message: "terlalu panjang"})
		}
	}
	req("bankName", in.BankName, 50)
	req("accountNumber", in.AccountNumber, 50)
	req("accountHolder", in.AccountHolder, 150)
	return errs
}

// ListBankAccounts returns every rekening (active first) — the admin list and
// the options read by cashiers (archived rows let old documents keep a label).
func (s *Service) ListBankAccounts(ctx context.Context, status string) ([]*contracts.BankAccount, error) {
	out, err := s.repo.ListBankAccounts(ctx, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if out == nil {
		out = []*contracts.BankAccount{}
	}
	return out, nil
}

// CreateBankAccount registers a rekening over a cash/bank GL account.
func (s *Service) CreateBankAccount(ctx context.Context, actorID int64, in BankAccountInput) (*contracts.BankAccount, error) {
	if errs := in.normalize(); len(errs) > 0 {
		return nil, apperror.Validation("", errs)
	}
	existing, err := s.repo.ListBankAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, b := range existing {
		if strings.EqualFold(b.BankName, in.BankName) && b.AccountNumber == in.AccountNumber {
			return nil, apperror.Conflict("Rekening dengan bank dan nomor yang sama sudah ada")
		}
	}
	if in.AccountID == 0 {
		if in.AccountCode == "" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "pilih akun kas/bank atau isi kode akun baru"}})
		}
		name := in.AccountName
		if name == "" {
			name = "Bank " + in.BankName + " " + in.AccountNumber
		}
		a, err := s.CreateAccount(ctx, actorID, in.AccountCode, name, contracts.AccountAsset, true)
		if err != nil {
			return nil, err
		}
		in.AccountID = a.ID
	}
	if err := s.checkRekeningAccount(ctx, in.AccountID); err != nil {
		return nil, err
	}
	b := &contracts.BankAccount{
		AccountID: in.AccountID, BankName: in.BankName, AccountNumber: in.AccountNumber,
		AccountHolder: in.AccountHolder, ShowOnDocuments: in.ShowOnDocuments, IsDefault: in.IsDefault,
		SortOrder: in.SortOrder,
	}
	id, err := s.repo.SaveBankAccount(ctx, b, actorID)
	if err != nil {
		return nil, bankDBError(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "bank_accounts.create", Entity: "bank_account", EntityID: id, ActorID: actorID,
		Note: in.BankName + " " + in.AccountNumber})
	return s.repo.BankAccountByID(ctx, id)
}

// UpdateBankAccount edits a rekening's identity and flags. Its GL account is
// fixed: payments and journals already point at it.
func (s *Service) UpdateBankAccount(ctx context.Context, actorID, id int64, in BankAccountInput) (*contracts.BankAccount, error) {
	cur, err := s.repo.BankAccountByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if cur == nil {
		return nil, apperror.NotFound("Rekening")
	}
	if errs := in.normalize(); len(errs) > 0 {
		return nil, apperror.Validation("", errs)
	}
	if in.AccountID != 0 && in.AccountID != cur.AccountID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "akun buku besar rekening tidak dapat diganti"}})
	}
	if in.IsDefault && cur.Status != "active" {
		return nil, apperror.Conflict("Rekening yang diarsipkan tidak bisa menjadi default")
	}
	cur.BankName, cur.AccountNumber, cur.AccountHolder = in.BankName, in.AccountNumber, in.AccountHolder
	cur.ShowOnDocuments, cur.IsDefault, cur.SortOrder = in.ShowOnDocuments, in.IsDefault, in.SortOrder
	if _, err := s.repo.SaveBankAccount(ctx, cur, actorID); err != nil {
		return nil, bankDBError(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "bank_accounts.update", Entity: "bank_account", EntityID: id, ActorID: actorID,
		Note: cur.BankName + " " + cur.AccountNumber})
	return s.repo.BankAccountByID(ctx, id)
}

// SetBankAccountStatus archives (no new bills/payments may pick it) or
// restores a rekening. History keeps pointing at it.
func (s *Service) SetBankAccountStatus(ctx context.Context, actorID, id int64, active bool) (*contracts.BankAccount, error) {
	cur, err := s.repo.BankAccountByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if cur == nil {
		return nil, apperror.NotFound("Rekening")
	}
	status := "archived"
	if active {
		status = "active"
		if err := s.checkLedgerAccount(ctx, cur.AccountID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SetBankAccountStatus(ctx, id, status, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	note := cur.BankName + " " + cur.AccountNumber
	if active {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "bank_accounts.restore", Entity: "bank_account", EntityID: id, ActorID: actorID, Note: note})
	} else {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "bank_accounts.archive", Entity: "bank_account", EntityID: id, ActorID: actorID, Note: note})
	}
	return s.repo.BankAccountByID(ctx, id)
}

// checkLedgerAccount: the GL account under a rekening must be an active cash
// asset that is not the cash-in-hand mapping (Kas Tunai is no bank account).
func (s *Service) checkLedgerAccount(ctx context.Context, accountID int64) error {
	a, err := s.repo.AccountByID(ctx, accountID)
	if err != nil {
		return apperror.Internal(err)
	}
	if a == nil || a.Status != "active" || a.Type != contracts.AccountAsset || !a.IsCash {
		return apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "harus akun aset kas/bank yang aktif"}})
	}
	if cash, err := s.repo.MappingAccount(ctx, "cash"); err != nil {
		return apperror.Internal(err)
	} else if cash != nil && cash.ID == accountID {
		return apperror.Validation("", []apperror.FieldError{{Field: "accountId", Message: "akun kas tunai tidak bisa dijadikan rekening"}})
	}
	return nil
}

// checkRekeningAccount adds the 1:1 rule for a new rekening.
func (s *Service) checkRekeningAccount(ctx context.Context, accountID int64) error {
	if err := s.checkLedgerAccount(ctx, accountID); err != nil {
		return err
	}
	if dup, err := s.repo.BankAccountByAccountID(ctx, accountID); err != nil {
		return apperror.Internal(err)
	} else if dup != nil {
		return apperror.Conflict("Akun " + dup.AccountCode + " sudah dipakai rekening " + dup.BankName + " " + dup.AccountNumber)
	}
	return nil
}

// LegacyBankAccount is one rekening typed as plain text in the document
// profile before the master existed (usually carried over by the ETL).
type LegacyBankAccount struct {
	BankName      string
	AccountNumber string
	AccountHolder string
}

// LegacyImport reports what ImportLegacyBankAccounts did.
type LegacyImport struct {
	Created []*contracts.BankAccount
	// Skipped lists entries left out ("BCA 123: alasan"): blank, duplicate or invalid.
	Skipped []string
}

// ImportLegacyBankAccounts moves the document profile's text rekening into the
// master once (analysis/004), so the master is the only source. It runs only
// while the master holds no rekening at all; afterwards it is a no-op. The
// first entry becomes the default on the bank mapping account (1010, which
// already carries every transfer so far); the others get new cash accounts
// 1011, 1012, … whose opening balance the accountant reclassifies from 1010.
// Every entry is validated before anything is written.
func (s *Service) ImportLegacyBankAccounts(ctx context.Context, legacy []LegacyBankAccount) (LegacyImport, error) {
	var out LegacyImport
	existing, err := s.repo.ListBankAccounts(ctx, "")
	if err != nil {
		return out, apperror.Internal(err)
	}
	if len(existing) > 0 || len(legacy) == 0 {
		return out, nil
	}
	var plan []BankAccountInput
	seen := map[string]bool{}
	for _, l := range legacy {
		in := BankAccountInput{BankName: l.BankName, AccountNumber: l.AccountNumber, AccountHolder: l.AccountHolder}
		label := strings.TrimSpace(l.BankName + " " + l.AccountNumber)
		key := accountNumberKey(l.AccountNumber)
		switch {
		case strings.TrimSpace(l.BankName) == "" && key == "":
			continue // baris kosong
		case seen[key]:
			out.Skipped = append(out.Skipped, label+": nomor ganda")
			continue
		}
		if errs := in.normalize(); len(errs) > 0 {
			out.Skipped = append(out.Skipped, label+": "+errs[0].Field+" "+errs[0].Message)
			continue
		}
		seen[key] = true
		in.ShowOnDocuments, in.IsDefault, in.SortOrder = true, len(plan) == 0, len(plan)
		plan = append(plan, in)
	}
	if len(plan) == 0 {
		return out, nil
	}
	if bank, err := s.repo.MappingAccount(ctx, "bank"); err != nil {
		return out, apperror.Internal(err)
	} else if bank != nil && s.checkRekeningAccount(ctx, bank.ID) == nil {
		plan[0].AccountID = bank.ID
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return out, apperror.Internal(err)
	}
	used := map[string]bool{}
	for _, a := range accounts {
		used[a.Code] = true
	}
	next := 1011
	for i := range plan {
		if plan[i].AccountID != 0 {
			continue
		}
		for next < 1100 && used[strconv.Itoa(next)] {
			next++
		}
		if next >= 1100 {
			return out, apperror.Conflict("Kode akun kas/bank 1011–1099 sudah habis")
		}
		plan[i].AccountCode = strconv.Itoa(next)
		used[plan[i].AccountCode] = true
	}
	for _, in := range plan {
		b, err := s.CreateBankAccount(ctx, 0, in)
		if err != nil {
			return out, err
		}
		out.Created = append(out.Created, b)
	}
	return out, nil
}

// accountNumberKey compares rekening numbers without spaces, dots or dashes.
func accountNumberKey(n string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, n)
}

func bankDBError(err error) error {
	if dberr.IsDuplicate(err) {
		return apperror.Conflict("Rekening dengan bank dan nomor yang sama sudah ada")
	}
	return apperror.Internal(err)
}

// BankPolicy implements contracts.BankAccountClient.
func (s *Service) BankPolicy(ctx context.Context) (contracts.BankPolicy, error) {
	active, def, err := s.repo.BankPolicy(ctx)
	if err != nil {
		return contracts.BankPolicy{}, apperror.Internal(err)
	}
	return contracts.BankPolicy{Required: active > 0, DefaultAccountID: def}, nil
}

// CheckBankAccount implements contracts.BankAccountClient.
func (s *Service) CheckBankAccount(ctx context.Context, accountID int64, field string) error {
	b, err := s.repo.BankAccountByAccountID(ctx, accountID)
	if err != nil {
		return apperror.Internal(err)
	}
	if b == nil || b.Status != "active" {
		return apperror.Validation("", []apperror.FieldError{{Field: field, Message: "rekening tidak ditemukan atau sudah diarsipkan"}})
	}
	a, err := s.repo.AccountByID(ctx, accountID)
	if err != nil {
		return apperror.Internal(err)
	}
	if a == nil || a.Status != "active" || !a.IsCash {
		return apperror.Validation("", []apperror.FieldError{{Field: field, Message: "akun buku besar rekening tidak aktif"}})
	}
	return nil
}

// CashOnHand implements contracts.FinanceClient: every cash/bank asset
// account, mapped or not.
func (s *Service) CashOnHand(ctx context.Context, branchID int64, asOf string) (int64, error) {
	to, err := asOfUpperBound(asOf)
	if err != nil {
		return 0, err
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return 0, apperror.Internal(err)
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	var total int64
	for _, a := range accounts {
		if a.IsCash && a.Type == contracts.AccountAsset {
			f := footings[a.ID]
			total += f[0] - f[1]
		}
	}
	return total, nil
}
