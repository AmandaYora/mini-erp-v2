package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/party/infrastructure/sqlc"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the party module tables.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// MaxCodeNumber returns the highest numeric suffix among codes shaped
// prefix+digits for the party type (archived rows included: a retired code
// is never reused), 0 when none.
func (r *Repository) MaxCodeNumber(ctx context.Context, partyType, prefix string) (int64, error) {
	return r.q.MaxPartyCodeNumber(ctx, sqlc.MaxPartyCodeNumberParams{
		Start: int32(len(prefix) + 1), PartyType: partyType, CodeLike: prefix + "%", StartCheck: int32(len(prefix) + 1),
	})
}

// GetParty returns nil, nil when missing.
func (r *Repository) GetParty(ctx context.Context, id int64) (*contracts.Party, error) {
	row, err := r.q.GetParty(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toParty(row), nil
}

// uniqueNonZero drops zero and duplicate ids, keeping first-seen order.
func uniqueNonZero(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// NamesByIDs resolves party names for many ids in one grouped read — the
// batch behind operational name resolution (A3). Unknown ids are simply
// absent from the map (best-effort, like the per-row loops it replaces).
func (r *Repository) NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	uniq := uniqueNonZero(ids)
	if len(uniq) == 0 {
		return out, nil
	}
	rows, err := r.q.ListPartyNames(ctx, uniq)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// PartyByCode returns nil, nil when the (code, type) pair is free.
func (r *Repository) PartyByCode(ctx context.Context, code, partyType string) (*contracts.Party, error) {
	row, err := r.q.GetPartyByCode(ctx, sqlc.GetPartyByCodeParams{Code: code, PartyType: partyType})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toParty(sqlc.GetPartyRow(row)), nil
}

// CreateParty inserts a party row.
func (r *Repository) CreateParty(ctx context.Context, p *contracts.Party, actorID int64) (int64, error) {
	res, err := r.q.InsertParty(ctx, sqlc.InsertPartyParams{
		Code:         p.Code,
		PartyType:    p.Type,
		Name:         p.Name,
		Phone:        nullStr(p.Phone),
		Email:        nullStr(p.Email),
		AddressText:  nullStr(p.Address),
		Notes:        nullStr(p.Notes),
		MemberTypeID: nullPtr(p.MemberTypeID),
		Attributes:   encodeAttributes(p.Attributes),
		CreatedBy:    actor(actorID),
		UpdatedBy:    actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateParty rewrites a party row.
func (r *Repository) UpdateParty(ctx context.Context, p *contracts.Party, actorID int64) error {
	return r.q.UpdateParty(ctx, sqlc.UpdatePartyParams{
		Name:         p.Name,
		Phone:        nullStr(p.Phone),
		Email:        nullStr(p.Email),
		AddressText:  nullStr(p.Address),
		Notes:        nullStr(p.Notes),
		MemberTypeID: nullPtr(p.MemberTypeID),
		Attributes:   encodeAttributes(p.Attributes),
		Now:          timeutil.NowUTC(),
		UpdatedBy:    actor(actorID),
		ID:           p.ID,
	})
}

// SetPartyStatus flips active/archived with timestamp.
func (r *Repository) SetPartyStatus(ctx context.Context, id int64, status string, actorID int64) error {
	now := timeutil.NowUTC()
	if status == "archived" {
		return r.q.ArchiveParty(ctx, sqlc.ArchivePartyParams{
			Status: status, ArchivedAt: sql.NullTime{Time: now, Valid: true}, Now: now, UpdatedBy: actor(actorID), ID: id,
		})
	}
	return r.q.RestoreParty(ctx, sqlc.RestorePartyParams{Status: status, Now: now, UpdatedBy: actor(actorID), ID: id})
}

// --- addresses --------------------------------------------------------------

// AddressesByParty returns a party's active addresses, primary first.
func (r *Repository) AddressesByParty(ctx context.Context, partyID int64) ([]*contracts.Address, error) {
	rows, err := r.q.ListActiveAddressesByParty(ctx, partyID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Address
	for _, row := range rows {
		out = append(out, toAddress(sqlc.GetAddressRow(row)))
	}
	return out, nil
}

// AddressByID returns nil, nil when missing or foreign-owned.
func (r *Repository) AddressByID(ctx context.Context, partyID, addrID int64) (*contracts.Address, error) {
	row, err := r.q.GetAddress(ctx, sqlc.GetAddressParams{ID: addrID, PartyID: partyID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toAddress(row), nil
}

// CreateAddress inserts an address row.
func (r *Repository) CreateAddress(ctx context.Context, a *contracts.Address) (int64, error) {
	res, err := r.q.InsertAddress(ctx, sqlc.InsertAddressParams{
		PartyID:       a.PartyID,
		Label:         nullStr(a.Label),
		RecipientName: nullStr(a.Recipient),
		Phone:         nullStr(a.Phone),
		AddressText:   a.Text,
		IsPrimary:     a.IsPrimary,
		SortOrder:     int32(a.SortOrder),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAddress rewrites an address row.
func (r *Repository) UpdateAddress(ctx context.Context, a *contracts.Address) error {
	return r.q.UpdateAddress(ctx, sqlc.UpdateAddressParams{
		Label:         nullStr(a.Label),
		RecipientName: nullStr(a.Recipient),
		Phone:         nullStr(a.Phone),
		AddressText:   a.Text,
		IsPrimary:     a.IsPrimary,
		SortOrder:     int32(a.SortOrder),
		Now:           timeutil.NowUTC(),
		ID:            a.ID,
	})
}

// ClearPrimary clears every primary flag of the party (exclusive-primary step).
func (r *Repository) ClearPrimary(ctx context.Context, partyID int64) error {
	return r.q.ClearPrimaryAddresses(ctx, partyID)
}

// PromoteOldestActive marks the oldest active address primary (keeps the
// exactly-one-primary invariant after the primary is archived).
func (r *Repository) PromoteOldestActive(ctx context.Context, partyID int64) error {
	return r.q.PromoteOldestActiveAddress(ctx, partyID)
}

// SetAddressStatus flips active/archived with timestamp.
func (r *Repository) SetAddressStatus(ctx context.Context, addrID int64, status string) error {
	now := timeutil.NowUTC()
	if status == "archived" {
		return r.q.ArchiveAddress(ctx, sqlc.ArchiveAddressParams{
			Status: status, ArchivedAt: sql.NullTime{Time: now, Valid: true}, Now: now, ID: addrID,
		})
	}
	return r.q.RestoreAddress(ctx, sqlc.RestoreAddressParams{Status: status, Now: now, ID: addrID})
}

// --- member types -----------------------------------------------------------

// MemberTypeByID returns nil, nil when missing.
func (r *Repository) MemberTypeByID(ctx context.Context, id int64) (*contracts.MemberType, error) {
	row, err := r.q.GetMemberType(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toMemberType(row), nil
}

// MemberTypeByCode returns nil, nil when the code is free.
func (r *Repository) MemberTypeByCode(ctx context.Context, code string) (*contracts.MemberType, error) {
	row, err := r.q.GetMemberTypeByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toMemberType(sqlc.GetMemberTypeRow(row)), nil
}

// CreateMemberType inserts a member type row.
func (r *Repository) CreateMemberType(ctx context.Context, m *contracts.MemberType, actorID int64) (int64, error) {
	res, err := r.q.InsertMemberType(ctx, sqlc.InsertMemberTypeParams{
		Code:                     m.Code,
		Name:                     m.Name,
		DescriptionText:          nullStr(m.Description),
		PriceBasis:               m.PriceBasis,
		PriceAdjustmentDirection: m.Direction,
		PriceAdjustmentType:      m.AdjustmentType,
		PriceAdjustmentValue:     m.Adjustment,
		RoundingMode:             m.RoundingMode,
		RoundingIncrement:        nullStep(m.RoundingStep),
		CreatedBy:                actor(actorID),
		UpdatedBy:                actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateMemberType rewrites a member type row.
func (r *Repository) UpdateMemberType(ctx context.Context, m *contracts.MemberType, actorID int64) error {
	return r.q.UpdateMemberType(ctx, sqlc.UpdateMemberTypeParams{
		Name:                     m.Name,
		DescriptionText:          nullStr(m.Description),
		PriceBasis:               m.PriceBasis,
		PriceAdjustmentDirection: m.Direction,
		PriceAdjustmentType:      m.AdjustmentType,
		PriceAdjustmentValue:     m.Adjustment,
		RoundingMode:             m.RoundingMode,
		RoundingIncrement:        nullStep(m.RoundingStep),
		Now:                      timeutil.NowUTC(),
		UpdatedBy:                actor(actorID),
		ID:                       m.ID,
	})
}

// CountActiveCustomersByMemberType counts active customers on a member type
// (the ones whose price would fall back to standard when the type is archived).
func (r *Repository) CountActiveCustomersByMemberType(ctx context.Context, memberTypeID int64) (int64, error) {
	return r.q.CountActiveCustomersByMemberType(ctx, sql.NullInt64{Int64: memberTypeID, Valid: true})
}

// SetMemberTypeStatus flips active/archived with timestamp.
func (r *Repository) SetMemberTypeStatus(ctx context.Context, id int64, status string, actorID int64) error {
	now := timeutil.NowUTC()
	if status == "archived" {
		return r.q.ArchiveMemberType(ctx, sqlc.ArchiveMemberTypeParams{
			Status: status, ArchivedAt: sql.NullTime{Time: now, Valid: true}, Now: now, UpdatedBy: actor(actorID), ID: id,
		})
	}
	return r.q.RestoreMemberType(ctx, sqlc.RestoreMemberTypeParams{Status: status, Now: now, UpdatedBy: actor(actorID), ID: id})
}

// LabelsByIDs resolves code+name for many parties in one grouped read, for
// readers that show both. Duplicate and zero ids collapse before the query.
func (r *Repository) LabelsByIDs(ctx context.Context, ids []int64) (map[int64]contracts.Label, error) {
	out := map[int64]contracts.Label{}
	uniq := uniqueNonZero(ids)
	if len(uniq) == 0 {
		return out, nil
	}
	rows, err := r.q.ListPartyLabels(ctx, uniq)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = contracts.Label{Code: row.Code, Name: row.Name}
	}
	return out, nil
}
