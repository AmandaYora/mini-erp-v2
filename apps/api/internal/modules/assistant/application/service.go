package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/assistant/infrastructure"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
)

// Service owns the assistant bot: intent engine, WhatsApp gateway liaison,
// authorizations, config, and run history. Assistant writes are exempt from
// the audit trail (legacy rule): runs + tool executions ARE the trail.
type Service struct {
	repo    *infrastructure.Repository
	engine  *Engine
	gateway *infrastructure.Gateway
	log     *slog.Logger

	rateMu sync.Mutex
	rates  map[string][]time.Time
}

// NewService wires the assistant use cases. Gateway callback is attached
// here so inbound socket events land in HandleInbound.
func NewService(
	repo *infrastructure.Repository,
	gateway *infrastructure.Gateway,
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	stock stockcontracts.StockClient,
	finance financecontracts.FinanceClient,
	log *slog.Logger,
) *Service {
	s := &Service{
		repo: repo, gateway: gateway, log: log,
		engine: NewEngine(branches, products, sales, purchasing, stock, finance),
		rates:  map[string][]time.Time{},
	}
	gateway.OnMessage = s.HandleInbound
	return s
}

// --- console & simulate ---------------------------------------------------------

// Ask runs the engine without any channel requirement (in-app console).
// It implements contracts.AssistantClient.
func (s *Service) Ask(ctx context.Context, actorID, branchID int64, message string) (*contracts.Answer, error) {
	if strings.TrimSpace(message) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "message", Message: "pesan wajib diisi"}})
	}
	if err := s.engine.CheckBranch(ctx, branchID); err != nil {
		return nil, err
	}
	return s.execute(ctx, 0, "", branchID, actorID, strings.TrimSpace(message))
}

// Simulate runs the full inbound pipeline for a phone number without
// sending anything (operator test console). Unlike legacy, it requires a
// connected channel too (KI-136: the real path must not skip the check
// the simulator enforces).
func (s *Service) Simulate(ctx context.Context, actorID, branchID int64, phone, message string) (*contracts.Answer, error) {
	phone = normalizePhone(phone)
	if !validPhone(phone) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "phone", Message: "Nomor WhatsApp tidak valid"}})
	}
	if strings.TrimSpace(message) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "message", Message: "pesan wajib diisi"}})
	}
	auth, err := s.repo.FindActiveByPhone(ctx, phone)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if auth == nil {
		return nil, apperror.Forbidden()
	}
	if !s.gateway.Connected() {
		return nil, apperror.Conflict("Kanal WhatsApp belum terhubung")
	}
	_ = s.repo.TouchSeen(ctx, auth.ID)
	if err := s.engine.CheckBranch(ctx, branchID); err != nil {
		return nil, err
	}
	ans, err := s.execute(ctx, 0, phone, branchID, actorID, strings.TrimSpace(message))
	if err != nil {
		return nil, err
	}
	return ans, nil
}

// execute runs detection → tools, persisting the run and tool rows.
// Tool failures record a failed run and return a friendly message (legacy
// rethrew HTTP-500 with no answer — never do that to a chat caller).
func (s *Service) execute(ctx context.Context, threadID int64, phone string, branchID, actorID int64, message string) (*contracts.Answer, error) {
	t0 := time.Now()
	intent, answer, calls, execErr := s.engine.Execute(ctx, branchID, message)
	status := contracts.RunCompleted
	failure := ""
	if execErr != nil {
		status = contracts.RunFailed
		failure = trunc(execErr.Error(), 200)
		answer = "Maaf, gagal mengambil data. Coba lagi sebentar."
	}
	runID, err := s.repo.CreateRun(ctx, threadID, branchID, actorID, phone, intent, contracts.ModeRule)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for i, c := range calls {
		in, _ := json.Marshal(c.Input)
		out, _ := json.Marshal(c.Output)
		_ = s.repo.InsertToolExecution(ctx, runID, i+1, c.Tool, string(in), trunc(string(out), 2000), c.DurationMs)
	}
	_ = s.repo.FinishRun(ctx, runID, status, answer, time.Since(t0).Milliseconds(), failure)
	if execErr != nil {
		s.log.Error("assistant run failed", "intent", intent, "error", execErr)
	}
	return &contracts.Answer{
		RunID: runID, BranchID: branchID, BranchName: s.engine.BranchLabel(ctx, branchID),
		Intent: intent, Mode: contracts.ModeRule, Text: answer,
	}, nil
}

