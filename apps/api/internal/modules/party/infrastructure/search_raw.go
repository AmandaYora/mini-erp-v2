package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/party/infrastructure/sqlc"
	"mini-erp/internal/shared/listquery"
)

// The free-text filters below AND one (col LIKE ? OR ...) group per
// whitespace-separated search word, so the number of conditions follows the
// number of words: sqlc.narg / sqlc.slice cannot express that (ADR-0012,
// scripts/raw-sql-allow.json).

// partySearchCols are the columns a free-text party search matches (F3a-5:
// email and address join code/name/phone).
var partySearchCols = []string{"code", "name", "phone", "email", "address_text"}

// PartyFilter is the party list filter set.
type PartyFilter struct {
	Type   string // "" matches both kinds
	Search string
	Status string
	// DuplicateName keeps parties whose name (case and outer spaces ignored)
	// is shared by another ACTIVE party of type Type (data-health A10).
	DuplicateName bool
}

// partyWhere is the single source of truth of the party list filter, shared
// by ListParties and CountParties (and SearchIDs). prefix is a table alias
// with trailing dot ("p.") or "".
func partyWhere(prefix string, f PartyFilter) (string, []any) {
	cols := make([]string, len(partySearchCols))
	for i, c := range partySearchCols {
		cols[i] = prefix + c
	}
	w := listquery.New().
		Eq(prefix+"party_type", f.Type).
		Eq(prefix+"status", f.Status).
		Like(f.Search, cols...)
	if f.DuplicateName {
		w.Raw("LOWER(TRIM("+prefix+"name)) IN (SELECT n FROM (SELECT LOWER(TRIM(name)) AS n FROM parties"+
			" WHERE status = 'active' AND party_type = ? GROUP BY n HAVING COUNT(*) > 1) d)", f.Type)
	}
	return w.SQL()
}

// ListParties searches code/name/phone per word, filtered by type + status.
func (r *Repository) ListParties(ctx context.Context, partyType, search, status string, limit, offset int) ([]*contracts.Party, error) {
	return r.ListPartiesFiltered(ctx, PartyFilter{Type: partyType, Search: search, Status: status}, limit, offset)
}

// ListPartiesFiltered lists one page of the filter set. Member codes ride
// along via LEFT JOIN — one query, no per-row lookup.
func (r *Repository) ListPartiesFiltered(ctx context.Context, f PartyFilter, limit, offset int) ([]*contracts.Party, error) {
	where, args := partyWhere("p.", f)
	args = append(args, limit, offset)
	// raw-sql: variable-arity per-word LIKE filter (listquery) shared with CountParties/SearchIDs
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.code, p.party_type, p.name, p.phone, p.email, p.address_text,
			p.notes, p.member_type_id, p.status, m.code, COALESCE(p.attributes, '{}'), p.archived_at
		 FROM parties p LEFT JOIN member_types m ON m.id = p.member_type_id
		 WHERE `+where+
			` ORDER BY p.name ASC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Party
	for rows.Next() {
		var row sqlc.GetPartyRow
		var memberCode sql.NullString
		if err := rows.Scan(&row.ID, &row.Code, &row.PartyType, &row.Name, &row.Phone, &row.Email,
			&row.AddressText, &row.Notes, &row.MemberTypeID, &row.Status, &memberCode, &row.Attributes, &row.ArchivedAt); err != nil {
			return nil, err
		}
		p := toParty(row)
		p.MemberCode = memberCode.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountParties counts the List filter set.
func (r *Repository) CountParties(ctx context.Context, partyType, search, status string) (int64, error) {
	return r.CountPartiesFiltered(ctx, PartyFilter{Type: partyType, Search: search, Status: status})
}

// CountPartiesFiltered counts the ListPartiesFiltered filter set.
func (r *Repository) CountPartiesFiltered(ctx context.Context, f PartyFilter) (int64, error) {
	where, args := partyWhere("", f)
	var total int64
	// raw-sql: same variable-arity filter as ListParties
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM parties WHERE "+where, args...).Scan(&total)
	return total, err
}

// SearchIDs returns up to limit ids of parties matching term (all statuses),
// oldest id first. partyType "" = both kinds.
func (r *Repository) SearchIDs(ctx context.Context, partyType, term string, limit int) ([]int64, error) {
	where, args := partyWhere("", PartyFilter{Type: partyType, Search: term})
	args = append(args, limit)
	// raw-sql: same variable-arity filter as ListParties
	rows, err := r.db.QueryContext(ctx,
		"SELECT id FROM parties WHERE "+where+" ORDER BY id LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
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
	// raw-sql: variable-arity per-word LIKE filter (one (code|name) group per search word)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, description_text, price_basis, price_adjustment_direction,
			price_adjustment_type, price_adjustment_value, rounding_mode, rounding_increment, status
		 FROM member_types `+where+" ORDER BY name ASC", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.MemberType
	for rows.Next() {
		var row sqlc.GetMemberTypeRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.DescriptionText, &row.PriceBasis,
			&row.PriceAdjustmentDirection, &row.PriceAdjustmentType, &row.PriceAdjustmentValue,
			&row.RoundingMode, &row.RoundingIncrement, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, toMemberType(row))
	}
	return out, rows.Err()
}
