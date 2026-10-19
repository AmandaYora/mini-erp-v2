package application

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/party/domain"
	"mini-erp/internal/modules/party/infrastructure"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// Service implements contracts.PartyClient/PricingClient plus party administration.
type Service struct {
	repo     *infrastructure.Repository
	products productcontracts.ProductClient
	// audit receives best-effort trail records after party mutations.
	audit auditcontracts.AuditClient
}

// NewService wires party use cases. Product data arrives via contracts only.
func NewService(repo *infrastructure.Repository, products productcontracts.ProductClient, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, products: products, audit: audit}
}

// GetByID resolves a party with its member code, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.Party, error) {
	p, err := s.repo.GetParty(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil {
		return nil, nil
	}
	return s.enrich(ctx, p)
}

// GetAddress resolves one active customer address scoped to its party.
func (s *Service) GetAddress(ctx context.Context, partyID, addressID int64) (*contracts.Address, error) {
	a, err := s.repo.AddressByID(ctx, partyID, addressID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if a == nil || a.Status != "active" {
		return nil, nil
	}
	return a, nil
}

// NamesByIDs resolves party names in one grouped read (A3) — no enrich,
// names only.
func (s *Service) NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	names, err := s.repo.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return names, nil
}

// SearchIDs implements contracts.PartyClient: ids of parties matching a term.
func (s *Service) SearchIDs(ctx context.Context, partyType, term string, limit int) ([]int64, error) {
	term = strings.TrimSpace(term)
	if len([]rune(term)) < contracts.MinSearchChars {
		return nil, nil
	}
	if limit <= 0 || limit > contracts.MaxSearchIDs {
		limit = contracts.MaxSearchIDs
	}
	ids, err := s.repo.SearchIDs(ctx, partyType, term, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
}

// LabelsByIDs resolves party code+name in one grouped read, for readers that
// display both (receivables/payables).
func (s *Service) LabelsByIDs(ctx context.Context, ids []int64) (map[int64]contracts.Label, error) {
	labels, err := s.repo.LabelsByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return labels, nil
}

func (s *Service) enrich(ctx context.Context, p *contracts.Party) (*contracts.Party, error) {
	if p.MemberTypeID != nil {
		m, err := s.repo.MemberTypeByID(ctx, *p.MemberTypeID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if m != nil {
			p.MemberCode = m.Code
		}
	}
	return p, nil
}

func validPhone(phone string) bool {
	if phone == "" {
		return true
	}
	digits := 0
	for _, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 7 && digits <= 20
}

// PartyInput is the create/update payload.
type PartyInput struct {
	Code         string
	Name         string
	Phone        string
	Email        string
	Address      string
	Notes        string
	MemberTypeID *int64
	ClearMember  bool
	// Attributes replaces the "Informasi Tambahan" block when non-nil
	// (an empty non-nil map clears it); nil keeps what is stored.
	Attributes map[string]string
}

// Bounds of the Informasi Tambahan block (safety net; legacy has none).
const (
	maxAttributes     = 20
	maxAttributeKey   = 60
	maxAttributeValue = 255
)

// cleanAttributes trims, validates and returns the attribute map.
func cleanAttributes(in map[string]string) (map[string]string, []apperror.FieldError) {
	if in == nil {
		return nil, nil
	}
	var fields []apperror.FieldError
	if len(in) > maxAttributes {
		fields = append(fields, apperror.FieldError{Field: "attributes", Message: "maksimal " + strconv.Itoa(maxAttributes) + " informasi tambahan"})
		return nil, fields
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key, val := strings.TrimSpace(k), strings.TrimSpace(v)
		switch {
		case key == "" || len([]rune(key)) > maxAttributeKey:
			fields = append(fields, apperror.FieldError{Field: "attributes", Message: "nama informasi tambahan wajib 1-" + strconv.Itoa(maxAttributeKey) + " karakter"})
		case val == "" || len([]rune(val)) > maxAttributeValue:
			fields = append(fields, apperror.FieldError{Field: "attributes", Message: "isi '" + key + "' wajib 1-" + strconv.Itoa(maxAttributeValue) + " karakter"})
		default:
			if _, dup := out[key]; dup {
				fields = append(fields, apperror.FieldError{Field: "attributes", Message: "nama '" + key + "' ganda"})
			}
			out[key] = val
		}
	}
	return out, fields
}

func (s *Service) validateParty(partyType string, in PartyInput) []apperror.FieldError {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	if strings.TrimSpace(in.Name) == "" {
		fail("name", "wajib diisi")
	}
	if !validPhone(strings.TrimSpace(in.Phone)) {
		fail("phone", "format telepon tidak valid")
	}
	if _, attrFields := cleanAttributes(in.Attributes); len(attrFields) > 0 {
		fields = append(fields, attrFields...)
	}
	return fields
}

// autoCodePrefix is the legacy sequential prefix (CUS-001 / SUP-001).
func autoCodePrefix(partyType string) string {
	if partyType == contracts.PartySupplier {
		return "SUP-"
	}
	return "CUS-"
}

// autoCode proposes the next sequential code (legacy parity: max + 1 over ALL
// rows of the type, archived included, 3-digit minimum). Two concurrent
// creators may propose the same code; the UNIQUE (code, party_type) key lets
// exactly one insert win and createParty retries the loser with a fresh read.
func (s *Service) autoCode(ctx context.Context, partyType string) (string, error) {
	prefix := autoCodePrefix(partyType)
	max, err := s.repo.MaxCodeNumber(ctx, partyType, prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, max+1), nil
}

// resolveMember validates the member type for customers. Suppliers never
// carry one.archived types are rejected — history keeps pointing at them,
// new assignments may not.
func (s *Service) resolveMember(ctx context.Context, partyType string, memberID *int64) (*contracts.MemberType, error) {
	if memberID == nil {
		return nil, nil
	}
	if partyType != contracts.PartyCustomer {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memberTypeId", Message: "hanya customer yang punya tipe member"}})
	}
	m, err := s.repo.MemberTypeByID(ctx, *memberID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if m == nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memberTypeId", Message: "tipe member tidak ditemukan"}})
	}
	if m.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memberTypeId", Message: "tipe member sudah diarsipkan"}})
	}
	return m, nil
}

// createParty is the shared customer/supplier insert.
func (s *Service) createParty(ctx context.Context, actorID int64, partyType string, in PartyInput) (*contracts.Party, error) {
	if fields := s.validateParty(partyType, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	auto := code == ""
	if !auto {
		if dup, err := s.repo.PartyByCode(ctx, code, partyType); err != nil {
			return nil, apperror.Internal(err)
		} else if dup != nil {
			return nil, dupConflict(dup)
		}
	}
	if _, err := s.resolveMember(ctx, partyType, in.MemberTypeID); err != nil {
		return nil, err
	}
	attrs, _ := cleanAttributes(in.Attributes)
	for attempt := 0; ; attempt++ {
		if auto {
			generated, err := s.autoCode(ctx, partyType)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			code = generated
		}
		id, err := s.repo.CreateParty(ctx, &contracts.Party{
			Code: code, Type: partyType, Name: strings.TrimSpace(in.Name),
			Phone: strings.TrimSpace(in.Phone), Email: strings.TrimSpace(in.Email),
			Address: in.Address, Notes: in.Notes, MemberTypeID: in.MemberTypeID,
			Attributes: attrs,
		}, actorID)
		if err != nil {
			if auto && attempt < 25 && dberr.IsDuplicate(err) {
				// Lost the race for this number: back off a little (jittered
				// so racers do not collide again in lockstep), read the new max.
				time.Sleep(time.Duration(rand.IntN(4*(attempt+1))+1) * time.Millisecond)
				continue
			}
			return nil, dberr.Map(err)
		}
		return s.GetByID(ctx, id)
	}
}

// dupConflict names the holder, adding "(ada di Sampah)" when archived
// (KI-71: reserved across archive, never a bare 500).
func dupConflict(dup *contracts.Party) *apperror.AppError {
	if dup.Status == "archived" {
		return apperror.Conflict("Kode '" + dup.Code + "' sudah digunakan (ada di Sampah)")
	}
	return apperror.Conflict("Kode '" + dup.Code + "' sudah digunakan")
}

// CreateCustomer inserts a customer.
func (s *Service) CreateCustomer(ctx context.Context, actorID int64, in PartyInput) (*contracts.Party, error) {
	p, err := s.createParty(ctx, actorID, contracts.PartyCustomer, in)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "customers.create", Entity: "customer", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

// CreateSupplier inserts a supplier.
func (s *Service) CreateSupplier(ctx context.Context, actorID int64, in PartyInput) (*contracts.Party, error) {
	p, err := s.createParty(ctx, actorID, contracts.PartySupplier, in)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "suppliers.create", Entity: "supplier", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

// UpdateCustomer rewrites a customer. Code is immutable. Omitting
// memberTypeId keeps the existing membership (never silently stripped,
// KI-67); ClearMember removes it explicitly.
func (s *Service) UpdateCustomer(ctx context.Context, actorID, id int64, in PartyInput) (*contracts.Party, error) {
	p, err := s.updateParty(ctx, actorID, id, contracts.PartyCustomer, in)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "customers.update", Entity: "customer", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

// UpdateSupplier rewrites a supplier.
func (s *Service) UpdateSupplier(ctx context.Context, actorID, id int64, in PartyInput) (*contracts.Party, error) {
	p, err := s.updateParty(ctx, actorID, id, contracts.PartySupplier, in)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "suppliers.update", Entity: "supplier", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Code})
	return p, nil
}

func (s *Service) updateParty(ctx context.Context, actorID, id int64, partyType string, in PartyInput) (*contracts.Party, error) {
	existing, err := s.repo.GetParty(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil || existing.Type != partyType {
		return nil, apperror.NotFound(partyLabel(partyType))
	}
	// Code is immutable: a changed code is rejected explicitly instead of
	// silently ignored (KI-62).
	if code := strings.ToUpper(strings.TrimSpace(in.Code)); code != "" && code != existing.Code {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "kode tidak dapat diubah"}})
	}
	if fields := s.validateParty(partyType, in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	memberID := existing.MemberTypeID
	if in.ClearMember {
		memberID = nil
	} else if in.MemberTypeID != nil {
		if _, err := s.resolveMember(ctx, partyType, in.MemberTypeID); err != nil {
			return nil, err
		}
		memberID = in.MemberTypeID
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.Phone = strings.TrimSpace(in.Phone)
	existing.Email = strings.TrimSpace(in.Email)
	existing.Address = in.Address
	existing.Notes = in.Notes
	existing.MemberTypeID = memberID
	if in.Attributes != nil {
		existing.Attributes, _ = cleanAttributes(in.Attributes)
	}
	if err := s.repo.UpdateParty(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

func partyLabel(partyType string) string {
	if partyType == contracts.PartySupplier {
		return "Supplier"
	}
	return "Customer"
}

// Archive flips active/archived with timestamp (no hard deletes — codes stay
// reserved and restore always works).
func (s *Service) Archive(ctx context.Context, actorID, id int64, partyType string) error {
	if err := s.setStatus(ctx, actorID, id, partyType, "archived"); err != nil {
		return err
	}
	action, entity := "customers.archive", "customer"
	if partyType == contracts.PartySupplier {
		action, entity = "suppliers.archive", "supplier"
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: action, Entity: entity, EntityID: id, BranchID: 0, ActorID: actorID})
	return nil
}

// Restore revives an archived party.
func (s *Service) Restore(ctx context.Context, actorID, id int64, partyType string) error {
	if err := s.setStatus(ctx, actorID, id, partyType, "active"); err != nil {
		return err
	}
	action, entity := "customers.restore", "customer"
	if partyType == contracts.PartySupplier {
		action, entity = "suppliers.restore", "supplier"
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: action, Entity: entity, EntityID: id, BranchID: 0, ActorID: actorID})
	return nil
}

func (s *Service) setStatus(ctx context.Context, actorID, id int64, partyType, status string) error {
	existing, err := s.repo.GetParty(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil || existing.Type != partyType {
		return apperror.NotFound(partyLabel(partyType))
	}
	if err := s.repo.SetPartyStatus(ctx, id, status, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ListResult is a paginated party page.
type ListResult struct {
	Parties []*contracts.Party
	Total   int64
}

// List searches parties per word (code/name/phone, KI-65) with status filter.
func (s *Service) List(ctx context.Context, partyType, search, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.CountParties(ctx, partyType, search, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	parties, err := s.repo.ListParties(ctx, partyType, search, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if parties == nil {
		parties = []*contracts.Party{}
	}
	return &ListResult{Parties: parties, Total: total}, nil
}

// --- addresses --------------------------------------------------------------

// AddressInput is the address create/update payload.
type AddressInput struct {
	Label     string
	Recipient string
	Phone     string
	Text      string
	IsPrimary bool
	SortOrder int
}

// ListAddresses returns a party's active addresses, primary first.
func (s *Service) ListAddresses(ctx context.Context, partyID int64, partyType string) ([]*contracts.Address, error) {
	if _, err := s.requireWritableParty(ctx, partyID, partyType, false); err != nil {
		return nil, err
	}
	addresses, err := s.repo.AddressesByParty(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if addresses == nil {
		addresses = []*contracts.Address{}
	}
	return addresses, nil
}

// requireWritableParty rejects unknown/type-mismatched parties, and — for
// writes — archived ones (KI-63: an archived customer's book is locked).
func (s *Service) requireWritableParty(ctx context.Context, partyID int64, partyType string, write bool) (*contracts.Party, error) {
	p, err := s.repo.GetParty(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p == nil || p.Type != partyType {
		return nil, apperror.NotFound(partyLabel(partyType))
	}
	if write && p.Status != "active" {
		return nil, apperror.Conflict(partyLabel(partyType) + " sudah diarsipkan")
	}
	return p, nil
}

// normalizePrimary enforces exactly one primary: the caller's mark wins,
// otherwise the first address keeps/claims it (KI-69: no-default unsupported).
func normalizePrimary(addresses []*contracts.Address, markedID int64, markPrimary bool) {
	if markPrimary {
		for _, a := range addresses {
			a.IsPrimary = a.ID == markedID
		}
		return
	}
	for _, a := range addresses {
		if a.IsPrimary {
			return
		}
	}
	if len(addresses) > 0 {
		addresses[0].IsPrimary = true
	}
}

// CreateAddress inserts an address, maintaining the single-primary invariant.
func (s *Service) CreateAddress(ctx context.Context, actorID, partyID int64, partyType string, in AddressInput) (*contracts.Address, error) {
	if _, err := s.requireWritableParty(ctx, partyID, partyType, true); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "address", Message: "wajib diisi"}})
	}
	if !validPhone(strings.TrimSpace(in.Phone)) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "phone", Message: "format telepon tidak valid"}})
	}
	id, err := s.repo.CreateAddress(ctx, &contracts.Address{
		PartyID: partyID, Label: in.Label, Recipient: in.Recipient,
		Phone: strings.TrimSpace(in.Phone), Text: in.Text,
		IsPrimary: in.IsPrimary, SortOrder: in.SortOrder,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	addresses, err := s.repo.AddressesByParty(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	normalizePrimary(addresses, id, in.IsPrimary)
	if err := s.persistPrimary(ctx, partyID, addresses); err != nil {
		return nil, apperror.Internal(err)
	}
	addr, err := s.repo.AddressByID(ctx, partyID, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "addresses.create", Entity: "party_address", EntityID: addr.ID, BranchID: 0, ActorID: actorID})
	return addr, nil
}

func (s *Service) persistPrimary(ctx context.Context, partyID int64, addresses []*contracts.Address) error {
	primary := int64(0)
	for _, a := range addresses {
		if a.IsPrimary {
			primary = a.ID
			break
		}
	}
	if err := s.repo.ClearPrimary(ctx, partyID); err != nil {
		return err
	}
	if primary == 0 {
		return nil
	}
	for _, a := range addresses {
		a.IsPrimary = a.ID == primary
		if err := s.repo.UpdateAddress(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// UpdateAddress rewrites an address row.
func (s *Service) UpdateAddress(ctx context.Context, actorID, partyID int64, partyType string, addrID int64, in AddressInput) (*contracts.Address, error) {
	if _, err := s.requireWritableParty(ctx, partyID, partyType, true); err != nil {
		return nil, err
	}
	existing, err := s.repo.AddressByID(ctx, partyID, addrID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil {
		return nil, apperror.NotFound("Alamat")
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "address", Message: "wajib diisi"}})
	}
	if !validPhone(strings.TrimSpace(in.Phone)) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "phone", Message: "format telepon tidak valid"}})
	}
	existing.Label, existing.Recipient = in.Label, in.Recipient
	existing.Phone = strings.TrimSpace(in.Phone)
	existing.Text, existing.SortOrder = in.Text, in.SortOrder
	if err := s.repo.UpdateAddress(ctx, existing); err != nil {
		return nil, apperror.Internal(err)
	}
	addresses, err := s.repo.AddressesByParty(ctx, partyID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	normalizePrimary(addresses, addrID, in.IsPrimary)
	if err := s.persistPrimary(ctx, partyID, addresses); err != nil {
		return nil, apperror.Internal(err)
	}
	addr, err := s.repo.AddressByID(ctx, partyID, addrID)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "addresses.update", Entity: "party_address", EntityID: addr.ID, BranchID: 0, ActorID: actorID})
	return addr, nil
}