func (s *Service) branchName(ctx context.Context, branchID int64) string {
	if branchID == 0 {
		return "Semua cabang"
	}
	// branches client is inside the engine; resolve cheaply via ListAll.
	// A miss reads as the raw id (never fail an answer over a label).
	return fmt.Sprintf("Cabang %d", branchID)
}

// --- inbound WhatsApp -------------------------------------------------------------

// HandleInbound processes one DM from the socket (gateway goroutine).
// Every exit is terminal: rate-cut and unauthorized senders get a reply
// but no run row; answered senders get thread/message/run rows.
func (s *Service) HandleInbound(phone, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		s.log.Error("assistant config", "error", err)
		return
	}
	if !s.allowRate(phone, cfg.RateLimitPerMin) {
		_ = s.gateway.SendText(ctx, phone, "Terlalu banyak pesan dalam waktu singkat. Silakan tunggu sebentar.")
		return
	}
	auth, err := s.repo.FindActiveByPhone(ctx, phone)
	if err != nil {
		s.log.Error("assistant auth", "error", err)
		return
	}
	if auth == nil {
		// KI-137 fix: revoked/unknown senders hear why (legacy stayed silent).
		taken, status, err := s.repo.PhoneTaken(ctx, phone, 0)
		msg := "Nomor belum diotorisasi untuk memakai asisten."
		if err == nil && taken && status == contracts.AuthRevoked {
			msg = "Akses nomor ini sudah dicabut. Hubungi admin."
		}
		_ = s.gateway.SendText(ctx, phone, msg)
		return
	}
	_ = s.repo.TouchSeen(ctx, auth.ID)

	threadID, err := s.repo.FindOpenThread(ctx, phone)
	if err != nil {
		s.log.Error("assistant thread", "error", err)
		return
	}
	if threadID == 0 {
		threadID, err = s.repo.CreateThread(ctx, phone, 0)
		if err != nil {
			s.log.Error("assistant thread", "error", err)
			return
		}
	}
	msgID, err := s.repo.InsertMessage(ctx, threadID, "in", text, "received", 0)
	if err != nil {
		s.log.Error("assistant message", "error", err)
		return
	}
	_ = s.repo.TouchThread(ctx, threadID)

	ans, err := s.execute(ctx, threadID, phone, 0, 0, text)
	if err != nil {
		_ = s.repo.SetMessageStatus(ctx, msgID, "failed", 0)
		_ = s.gateway.SendText(ctx, phone, "Maaf, gagal mengambil data. Coba lagi sebentar.")
		return
	}
	if err := s.gateway.SendText(ctx, phone, ans.Text); err != nil {
		s.log.Error("assistant send", "phone", phone, "error", err)
		_ = s.repo.SetMessageStatus(ctx, msgID, "failed", ans.RunID)
		return
	}
	_, _ = s.repo.InsertMessage(ctx, threadID, "out", ans.Text, "processed", ans.RunID)
	_ = s.repo.SetMessageStatus(ctx, msgID, "processed", ans.RunID)
}

