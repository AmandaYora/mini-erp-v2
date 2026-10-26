package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	financeapp "mini-erp/internal/modules/finance/application"
)

// ImportLegacyBankAccounts moves the rekening typed as text in the company's
// document profile (Pengaturan › Perusahaan, carried over by the ETL) into the
// finance master once, so the master is the only source (analysis/004,
// ADR-0026). It runs on every start and is a no-op as soon as the master holds
// any rekening; after a reload of legacy data (cutover) it runs again on the
// next start. The text stays in the profile untouched.
func (m *Modules) ImportLegacyBankAccounts(ctx context.Context) (financeapp.LegacyImport, error) {
	settings, err := m.Company.Service().GetSettings(ctx)
	if err != nil {
		return financeapp.LegacyImport{}, err
	}
	raw := strings.TrimSpace(settings["document_profile"])
	if raw == "" {
		return financeapp.LegacyImport{}, nil
	}
	var profile struct {
		BankAccounts []struct {
			Bank   string `json:"bank"`
			Holder string `json:"holder"`
			Number string `json:"number"`
		} `json:"bankAccounts"`
	}
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return financeapp.LegacyImport{}, fmt.Errorf("document_profile bukan JSON valid: %w", err)
	}
	if len(profile.BankAccounts) == 0 {
		return financeapp.LegacyImport{}, nil
	}
	// The old print only showed bank + number; a blank holder falls back to the company name.
	holder := ""
	if p, err := m.Company.Service().GetProfile(ctx); err == nil && p != nil {
		holder = strings.TrimSpace(p.LegalName)
		if holder == "" {
			holder = strings.TrimSpace(p.Name)
		}
	}
	legacy := make([]financeapp.LegacyBankAccount, 0, len(profile.BankAccounts))
	for _, b := range profile.BankAccounts {
		h := strings.TrimSpace(b.Holder)
		if h == "" {
			h = holder
		}
		legacy = append(legacy, financeapp.LegacyBankAccount{BankName: b.Bank, AccountNumber: b.Number, AccountHolder: h})
	}
	return m.Finance.Service().ImportLegacyBankAccounts(ctx, legacy)
}
