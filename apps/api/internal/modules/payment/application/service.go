package application

import (
	"context"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.PaymentClient plus payment administration.
// Outstanding figures are always computed live from orders minus active
// allocations — no stored balances to drift.
type Service struct {
	repo            *infrastructure.Repository
	parties         partycontracts.PartyClient
	purchasing      purchasingcontracts.PurchaseOrderClient
	sales           salescontracts.SalesOrderClient
	salesReturns    salesreturncontracts.ReturnsClient
	purchaseReturns purchasereturncontracts.ReturnsClient
	branches        branchcontracts.BranchClient
	media           mediacontracts.MediaClient
	audit           auditcontracts.AuditClient
}

// NewService wires payment use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	parties partycontracts.PartyClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	salesReturns salesreturncontracts.ReturnsClient,
	purchaseReturns purchasereturncontracts.ReturnsClient,
	branches branchcontracts.BranchClient,
	media mediacontracts.MediaClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, parties: parties, purchasing: purchasing,
		sales: sales, salesReturns: salesReturns, purchaseReturns: purchaseReturns,
		branches: branches, media: media, audit: audit}
}

// GetByID resolves a payment with allocations and names, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.Payment, error) {
	p, err := s.repo.GetPayment(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, nil
	}
	return s.enrich(ctx, p)
}