// allowRate is a sliding-window limiter (in-memory, resets on restart).
func (s *Service) allowRate(phone string, perMin int) bool {
	if perMin <= 0 {
		perMin = 10
	}
	now := time.Now()
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	kept := s.rates[phone][:0]
	for _, t := range s.rates[phone] {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= perMin {
		s.rates[phone] = kept
		return false
	}
	s.rates[phone] = append(kept, now)
	return true
}

// --- channel ------------------------------------------------------------------------

// ChannelStatus merges socket truth with the persisted row for UI polling.
func (s *Service) ChannelStatus(ctx context.Context) (*contracts.ChannelStatus, error) {
	st, err := s.repo.GetChannel(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &contracts.ChannelStatus{
		Connected: s.gateway.Connected(),
		Phone:     firstNonEmpty(s.gateway.Phone(), st.Phone),
		QRDataURL: s.gateway.QRDataURL(),
		LastError: firstNonEmpty(s.gateway.LastError(), st.LastError),
		UpdatedAt: st.UpdatedAt,
	}, nil
}

// Connect starts the socket (QR while unpaired, silent login when paired).
// Gateway failures keep their Indonesian message via Conflict so the
// operator sees the reason (Internal would hide it by design).
func (s *Service) Connect(ctx context.Context) error {
	if err := s.gateway.Connect(ctx); err != nil {
		msg := err.Error()
		_ = s.repo.SetChannel(ctx, "disconnected", s.gateway.Phone(), msg)
		return apperror.Conflict(msg)
	}
	_ = s.repo.SetChannel(ctx, "connecting", s.gateway.Phone(), "")
	return nil
}

// Boot attempts a silent reconnect when a session was paired before.
// Best-effort: failures only log (server must boot without a phone).
func (s *Service) Boot(ctx context.Context) {
	if !s.gateway.Paired(ctx) {
		return
	}
	if err := s.gateway.Connect(ctx); err != nil {
		s.log.Error("whatsapp boot reconnect", "error", err)
		_ = s.repo.SetChannel(ctx, "disconnected", "", err.Error())
		return
	}
	_ = s.repo.SetChannel(ctx, "connected", s.gateway.Phone(), "")
}

// Disconnect drops the socket, keeping the session (reconnect needs no QR).
func (s *Service) Disconnect(ctx context.Context) error {
	s.gateway.Disconnect()
	_ = s.repo.SetChannel(ctx, "disconnected", s.gateway.Phone(), "")
	return nil
}

// ResetSession wipes the session: the next connect needs a fresh QR scan.
func (s *Service) ResetSession(ctx context.Context) error {
	if err := s.gateway.Reset(ctx); err != nil {
		return apperror.Internal(err)
	}
	_ = s.repo.SetChannel(ctx, "disconnected", "", "")
	return nil
}

// --- authorizations -------------------------------------------------------------------

// AuthorizationInput is the validated whitelist payload.
type AuthorizationInput struct {
	Phone          string
	Name           string
	AccessLevel    string
	IsPrimaryOwner bool
}

// ListAuthorizations returns active rows (or all with revoked).
func (s *Service) ListAuthorizations(ctx context.Context, includeRevoked bool) ([]*contracts.Authorization, error) {
	rows, err := s.repo.ListAuthorizations(ctx, includeRevoked)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.Authorization{}
	}
	return rows, nil
}

// CreateAuthorization whitelists a sender. A revoked number with the same
// digits reactivates in place (legacy); the first owner without an active
// primary becomes primary automatically.
func (s *Service) CreateAuthorization(ctx context.Context, actorID int64, in AuthorizationInput) (*contracts.Authorization, error) {
	a, appErr := s.validateAuthorization(in)
	if appErr != nil {
		return nil, appErr
	}
	taken, status, err := s.repo.PhoneTaken(ctx, a.Phone, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if taken && status == contracts.AuthActive {
		return nil, apperror.Conflict("Nomor " + a.Phone + " sudah terdaftar")
	}
	if taken && status == contracts.AuthRevoked {
		existing, err := s.repo.ListAuthorizations(ctx, true)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, e := range existing {
			if e.Phone == a.Phone {
				a.ID = e.ID
				if err := s.repo.Reactivate(ctx, e.ID, a, actorID); err != nil {
					return nil, apperror.Internal(err)
				}
				return s.afterPrimary(ctx, a)
			}
		}
	}
	if a.AccessLevel == contracts.AccessOwner && !s.hasActivePrimary(ctx) {
		a.IsPrimaryOwner = true
	}
	id, err := s.repo.CreateAuthorization(ctx, a, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	a.ID = id
	return s.afterPrimary(ctx, a)
}

// UpdateAuthorization rewrites an active row (revoked reads as missing:
// re-add the number to reactivate it).
func (s *Service) UpdateAuthorization(ctx context.Context, actorID, id int64, in AuthorizationInput) (*contracts.Authorization, error) {
	if id <= 0 {
		return nil, apperror.NotFound("Nomor WhatsApp")
	}
	a, appErr := s.validateAuthorization(in)
	if appErr != nil {
		return nil, appErr
	}
	taken, _, err := s.repo.PhoneTaken(ctx, a.Phone, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if taken {
		return nil, apperror.Conflict("Nomor " + a.Phone + " sudah terdaftar")
	}
	a.ID = id
	ok, err := s.repo.UpdateAuthorization(ctx, a, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !ok {
		return nil, apperror.NotFound("Nomor WhatsApp")
	}
	return s.afterPrimary(ctx, a)
}

// RevokeAuthorization soft-revokes a row (never deletes history).
func (s *Service) RevokeAuthorization(ctx context.Context, actorID, id int64) error {
	if id <= 0 {
		return apperror.NotFound("Nomor WhatsApp")
	}
	ok, err := s.repo.Revoke(ctx, id, actorID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !ok {
		return apperror.NotFound("Nomor WhatsApp")
	}
	return nil
}

// afterPrimary enforces the single-primary invariant for owner primaries.
func (s *Service) afterPrimary(ctx context.Context, a *contracts.Authorization) (*contracts.Authorization, error) {
	if a.IsPrimaryOwner && a.AccessLevel == contracts.AccessOwner {
		if err := s.repo.DemoteOthers(ctx, a.ID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	return a, nil
}

func (s *Service) hasActivePrimary(ctx context.Context) bool {
	rows, err := s.repo.ListAuthorizations(ctx, false)
	if err != nil {
		return false
	}
	for _, r := range rows {
		if r.IsPrimaryOwner {
			return true
		}
	}
	return false
}

func (s *Service) validateAuthorization(in AuthorizationInput) (*contracts.Authorization, *apperror.AppError) {
	phone := normalizePhone(in.Phone)
	if !validPhone(phone) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "phone", Message: "Nomor WhatsApp tidak valid"}})
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "nama wajib diisi"}})
	}
	level := in.AccessLevel
	if level != contracts.AccessOwner && level != contracts.AccessAuthorizedParty {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "accessLevel", Message: "level tidak dikenal"}})
	}
	return &contracts.Authorization{
		Phone: phone, Name: strings.TrimSpace(in.Name),
		AccessLevel: level, Status: contracts.AuthActive, IsPrimaryOwner: in.IsPrimaryOwner,
	}, nil
}

