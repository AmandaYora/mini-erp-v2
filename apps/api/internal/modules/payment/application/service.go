package application

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
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
	// reverser reverses the journal of a cancelled payment (set after finance
	// is built; nil = not wired, cancellation still succeeds).
	reverser financecontracts.SourceReverser
}

// SetReverser wires finance's source reverser (composition-root privilege).
func (s *Service) SetReverser(r financecontracts.SourceReverser) { s.reverser = r }

// reverseJournals reverses the books for cancelled payment legs. A failure never
// undoes the cancellation: it is audited and surfaces as an orphan the
// readiness gate reports.
func (s *Service) reverseJournals(ctx context.Context, actorID, branchID int64, ids []int64, number string) {
	if s.reverser == nil {
		return
	}
	for _, id := range ids {
		if _, err := s.reverser.ReverseForSource(ctx, actorID, branchID, "payment", id, "Pembayaran "+number+" dibatalkan"); err != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "payment", EntityID: id, BranchID: branchID, ActorID: actorID, Note: err.Error()})
		}
	}
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
	// charge: a return that the customer owes us (exchange costing more), so
	// its money flows in the normal direction, not as a refund.
	charge bool
}

// resolveOrder validates one allocation target: exists, same branch+party,
// right lifecycle (confirmed/completed orders, confirmed returns), with live
// outstanding. Return types resolve their party through the parent order.
func (s *Service) resolveOrder(ctx context.Context, branchID, partyID int64, orderType string, orderID int64) (*orderRef, error) {
	var grand int64
	var status string
	var orderParty int64
	var orderBranch int64
	var charge bool
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
		// N-40: what was kept as customer balance is no longer refundable
		// nor offsettable (CreditSettled is 0 for a charge-kind return).
		grand, status, orderParty, orderBranch = r.Total-r.CreditSettled, r.Status, so.PartyID, r.Branch
		charge = r.Kind == salesreturncontracts.KindCharge
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
		// N-40: what was kept as supplier balance is no longer refundable
		// nor offsettable.
		grand, status, orderParty, orderBranch = r.Total-r.CreditSettled, r.Status, po.PartyID, r.Branch
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
	if isReturnType(orderType) {
		// N-41: the saldo spent on bills allocates onto the return too, but it is
		// already out of `grand` (Total - CreditSettled); count only what was
		// refunded or offset the ordinary way.
		used, err := s.repo.CreditUsedForReturn(ctx, orderType, orderID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		paid -= used
	}
	return &orderRef{orderType: orderType, orderID: orderID,
		grandTotal: grand, outstanding: grand - paid, charge: charge}, nil
}

// CreateOption sets optional payment data.
type CreateOption func(*createOptions)

type createOptions struct {
	clientRef      string
	amountTendered int64
	reference      string
}

// WithClientRef makes Create idempotent: the same reference returns the
// payment it already created instead of recording a second one.
func WithClientRef(ref string) CreateOption {
	return func(o *createOptions) { o.clientRef = strings.TrimSpace(ref) }
}

// WithAmountTendered records the cash handed over (cash payments only).
func WithAmountTendered(amount int64) CreateOption {
	return func(o *createOptions) { o.amountTendered = amount }
}

// WithReference records the transfer/check reference number.
func WithReference(ref string) CreateOption {
	return func(o *createOptions) { o.reference = strings.TrimSpace(ref) }
}

// lockKey picks the serialization key for money movement: the party, or — for
// walk-in payments with no party — the first allocated order, so unrelated
// walk-in checkouts never queue behind one shared "party 0" lock.
func lockKey(partyID int64, allocs []AllocationInput) string {
	if partyID != 0 || len(allocs) == 0 {
		return "party:" + strconv.FormatInt(partyID, 10)
	}
	t := allocs[0].OrderType
	if t == "" {
		t = contracts.OrderSales
	}
	return "wk:" + t + "/" + strconv.FormatInt(allocs[0].OrderID, 10)
}

// Create records a payment. Empty allocations auto-fill FIFO (oldest due
// first); explicit allocations are validated line by line. Overpay per order
// is rejected; the allocation sum must equal the amount. Only
// confirmed/completed orders take money — drafts are not commitments.
//
// Serialized per party (doclock): two simultaneous payments cannot both read
// the same outstanding figure and jointly overpay it. With a client reference
// the call is idempotent.
func (s *Service) Create(ctx context.Context, actorID, branchID, partyID int64, method, paidAt, notes string, amount int64, allocs []AllocationInput, opts ...CreateOption) (*contracts.Payment, error) {
	var o createOptions
	for _, opt := range opts {
		opt(&o)
	}
	out, _, err := s.createPayment(ctx, actorID, branchID, partyID, method, paidAt, notes, amount, allocs, o)
	return out, err
}

func (s *Service) createPayment(ctx context.Context, actorID, branchID, partyID int64, method, paidAt, notes string, amount int64, allocs []AllocationInput, o createOptions) (*contracts.Payment, bool, error) {
	var out *contracts.Payment
	var replayed bool
	err := doclock.Lock(lockKey(partyID, allocs), func() error {
		res, rep, err := s.createLocked(ctx, actorID, branchID, partyID, method, paidAt, notes, amount, allocs, o)
		if err != nil {
			return err
		}
		out, replayed = res, rep
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return out, replayed, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, partyID int64, method, paidAt, notes string, amount int64, allocs []AllocationInput, o createOptions) (*contracts.Payment, bool, error) {
	if len(o.clientRef) > 64 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "maksimal 64 karakter"}})
	}
	if o.clientRef != "" {
		existing, err := s.repo.PaymentByClientRef(ctx, branchID, o.clientRef)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if existing != nil {
			if existing.PartyID != partyID || existing.Amount != amount {
				return nil, false, apperror.Conflict("Referensi klien sudah dipakai untuk pembayaran lain")
			}
			out, err := s.enrich(ctx, existing)
			return out, true, err
		}
	}
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
			return nil, false, apperror.Internal(err)
		}
		if party == nil {
			return nil, false, apperror.NotFound("Pihak")
		}
		// An archived party can still settle what it owes (legacy BR-05):
		// archiving stops NEW orders, it must not strand an open receivable.
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
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "tipe pihak tidak dikenal"}})
	}
	if amount <= 0 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	if method != contracts.MethodCash && method != contracts.MethodTransfer && method != contracts.MethodCheck {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "method", Message: "harus cash, transfer, atau check"}})
	}
	if len(o.reference) > 100 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "reference", Message: "maksimal 100 karakter"}})
	}
	if o.amountTendered != 0 {
		if method != contracts.MethodCash {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amountTendered", Message: "hanya untuk pembayaran tunai"}})
		}
		if o.amountTendered < amount {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amountTendered", Message: "uang diterima kurang dari nominal pembayaran"}})
		}
	}
	if strings.TrimSpace(paidAt) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(paidAt)); err != nil {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "paidAt", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	} else {
		paidAt = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	if len(allocs) == 0 {
		if partyID == 0 {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "walk-in wajib alokasi eksplisit"}})
		}
		allocs = s.fifo(ctx, branchID, partyID, orderType, amount)
		if len(allocs) == 0 {
			if s.hasOpenBills(ctx, branchID, partyID, orderType) {
				return nil, false, apperror.Conflict("Nominal melebihi total tagihan terbuka")
			}
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "tidak ada tagihan terbuka untuk dialokasi"}})
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
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "tipe order tidak sesuai pihak"}})
		}
		// One payment, one money direction: normal bills flow with the
		// party default, refunds flow back. Mixing both in one payment is
		// the settle-credit endpoint's job, not this one's.
		ref, err := s.resolveOrder(ctx, branchID, partyID, a.OrderType, a.OrderID)
		if err != nil {
			return nil, false, err
		}
		wantDir := direction
		if isReturnType(a.OrderType) && !ref.charge {
			wantDir = opposite(direction)
		}
		if i == 0 {
			paymentDir = wantDir
		} else if wantDir != paymentDir {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: "satu pembayaran satu arah (pisahkan tagihan dan refund)"}})
		}
		if a.Amount <= 0 {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "nominal harus lebih dari 0"}})
		}
		key := a.OrderType + "/" + strconv.FormatInt(a.OrderID, 10)
		if seen[key] {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "allocations", Message: tag + "order ganda"}})
		}
		seen[key] = true
		if a.Amount > ref.outstanding {
			return nil, false, apperror.Conflict("Melebihi sisa tagihan (" + tag + "sisa " + strconv.FormatInt(ref.outstanding, 10) + ")")
		}
		total += a.Amount
		stored = append(stored, &contracts.Allocation{OrderType: a.OrderType, OrderID: a.OrderID, Amount: a.Amount})
	}
	if total != amount {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus sama dengan total alokasi"}})
	}
	id, err := s.insertPayment(ctx, actorID, branchID, party, paymentDir, amount, method, strings.TrimSpace(paidAt), notes, stored, o)
	if err != nil {
		return nil, false, err
	}
	out, err := s.GetByID(ctx, id)
	return out, false, err
}

