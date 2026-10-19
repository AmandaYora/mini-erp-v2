package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/assistant/infrastructure"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// Service owns the assistant bot: intent engine, WhatsApp gateway liaison,
// authorizations, config, and run history. Chat/simulate/inbound runs are
// exempt from the audit trail (legacy rule: runs + tool executions ARE the
// trail); the sensitive management acts below ARE trailed (F1-11): channel
// connect/disconnect/reset, authorization create/update/revoke, config
// updates.
type Service struct {
	repo    *infrastructure.Repository
	engine  *Engine
	gateway *infrastructure.Gateway
	log     *slog.Logger
	audit   auditcontracts.AuditClient
	// users resolves a console operator's branch access (AS-01). Set by the
	// composition root (user is built before assistant, but a setter keeps the
	// constructor stable for the dev seed).
	users usercontracts.UserClient

	rateMu sync.Mutex
	rates  map[string][]time.Time

	// base is the application lifecycle context: inbound work derives from it
	// so shutdown cancels it; inflight lets Close wait for running replies.
	base     context.Context
	inflight sync.WaitGroup
}

// SetUserAccess injects the user contract used to bound console/simulate runs
// to the operator's own branches.
func (s *Service) SetUserAccess(u usercontracts.UserClient) { s.users = u }

// scopeFor builds the run scope. A signed-in operator (actorID != 0) is bound
// to their own branch access — fail closed if the lookup is unavailable;
// system callers and whitelisted WhatsApp senders (actorID 0) are not.
func (s *Service) scopeFor(ctx context.Context, actorID, branchID int64) (Scope, error) {
	sc := Scope{BranchID: branchID}
	if actorID == 0 {
		return sc, nil
	}
	sc.Restricted = true
	if s.users == nil {
		return sc, nil
	}
	access, err := s.users.GetBranchAccess(ctx, actorID)
	if err != nil {
		return sc, apperror.Internal(err)
	}
	for _, a := range access {
		sc.Allowed = append(sc.Allowed, a.BranchID)
	}
	return sc, nil
}

// SetContext binds inbound handling and the gateway to the app lifecycle.
func (s *Service) SetContext(ctx context.Context) {
	s.base = ctx
	s.gateway.SetContext(ctx)
}

// Close stops the assistant: waits (bounded by ctx) for in-flight inbound
// replies, then closes the gateway. Idempotent.
func (s *Service) Close(ctx context.Context) {
	done := make(chan struct{})
	go func() { s.inflight.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	s.gateway.Close()
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
	audit auditcontracts.AuditClient,
) *Service {
	s := &Service{
		repo: repo, gateway: gateway, log: log, audit: audit,
		engine: NewEngine(branches, products, sales, purchasing, stock, finance),
		rates:  map[string][]time.Time{},
		base:   context.Background(),
	}
	gateway.OnMessage = s.HandleInbound
	return s
}

// trail records a sensitive management act best-effort (F1-11): audit
// failure never fails the operation. Entity names follow the F3b-9 design
// (assistant_channel / assistant_authorization / assistant_config).
func (s *Service) trail(ctx context.Context, action, entity string, entityID, actorID int64, note string) {
	s.trailDiff(ctx, action, entity, entityID, actorID, note, nil, nil)
}

// trailDiff is trail with before/after detail (F3b-8). Phones in the diff
// must already be masked by the caller.
func (s *Service) trailDiff(ctx context.Context, action, entity string, entityID, actorID int64, note string, before, after any) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{
		Action: action, Entity: entity, EntityID: entityID, ActorID: actorID, Note: note,
		Before: before, After: after,
	})
}

// authView is the audit-safe shape of an authorization (phone masked).
func authView(a *contracts.Authorization) map[string]any {
	if a == nil {
		return nil
	}
	return map[string]any{"phone": "…" + last4(a.Phone), "name": a.Name, "accessLevel": a.AccessLevel, "status": a.Status}
}

func (s *Service) authByID(ctx context.Context, id int64) *contracts.Authorization {
	rows, err := s.repo.ListAuthorizations(ctx, true)
	if err != nil {
		return nil
	}
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// last4 returns the last 4 digits for audit notes: enough to tell numbers
// apart, never the full number (F1-11).
func last4(phone string) string {
	digits := normalizePhone(phone)
	if len(digits) <= 4 {
		return digits
	}
	return digits[len(digits)-4:]
}

// --- console & simulate ---------------------------------------------------------

// Ask runs the engine without any channel requirement (in-app console).
// It implements contracts.AssistantClient.
func (s *Service) Ask(ctx context.Context, actorID, branchID int64, message string) (*contracts.Answer, error) {
	if strings.TrimSpace(message) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "message", Message: "pesan wajib diisi"}})
	}
	sc, err := s.scopeFor(ctx, actorID, branchID)
	if err != nil {
		return nil, err
	}
	if err := s.engine.CheckScope(ctx, sc); err != nil {
		return nil, err
	}
	return s.execute(ctx, 0, "", sc, actorID, strings.TrimSpace(message))
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
	sc, err := s.scopeFor(ctx, actorID, branchID)
	if err != nil {
		return nil, err
	}
	if err := s.engine.CheckScope(ctx, sc); err != nil {
		return nil, err
	}
	ans, err := s.execute(ctx, 0, phone, sc, actorID, strings.TrimSpace(message))
	if err != nil {
		return nil, err
	}
	return ans, nil
}

