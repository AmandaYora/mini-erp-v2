package infrastructure

import (
	"database/sql"
	"encoding/json"

	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/party/infrastructure/sqlc"
)

// toParty maps a generated party row to the public contracts type. The
// single-row projections share one field set; GetPartyByCodeRow is converted to
// GetPartyRow first. MemberCode is only filled by the list query.
func toParty(r sqlc.GetPartyRow) *contracts.Party {
	p := &contracts.Party{
		ID:         r.ID,
		Code:       r.Code,
		Type:       r.PartyType,
		Name:       r.Name,
		Phone:      r.Phone.String,
		Email:      r.Email.String,
		Address:    r.AddressText.String,
		Notes:      r.Notes.String,
		Status:     r.Status,
		Attributes: decodeAttributes(r.Attributes),
	}
	if r.ArchivedAt.Valid {
		at := r.ArchivedAt.Time.UTC()
		p.ArchivedAt = &at
	}
	if r.MemberTypeID.Valid {
		id := r.MemberTypeID.Int64
		p.MemberTypeID = &id
	}
	return p
}

// decodeAttributes never returns nil: the API always shows a (possibly
// empty) object. A corrupt value reads as empty rather than failing a read.
func decodeAttributes(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) > 0 {
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			for k, v := range m {
				switch t := v.(type) {
				case string:
					out[k] = t
				case nil:
				default:
					b, _ := json.Marshal(t) // legacy metadata may hold numbers/bools
					out[k] = string(b)
				}
			}
		}
	}
	return out
}

// encodeAttributes stores an empty map as SQL NULL.
func encodeAttributes(m map[string]string) json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// nullPtr maps a nil pointer to SQL NULL.
func nullPtr(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// actor stores the acting user id as-is (0 = system, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// nullStep maps a zero rounding step to SQL NULL.
func nullStep(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: v != 0} }

func toAddress(r sqlc.GetAddressRow) *contracts.Address {
	return &contracts.Address{
		ID:        r.ID,
		PartyID:   r.PartyID,
		Label:     r.Label.String,
		Recipient: r.RecipientName.String,
		Phone:     r.Phone.String,
		Text:      r.AddressText,
		IsPrimary: r.IsPrimary,
		SortOrder: int(r.SortOrder),
		Status:    r.Status,
	}
}

func toMemberType(r sqlc.GetMemberTypeRow) *contracts.MemberType {
	return &contracts.MemberType{
		ID:             r.ID,
		Code:           r.Code,
		Name:           r.Name,
		Description:    r.DescriptionText.String,
		PriceBasis:     r.PriceBasis,
		Direction:      r.PriceAdjustmentDirection,
		AdjustmentType: r.PriceAdjustmentType,
		Adjustment:     r.PriceAdjustmentValue,
		RoundingMode:   r.RoundingMode,
		RoundingStep:   r.RoundingIncrement.Float64,
		Status:         r.Status,
	}
}
