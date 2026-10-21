package application

import (
	"context"
	"errors"
	"fmt"

	"mini-erp/internal/modules/finance/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/shared/apperror"
)

// partyControlKeys are the mapped control accounts the books keep PER PARTY,
// with the party type each one takes. The per-party balances and the aging
// reconciliation read only lines that carry a party, so a typed leg on one of
// them without a party would move the balance sheet yet belong to no customer
// or supplier (F5).
var partyControlKeys = map[string]string{
	"receivable":       partycontracts.PartyCustomer,
	"customer_advance": partycontracts.PartyCustomer,
	"payable":          partycontracts.PartySupplier,
	"supplier_advance": partycontracts.PartySupplier,
}

// partyRule says whether a typed leg on a control account must name a party.
type partyRule int

const (
	// partyRequired: commercial journals (manual, opening) — they feed the
	// per-party balances and aging.
	partyRequired partyRule = iota
	// partyOptional: fiscal adjustments (scope 1) — never read per party, so
	// a party is accepted (and checked) but not demanded.
	partyOptional
)

// partyControls maps the account id of each per-party control account to the
// party type it takes (one cached mapping read).
func (s *Service) partyControls(ctx context.Context) (map[int64]string, error) {
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make(map[int64]string, len(partyControlKeys))
	for key, typ := range partyControlKeys {
		if a := maps[key]; a != nil {
			out[a.ID] = typ
		}
	}
	return out, nil
}

// lineParty checks the party of typed leg i against its account. A leg on a
// per-party control account names an existing party of the matching type
// (customer for Piutang/Uang Muka Pelanggan, supplier for Hutang/Uang Muka
// Supplier). Any other leg must not name one: the party would be stored yet
// read by no report, so a stray id is refused rather than silently kept or
// dropped.
func (s *Service) lineParty(ctx context.Context, controls map[int64]string, rule partyRule, i int, a *contracts.Account, partyID int64) (int64, error) {
	want, control := controls[a.ID]
	if !control {
		if partyID != 0 {
			return 0, lineError(i, fmt.Sprintf("akun %s %s tidak memakai pelanggan/pemasok", a.Code, a.Name))
		}
		return 0, nil
	}
	label := "pelanggan"
	if want == partycontracts.PartySupplier {
		label = "pemasok"
	}
	if partyID == 0 {
		if rule == partyOptional {
			return 0, nil
		}
		return 0, lineError(i, fmt.Sprintf("akun %s %s wajib memilih %s", a.Code, a.Name, label))
	}
	if s.parties == nil {
		return 0, apperror.Internal(errors.New("party client not wired"))
	}
	p, err := s.parties.GetByID(ctx, partyID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if p == nil {
		return 0, lineError(i, label+" tidak ditemukan")
	}
	if p.Type != want {
		return 0, lineError(i, fmt.Sprintf("akun %s %s hanya untuk %s; %s bukan %s", a.Code, a.Name, label, p.Name, label))
	}
	return p.ID, nil
}

func lineError(i int, msg string) error {
	return apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: fmt.Sprintf("baris %d: %s", i+1, msg)}})
}
