package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/database"
	auditapp "mini-erp/internal/modules/audit/application"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/server"
	"mini-erp/internal/shared/housekeeping"
	"mini-erp/internal/shared/logger"
	"mini-erp/internal/shared/signedurl"
)

// version disematkan saat build Docker via -ldflags -X main.version=<gitsha>
// (F6-1). Default dev untuk build lokal.
var version = "dev"

func main() {
	log := logger.New().With("service", "mini-erp", "version", version, "env", os.Getenv("APP_ENV"))

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.OpenWithPool(cfg.DBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBLife, cfg.DBIdleTime)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// F6-3: pagar versi migrasi — biner baru menolak jalan di schema lama.
	// Migrasi dijalankan sebagai langkah deploy eksplisit
	// (docker compose --profile ops run --rm migrate up), bukan saat start.
	if dir, derr := database.FindMigrationsDir(); derr != nil {
		log.Warn("pagar versi migrasi dilewati", "error", derr)
	} else if expected, verr := database.MaxMigrationVersion(dir); verr != nil {
		log.Warn("pagar versi migrasi dilewati", "error", verr)
	} else if warn, skipped, cerr := database.CheckMigrationVersion(context.Background(), db, expected, cfg.Env == "production"); cerr != nil {
		log.Error("versi schema database", "error", cerr)
		os.Exit(1)
	} else if skipped {
		log.Warn("pagar versi migrasi dilewati (tanpa tabel schema_migrations; dev/test)", "expected", expected)
	} else if warn != "" {
		log.Warn("versi schema database", "warning", warn, "expected", expected)
	} else {
		log.Info("versi schema database cocok", "version", expected)
	}

	// Storage layout: uploads under MediaDir (served, signed URLs), secrets such
	// as the WhatsApp session under SessionDir (never served). Files left by the
	// old layout, where the whole storage root was public, are moved once.
	moved, skipped, err := config.PrepareStorageLayout(cfg)
	if err != nil {
		log.Error("prepare storage layout", "error", err)
		os.Exit(1)
	}
	if moved > 0 || len(skipped) > 0 {
		log.Info("storage layout migrated", "moved", moved, "skipped", skipped)
	}
	mediaSigner := signedurl.New(cfg.MediaSigningKey)

	// Modules meet only through contracts/; bootstrap.Wire is the one place
	// that builds them all and closes the late-bound edges (shared verbatim with
	// the integration harness, so tests exercise the production wiring).
	mods, err := bootstrap.Wire(db, bootstrap.Options{
		Log: log, JWTSecret: cfg.JWTSecret, AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL,
		Storage: cfg.Storage, MediaDir: cfg.MediaDir, Signer: mediaSigner, SessionDir: cfg.SessionDir,
	})
	if err != nil {
		log.Error("wire modules", "error", err)
		os.Exit(1)
	}
	auditMod, userMod, branchMod, companyMod, authMod := mods.Audit, mods.User, mods.Branch, mods.Company, mods.Auth
	mediaMod, productMod, partyMod, stockMod := mods.Media, mods.Product, mods.Party, mods.Stock
	purchasingMod, salesMod, goodsreceiptMod, deliveryMod := mods.Purchasing, mods.Sales, mods.GoodsReceipt, mods.Delivery
	salesreturnMod, purchasereturnMod, paymentMod, financeMod := mods.SalesReturn, mods.PurchaseReturn, mods.Payment, mods.Finance
	dashboardMod, reportingMod, assistantMod := mods.Dashboard, mods.Reporting, mods.Assistant

	// Authorization seed (F1-6): the permission catalog and system/default
	// roles exist before the first request, even on a fresh database. New
	// modules' codes attach to owner/superadmin automatically.
	func() {
		boot, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := bootstrap.EnsureAuthz(boot, userMod.Service()); err != nil {
			log.Error("seed otorisasi", "error", err)
			os.Exit(1)
		}
	}()

	// Lifecycle (F6-6/F8-17): one signal-driven context owns every background
	// goroutine; bg lets shutdown wait for them before the DB closes.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var bg sync.WaitGroup
	assistantMod.SetContext(ctx)
	bg.Add(1)
	go func() {
		defer bg.Done()
		boot, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		assistantMod.Boot(boot)
	}()

	srv := server.New(cfg, log)
	// Readiness (F6-4): the database must answer; detail goes to the log only.
	srv.SetReady(func(c context.Context) error { return db.PingContext(c) })
	// OP-06: panics and unexpected 5xx responses reach the audit trail (deduped,
	// rate-limited, no request data) so they show on the audit page.
	srv.SetErrorReporter(func(c context.Context, e server.UnexpectedError) {
		auditMod.Service().RecordUnexpected(c, auditapp.Unexpected{Route: e.Route, Status: e.Status, Panic: e.Panic,
			Message: e.Message, RequestID: e.RequestID, UserID: e.UserID, BranchID: e.BranchID})
	})
	bg.Add(1)
	go func() {
		defer bg.Done()
		publish := func() {
			st := db.Stats()
			server.SetDBPool(st.OpenConnections, st.InUse, st.Idle, st.WaitCount)
		}
		publish()
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				publish()
			}
		}
	}()
	mux := srv.Mux()
	authMod.RegisterRoutes(mux)
	perm := authMod.Permission
	branchGuard := authcontracts.RequireBranch
	// authn is bare authentication (no permission, no branch): for endpoints
	// like branches/my-access that a branchless session must reach (J6).
	authn := func(next func(http.ResponseWriter, *http.Request)) http.Handler {
		return authcontracts.Authenticate(log, authMod.Sessions(), http.HandlerFunc(next))
	}
	userMod.RegisterRoutes(mux, perm)
	branchMod.RegisterRoutes(mux, perm, authn)
	companyMod.RegisterRoutes(mux, perm, authn)
	productMod.RegisterRoutes(mux, mediaMod.Media(), perm)
	partyMod.RegisterRoutes(mux, perm)
	stockMod.RegisterRoutes(mux, perm)
	purchasingMod.RegisterRoutes(mux, perm, branchGuard)
	salesMod.RegisterRoutes(mux, perm, branchGuard)
	goodsreceiptMod.RegisterRoutes(mux, perm, branchGuard)
	deliveryMod.RegisterRoutes(mux, mediaMod.Media(), perm, branchGuard)
	salesreturnMod.RegisterRoutes(mux, perm, branchGuard)
	purchasereturnMod.RegisterRoutes(mux, perm, branchGuard)
	paymentMod.RegisterRoutes(mux, perm, branchGuard)
	financeMod.RegisterRoutes(mux, perm, branchGuard, authn)
	auditMod.RegisterRoutes(mux, perm, branchGuard)
	dashboardMod.RegisterRoutes(mux, perm, branchGuard)
	reportingMod.RegisterRoutes(mux, perm, branchGuard)
	assistantMod.RegisterRoutes(mux, perm, branchGuard)
	mods.RegisterOps(mux, perm, branchGuard)
	srv.MountStatic(cfg.PublicDir)
	srv.MountMedia(cfg.MediaDir, mediaSigner)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	// A listen failure stops the app through the same path as a signal, so
	// deferred cleanup (DB close) still runs instead of os.Exit.
	var exitCode atomic.Int32
	go func() {
		log.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "error", err)
			exitCode.Store(1)
			stop()
		}
	}()

	// Daily retention sweep (F4-10): 02:00 WIB, advisory-locked, bounded.
	hk := &housekeeping.Runner{DB: db, Log: log, Tasks: mods.HousekeepingTasksDynamic(mods.SettingsRetention, nil)}
	bg.Add(1)
	go func() {
		defer bg.Done()
		hk.Loop(ctx, 2)
	}()

	// Expired stock holds (abandoned POS carts) are returned to availability
	// every minute; each pass is bounded and idempotent (A6).
	bg.Add(1)
	go func() {
		defer bg.Done()
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if n, err := stockMod.Service().SweepExpired(ctx, 500); err != nil {
					log.Error("penyapu reservasi", "error", err)
				} else if n > 0 {
					log.Info("reservasi kedaluwarsa disapu", "count", n)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	// Order: stop advertising readiness, drain HTTP, stop background work and
	// the WhatsApp gateway, then (deferred) close the database.
	srv.MarkNotReady()
	timeout := cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	shutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdown); err != nil {
		log.Warn("http shutdown", "error", err)
	}
	assistantMod.Close(shutdown)
	waited := make(chan struct{})
	go func() { bg.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-shutdown.Done():
		log.Warn("background goroutines did not stop in time")
	}
	log.Info("api stopped")
	if exitCode.Load() != 0 {
		stop()
		_ = db.Close()
		os.Exit(int(exitCode.Load()))
	}
}
