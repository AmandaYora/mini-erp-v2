package contracts

import "mini-erp/internal/shared/health"

// Data-health alerts (ADR-0025). Alerts are computed from live data and never
// stored: only a snooze is. Field names follow the data-health spec §3.1.

// Severity levels, most urgent first.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Areas group the alerts on the data-health page (fixed display order).
const (
	AreaFinance  = "keuangan"
	AreaStock    = "persediaan"
	AreaMaster   = "master"
	AreaDocument = "dokumen"
	AreaSecurity = "keamanan"
)

// Permission codes of the data-health surface.
const (
	PermAlertsViewAll  = "dashboard.alerts.view_all"
	PermAlertsSecurity = "dashboard.alerts.security"
)

// Check statuses reported per check.
const (
	CheckOK      = "ok"
	CheckAlert   = "alert"
	CheckPending = "pending"
	CheckError   = "error"
)

// Alert is one active data problem with its size, impact and way out.
type Alert struct {
	ID          string           `json:"id"`
	Area        string           `json:"area"`
	AreaLabel   string           `json:"areaLabel"`
	Severity    string           `json:"severity"`
	Title       string           `json:"title"`
	Impact      string           `json:"impact"`
	Count       int64            `json:"count"`
	CountUnit   string           `json:"countUnit"`
	AmountRp    *int64           `json:"amountRp"`
	AmountLabel string           `json:"amountLabel"`
	Meta        map[string]int64 `json:"meta"`
	Sample      []health.Sample  `json:"sample"`
	// Action is nil when there is nothing the session can open; Hint then
	// says what to do instead (A1 for an owner: ask the superadmin holder).
	Action     *AlertAction `json:"action"`
	Hint       string       `json:"hint,omitempty"`
	OldestAt   *string      `json:"oldestAt"`
	Snooze     AlertSnooze  `json:"snooze"`
	Tier       string       `json:"tier"`
	ComputedAt string       `json:"computedAt"`
}

// AlertAction is the filtered page that resolves the alert. The frontend
// hides it when the session lacks Perm.
type AlertAction struct {
	Label string `json:"label"`
	To    string `json:"to"`
	Perm  string `json:"perm"`
}

// AlertSnooze is the alert's snooze rule and, when snoozed, its state.
type AlertSnooze struct {
	Snoozable      bool    `json:"snoozable"`
	MaxDays        int     `json:"maxDays"`
	SnoozedAt      *string `json:"snoozedAt"`
	SnoozedUntil   *string `json:"snoozedUntil"`
	Reason         *string `json:"reason"`
	ByName         *string `json:"byName"`
	CountAtSnooze  *int64  `json:"countAtSnooze"`
	AmountAtSnooze *int64  `json:"amountAtSnooze"`
	// Reactivated is true when a snooze lapsed because the problem grew
	// (count up, or rupiah up more than 10 %) since it was snoozed.
	Reactivated bool `json:"reactivated"`
}

// BySeverity counts active alerts per level.
type BySeverity struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// FailedCheck is a check that could not run; Message is safe to show.
type FailedCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Message string `json:"message"`
}

// AlertSummary is the dashboard card: the top critical/warning alerts.
type AlertSummary struct {
	Items        []Alert       `json:"items"`
	TotalActive  int           `json:"totalActive"`
	BySeverity   BySeverity    `json:"bySeverity"`
	SnoozedCount int           `json:"snoozedCount"`
	FailedChecks []FailedCheck `json:"failedChecks"`
	Pending      bool          `json:"pending"`
	ComputedAt   string        `json:"computedAt"`
}

// CheckStatus is one check's state on the data-health page.
type CheckStatus struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Area       string `json:"area"`
	Status     string `json:"status"`
	Tier       string `json:"tier"`
	ComputedAt string `json:"computedAt"`
}

// AreaAlerts is one area's active alerts (empty when the area is healthy).
type AreaAlerts struct {
	Area      string  `json:"area"`
	AreaLabel string  `json:"areaLabel"`
	Items     []Alert `json:"items"`
}

// DataHealth is the full data-health page.
type DataHealth struct {
	Areas        []AreaAlerts  `json:"areas"`
	Snoozed      []Alert       `json:"snoozed"`
	Checks       []CheckStatus `json:"checks"`
	FailedChecks []FailedCheck `json:"failedChecks"`
	BySeverity   BySeverity    `json:"bySeverity"`
	Pending      bool          `json:"pending"`
	ComputedAt   string        `json:"computedAt"`
}