// ArchiveAddress archives an address. Archiving the primary promotes the
// oldest remaining active address, preserving exactly-one-primary (KI-69).
func (s *Service) ArchiveAddress(ctx context.Context, actorID, partyID int64, partyType string, addrID int64) error {
	if _, err := s.requireWritableParty(ctx, partyID, partyType, true); err != nil {
		return err
	}
	existing, err := s.repo.AddressByID(ctx, partyID, addrID)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil {
		return apperror.NotFound("Alamat")
	}
	wasPrimary := existing.IsPrimary
	if err := s.repo.SetAddressStatus(ctx, addrID, "archived"); err != nil {
		return apperror.Internal(err)
	}
	if wasPrimary {
		if err := s.repo.PromoteOldestActive(ctx, partyID); err != nil {
			return apperror.Internal(err)
		}
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "addresses.archive", Entity: "party_address", EntityID: addrID, BranchID: 0, ActorID: actorID})
	return nil
}

// --- member types -----------------------------------------------------------

// MemberTypeInput is the member type payload.
type MemberTypeInput struct {
	Code        string
	Name        string
	Description string
	Basis       string
	Direction   string
	Type        string
	Value       float64
	Mode        string
	Step        float64
	Confirmed   bool
}

func (s *Service) validateMemberType(in MemberTypeInput) []apperror.FieldError {
	var fields []apperror.FieldError
	fail := func(field, msg string) {
		fields = append(fields, apperror.FieldError{Field: field, Message: msg})
	}
	if strings.TrimSpace(in.Name) == "" {
		fail("name", "wajib diisi")
	}
	switch in.Basis {
	case domain.BasisSellingPrice, domain.BasisMinSellingPrice, domain.BasisPurchasePrice:
	default:
		fail("basis", "harus selling_price, min_selling_price, atau purchase_price")
	}
	if in.Direction != domain.DirMinus && in.Direction != domain.DirPlus {
		fail("direction", "harus minus atau plus")
	}
	if in.Type != domain.AdjustPercent && in.Type != domain.AdjustNominal {
		fail("type", "harus percent atau nominal")
	}
	if in.Type == domain.AdjustPercent {
		switch {
		case in.Value < 0:
			fail("value", "tidak boleh negatif")
		case in.Direction == domain.DirMinus && in.Value > 100:
			fail("value", "potongan persen maksimal 100")
		case in.Value > domain.MaxPercent:
			fail("value", "persen maksimal "+strconv.Itoa(domain.MaxPercent))
		}
	}
	if in.Type == domain.AdjustNominal && in.Value < 0 {
		fail("value", "tidak boleh negatif")
	}
	// Mode+increment must pair (KI-73): a mode without a step, or a step
	// without a mode, is rejected — except step 0 with mode none.
	if in.Mode == "" {
		in.Mode = domain.RoundNone
	}
	if _, _, ok := domain.NormalizeRounding(in.Mode, in.Step); !ok {
		fail("mode", "harus none, nearest, up, atau down")
	}
	if in.Mode != domain.RoundNone && in.Step <= 0 {
		fail("step", "kelipatan wajib diisi bila mode pembulatan dipakai")
	}
	if in.Mode == domain.RoundNone && in.Step != 0 {
		fail("step", "kelipatan harus kosong bila tanpa pembulatan")
	}
	// Nominal above the confirmation bar needs an explicit resubmit (KI-76).
	if in.Type == domain.AdjustNominal && in.Value > domain.NominalConfirmThreshold && !in.Confirmed {
		fail("value", "melebihi ambang konfirmasi — kirim ulang dengan konfirmasi")
	}
	return fields
}

