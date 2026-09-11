package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/party/contracts"
)

// Repository owns the party module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const partyColumns = `id, code, party_type, name, phone, email, address_text,
	notes, member_type_id, status`

func scanParty(row *sql.Row, p *contracts.Party) error {
	var phone, email, address, notes sql.NullString
	var member sql.NullInt64
	err := row.Scan(&p.ID, &p.Code, &p.Type, &p.Name, &phone, &email,
		&address, &notes, &member, &p.Status)
	p.Phone, p.Email, p.Address, p.Notes = phone.String, email.String, address.String, notes.String
	if member.Valid {
		p.MemberTypeID = &member.Int64
	}
	return err
}

// GetParty returns nil, nil when missing.
func (r *Repository) GetParty(ctx context.Context, id int64) (*contracts.Party, error) {
	p := &contracts.Party{}
	err := scanParty(r.db.QueryRowContext(ctx,
		"SELECT "+partyColumns+" FROM parties WHERE id = ?", id), p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// NamesByIDs resolves party names for many ids in one grouped read — the
// batch behind operational name resolution (A3). Unknown ids are simply
// absent from the map (best-effort, like the per-row loops it replaces).
func (r *Repository) NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	seen := map[int64]bool{}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	if len(args) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(args)-1) + "?"
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name FROM parties WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// PartyByCode returns nil, nil when the (code, type) pair is free.
func (r *Repository) PartyByCode(ctx context.Context, code, partyType string) (*contracts.Party, error) {
	p := &contracts.Party{}
	err := scanParty(r.db.QueryRowContext(ctx,
		"SELECT "+partyColumns+" FROM parties WHERE code = ? AND party_type = ?", code, partyType), p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// CreateParty inserts a party row.
func (r *Repository) CreateParty(ctx context.Context, p *contracts.Party, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO parties
		 (code, party_type, name, phone, email, address_text, notes, member_type_id,
		  status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		p.Code, p.Type, p.Name, nullIfEmpty(p.Phone), nullIfEmpty(p.Email),
		nullIfEmpty(p.Address), nullIfEmpty(p.Notes), nullInt64(p.MemberTypeID),
		actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateParty rewrites a party row.
func (r *Repository) UpdateParty(ctx context.Context, p *contracts.Party, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE parties SET name = ?, phone = ?, email = ?, address_text = ?, notes = ?,
		 member_type_id = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		p.Name, nullIfEmpty(p.Phone), nullIfEmpty(p.Email), nullIfEmpty(p.Address),
		nullIfEmpty(p.Notes), nullInt64(p.MemberTypeID), actorID, p.ID)
	return err
}

// SetPartyStatus flips active/archived with timestamp.
func (r *Repository) SetPartyStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE parties SET status = ?,
		 archived_at = CASE WHEN ? = 'archived' THEN UTC_TIMESTAMP(6) ELSE NULL END,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		status, status, actorID, id)
	return err
}

// ListParties searches code/name/phone per word, filtered by type + status.
// Member codes ride along via LEFT JOIN — one query, no per-row lookup.
func (r *Repository) ListParties(ctx context.Context, partyType, search, status string, limit, offset int) ([]*contracts.Party, error) {
	conds := []string{"p.party_type = ?"}
	args := []any{partyType}
	if status != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(p.code LIKE ? OR p.name LIKE ? OR p.phone LIKE ?)")
		args = append(args, like, like, like)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.code, p.party_type, p.name, p.phone, p.email, p.address_text,
			p.notes, p.member_type_id, p.status, m.code
		 FROM parties p LEFT JOIN member_types m ON m.id = p.member_type_id
		 WHERE `+strings.Join(conds, " AND ")+
			` ORDER BY p.name ASC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Party
	for rows.Next() {
		p := &contracts.Party{}
		var phone, email, address, notes, memberCode sql.NullString
		var member sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Code, &p.Type, &p.Name, &phone, &email,
			&address, &notes, &member, &p.Status, &memberCode); err != nil {
			return nil, err
		}
		p.Phone, p.Email, p.Address, p.Notes = phone.String, email.String, address.String, notes.String
		if member.Valid {
			p.MemberTypeID = &member.Int64
		}
		p.MemberCode = memberCode.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountParties counts the List filter set.
func (r *Repository) CountParties(ctx context.Context, partyType, search, status string) (int64, error) {
	conds := []string{"party_type = ?"}
	args := []any{partyType}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(code LIKE ? OR name LIKE ? OR phone LIKE ?)")
		args = append(args, like, like, like)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM parties WHERE "+strings.Join(conds, " AND "), args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// --- addresses --------------------------------------------------------------

const addressColumns = `id, party_id, label, recipient_name, phone,
	address_text, is_primary, sort_order, status`

func scanAddress(row *sql.Row, a *contracts.Address) error {
	var label, recipient, phone sql.NullString
	var primary int
	err := row.Scan(&a.ID, &a.PartyID, &label, &recipient, &phone,
		&a.Text, &primary, &a.SortOrder, &a.Status)
	a.Label, a.Recipient, a.Phone = label.String, recipient.String, phone.String
	a.IsPrimary = primary == 1
	return err
}

// AddressesByParty returns a party's active addresses, primary first.
func (r *Repository) AddressesByParty(ctx context.Context, partyID int64) ([]*contracts.Address, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+addressColumns+" FROM party_delivery_addresses WHERE party_id = ? AND status = 'active' ORDER BY is_primary DESC, sort_order ASC, id ASC",
		partyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Address
	for rows.Next() {
		a := &contracts.Address{}
		var label, recipient, phone sql.NullString
		var primary int
		if err := rows.Scan(&a.ID, &a.PartyID, &label, &recipient, &phone,
			&a.Text, &primary, &a.SortOrder, &a.Status); err != nil {
			return nil, err
		}
		a.Label, a.Recipient, a.Phone = label.String, recipient.String, phone.String
		a.IsPrimary = primary == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddressByID returns nil, nil when missing or foreign-owned.
func (r *Repository) AddressByID(ctx context.Context, partyID, addrID int64) (*contracts.Address, error) {
	a := &contracts.Address{}
	err := scanAddress(r.db.QueryRowContext(ctx,
		"SELECT "+addressColumns+" FROM party_delivery_addresses WHERE id = ? AND party_id = ?",
		addrID, partyID), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// CreateAddress inserts an address row.
func (r *Repository) CreateAddress(ctx context.Context, a *contracts.Address) (int64, error) {
	def := 0
	if a.IsPrimary {
		def = 1
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO party_delivery_addresses
		 (party_id, label, recipient_name, phone, address_text, is_primary, sort_order, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
		a.PartyID, nullIfEmpty(a.Label), nullIfEmpty(a.Recipient), nullIfEmpty(a.Phone),
		a.Text, def, a.SortOrder)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAddress rewrites an address row.
func (r *Repository) UpdateAddress(ctx context.Context, a *contracts.Address) error {
	def := 0
	if a.IsPrimary {
		def = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE party_delivery_addresses SET label = ?, recipient_name = ?, phone = ?,
		 address_text = ?, is_primary = ?, sort_order = ?, updated_at = UTC_TIMESTAMP(6)
		 WHERE id = ?`,
		nullIfEmpty(a.Label), nullIfEmpty(a.Recipient), nullIfEmpty(a.Phone),
		a.Text, def, a.SortOrder, a.ID)
	return err
}

