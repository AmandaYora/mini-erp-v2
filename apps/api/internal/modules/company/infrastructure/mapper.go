package infrastructure

import (
	"database/sql"
	"time"

	"mini-erp/internal/modules/company/contracts"
	"mini-erp/internal/modules/company/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

func toProfile(r sqlc.GetCompanyProfileRow) *contracts.Profile {
	return &contracts.Profile{
		ID: r.ID, Name: r.Name,
		LegalName: r.LegalName.String, Address: r.Address.String, City: r.City.String,
		Phone: r.Phone.String, Email: r.Email.String, TaxID: r.TaxID.String,
	}
}

// actor keeps the previous behaviour: the actor id is always bound (0 included).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

func insertParams(p *contracts.Profile, actorID int64) sqlc.InsertCompanyProfileParams {
	return sqlc.InsertCompanyProfileParams{
		Name: p.Name, LegalName: dbutil.NullString(p.LegalName), Address: dbutil.NullString(p.Address),
		City: dbutil.NullString(p.City), Phone: dbutil.NullString(p.Phone), Email: dbutil.NullString(p.Email),
		TaxID: dbutil.NullString(p.TaxID), CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
	}
}

func updateParams(p *contracts.Profile, actorID, id int64, now time.Time) sqlc.UpdateCompanyProfileParams {
	return sqlc.UpdateCompanyProfileParams{
		Name: p.Name, LegalName: dbutil.NullString(p.LegalName), Address: dbutil.NullString(p.Address),
		City: dbutil.NullString(p.City), Phone: dbutil.NullString(p.Phone), Email: dbutil.NullString(p.Email),
		TaxID: dbutil.NullString(p.TaxID), Now: now, UpdatedBy: actor(actorID), ID: id,
	}
}

func settingParams(key, value string, actorID int64, now time.Time) sqlc.UpsertCompanySettingParams {
	return sqlc.UpsertCompanySettingParams{
		SettingKey: key, SettingValue: dbutil.NullString(value), UpdatedBy: actor(actorID), Now: now,
	}
}
