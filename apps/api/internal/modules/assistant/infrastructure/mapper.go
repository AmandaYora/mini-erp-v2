package infrastructure

import (
	"database/sql"
	"time"

	"mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/assistant/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// timeLayout renders DATETIME(6) without microseconds for JSON readers
// (what DATE_FORMAT(col, '%Y-%m-%d %H:%i:%s') produced before sqlc).
const timeLayout = "2006-01-02 15:04:05"

func toChannelState(r sqlc.GetChannelRow) *ChannelState {
	return &ChannelState{
		State: r.State, Phone: r.Phone.String, LastError: r.LastError.String,
		UpdatedAt: r.UpdatedAt.Format(timeLayout),
	}
}

func toBotConfig(r sqlc.GetConfigRow) *BotConfig {
	return &BotConfig{
		Mode: r.Mode, RateLimitPerMin: int(r.RateLimitPerMinute), UpdatedAt: r.UpdatedAt.Format(timeLayout),
	}
}

// toAuthorization maps the (identical) list/find row projections.
func toAuthorization(id int64, phone, name, level, status string, primary bool, seen sql.NullTime) *contracts.Authorization {
	return &contracts.Authorization{
		ID: id, Phone: phone, Name: name, AccessLevel: level, Status: status,
		IsPrimaryOwner: primary, LastSeenAt: dbutil.FormatNullTime(seen),
	}
}

func toRunStat(r sqlc.RunStatsRow) *contracts.RunStat {
	return &contracts.RunStat{
		// Scanning a DATE into a string rendered RFC3339 before sqlc; kept for the API.
		Day: r.Day.Format(time.RFC3339Nano), Intent: r.Intent, Mode: r.Mode,
		Total: r.Total, Success: r.Success, AvgDurationMs: int64(r.AvgDurationMs + 0.5),
	}
}