func (s *Service) enrich(ctx context.Context, p *contracts.Payment) (*contracts.Payment, error) {
	allocs, err := s.repo.AllocationsByPayment(ctx, p.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if allocs == nil {
		allocs = []*contracts.Allocation{}
	}
	for _, a := range allocs {
		a.OrderNumber = s.orderNumber(ctx, a.OrderType, a.OrderID)
	}
	p.Allocations = allocs
	party, err := s.parties.GetByID(ctx, p.PartyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if party != nil {
		p.PartyName = party.Name
	}
	return p, nil
}

func (s *Service) orderNumber(ctx context.Context, orderType string, orderID int64) string {
	switch orderType {
	case contracts.OrderPurchase:
		if o, err := s.purchasing.GetByID(ctx, orderID); err == nil && o != nil {
			return o.Number
		}
	case contracts.OrderSales:
		if o, err := s.sales.GetByID(ctx, orderID); err == nil && o != nil {
			return o.Number
		}
	case contracts.OrderSalesReturn:
		if r, err := s.salesReturns.GetReturn(ctx, orderID); err == nil && r != nil {
			return r.Number
		}
	case contracts.OrderPurchaseReturn:
		if r, err := s.purchaseReturns.GetReturn(ctx, orderID); err == nil && r != nil {
			return r.Number
		}
	}
	return ""
}

// mustOwnBranch loads the payment and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.Payment, error) {
	p, err := s.repo.GetPayment(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil || p.BranchID != branchID {
		return nil, apperror.NotFound("Pembayaran")
	}
	return p, nil
}

// AllocationInput is one explicit allocation line.
type AllocationInput struct {
	OrderType string
	OrderID   int64
	Amount    int64
}

// orderRef is one validated allocation target with live outstanding.
type orderRef struct {
	orderType   string
	orderID     int64
	grandTotal  int64
	outstanding int64
}

// resolveOrder validates one allocation target: exists, same branch+party,
// right lifecycle (confirmed/completed orders, confirmed returns), with live
// outstanding. Return types resolve their party through the parent order.
func (s *Service) resolveOrder(ctx context.Context, branchID, partyID int64, orderType string, orderID int64) (*orderRef, error) {
	var grand int64
	var status string
	var orderParty int64
	var orderBranch int64
	switch orderType {
	case contracts.OrderPurchase:
		o, err := s.purchasing.GetByID(ctx, orderID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if o == nil {
			return nil, apperror.NotFound("Order")
		}
		grand, status, orderParty, orderBranch = o.GrandTotal, o.Status, o.PartyID, o.BranchID
	case contracts.OrderSales:
		o, err := s.sales.GetByID(ctx, orderID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if o == nil {
			return nil, apperror.NotFound("Order")
		}
		grand, status, orderParty, orderBranch = o.GrandTotal, o.Status, o.PartyID, o.BranchID
	case contracts.OrderSalesReturn:
		r, err := s.salesReturns.GetReturn(ctx, orderID)
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
		grand, status, orderParty, orderBranch = r.Total, r.Status, so.PartyID, r.Branch
	case contracts.OrderPurchaseReturn:
		r, err := s.purchaseReturns.GetReturn(ctx, orderID)
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
		grand, status, orderParty, orderBranch = r.Total, r.Status, po.PartyID, r.Branch
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderType", Message: "tipe order tidak dikenal"}})
	}
	if orderBranch != branchID || orderParty != partyID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderId", Message: "order bukan milik pihak/cabang ini"}})
	}
	if status == purchasingcontracts.StatusCancelled || status == salescontracts.StatusCancelled {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderId", Message: "order sudah dibatalkan"}})
	}
	if status != purchasingcontracts.StatusConfirmed && status != purchasingcontracts.StatusCompleted {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderId", Message: "order harus dikonfirmasi dulu"}})
	}
	paid, err := s.repo.PaidForOrder(ctx, orderType, orderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &orderRef{orderType: orderType, orderID: orderID,
		grandTotal: grand, outstanding: grand - paid}, nil
}

// Create records a payment. Empty allocations auto-fill FIFO (oldest
// outstanding first); explicit allocations are validated line by line.
// Overpay per order is rejected; the allocation sum must equal the amount.
// Only confirmed/completed orders take money — drafts are not commitments.
//
// Serialized per party (doclock): two simultaneous payments cannot both read
// the same outstanding figure and jointly overpay it.
func (s *Service) Create(ctx context.Context, actorID, branchID, partyID int64, method, paidAt, notes string, amount int64, allocs []AllocationInput) (*contracts.Payment, error) {
	var out *contracts.Payment
	err := doclock.Lock("party:"+strconv.FormatInt(partyID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, partyID, method, paidAt, notes, amount, allocs)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.create", Entity: "payment", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, partyID int64, method, paidAt, notes string, amount int64, allocs []AllocationInput) (*contracts.Payment, error) {
	var party *partycontracts.Party
	if partyID == 0 {
		// Walk-in cash (POS tanpa customer): no master record exists, so a
		// synthetic customer stands in. Explicit allocations are mandatory
		// below — FIFO must never sweep unrelated walk-in orders.
		party = &partycontracts.Party{ID: 0, Type: partycontracts.PartyCustomer}
	} else {
		var err error
		party, err = s.parties.GetByID(ctx, partyID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if party == nil {
			return nil, apperror.NotFound("Pihak")
		}
		if party.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "pihak sudah diarsipkan"}})
		}
	}
	var direction, orderType string
	var allowedTypes map[string]bool
	switch party.Type {
	case partycontracts.PartyCustomer:
		direction, orderType = contracts.DirectionIn, contracts.OrderSales
		allowedTypes = map[string]bool{contracts.OrderSales: true, contracts.OrderSalesReturn: true}
	case partycontracts.PartySupplier:
		direction, orderType = contracts.DirectionOut, contracts.OrderPurchase
		allowedTypes = map[string]bool{contracts.OrderPurchase: true, contracts.OrderPurchaseReturn: true}
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "tipe pihak tidak dikenal"}})
	}
	if amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	if method != "cash" && method != "transfer" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "method", Message: "harus cash atau transfer"}})
	}
	if strings.TrimSpace(paidAt) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(paidAt)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "paidAt", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	} else {
		paidAt = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	if len(allocs) == 0 {
		if partyID == 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "walk-in wajib alokasi eksplisit"}})
		}
		allocs = s.fifo(ctx, branchID, partyID, orderType, amount)
		if len(allocs) == 0 {
			if s.hasOpenBills(ctx, branchID, partyID, orderType) {
				return nil, apperror.Conflict("Nominal melebihi total tagihan terbuka")
			}
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "tidak ada tagihan terbuka untuk dialokasi"}})
		}
	}
	var total int64
	seen := map[string]bool{}
	stored := make([]*contracts.Allocation, 0, len(allocs))
	paymentDir := direction
	for i, a := range allocs {
		tag := "alokasi baris " + strconv.Itoa(i+1) + ": "
		if a.OrderType == "" {
			a.OrderType = orderType
		}
		if !allowedTypes[a.OrderType] {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "tipe order tidak sesuai pihak"}})
		}
		// One payment, one money direction: normal bills flow with the
		// party default, refunds flow back. Mixing both in one payment is
		// the settle-credit endpoint's job, not this one's.
		wantDir := direction
		if isReturnType(a.OrderType) {
			wantDir = opposite(direction)
		}
		if i == 0 {
			paymentDir = wantDir
		} else if wantDir != paymentDir {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "satu pembayaran satu arah (pisahkan tagihan dan refund)"}})
		}
		if a.Amount <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "nominal harus lebih dari 0"}})
		}
		key := a.OrderType + "/" + strconv.FormatInt(a.OrderID, 10)
		if seen[key] {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "order ganda"}})
		}
		seen[key] = true
		ref, err := s.resolveOrder(ctx, branchID, partyID, a.OrderType, a.OrderID)
		if err != nil {
			return nil, err
		}
		if a.Amount > ref.outstanding {
			return nil, apperror.Conflict("Melebihi sisa tagihan (" + tag + "sisa " + strconv.FormatInt(ref.outstanding, 10) + ")")
		}
		total += a.Amount
		stored = append(stored, &contracts.Allocation{OrderType: a.OrderType, OrderID: a.OrderID, Amount: a.Amount})
	}
	if total != amount {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus sama dengan total alokasi"}})
	}
	id, err := s.insertPayment(ctx, actorID, branchID, party, paymentDir, amount, method, strings.TrimSpace(paidAt), notes, stored)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// insertPayment numbers and stores one payment row. Callers own validation;