// --- config & stats ---------------------------------------------------------------------

// BotConfig is the tuning view (effective mode included, KI-141).
type BotConfig struct {
	Mode            string
	EffectiveMode   string
	RateLimitPerMin int
	UpdatedAt       string
}

// GetConfig returns tuning plus the effective mode.
func (s *Service) GetConfig(ctx context.Context) (*BotConfig, error) {
	cfg, err := s.repo.GetConfig(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &BotConfig{
		Mode: cfg.Mode, EffectiveMode: contracts.ModeRule,
		RateLimitPerMin: cfg.RateLimitPerMin, UpdatedAt: cfg.UpdatedAt,
	}, nil
}

// UpdateConfig rewrites tuning. Only rule_based exists without AI: other
// modes are rejected explicitly (legacy normalized silently).
func (s *Service) UpdateConfig(ctx context.Context, mode string, rateLimit int) (*BotConfig, error) {
	if mode != contracts.ModeRule {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "mode", Message: "mode tidak dikenal"}})
	}
	if rateLimit < 1 || rateLimit > 120 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "rateLimitPerMinute", Message: "batas 1–120 per menit"}})
	}
	if err := s.repo.UpdateConfig(ctx, mode, rateLimit); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.GetConfig(ctx)
}

// RunStats aggregates the last 7 days per day×intent×mode.
func (s *Service) RunStats(ctx context.Context) ([]*contracts.RunStat, error) {
	rows, err := s.repo.RunStats(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.RunStat{}
	}
	return rows, nil
}

// --- helpers ------------------------------------------------------------------------------

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validPhone(s string) bool {
	if len(s) < 8 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
