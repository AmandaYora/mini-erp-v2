package contracts

import "context"

// Profile is the singleton company identity printed on documents.
type Profile struct {
	ID        int64
	Name      string
	LegalName string
	Address   string
	City      string
	Phone     string
	Email     string
	TaxID     string
}

// PrintSettingKeys is the allow-list of company_settings keys the print
// profile may expose to every authenticated user (F3a-6). Nothing else in
// company_settings is ever returned by that endpoint. `document_profile` is
// the single JSON key holding letterhead, bank accounts and the continuous
// paper calibration.
var PrintSettingKeys = []string{"document_profile"}

// PrintProfile is the minimum the printing screens need: identity from the
// company profile plus the allow-listed settings (key -> raw text).
type PrintProfile struct {
	Name      string
	LegalName string
	Address   string
	City      string
	Phone     string
	Settings  map[string]string
}

// CompanyClient is the public surface of the company module.
type CompanyClient interface {
	// GetProfile returns the singleton profile, or nil when never saved.
	GetProfile(ctx context.Context) (*Profile, error)
	// GetSettings returns all operational settings as key → raw text
	// (plain string or serialized JSON — the reader decides how to parse).
	GetSettings(ctx context.Context) (map[string]string, error)
}
