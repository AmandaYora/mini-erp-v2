package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Access levels for WhatsApp sender authorizations.
const (
	AccessOwner           = "owner"
	AccessAuthorizedParty = "authorized_party"
)

// Authorization statuses.
const (
	AuthActive  = "active"
	AuthRevoked = "revoked"
)

// Run statuses and modes.
const (
	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"
	ModeRule     = "rule_based"
)

// Authorization is one whitelisted WhatsApp sender.
type Authorization struct {
	ID             int64
	Phone          string
	Name           string
	AccessLevel    string
	Status         string
	IsPrimaryOwner bool
	LastSeenAt     string
}

// ChannelStatus is the WhatsApp gateway state for setup UI polling.
type ChannelStatus struct {
	Connected bool
	Phone     string
	QRDataURL string
	LastError string
	UpdatedAt string
}

// Answer is one executed assistant reply (console, simulate, inbound).
type Answer struct {
	RunID      int64
	BranchID   int64
	BranchName string
	Intent     string
	Mode       string
	Text       string
}

// RunStat is one day×intent×mode aggregate for the stats UI.
type RunStat struct {
	Day           string
	Intent        string
	Mode          string
	Total         int64
	Success       int64
	AvgDurationMs int64
}

// AssistantClient is the public surface of the assistant module. No other
// module consumes it yet; it exists so the composition root and future
// notifiers share one contract instead of internals.
type AssistantClient interface {
	// Ask runs the intent engine without any channel requirement
	// (in-app console, system-initiated summaries).
	Ask(ctx context.Context, actorID, branchID int64, message string) (*Answer, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "assistant.view", Name: "Asisten: Lihat"},
		{Code: "assistant.manage", Name: "Asisten: Kelola"},
	}
}