// numberedPayment builds one payment row with its allocated number. Callers
// own validation; method "offset" marks non-cash credit legs (settle-credit).
func (s *Service) numberedPayment(ctx context.Context, branchID int64, party *partycontracts.Party, direction string, amount int64, method, paidAt, notes string, stored []*contracts.Allocation, o createOptions) (*contracts.Payment, error) {
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocPayment)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &contracts.Payment{
		Number: number, BranchID: branchID, PartyID: party.ID, PartyType: party.Type,
		Direction: direction, Amount: amount, Method: method,
		PaidAt: paidAt, Notes: notes, Allocations: stored,
		ClientRef: o.clientRef, AmountTendered: o.amountTendered, Reference: o.reference,
	}, nil
}

func (s *Service) insertPayment(ctx context.Context, actorID, branchID int64, party *partycontracts.Party, direction string, amount int64, method, paidAt, notes string, stored []*contracts.Allocation, o createOptions) (int64, error) {
	pay, err := s.numberedPayment(ctx, branchID, party, direction, amount, method, paidAt, notes, stored, o)
	if err != nil {
		return 0, err
	}
	// The trail co-commits with the payment (ADR-0011): an audit failure
	// rolls the payment back instead of leaving money untrailed.
	id, err := s.repo.CreatePayment(ctx, pay, actorID, func(ctx context.Context, tx *sql.Tx, id int64) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "payments.create", Entity: "payment", EntityID: id, BranchID: branchID, ActorID: actorID,
			Note: pay.Number,
			After: map[string]any{
				"direction": direction, "method": method, "amount": amount,
				"partyId": party.ID, "allocations": stored,
			},
		})
	})
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