// method "offset" marks non-cash credit legs (settle-credit only).
func (s *Service) insertPayment(ctx context.Context, actorID, branchID int64, party *partycontracts.Party, direction string, amount int64, method, paidAt, notes string, stored []*contracts.Allocation) (int64, error) {
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocPayment)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	id, err := s.repo.CreatePayment(ctx, &contracts.Payment{
		Number: number, BranchID: branchID, PartyID: party.ID, PartyType: party.Type,
		Direction: direction, Amount: amount, Method: method,
		PaidAt: paidAt, Notes: notes, Allocations: stored,
	}, actorID)
	if err != nil {
		return 0, dberr.Map(err)
	}
	return id, nil
}

func isReturnType(t string) bool {
	return t == contracts.OrderSalesReturn || t == contracts.OrderPurchaseReturn
}

func opposite(d string) string {
	if d == contracts.DirectionIn {
		return contracts.DirectionOut
	}
	return contracts.DirectionIn
}

// fifo fills amount across the party's oldest outstanding orders first.
// It returns nil when nothing is open (caller reports the empty case).
// Paid totals batch in one query; a batch failure fails closed (nil →
// caller reports overpay) rather than silently skipping rows.
func (s *Service) fifo(ctx context.Context, branchID, partyID int64, orderType string, amount int64) []AllocationInput {
	type row struct {
		id, outstanding int64
	}
	var rows []row
	if orderType == contracts.OrderPurchase {
		orders, err := s.purchasing.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil
		}
		type cand struct {
			id, total int64
		}
		var cands []cand
		for i := len(orders) - 1; i >= 0; i-- {
			o := orders[i]
			if o.Status == purchasingcontracts.StatusCancelled || o.Status == purchasingcontracts.StatusDraft {
				continue
			}
			cands = append(cands, cand{id: o.ID, total: o.GrandTotal})
		}
		ids := make([]int64, 0, len(cands))
		for _, c := range cands {
			ids = append(ids, c.id)
		}
		paidByOrder, err := s.repo.PaidForOrders(ctx, contracts.OrderPurchase, ids)
		if err != nil {
			return nil
		}
		for _, c := range cands {
			if outstanding := c.total - paidByOrder[c.id]; outstanding > 0 {
				rows = append(rows, row{id: c.id, outstanding: outstanding})
			}
		}
	} else {
		orders, err := s.sales.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil
		}
		type cand struct {
			id, total int64
		}
		var cands []cand
		for i := len(orders) - 1; i >= 0; i-- {
			o := orders[i]
			if o.Status == salescontracts.StatusCancelled || o.Status == salescontracts.StatusDraft {
				continue
			}
			cands = append(cands, cand{id: o.ID, total: o.GrandTotal})
		}
		ids := make([]int64, 0, len(cands))
		for _, c := range cands {
			ids = append(ids, c.id)
		}
		paidByOrder, err := s.repo.PaidForOrders(ctx, contracts.OrderSales, ids)
		if err != nil {
			return nil
		}
		for _, c := range cands {
			if outstanding := c.total - paidByOrder[c.id]; outstanding > 0 {
				rows = append(rows, row{id: c.id, outstanding: outstanding})
			}
		}
	}
	var out []AllocationInput
	remaining := amount
	for _, r := range rows {
		if remaining <= 0 {
			break
		}
		take := r.outstanding
		if take > remaining {
			take = remaining
		}
		out = append(out, AllocationInput{OrderType: orderType, OrderID: r.id, Amount: take})
		remaining -= take
	}
	if remaining > 0 {
		return nil // amount exceeds total outstanding — caller reports overpay
	}
	return out
}