// CreateMemberType inserts a member type (code immutable once issued,
// reserved across archive per KI-71).
func (s *Service) CreateMemberType(ctx context.Context, actorID int64, in MemberTypeInput) (*contracts.MemberType, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.MemberTypeByCode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if dup != nil {
		return nil, memberDupConflict(dup)
	}
	in = normalizeMemberInput(in)
	if fields := s.validateMemberType(in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	id, err := s.repo.CreateMemberType(ctx, &contracts.MemberType{
		Code: code, Name: strings.TrimSpace(in.Name), Description: in.Description,
		PriceBasis: in.Basis, Direction: in.Direction,
		AdjustmentType: in.Type,
		Adjustment:     in.Value, RoundingMode: in.Mode,
		RoundingStep: in.Step,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	m, err := s.repo.MemberTypeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "member_types.create", Entity: "member_type", EntityID: m.ID, BranchID: 0, ActorID: actorID, Note: m.Code})
	return m, nil
}

func normalizeMemberInput(in MemberTypeInput) MemberTypeInput {
	in.Basis = orDefault(in.Basis, domain.BasisSellingPrice)
	in.Direction = orDefault(in.Direction, domain.DirMinus)
	in.Type = orDefault(in.Type, domain.AdjustPercent)
	in.Mode = orDefault(in.Mode, domain.RoundNone)
	// Pre-parity names (round_100, floor, ceil...) are folded into the
	// legacy vocabulary; unknown names are left for validation to reject.
	if m, st, ok := domain.NormalizeRounding(in.Mode, in.Step); ok {
		in.Mode, in.Step = m, st
	}
	return in
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func memberDupConflict(dup *contracts.MemberType) *apperror.AppError {
	if dup.Status == "archived" {
		return apperror.Conflict("Kode member '" + dup.Code + "' sudah digunakan (ada di Sampah)")
	}
	return apperror.Conflict("Kode member '" + dup.Code + "' sudah digunakan")
}

// memberTypeSnapshot is the audit view of a member type (F3b-8).
func memberTypeSnapshot(m *contracts.MemberType) map[string]any {
	return map[string]any{
		"code": m.Code, "name": m.Name, "description": m.Description, "priceBasis": m.PriceBasis,
		"direction": m.Direction, "adjustmentType": m.AdjustmentType, "adjustment": m.Adjustment,
		"roundingMode": m.RoundingMode, "roundingStep": m.RoundingStep, "status": m.Status,
	}
}

// UpdateMemberType rewrites a member type (code immutable).
func (s *Service) UpdateMemberType(ctx context.Context, actorID, id int64, in MemberTypeInput) (*contracts.MemberType, error) {
	existing, err := s.repo.MemberTypeByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if existing == nil {
		return nil, apperror.NotFound("Tipe member")
	}
	in = normalizeMemberInput(in)
	if fields := s.validateMemberType(in); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	beforeMember := memberTypeSnapshot(existing)
	existing.Name = strings.TrimSpace(in.Name)
	existing.Description = in.Description
	existing.PriceBasis = in.Basis
	existing.Direction = in.Direction
	existing.AdjustmentType = in.Type
	existing.Adjustment = in.Value
	existing.RoundingMode = in.Mode
	existing.RoundingStep = in.Step
	if err := s.repo.UpdateMemberType(ctx, existing, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.repo.MemberTypeByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{
		Action: "member_types.update", Entity: "member_type", EntityID: updated.ID, BranchID: 0, ActorID: actorID, Note: updated.Code,
		Before: beforeMember, After: memberTypeSnapshot(updated),
	})
	return updated, nil
}

// CountMemberTypeCustomers is the archive-impact figure (D3/PR-09): how many
// active customers currently hold this member type. Archiving stays allowed;
// those customers then fall back to standard prices (KI-72), so the dialog
// shows the number before the user confirms.
func (s *Service) CountMemberTypeCustomers(ctx context.Context, id int64) (int64, error) {
	existing, err := s.repo.MemberTypeByID(ctx, id)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if existing == nil {
		return 0, apperror.NotFound("Tipe member")
	}
	n, err := s.repo.CountActiveCustomersByMemberType(ctx, id)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// ArchiveMemberType archives a member type (history keeps pointing at it).
func (s *Service) ArchiveMemberType(ctx context.Context, actorID, id int64) error {
	if err := s.setMemberStatus(ctx, actorID, id, "archived"); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "member_types.archive", Entity: "member_type", EntityID: id, BranchID: 0, ActorID: actorID})
	return nil
}

// RestoreMemberType revives an archived member type (KI-71).
func (s *Service) RestoreMemberType(ctx context.Context, actorID, id int64) error {
	if err := s.setMemberStatus(ctx, actorID, id, "active"); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "member_types.restore", Entity: "member_type", EntityID: id, BranchID: 0, ActorID: actorID})
	return nil
}

