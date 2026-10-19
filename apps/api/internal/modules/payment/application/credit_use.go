package application

import (
	"context"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/payment/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Using a party's saldo (uang muka) on a new bill (N-41).
//
// A confirmed return can leave value as the party's balance (settlement
// customer_credit / supplier_credit, journalled Dr Piutang / Cr Uang Muka
// Pelanggan and Dr Uang Muka Supplier / Cr Hutang, K-04a). Using that balance
// on an open bill is a pair of payment legs, written in ONE transaction and
// linked by pair_id (cancelling either cancels both):
//
//   - the RETURN leg (method "offset", allocated to the return) consumes the
//     saldo; like every offset it never reaches the books;
//   - the BILL leg (method "advance", allocated to the bill) pays the bill. It
//     is the journalled leg: Dr Uang Muka Pelanggan / Cr Piutang on the sales
//     side, Dr Hutang / Cr Uang Muka Supplier on the purchase side.
//
// Both legs carry the return as credit_source, which is the trail of the used
// amount: used(return) = Σ active allocations onto the return by legs naming it.
// What a return still holds is CreditSettled − used; a use can never exceed it
// nor the bill's open amount. Ordinary refunds/offsets of the return only see
// what is NOT kept as saldo (resolveOrder), so one rupiah is never spent twice.

// creditHolder is a validated saldo source: its party, number and amounts.
type creditHolder struct {
	number   string
	credited int64 // part of the return kept as saldo (settled as credit)
	used     int64 // part of the saldo already spent on bills
}

func (h creditHolder) remaining() int64 { return h.credited - h.used }

// holder resolves one return as a saldo source for a party in a branch: the
// return must be confirmed and belong to that party (through its parent order).
func (s *Service) holder(ctx context.Context, branchID, partyID int64, returnType string, returnID int64) (*creditHolder, error) {
	var number, status string
	var credited, orderParty, retBranch int64
	switch returnType {
	case contracts.OrderSalesReturn:
		r, err := s.salesReturns.GetReturn(ctx, returnID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if r == nil {
			return nil, apperror.NotFound("Retur")
		}
		so, err := s.sales.GetByID(ctx, r.OrderID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if so == nil {
			return nil, apperror.NotFound("Order")
		}
		number, status, credited, orderParty, retBranch = r.Number, r.Status, r.CreditSettled, so.PartyID, r.Branch
		if status != salesreturncontracts.StatusConfirmed {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnId", Message: "retur harus terkonfirmasi"}})
		}
	case contracts.OrderPurchaseReturn:
		r, err := s.purchaseReturns.GetReturn(ctx, returnID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if r == nil {
			return nil, apperror.NotFound("Retur")
		}
		po, err := s.purchasing.GetByID(ctx, r.OrderID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if po == nil {
			return nil, apperror.NotFound("Order")
		}
		number, status, credited, orderParty, retBranch = r.Number, r.Status, r.CreditSettled, po.PartyID, r.Branch
		if status != purchasereturncontracts.StatusConfirmed {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnId", Message: "retur harus terkonfirmasi"}})
		}
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnType", Message: "tipe retur tidak dikenal"}})
	}
	if retBranch != branchID || orderParty != partyID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnId", Message: "retur bukan milik pihak/cabang ini"}})
	}
	used, err := s.repo.CreditUsedForReturn(ctx, returnType, returnID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &creditHolder{number: number, credited: credited, used: used}, nil
}

// UseCredit spends part of a return's saldo on an open bill of the same party.
func (s *Service) UseCredit(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes string) (*contracts.Payment, error) {
	var out *contracts.Payment
	err := doclock.Lock("party:"+strconv.FormatInt(partyID, 10), func() error {
		res, err := s.useCreditLocked(ctx, actorID, branchID, partyID, returnType, returnID, orderType, orderID, amount, notes)
		out = res
		return err
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.use_credit", Entity: "payment", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) useCreditLocked(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes string) (*contracts.Payment, error) {
	party, err := s.parties.GetByID(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if party == nil {
		return nil, apperror.NotFound("Pihak")
	}
	if party.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "pihak sudah diarsipkan"}})
	}
	// Compatible pairs only, exactly like SettleCredit: a customer spends the
	// saldo of a sales return on a sales bill, a supplier's saldo (our advance
	// with them) on a purchase bill.
	var returnDir, billDir string
	switch {
	case party.Type == partycontracts.PartyCustomer &&
		returnType == contracts.OrderSalesReturn && orderType == contracts.OrderSales:
		returnDir, billDir = contracts.DirectionOut, contracts.DirectionIn
	case party.Type == partycontracts.PartySupplier &&
		returnType == contracts.OrderPurchaseReturn && orderType == contracts.OrderPurchase:
		returnDir, billDir = contracts.DirectionIn, contracts.DirectionOut
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnType", Message: "pasangan retur-tagihan tidak sesuai pihak"}})
	}
	if amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	if len(strings.TrimSpace(notes)) > 255 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "notes", Message: "maksimal 255 karakter"}})
	}
	h, err := s.holder(ctx, branchID, partyID, returnType, returnID)
	if err != nil {
		return nil, err
	}
	if amount > h.remaining() {
		return nil, apperror.Conflict("Melebihi saldo uang muka retur " + h.number + " (sisa " + strconv.FormatInt(h.remaining(), 10) + ")")
	}
	bill, err := s.resolveOrder(ctx, branchID, partyID, orderType, orderID)
	if err != nil {
		return nil, err
	}
	if amount > bill.outstanding {
		return nil, apperror.Conflict("Melebihi sisa tagihan (sisa " + strconv.FormatInt(bill.outstanding, 10) + ")")
	}
	paidAt := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	note := "Pakai saldo uang muka retur " + h.number
	if extra := strings.TrimSpace(notes); extra != "" {
		note += ": " + extra
	}
	returnLeg, err := s.numberedPayment(ctx, branchID, party, returnDir, amount, contracts.MethodOffset, paidAt,
		note, []*contracts.Allocation{{OrderType: returnType, OrderID: returnID, Amount: amount}}, createOptions{})
	if err != nil {
		return nil, err
	}
	billLeg, err := s.numberedPayment(ctx, branchID, party, billDir, amount, contracts.MethodAdvance, paidAt,
		note, []*contracts.Allocation{{OrderType: orderType, OrderID: orderID, Amount: amount}}, createOptions{})
	if err != nil {
		return nil, err
	}
	returnLeg.CreditSourceType, returnLeg.CreditSourceID = returnType, returnID
	billLeg.CreditSourceType, billLeg.CreditSourceID = returnType, returnID
	_, billID, err := s.repo.CreatePaymentPair(ctx, returnLeg, billLeg, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, billID)
}