// hasOpenBills reports whether the party has any payable/receivable left.
// One batched totals query instead of one per order.
func (s *Service) hasOpenBills(ctx context.Context, branchID, partyID int64, orderType string) bool {
	type cand struct {
		id, total int64
	}
	var cands []cand
	if orderType == contracts.OrderPurchase {
		orders, err := s.purchasing.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return false
		}
		for _, o := range orders {
			if o.Status == purchasingcontracts.StatusCancelled || o.Status == purchasingcontracts.StatusDraft {
				continue
			}
			cands = append(cands, cand{id: o.ID, total: o.GrandTotal})
		}
	} else {
		orders, err := s.sales.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return false
		}
		for _, o := range orders {
			if o.Status == salescontracts.StatusCancelled || o.Status == salescontracts.StatusDraft {
				continue
			}
			cands = append(cands, cand{id: o.ID, total: o.GrandTotal})
		}
	}
	ids := make([]int64, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.id)
	}
	paid, err := s.repo.PaidForOrders(ctx, orderType, ids)
	if err != nil {
		return false
	}
	for _, c := range cands {
		if c.total-paid[c.id] > 0 {
			return true
		}
	}
	return false
}

// Cancel voids a payment. Outstanding figures recompute live, so allocated
// orders automatically reopen — no manual reversal rows needed.
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.Payment, error) {
	p, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if p.Status != "active" {
		return nil, apperror.Conflict("Pembayaran sudah dibatalkan")
	}
	if err := s.repo.SetStatus(ctx, id, "cancelled", actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.cancel", Entity: "payment", EntityID: id, BranchID: branchID, ActorID: actorID, Note: p.Number})
	return s.GetByID(ctx, id)
}

// SettleCredit offsets a confirmed return against an open bill with no cash
// moving: two contra legs (refund leg + bill leg) that net to zero. Both legs
// carry method "offset" so cash reports never mistake them for real money.
//
// Serialized per party with the rest of money movement. If the second leg
// fails, the first is cancelled automatically; if that recovery fails too,
// the error names the stranded leg for manual void.
func (s *Service) SettleCredit(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes string) (*contracts.Payment, error) {
	var out *contracts.Payment
	err := doclock.Lock("party:"+strconv.FormatInt(partyID, 10), func() error {
		res, err := s.settleCreditLocked(ctx, actorID, branchID, partyID, returnType, returnID, orderType, orderID, amount, notes)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.settle_credit", Entity: "payment", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) settleCreditLocked(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes string) (*contracts.Payment, error) {
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
	// Compatible pairs only: a customer offsets sales returns against sales
	// bills; a supplier offsets purchase returns against purchase bills.
	var refundDir, billDir string
	switch {
	case party.Type == partycontracts.PartyCustomer &&
		returnType == contracts.OrderSalesReturn && orderType == contracts.OrderSales:
		refundDir, billDir = contracts.DirectionOut, contracts.DirectionIn
	case party.Type == partycontracts.PartySupplier &&
		returnType == contracts.OrderPurchaseReturn && orderType == contracts.OrderPurchase:
		refundDir, billDir = contracts.DirectionIn, contracts.DirectionOut
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnType", Message: "pasangan retur-tagihan tidak sesuai pihak"}})
	}
	if amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	retRef, err := s.resolveOrder(ctx, branchID, partyID, returnType, returnID)
	if err != nil {
		return nil, err
	}
	billRef, err := s.resolveOrder(ctx, branchID, partyID, orderType, orderID)
	if err != nil {
		return nil, err
	}
	if amount > retRef.outstanding {
		return nil, apperror.Conflict("Melebihi sisa retur (sisa " + strconv.FormatInt(retRef.outstanding, 10) + ")")
	}
	if amount > billRef.outstanding {
		return nil, apperror.Conflict("Melebihi sisa tagihan (sisa " + strconv.FormatInt(billRef.outstanding, 10) + ")")
	}
	paidAt := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	note := strings.TrimSpace(notes)
	if note == "" {
		note = "Offset kredit retur"
	} else {
		note = "Offset " + note
	}
	leg1, err := s.insertPayment(ctx, actorID, branchID, party, refundDir, amount, "offset", paidAt,
		note, []*contracts.Allocation{{OrderType: returnType, OrderID: returnID, Amount: amount}})
	if err != nil {
		return nil, err
	}
	leg2, err := s.insertPayment(ctx, actorID, branchID, party, billDir, amount, "offset", paidAt,
		note, []*contracts.Allocation{{OrderType: orderType, OrderID: orderID, Amount: amount}})
	if err != nil {
		if cerr := s.repo.SetStatus(ctx, leg1, "cancelled", actorID); cerr != nil {
			return nil, apperror.Internal(withSuffix(err,
				"kaki pertama "+strconv.FormatInt(leg1, 10)+" harus dibatalkan manual"))
		}
		return nil, err
	}
	return s.GetByID(ctx, leg2)
}

func withSuffix(err error, suffix string) error {
	if ae, ok := err.(*apperror.AppError); ok {
		return &apperror.AppError{Code: ae.Code, Message: ae.Message + " (" + suffix + ")", Fields: ae.Fields}
	}
	return &apperror.AppError{Code: apperror.CodeInternal, Message: err.Error() + " (" + suffix + ")"}
}

// GetPartyBalance computes the live position: every non-cancelled document
// minus active allocations.
func (s *Service) GetPartyBalance(ctx context.Context, branchID, partyID int64) (*contracts.PartyBalance, error) {
	party, err := s.parties.GetByID(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if party == nil {
		return nil, apperror.NotFound("Pihak")
	}
	bal := &contracts.PartyBalance{PartyID: partyID, PartyName: party.Name,
		Orders: []contracts.OrderBalance{}, Returns: []contracts.OrderBalance{}}
	if party.Type == partycontracts.PartyCustomer {
		orders, err := s.sales.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		open := orders[:0]
		for _, o := range orders {
			if o.Status == salescontracts.StatusCancelled || o.Status == salescontracts.StatusDraft {
				continue
			}
			open = append(open, o)
		}
		ids := make([]int64, 0, len(open))
		for _, o := range open {
			ids = append(ids, o.ID)
		}
		paidByOrder, err := s.repo.PaidForOrders(ctx, contracts.OrderSales, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, o := range open {
			paid := paidByOrder[o.ID]
			bal.Orders = append(bal.Orders, contracts.OrderBalance{
				OrderType: contracts.OrderSales, OrderID: o.ID, Number: o.Number,
				GrandTotal: o.GrandTotal, Paid: paid, Outstanding: o.GrandTotal - paid,
			})
			bal.TotalBilled += o.GrandTotal
			bal.TotalPaid += paid
		}
		if err := s.appendReturnBalances(ctx, bal, branchID, true); err != nil {
			return nil, err
		}
	} else {
		orders, err := s.purchasing.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		open := orders[:0]
		for _, o := range orders {
			if o.Status == purchasingcontracts.StatusCancelled || o.Status == purchasingcontracts.StatusDraft {
				continue
			}
			open = append(open, o)
		}
		ids := make([]int64, 0, len(open))
		for _, o := range open {
			ids = append(ids, o.ID)
		}
		paidByOrder, err := s.repo.PaidForOrders(ctx, contracts.OrderPurchase, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, o := range open {
			paid := paidByOrder[o.ID]
			bal.Orders = append(bal.Orders, contracts.OrderBalance{
				OrderType: contracts.OrderPurchase, OrderID: o.ID, Number: o.Number,
				GrandTotal: o.GrandTotal, Paid: paid, Outstanding: o.GrandTotal - paid,
			})
			bal.TotalBilled += o.GrandTotal
			bal.TotalPaid += paid
		}
		if err := s.appendReturnBalances(ctx, bal, branchID, false); err != nil {
			return nil, err
		}
	}
	// Oldest first for the ledger view.
	sort.Slice(bal.Orders, func(i, j int) bool { return bal.Orders[i].OrderID < bal.Orders[j].OrderID })
	sort.Slice(bal.Returns, func(i, j int) bool { return bal.Returns[i].OrderID < bal.Returns[j].OrderID })
	bal.Outstanding = (bal.TotalBilled - bal.TotalPaid) - (bal.TotalReturned - bal.TotalRefunded)
	return bal, nil
}

// appendReturnBalances folds confirmed returns into the balance. isSales
// selects the customer (sales returns) or supplier (purchase returns) side.
// Only returns whose parent order belongs to this party count. Parent
// parties resolve in one batch read per side (A3), never one GetByID per
// return.
func (s *Service) appendReturnBalances(ctx context.Context, bal *contracts.PartyBalance, branchID int64, isSales bool) error {
	if isSales {
		returns, err := s.salesReturns.ListByBranch(ctx, branchID)
		if err != nil {
			return apperror.Internal(err)
		}
		type match struct {
			id, total int64
			number    string
		}
		confirmed := returns[:0:0]
		orderIDs := make([]int64, 0, len(returns))
		for _, r := range returns {
			if r.Status != salesreturncontracts.StatusConfirmed {
				continue
			}
			confirmed = append(confirmed, r)
			orderIDs = append(orderIDs, r.OrderID)
		}
		parties, err := s.sales.OrderParties(ctx, orderIDs)
		if err != nil {
			return apperror.Internal(err)
		}
		var matched []match
		for _, r := range confirmed {
			partyID, ok := parties[r.OrderID]
			if !ok || partyID != bal.PartyID {
				continue
			}
			matched = append(matched, match{id: r.ID, total: r.Total, number: r.Number})
		}
		ids := make([]int64, 0, len(matched))
		for _, m := range matched {
			ids = append(ids, m.id)
		}
		refundedByReturn, err := s.repo.PaidForOrders(ctx, contracts.OrderSalesReturn, ids)
		if err != nil {
			return apperror.Internal(err)
		}
		for _, m := range matched {
			refunded := refundedByReturn[m.id]
			bal.Returns = append(bal.Returns, contracts.OrderBalance{
				OrderType: contracts.OrderSalesReturn, OrderID: m.id, Number: m.number,
				GrandTotal: m.total, Paid: refunded, Outstanding: m.total - refunded,
			})
			bal.TotalReturned += m.total
			bal.TotalRefunded += refunded
		}
		return nil
	}
	returns, err := s.purchaseReturns.ListByBranch(ctx, branchID)
	if err != nil {
		return apperror.Internal(err)
	}
	type match struct {
		id, total int64
		number    string
	}
	confirmed := returns[:0:0]
	orderIDs := make([]int64, 0, len(returns))
	for _, r := range returns {
		if r.Status != purchasereturncontracts.StatusConfirmed {
			continue
		}
		confirmed = append(confirmed, r)
		orderIDs = append(orderIDs, r.OrderID)
	}
	parties, err := s.purchasing.OrderParties(ctx, orderIDs)
	if err != nil {
		return apperror.Internal(err)
	}
	var matched []match
	for _, r := range confirmed {
		partyID, ok := parties[r.OrderID]
		if !ok || partyID != bal.PartyID {
			continue
		}
		matched = append(matched, match{id: r.ID, total: r.Total, number: r.Number})
	}
	ids := make([]int64, 0, len(matched))
	for _, m := range matched {
		ids = append(ids, m.id)
	}
	refundedByReturn, err := s.repo.PaidForOrders(ctx, contracts.OrderPurchaseReturn, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, m := range matched {
		refunded := refundedByReturn[m.id]
		bal.Returns = append(bal.Returns, contracts.OrderBalance{
			OrderType: contracts.OrderPurchaseReturn, OrderID: m.id, Number: m.number,
			GrandTotal: m.total, Paid: refunded, Outstanding: m.total - refunded,
		})
		bal.TotalReturned += m.total
		bal.TotalRefunded += refunded
	}
	return nil
}

// HasActiveAllocations reports whether any active payment allocates to
// the document.
func (s *Service) HasActiveAllocations(ctx context.Context, orderType string, orderID int64) (bool, error) {
	paid, err := s.repo.PaidForOrder(ctx, orderType, orderID)
	if err != nil {
		return false, apperror.Internal(err)
	}
	return paid > 0, nil
}

// Ledger lists a party's active payments with allocations and order numbers.
func (s *Service) Ledger(ctx context.Context, branchID, partyID int64, page, limit int) ([]*contracts.Payment, int64, error) {
	total, err := s.repo.Count(ctx, branchID, partyID, "active")
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	payments, err := s.repo.LedgerByParty(ctx, branchID, partyID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	out := make([]*contracts.Payment, 0, len(payments))
	for _, p := range payments {
		full, err := s.enrich(ctx, p)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, full)
	}
	return out, total, nil
}

// Proof records a transfer-slip photo under the payment.
func (s *Service) Proof(ctx context.Context, actorID, branchID, id int64, up mediacontracts.Upload) (string, error) {
	p, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return "", err
	}
	f, err := s.media.Upload(ctx, mediacontracts.OwnerPaymentProof, p.ID, up, actorID)
	if err != nil {
		return "", err
	}
	url, err := s.media.GetURL(ctx, f)
	if err != nil {
		return "", err
	}
	return url, nil
}

// Proofs lists a payment's proof photos.
func (s *Service) Proofs(ctx context.Context, branchID, id int64) ([]map[string]any, error) {
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return nil, err
	}
	files, err := s.media.List(ctx, mediacontracts.OwnerPaymentProof, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		url, err := s.media.GetURL(ctx, f)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		out = append(out, map[string]any{
			"id": f.ID, "originalName": f.OriginalName, "mime": f.MIME,
			"sizeBytes": f.SizeBytes, "url": url,
		})
	}
	return out, nil
}

// ListResult is a paginated payment page.
type ListResult struct {
	Payments []*contracts.Payment
	Total    int64
}

// List searches payments of a branch, newest first.
func (s *Service) List(ctx context.Context, branchID, partyID int64, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, partyID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	payments, err := s.repo.List(ctx, branchID, partyID, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if payments == nil {
		payments = []*contracts.Payment{}
	}
	return &ListResult{Payments: payments, Total: total}, nil
}

// ListActive lists active payments newest-first for the derived posting
// queue. Cancelled payments never post. One bounded read; the journal index
// decides what is still unposted.
func (s *Service) ListActive(ctx context.Context, branchID int64, limit int) ([]*contracts.Payment, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	res, err := s.List(ctx, branchID, 0, "active", 1, limit)
	if err != nil {
		return nil, err
	}
	return res.Payments, nil
}

// OrderIDsByPayments maps payment id -> the sales order ids it settles.
// A payment may settle several orders, so the value is a slice.
func (s *Service) OrderIDsByPayments(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	out, err := s.repo.OrderIDsByPayments(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