// ClearPrimary clears every primary flag of the party (exclusive-primary step).
func (r *Repository) ClearPrimary(ctx context.Context, partyID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE party_delivery_addresses SET is_primary = 0 WHERE party_id = ? AND status = 'active'",
		partyID)
	return err
}

// PromoteOldestActive marks the oldest active address primary (keeps the
// exactly-one-primary invariant after the primary is archived).
func (r *Repository) PromoteOldestActive(ctx context.Context, partyID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE party_delivery_addresses SET is_primary = 1 WHERE id = (
			SELECT id FROM (SELECT id FROM party_delivery_addresses
			WHERE party_id = ? AND status = 'active' ORDER BY id ASC LIMIT 1) t)`,
		partyID)
	return err
}

// SetAddressStatus flips active/archived with timestamp.
func (r *Repository) SetAddressStatus(ctx context.Context, addrID int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE party_delivery_addresses SET status = ?,
		 archived_at = CASE WHEN ? = 'archived' THEN UTC_TIMESTAMP(6) ELSE NULL END,
		 updated_at = UTC_TIMESTAMP(6) WHERE id = ?`,
		status, status, addrID)
	return err
}

// --- member types -----------------------------------------------------------

func scanMemberType(row *sql.Row, m *contracts.MemberType) error {
	return row.Scan(&m.ID, &m.Code, &m.Name, &m.Description, &m.PriceBasis,
		&m.Direction, &m.AdjustmentType, &m.Adjustment, &m.RoundingMode,
		&m.RoundingStep, &m.Status)
}

