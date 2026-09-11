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

// CompanyClient is the public surface of the company module.
type CompanyClient interface {
	// GetProfile returns the singleton profile, or nil when never saved.
	GetProfile(ctx context.Context) (*Profile, error)
	// GetSettings returns all operational settings as key → raw text
	// (plain string or serialized JSON — the reader decides how to parse).
	GetSettings(ctx context.Context) (map[string]string, error)
}