// fifoCand is one open bill considered by FIFO allocation.
type fifoCand struct {
	id        int64
	total     int64
	orderDate string
	dueDate   string
}

// due is the date a bill is ranked by: its due date when it has one (net
// terms), else the order date.
func (c fifoCand) due() string {
	if c.dueDate != "" {
		return c.dueDate
	}
	return c.orderDate
}

// sortFIFO orders open bills oldest-due first; ties fall back to the order
// date, then the id (creation order). Pure and unit-tested.
func sortFIFO(c []fifoCand) {
	sort.SliceStable(c, func(i, j int) bool {
		if c[i].due() != c[j].due() {
			return c[i].due() < c[j].due()
		}
		if c[i].orderDate != c[j].orderDate {
			return c[i].orderDate < c[j].orderDate
		}
		return c[i].id < c[j].id
	})
}

// fifo fills amount across the party's open bills, oldest due first.
// It returns nil when nothing is open (caller reports the empty case).
// Paid totals batch in one query; a batch failure fails closed (nil →
// caller reports overpay) rather than silently skipping rows.
func (s *Service) fifo(ctx context.Context, branchID, partyID int64, orderType string, amount int64) []AllocationInput {
	var cands []fifoCand
	if orderType == contracts.OrderPurchase {
		orders, err := s.purchasing.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil
		}
		for _, o := range orders {
			if o.Status == purchasingcontracts.StatusCancelled || o.Status == purchasingcontracts.StatusDraft {
				continue
			}
			cands = append(cands, fifoCand{id: o.ID, total: o.GrandTotal, orderDate: o.OrderDate, dueDate: o.DueDate})
		}
	} else {
		orders, err := s.sales.ListByParty(ctx, branchID, partyID)
		if err != nil {
			return nil
		}
		for _, o := range orders {
			if o.Status == salescontracts.StatusCancelled || o.Status == salescontracts.StatusDraft {
				continue
			}
			cands = append(cands, fifoCand{id: o.ID, total: o.GrandTotal, orderDate: o.OrderDate, dueDate: o.DueDate})
		}
	}
	ids := make([]int64, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.id)
	}
	paidByOrder, err := s.repo.PaidForOrders(ctx, orderType, ids)
	if err != nil {
		return nil
	}
	open := cands[:0]
	for _, c := range cands {
		if c.total-paidByOrder[c.id] > 0 {
			open = append(open, c)
		}
	}
	sortFIFO(open)
	var out []AllocationInput
	remaining := amount
	for _, c := range open {
		if remaining <= 0 {
			break
		}
		take := c.total - paidByOrder[c.id]
		if take > remaining {
			take = remaining
		}
		out = append(out, AllocationInput{OrderType: orderType, OrderID: c.id, Amount: take})
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

// Cancel voids a payment together with its paired leg (a credit offset is
// two legs that stand or fall together). Outstanding figures recompute live,
// so allocated orders automatically reopen — no manual reversal rows needed.
//
// Under the party lock the status is re-read and flipped with a CAS, so two
// simultaneous cancels cancel once (the second replays). Before cancelling,
// every sales/purchase order it settles is checked: a COD/prepaid order that
// already completed must not silently become "unpaid" again (legacy rule).
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.Payment, error) {
	first, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	allocs, err := s.repo.AllocationsByPayment(ctx, first.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var key []AllocationInput
	for _, a := range allocs {
		key = append(key, AllocationInput{OrderType: a.OrderType, OrderID: a.OrderID})
	}
	var replayed bool
	var legIDs []int64
	err = doclock.Lock(lockKey(first.PartyID, key), func() error {
		p, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		if p.Status != "active" {
			replayed = true
			return nil
		}
		legs := []*contracts.Payment{p}
		legIDs = []int64{p.ID}
		if p.PairID != 0 {
			if other, err := s.repo.PairedPayment(ctx, p.PairID); err != nil {
				return apperror.Internal(err)
			} else if other != nil && other.Status == "active" {
				legs = append(legs, other)
				legIDs = append(legIDs, other.ID)
			}
		}
		if err := s.guardCancel(ctx, legs); err != nil {
			return err
		}
		n, err := s.repo.CancelWithPair(ctx, p.ID, actorID, func(ctx context.Context, tx *sql.Tx) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "payments.cancel", Entity: "payment", EntityID: id, BranchID: branchID, ActorID: actorID,
				Note:   first.Number,
				Before: map[string]any{"status": "active", "amount": p.Amount, "legs": legIDs},
				After:  map[string]any{"status": "cancelled"},
			})
		})
		if err != nil {
			return apperror.Internal(err)
		}
		if n == 0 {
			replayed = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !replayed {
		s.reverseJournals(ctx, actorID, branchID, legIDs, first.Number)
	}
	return s.GetByID(ctx, id)
}

// guardCancel refuses a cancellation that would reopen a settled COD/prepaid
// order: after removing these legs' allocations the order would owe money
// again although it already completed.
func (s *Service) guardCancel(ctx context.Context, legs []*contracts.Payment) error {
	removed := map[string]int64{} // "type/id" -> amount these legs settle
	for _, leg := range legs {
		allocs, err := s.repo.AllocationsByPayment(ctx, leg.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		for _, a := range allocs {
			if a.OrderType == contracts.OrderSales || a.OrderType == contracts.OrderPurchase {
				removed[a.OrderType+"/"+strconv.FormatInt(a.OrderID, 10)] += a.Amount
			}
		}
	}
	for key, amount := range removed {
		parts := strings.SplitN(key, "/", 2)
		orderType := parts[0]
		orderID, _ := strconv.ParseInt(parts[1], 10, 64)
		var status, terms, number string
		var grand int64
		switch orderType {
		case contracts.OrderSales:
			o, err := s.sales.GetByID(ctx, orderID)
			if err != nil {
				return apperror.Internal(err)
			}
			if o == nil {
				continue
			}
			status, terms, number, grand = o.Status, o.PaymentTerms, o.Number, o.GrandTotal
		default:
			o, err := s.purchasing.GetByID(ctx, orderID)
			if err != nil {
				return apperror.Internal(err)
			}
			if o == nil {
				continue
			}
			status, terms, number, grand = o.Status, o.PaymentTerms, o.Number, o.GrandTotal
		}
		if status != salescontracts.StatusCompleted || terms == "net" {
			continue
		}
		paid, err := s.repo.PaidForOrder(ctx, orderType, orderID)
		if err != nil {
			return apperror.Internal(err)
		}
		if grand-(paid-amount) > 0 {
			return apperror.Conflict("Order " + number + " (" + terms + ") sudah selesai; membatalkan pembayaran ini membuatnya belum lunas. Ubah termin atau buat pembayaran pengganti lebih dulu")
		}
	}
	return nil
}

// SettleCredit offsets a confirmed return against an open bill with no cash
// moving: two contra legs (refund leg + bill leg) that net to zero. Both legs
// carry method "offset" so cash reports never mistake them for real money.
//
// Serialized per party with the rest of money movement. Both legs are written
// in ONE transaction (and linked by pair_id), so a failure can never strand a
// lone leg; cancelling either leg cancels both.
func (s *Service) SettleCredit(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes string) (*contracts.Payment, error) {
	out, replayed, err := s.settleCredit(ctx, actorID, branchID, partyID, returnType, returnID, orderType, orderID, amount, notes, "")
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.settle_credit", Entity: "payment", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	}
	return out, nil
}

func (s *Service) settleCredit(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes, clientRef string) (*contracts.Payment, bool, error) {
	var out *contracts.Payment
	var replayed bool
	err := doclock.Lock("party:"+strconv.FormatInt(partyID, 10), func() error {
		res, rep, err := s.settleCreditLocked(ctx, actorID, branchID, partyID, returnType, returnID, orderType, orderID, amount, notes, clientRef)
		if err != nil {
			return err
		}
		out, replayed = res, rep
		return nil
	})
	return out, replayed, err
}

func (s *Service) settleCreditLocked(ctx context.Context, actorID, branchID, partyID int64, returnType string, returnID int64, orderType string, orderID int64, amount int64, notes, clientRef string) (*contracts.Payment, bool, error) {
	if clientRef != "" {
		// The bill leg carries the reference; finding it returns the pair.
		existing, err := s.repo.PaymentByClientRef(ctx, branchID, clientRef)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if existing != nil {
			out, err := s.enrich(ctx, existing)
			return out, true, err
		}
	}
	party, err := s.parties.GetByID(ctx, partyID)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if party == nil {
		return nil, false, apperror.NotFound("Pihak")
	}
	if party.Status != "active" {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "pihak sudah diarsipkan"}})
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
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "returnType", Message: "pasangan retur-tagihan tidak sesuai pihak"}})
	}
	if amount <= 0 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	retRef, err := s.resolveOrder(ctx, branchID, partyID, returnType, returnID)
	if err != nil {
		return nil, false, err
	}
	billRef, err := s.resolveOrder(ctx, branchID, partyID, orderType, orderID)
	if err != nil {
		return nil, false, err
	}
	if amount > retRef.outstanding {
		return nil, false, apperror.Conflict("Melebihi sisa retur (sisa " + strconv.FormatInt(retRef.outstanding, 10) + ")")
	}
	if amount > billRef.outstanding {
		return nil, false, apperror.Conflict("Melebihi sisa tagihan (sisa " + strconv.FormatInt(billRef.outstanding, 10) + ")")
	}
	paidAt := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	note := strings.TrimSpace(notes)
	if note == "" {
		note = "Offset kredit retur"
	} else {
		note = "Offset " + note
	}
	refundLeg, err := s.numberedPayment(ctx, branchID, party, refundDir, amount, contracts.MethodOffset, paidAt,
		note, []*contracts.Allocation{{OrderType: returnType, OrderID: returnID, Amount: amount}}, createOptions{})
	if err != nil {
		return nil, false, err
	}
	billLeg, err := s.numberedPayment(ctx, branchID, party, billDir, amount, contracts.MethodOffset, paidAt,
		note, []*contracts.Allocation{{OrderType: orderType, OrderID: orderID, Amount: amount}}, createOptions{clientRef: clientRef})
	if err != nil {
		return nil, false, err
	}
	_, billID, err := s.repo.CreatePaymentPair(ctx, refundLeg, billLeg, actorID)
	if err != nil {
		// A racing twin with the same client reference already wrote the pair
		// (the process lock does not cover other instances): replay it.
		if clientRef != "" {
			if existing, lerr := s.repo.PaymentByClientRef(ctx, branchID, clientRef); lerr == nil && existing != nil {
				out, eerr := s.enrich(ctx, existing)
				return out, true, eerr
			}
		}
		return nil, false, dberr.Map(err)
	}
	out, err := s.GetByID(ctx, billID)
	return out, false, err
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
		returns, err := s.salesReturns.ConfirmedByOrders(ctx, branchID, orderIDsOf(bal, contracts.OrderSales))
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
	returns, err := s.purchaseReturns.ConfirmedByOrders(ctx, branchID, orderIDsOf(bal, contracts.OrderPurchase))
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
	out, err := s.enrichBatch(ctx, payments)
	if err != nil {
		return nil, 0, err
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

// CancelledAmong returns which of ids are cancelled.
func (s *Service) CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	out, err := s.repo.CancelledAmong(ctx, branchID, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ListActiveAfter pages active payments by ascending id after a cursor.
func (s *Service) ListActiveAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.Payment, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	ps, err := s.repo.ActiveAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ps, nil
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

// OffsetReturn offsets a confirmed return against an open bill (see
// contracts.OffsetInput). The bill leg carries ClientRef, so a repeat returns
// the legs already written.
func (s *Service) OffsetReturn(ctx context.Context, in contracts.OffsetInput) (*contracts.Payment, bool, error) {
	out, replayed, err := s.settleCredit(ctx, in.ActorID, in.BranchID, in.PartyID, in.ReturnType, in.ReturnID,
		in.OrderType, in.OrderID, in.Amount, in.Notes, strings.TrimSpace(in.ClientRef))
	if err == nil && !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "payments.offset_return", Entity: "payment", EntityID: out.ID, BranchID: in.BranchID, ActorID: in.ActorID, Note: out.Number})
	}
	return out, replayed, err
}