func (s *Service) setMemberStatus(ctx context.Context, actorID, id int64, status string) error {
	existing, err := s.repo.MemberTypeByID(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if existing == nil {
		return apperror.NotFound("Tipe member")
	}
	if err := s.repo.SetMemberTypeStatus(ctx, id, status, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ListMemberTypes searches code/name per word.
func (s *Service) ListMemberTypes(ctx context.Context, search, status string) ([]*contracts.MemberType, error) {
	members, err := s.repo.ListMemberTypes(ctx, search, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if members == nil {
		members = []*contracts.MemberType{}
	}
	return members, nil
}

// --- pricing ----------------------------------------------------------------

// Quote prices product lines for a customer. customerID 0 means walk-in
// (standard prices, no member). Request-level problems (no lines, too many,
// unknown customer) are 400/404s; a problem with one PRODUCT is reported on
// its own line (QuoteLine.Error) so one bad line never fails the cart. An
// inactive membership degrades to standard with the memberInactive marker
// set (KI-72). Products resolve in ONE batch read, never one per line.
func (s *Service) Quote(ctx context.Context, customerID int64, productIDs []int64) (*contracts.QuoteResult, error) {
	if appErr := checkQuoteLines(productIDs); appErr != nil {
		return nil, appErr
	}
	res := &contracts.QuoteResult{Lines: []contracts.QuoteLine{}}
	var member *contracts.MemberType
	if customerID != 0 {
		customer, err := s.repo.GetParty(ctx, customerID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if customer == nil || customer.Type != contracts.PartyCustomer {
			return nil, apperror.NotFound("Customer")
		}
		if customer.MemberTypeID != nil {
			m, err := s.repo.MemberTypeByID(ctx, *customer.MemberTypeID)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			if m == nil || m.Status != "active" || customer.Status != "active" {
				res.MemberInactive = true
			} else {
				member = m
				res.MemberType = m.Code
			}
		}
	}
	return s.priceLines(ctx, res, member, productIDs)
}

func checkQuoteLines(productIDs []int64) *apperror.AppError {
	if len(productIDs) == 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "productIds", Message: "wajib diisi"}})
	}
	if len(productIDs) > 100 {
		return apperror.Validation("", []apperror.FieldError{{Field: "productIds", Message: "maksimal 100 baris per quote"}})
	}
	return nil
}

// QuoteForMemberType prices lines for an explicit member type with no
// customer (PR-10: "what would this tier pay?" — price checks, the member
// type form preview). An unknown type is a 404; an archived one degrades to
// standard prices with the memberInactive marker, like an inactive membership.
func (s *Service) QuoteForMemberType(ctx context.Context, memberTypeID int64, productIDs []int64) (*contracts.QuoteResult, error) {
	if appErr := checkQuoteLines(productIDs); appErr != nil {
		return nil, appErr
	}
	m, err := s.repo.MemberTypeByID(ctx, memberTypeID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if m == nil {
		return nil, apperror.NotFound("Tipe member")
	}
	res := &contracts.QuoteResult{Lines: []contracts.QuoteLine{}}
	var member *contracts.MemberType
	if m.Status != "active" {
		res.MemberInactive = true
	} else {
		member = m
		res.MemberType = m.Code
	}
	return s.priceLines(ctx, res, member, productIDs)
}

// priceLines prices every product line (one batch read) under the resolved
// member rule, or at standard prices when member is nil.
func (s *Service) priceLines(ctx context.Context, res *contracts.QuoteResult, member *contracts.MemberType, productIDs []int64) (*contracts.QuoteResult, error) {
	infos, err := s.products.PricingByIDs(ctx, productIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, pid := range productIDs {
		line := contracts.QuoteLine{ProductID: pid}
		info, ok := infos[pid]
		switch {
		case !ok:
			line.Error = "produk tidak ditemukan"
		case info.Status != "active":
			line.StandardPrice, line.MemberPrice = info.SellingPrice, info.SellingPrice
			line.Error = "produk '" + info.Code + "' tidak aktif"
		default:
			line.StandardPrice, line.MemberPrice = info.SellingPrice, info.SellingPrice
			if member != nil {
				price, msg := memberPrice(member, info)
				if msg != "" {
					line.Error = msg
				} else {
					line.MemberPrice = price
					line.Applied = true
				}
			} else {
				line.MemberInactive = res.MemberInactive
			}
		}
		res.Lines = append(res.Lines, line)
	}
	return res, nil
}

// memberPrice applies a member rule to one product. A non-empty message is
// the line error: the rule cannot price this product (legacy requirePrice —
// an empty basis price is an error, never a silent zero).
func memberPrice(m *contracts.MemberType, p productcontracts.PricingInfo) (int64, string) {
	base, err := domain.BasePrice(m.PriceBasis, p.SellingPrice, p.MinSellingPrice,
		p.PurchasePrice, p.PurchaseFactor, p.SalesFactor)
	if err != nil {
		if errors.Is(err, domain.ErrBasePrice) {
			return 0, "Produk '" + p.Code + "' belum memiliki harga " + basisLabel(m.PriceBasis)
		}
		return 0, "Konversi satuan produk '" + p.Code + "' tidak valid"
	}
	price := domain.ApplyRule(base, m.Direction, m.AdjustmentType, m.Adjustment, m.RoundingMode, m.RoundingStep)
	if price < 0 {
		return 0, "harga member produk '" + p.Code + "' negatif"
	}
	return price, ""
}

func basisLabel(basis string) string {
	switch basis {
	case domain.BasisMinSellingPrice:
		return "jual minimum"
	case domain.BasisPurchasePrice:
		return "beli"
	}
	return "jual normal"
}
