package bootstrap

import (
	"context"
	"time"

	"mini-erp/internal/shared/housekeeping"
)

// HousekeepingTasks builds the F4-10 retention tasks. Each table is pruned
// by the module that owns it; nothing here touches SQL. Finance and document
// data are never pruned. now is injectable for tests.
func (m *Modules) HousekeepingTasks(ret housekeeping.Retention, now func() time.Time) []housekeeping.Task {
	ret = ret.Normalized()
	return m.HousekeepingTasksDynamic(func(context.Context) housekeeping.Retention { return ret }, now)
}

// SettingsRetention resolves the windows at the moment of a sweep: env
// override > company_settings (retention.*) > default. A settings read error
// degrades to env/defaults.
func (m *Modules) SettingsRetention(ctx context.Context) housekeeping.Retention {
	settings, err := m.Company.Company().GetSettings(ctx)
	if err != nil {
		settings = nil
	}
	return housekeeping.Resolve(settings, housekeeping.EnvOverrides())
}

// HousekeepingTasksDynamic is HousekeepingTasks with the windows read per
// task run, so an edited company setting applies at the next sweep without a
// restart.
func (m *Modules) HousekeepingTasksDynamic(get func(context.Context) housekeeping.Retention, now func() time.Time) []housekeeping.Task {
	if now == nil {
		now = time.Now
	}
	cur := func(ctx context.Context) housekeeping.Retention { return get(ctx).Normalized() }
	cut := func(days int) time.Time { return housekeeping.Cutoff(now(), days) }
	return []housekeeping.Task{
		{Name: "stock_reservations", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Stock.Service().PurgeReservations(ctx, cut(cur(ctx).ReservationDays), n)
		}},
		{Name: "user_sessions", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Auth.Service().PurgeSessions(ctx, cut(cur(ctx).SessionDays), n)
		}},
		{Name: "assistant_tool_executions", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Assistant.Service().PurgeToolExecutions(ctx, cut(cur(ctx).AssistantDays), n)
		}},
		{Name: "assistant_messages", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Assistant.Service().PurgeMessages(ctx, cut(cur(ctx).AssistantDays), n)
		}},
		{Name: "assistant_runs", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Assistant.Service().PurgeRuns(ctx, cut(cur(ctx).AssistantDays), n)
		}},
		{Name: "assistant_threads", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Assistant.Service().PurgeIdleThreads(ctx, cut(cur(ctx).IdleThreadDays), n)
		}},
		{Name: "audit_logs", Run: func(ctx context.Context, n int) (int64, error) {
			return m.Audit.Service().Purge(ctx, cut(cur(ctx).AuditDays), n)
		}},
	}
}