// CreditBalance is one return that still holds (or held) saldo for a party.
type CreditBalance struct {
	ReturnType string `json:"returnType"`
	ReturnID   int64  `json:"returnId"`
	Number     string `json:"number"`
	Credited   int64  `json:"credited"`
	Used       int64  `json:"used"`
	Remaining  int64  `json:"remaining"`
}

// CreditBalances lists the party's returns with saldo (credited > 0), oldest
// first, with what was used and what remains — in grouped reads only: the
// party's returns (one batch), their credited sums (one batch) and used sums
// (one batch), never one query per return.
func (s *Service) CreditBalances(ctx context.Context, branchID, partyID int64) ([]CreditBalance, error) {
	party, err := s.parties.GetByID(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if party == nil {
		return nil, apperror.NotFound("Pihak")
	}
	type row struct {
		id       int64
		number   string
		credited int64
	}
	var rows []row
	var returnType string
	if party.Type == partycontracts.PartyCustomer {
		returnType = contracts.OrderSalesReturn
		orders, err := s.sales.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		ids := make([]int64, 0, len(orders))
		for _, o := range orders {
			ids = append(ids, o.ID)
		}
		rets, err := s.salesReturns.ConfirmedByOrders(ctx, branchID, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		var confirmed []*salesreturncontracts.Return
		retIDs := make([]int64, 0, len(rets))
		for _, r := range rets {
			if r.Status == salesreturncontracts.StatusConfirmed {
				confirmed = append(confirmed, r)
				retIDs = append(retIDs, r.ID)
			}
		}
		credited, err := s.salesReturns.CreditSettledByReturns(ctx, retIDs)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range confirmed {
			if credited[r.ID] > 0 {
				rows = append(rows, row{r.ID, r.Number, credited[r.ID]})
			}
		}
	} else {
		returnType = contracts.OrderPurchaseReturn
		orders, err := s.purchasing.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		ids := make([]int64, 0, len(orders))
		for _, o := range orders {
			ids = append(ids, o.ID)
		}
		rets, err := s.purchaseReturns.ConfirmedByOrders(ctx, branchID, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		var confirmed []*purchasereturncontracts.Return
		retIDs := make([]int64, 0, len(rets))
		for _, r := range rets {
			if r.Status == purchasereturncontracts.StatusConfirmed {
				confirmed = append(confirmed, r)
				retIDs = append(retIDs, r.ID)
			}
		}
		credited, err := s.purchaseReturns.CreditSettledByReturns(ctx, retIDs)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range confirmed {
			if credited[r.ID] > 0 {
				rows = append(rows, row{r.ID, r.Number, credited[r.ID]})
			}
		}
	}
	retIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		retIDs = append(retIDs, r.id)
	}
	used, err := s.repo.CreditUsedForReturns(ctx, returnType, retIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]CreditBalance, 0, len(rows))
	for _, r := range rows {
		out = append(out, CreditBalance{
			ReturnType: returnType, ReturnID: r.id, Number: r.number,
			Credited: r.credited, Used: used[r.id], Remaining: r.credited - used[r.id],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReturnID < out[j].ReturnID })
	return out, nil
}
