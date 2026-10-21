package application

import (
	"context"

	"mini-erp/internal/shared/apperror"
)

// PartyControlBalances returns each counterparty's balance on one side's
// control accounts, as the books hold it: receivable side = Piutang + Uang
// Muka Pelanggan (debit − credit: positive = the customer owes us), payable
// side = Hutang + Uang Muka Supplier (credit − debit: positive = we owe the
// supplier). Party-attributed legs only; the reconciliation compares these to
// the documents (payment module party balance).
func (s *Service) PartyControlBalances(ctx context.Context, branchID int64, side string) (map[int64]int64, error) {
	keys, sign := []string{"receivable", "customer_advance"}, int64(1)
	if side == "payable" {
		keys, sign = []string{"payable", "supplier_advance"}, -1
	} else if side != "receivable" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "side", Message: "harus receivable atau payable"}})
	}
	out := map[int64]int64{}
	for _, k := range keys {
		acct, err := s.repo.MappingAccount(ctx, k)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if acct == nil {
			continue
		}
		footings, err := s.repo.PartyFootings(ctx, branchID, acct.ID, "")
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, f := range footings {
			out[f.PartyID] += sign * (f.Debit - f.Credit)
		}
	}
	return out, nil
}