// PaymentInfoByOrders implements the contract (see there).
func (s *Service) PaymentInfoByOrders(ctx context.Context, orderType string, ids []int64) (map[int64]contracts.OrderPaymentInfo, error) {
	info, err := s.repo.PaymentInfoByOrders(ctx, orderType, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return info, nil
}

// PaidForOrders sums active allocations per order id in one grouped read.
func (s *Service) PaidForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error) {
	paid, err := s.repo.PaidForOrders(ctx, orderType, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return paid, nil
}

// RecordForOrder records one payment in full against one order — the
// settlement step of a delivery/receipt and the POS checkout. The party and
// money direction come from the order, the amount defaults to the whole
// outstanding, and the call is idempotent per ClientRef (a replay returns the
// payment already recorded and reports replayed=true).
func (s *Service) RecordForOrder(ctx context.Context, in contracts.RecordInput) (*contracts.Payment, bool, error) {
	if in.OrderType != contracts.OrderSales && in.OrderType != contracts.OrderPurchase && in.OrderType != contracts.OrderSalesReturn {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "orderType", Message: "harus sales, purchase, atau sales_return"}})
	}
	var partyID int64
	switch in.OrderType {
	case contracts.OrderSalesReturn:
		// Exchange surcharge (K-04e): the customer owes us the difference of a
		// replacement that costs more. Only the charge kind can be collected
		// here; a refund-kind return would flip the money direction out.
		r, err := s.salesReturns.GetReturn(ctx, in.OrderID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if r == nil || r.Branch != in.BranchID {
			return nil, false, apperror.NotFound("Retur")
		}
		if r.Kind != salesreturncontracts.KindCharge {
			return nil, false, apperror.Conflict("Retur ini bukan tambah bayar tukar: tidak ada tagihan selisih yang bisa dipungut")
		}
		so, err := s.sales.GetByID(ctx, r.OrderID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if so == nil {
			return nil, false, apperror.NotFound("Order")
		}
		partyID = so.PartyID
	case contracts.OrderSales:
		o, err := s.sales.GetByID(ctx, in.OrderID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if o == nil || o.BranchID != in.BranchID {
			return nil, false, apperror.NotFound("Order")
		}
		partyID = o.PartyID
	default:
		o, err := s.purchasing.GetByID(ctx, in.OrderID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if o == nil || o.BranchID != in.BranchID {
			return nil, false, apperror.NotFound("Order")
		}
		partyID = o.PartyID
	}
	// Replay first: the outstanding changes once the payment exists, so the
	// default amount must not be recomputed for a call we already served.
	if ref := strings.TrimSpace(in.ClientRef); ref != "" {
		existing, err := s.repo.PaymentByClientRef(ctx, in.BranchID, ref)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if existing != nil {
			allocs, err := s.repo.AllocationsByPayment(ctx, existing.ID)
			if err != nil {
				return nil, false, apperror.Internal(err)
			}
			if len(allocs) != 1 || allocs[0].OrderType != in.OrderType || allocs[0].OrderID != in.OrderID {
				return nil, false, apperror.Conflict("Referensi klien sudah dipakai untuk pembayaran lain")
			}
			out, err := s.enrich(ctx, existing)
			return out, true, err
		}
	}
	amount := in.Amount
	if amount == 0 {
		ref, err := s.resolveOrder(ctx, in.BranchID, partyID, in.OrderType, in.OrderID)
		if err != nil {
			return nil, false, err
		}
		amount = ref.outstanding
	}
	method := in.Method
	if method == "" {
		method = contracts.MethodCash
	}
	opts := createOptions{clientRef: strings.TrimSpace(in.ClientRef), amountTendered: in.AmountTendered, reference: strings.TrimSpace(in.Reference)}
	return s.createPayment(ctx, in.ActorID, in.BranchID, partyID, method, in.PaidAt, in.Notes, amount,
		[]AllocationInput{{OrderType: in.OrderType, OrderID: in.OrderID, Amount: amount}}, opts)
}

// orderIDsOf lists the ids of the party's orders of one type already gathered
// in the balance: the returns it reads are only those of its own orders.
func orderIDsOf(bal *contracts.PartyBalance, orderType string) []int64 {
	ids := make([]int64, 0, len(bal.Orders))
	for _, o := range bal.Orders {
		if o.OrderType == orderType {
			ids = append(ids, o.OrderID)
		}
	}
	return ids
}