// execute runs detection → tools, persisting the run and tool rows.
// Tool failures record a failed run and return a friendly message (legacy
// rethrew HTTP-500 with no answer — never do that to a chat caller).
func (s *Service) execute(ctx context.Context, threadID int64, phone string, sc Scope, actorID int64, message string) (*contracts.Answer, error) {
	t0 := time.Now()
	out, execErr := s.engine.Run(ctx, sc, message)
	if out == nil {
		out = &Outcome{}
	}
	intent, answer, calls := out.Intent, out.Answer, out.Calls
	// The run records the branch it was actually narrowed to (explicit pick or
	// named in the text); several branches stay 0.
	branchID, branchName := out.BranchID, out.BranchName
	if branchName == "" {
		branchName = "Semua cabang"
	}
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
		outJSON, _ := json.Marshal(c.Output)
		if c.Output == nil {
			outJSON = nil
		}
		_ = s.repo.InsertToolExecution(ctx, runID, i+1, c.Tool, string(in), trunc(string(outJSON), 2000), c.DurationMs, c.Status, trunc(c.Error, 1000))
	}
	_ = s.repo.FinishRun(ctx, runID, status, answer, time.Since(t0).Milliseconds(), failure)
	if execErr != nil {
		s.log.Error("assistant run failed", "intent", intent, "error", execErr)
	}
	return &contracts.Answer{
		RunID: runID, BranchID: branchID, BranchName: branchName,
		Intent: intent, Mode: contracts.ModeRule, Text: answer,
	}, nil
}

// --- inbound WhatsApp -------------------------------------------------------------

// HandleInbound processes one DM from the socket (gateway goroutine).
// Every exit is terminal: rate-cut and unauthorized senders get a reply
// but no run row; answered senders get thread/message/run rows.
func (s *Service) HandleInbound(phone, text string) {
	s.inflight.Add(1)
	defer s.inflight.Done()
	ctx, cancel := context.WithTimeout(s.base, 60*time.Second)
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

	ans, err := s.execute(ctx, threadID, phone, Scope{}, 0, text)
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
	// Numbers that stopped writing must not pile up forever (F3b-9).
	if len(s.rates) > 10000 {
		s.rates = map[string][]time.Time{}
	}
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
func (s *Service) Connect(ctx context.Context, actorID int64) error {
	if err := s.gateway.Connect(ctx); err != nil {
		msg := err.Error()
		_ = s.repo.SetChannel(ctx, "disconnected", s.gateway.Phone(), msg)
		return apperror.Conflict(msg)
	}
	_ = s.repo.SetChannel(ctx, "connecting", s.gateway.Phone(), "")
	s.trail(ctx, "assistant.channel_connect", "assistant_channel", 0, actorID, "…"+last4(s.gateway.Phone()))
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
func (s *Service) Disconnect(ctx context.Context, actorID int64) error {
	s.gateway.Disconnect()
	_ = s.repo.SetChannel(ctx, "disconnected", s.gateway.Phone(), "")
	s.trail(ctx, "assistant.channel_disconnect", "assistant_channel", 0, actorID, "…"+last4(s.gateway.Phone()))
	return nil
}

// ResetSession wipes the session: the next connect needs a fresh QR scan.
func (s *Service) ResetSession(ctx context.Context, actorID int64) error {
	if err := s.gateway.Reset(ctx); err != nil {
		return apperror.Internal(err)
	}
	_ = s.repo.SetChannel(ctx, "disconnected", "", "")
	s.trail(ctx, "assistant.channel_reset", "assistant_channel", 0, actorID, "")
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
				s.trail(ctx, "assistant.authorization_create", "assistant_authorization", a.ID, actorID, "reaktivasi …"+last4(a.Phone))
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
	s.trailDiff(ctx, "assistant.authorization_create", "assistant_authorization", a.ID, actorID, "…"+last4(a.Phone), nil, authView(a))
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
	before := authView(s.authByID(ctx, id))
	ok, err := s.repo.UpdateAuthorization(ctx, a, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !ok {
		return nil, apperror.NotFound("Nomor WhatsApp")
	}
	s.trailDiff(ctx, "assistant.authorization_update", "assistant_authorization", a.ID, actorID, "…"+last4(a.Phone), before, authView(a))
	return s.afterPrimary(ctx, a)
}

// RevokeAuthorization soft-revokes a row (never deletes history).
func (s *Service) RevokeAuthorization(ctx context.Context, actorID, id int64) error {
	if id <= 0 {
		return apperror.NotFound("Nomor WhatsApp")
	}
	before := authView(s.authByID(ctx, id))
	ok, err := s.repo.Revoke(ctx, id, actorID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !ok {
		return apperror.NotFound("Nomor WhatsApp")
	}
	s.trailDiff(ctx, "assistant.authorization_revoke", "assistant_authorization", id, actorID, "", before, map[string]any{"status": contracts.AuthRevoked})
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
		// Only an owner can be the primary owner: a flag on an authorized
		// party would persist without ever demoting the real primary (N-43).
		AccessLevel: level, Status: contracts.AuthActive,
		IsPrimaryOwner: in.IsPrimaryOwner && level == contracts.AccessOwner,
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
func (s *Service) UpdateConfig(ctx context.Context, actorID int64, mode string, rateLimit int) (*BotConfig, error) {
	if mode != contracts.ModeRule {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "mode", Message: "mode tidak dikenal"}})
	}
	if rateLimit < 1 || rateLimit > 120 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "rateLimitPerMinute", Message: "batas 1–120 per menit"}})
	}
	var before any
	if old, err := s.repo.GetConfig(ctx); err == nil && old != nil {
		before = map[string]any{"mode": old.Mode, "rateLimit": old.RateLimitPerMin}
	}
	if err := s.repo.UpdateConfig(ctx, mode, rateLimit); err != nil {
		return nil, apperror.Internal(err)
	}
	s.trailDiff(ctx, "assistant.config_update", "assistant_config", 0, actorID, mode,
		before, map[string]any{"mode": mode, "rateLimit": rateLimit})
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