const memberColumns = `id, code, name, description_text, price_basis,
	price_adjustment_direction, price_adjustment_type, price_adjustment_value,
	rounding_mode, rounding_increment, status`

// MemberTypeByID returns nil, nil when missing.
func (r *Repository) MemberTypeByID(ctx context.Context, id int64) (*contracts.MemberType, error) {
	m := &contracts.MemberType{}
	var desc sql.NullString
	var step sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		"SELECT "+memberColumns+" FROM member_types WHERE id = ?", id).
		Scan(&m.ID, &m.Code, &m.Name, &desc, &m.PriceBasis, &m.Direction,
			&m.AdjustmentType, &m.Adjustment, &m.RoundingMode, &step, &m.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Description = desc.String
	m.RoundingStep = step.Float64
	return m, nil
}

// MemberTypeByCode returns nil, nil when the code is free.
func (r *Repository) MemberTypeByCode(ctx context.Context, code string) (*contracts.MemberType, error) {
	m := &contracts.MemberType{}
	var desc sql.NullString
	var step sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		"SELECT "+memberColumns+" FROM member_types WHERE code = ?", code).
		Scan(&m.ID, &m.Code, &m.Name, &desc, &m.PriceBasis, &m.Direction,
			&m.AdjustmentType, &m.Adjustment, &m.RoundingMode, &step, &m.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Description = desc.String
	m.RoundingStep = step.Float64
	return m, nil
}

// ListMemberTypes searches code/name per word.
func (r *Repository) ListMemberTypes(ctx context.Context, search, status string) ([]*contracts.MemberType, error) {
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(code LIKE ? OR name LIKE ?)")
		args = append(args, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+memberColumns+" FROM member_types "+where+" ORDER BY name ASC", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.MemberType
	for rows.Next() {
		m := &contracts.MemberType{}
		var desc sql.NullString
		var step sql.NullFloat64
		if err := rows.Scan(&m.ID, &m.Code, &m.Name, &desc, &m.PriceBasis, &m.Direction,
			&m.AdjustmentType, &m.Adjustment, &m.RoundingMode, &step, &m.Status); err != nil {
			return nil, err
		}
		m.Description = desc.String
		m.RoundingStep = step.Float64
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMemberType inserts a member type row.
func (r *Repository) CreateMemberType(ctx context.Context, m *contracts.MemberType, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO member_types
		 (code, name, description_text, price_basis, price_adjustment_direction,
		  price_adjustment_type, price_adjustment_value, rounding_mode, rounding_increment,
		  status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		m.Code, m.Name, nullIfEmpty(m.Description), m.PriceBasis, m.Direction,
		m.AdjustmentType, m.Adjustment, m.RoundingMode, nullStep(m.RoundingStep),
		actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateMemberType rewrites a member type row.
func (r *Repository) UpdateMemberType(ctx context.Context, m *contracts.MemberType, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE member_types SET name = ?, description_text = ?, price_basis = ?,
		 price_adjustment_direction = ?, price_adjustment_type = ?, price_adjustment_value = ?,
		 rounding_mode = ?, rounding_increment = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		m.Name, nullIfEmpty(m.Description), m.PriceBasis, m.Direction,
		m.AdjustmentType, m.Adjustment, m.RoundingMode, nullStep(m.RoundingStep),
		actorID, m.ID)
	return err
}

// SetMemberTypeStatus flips active/archived with timestamp.
func (r *Repository) SetMemberTypeStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE member_types SET status = ?,
		 archived_at = CASE WHEN ? = 'archived' THEN UTC_TIMESTAMP(6) ELSE NULL END,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		status, status, actorID, id)
	return err
}

func nullStep(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}
