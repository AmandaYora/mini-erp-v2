package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mini-erp/internal/config"
	"mini-erp/internal/database"
	"mini-erp/internal/modules/assistant"
	"mini-erp/internal/modules/audit"
	"mini-erp/internal/modules/auth"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/branch"
	"mini-erp/internal/modules/company"
	"mini-erp/internal/modules/dashboard"
	"mini-erp/internal/modules/delivery"
	"mini-erp/internal/modules/finance"
	"mini-erp/internal/modules/goodsreceipt"
	"mini-erp/internal/modules/media"
	"mini-erp/internal/modules/party"
	"mini-erp/internal/modules/payment"
	"mini-erp/internal/modules/product"
	"mini-erp/internal/modules/purchasereturn"
	"mini-erp/internal/modules/purchasing"
	"mini-erp/internal/modules/reporting"
	"mini-erp/internal/modules/sales"
	"mini-erp/internal/modules/salesreturn"
	"mini-erp/internal/modules/stock"
	"mini-erp/internal/modules/user"
	"mini-erp/internal/server"
	"mini-erp/internal/shared/logger"
)

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.DBDSN)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// L1 identity package: auth ⇄ user ⇄ branch (+ company). Modules meet
	// only through contracts/ — main is the sole place that sees internals.
	// Audit is built first: every mutating module receives its client.
	auditMod := audit.NewModule(db, log)
	userMod := user.NewModule(db, auditMod.Audit())
	branchMod := branch.NewModule(db, userMod.Users(), auditMod.Audit())
	companyMod := company.NewModule(db, auditMod.Audit())
	authMod := auth.NewModule(db,
		userMod.Users(), userMod.Roles(), branchMod.Branches(), companyMod.Company(),
		cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL, log, auditMod.Audit(),
	)
	mediaMod, err := media.NewModule(db, cfg.Storage, cfg.StorageDir, auditMod.Audit())
	if err != nil {
		log.Error("init media storage", "error", err)
		os.Exit(1)
	}
	productMod := product.NewModule(db, auditMod.Audit())
	partyMod := party.NewModule(db, productMod.Products(), auditMod.Audit())
	stockMod := stock.NewModule(db, productMod.Products(), branchMod.Branches(),
		userMod.Users(), userMod.Roles(), companyMod.Company(), auditMod.Audit())
	purchasingMod := purchasing.NewModule(db, partyMod.Parties(), productMod.Products(), branchMod.Branches(), auditMod.Audit())
	salesMod := sales.NewModule(db, partyMod.Parties(), productMod.Products(), partyMod.Pricing(), branchMod.Branches(), auditMod.Audit())
	goodsreceiptMod := goodsreceipt.NewModule(db, purchasingMod.PurchaseOrders(), productMod.Products(), stockMod.Stock(), auditMod.Audit())
	deliveryMod := delivery.NewModule(db, salesMod.Service(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), mediaMod.Media(), auditMod.Audit())
	salesreturnMod := salesreturn.NewModule(db, salesMod.SalesOrders(), deliveryMod.Deliveries(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), auditMod.Audit())
	purchasereturnMod := purchasereturn.NewModule(db, purchasingMod.PurchaseOrders(), goodsreceiptMod.GoodsReceipts(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), auditMod.Audit())
	paymentMod := payment.NewModule(db, partyMod.Parties(), purchasingMod.PurchaseOrders(), salesMod.SalesOrders(), salesreturnMod.Returns(), purchasereturnMod.Returns(), branchMod.Branches(), mediaMod.Media(), auditMod.Audit())
	salesreturnMod.SetPayments(paymentMod.Payments())
	purchasereturnMod.SetPayments(paymentMod.Payments())
	financeMod := finance.NewModule(db, branchMod.Branches(), productMod.Products(), partyMod.Parties(),
		purchasingMod.PurchaseOrders(), salesMod.SalesOrders(),
		salesreturnMod.SalesReturns(), purchasereturnMod.PurchaseReturns(),
		goodsreceiptMod.GoodsReceipts(), deliveryMod.Deliveries(),
		paymentMod.Payments(), stockMod.Stock(), auditMod.Audit())
	financeMod.SetOps(salesMod.Service(), purchasingMod.Service())
	// L8 readers: dashboard (finance figures + operational widgets over
	// stock/sales/purchasing contracts) + reporting (finance figures).
	dashboardMod := dashboard.NewModule(financeMod.Finance(), stockMod.Stock(),
		salesMod.Service(), purchasingMod.Service())
	reportingMod := reporting.NewModule(financeMod.Finance())
	// L9 assistant: WhatsApp bot + tooling over provider contracts.
	assistantMod := assistant.NewModule(db, cfg.StorageDir, branchMod.Branches(), productMod.Products(),
		purchasingMod.PurchaseOrders(), salesMod.SalesOrders(), stockMod.Stock(),
		financeMod.Finance(), log)
	go func() {
		boot, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		assistantMod.Boot(boot)
	}()

	srv := server.New(cfg, log)
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
	companyMod.RegisterRoutes(mux, perm)
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
	financeMod.RegisterRoutes(mux, perm, branchGuard)
	auditMod.RegisterRoutes(mux, perm, branchGuard)
	dashboardMod.RegisterRoutes(mux, perm, branchGuard)
	reportingMod.RegisterRoutes(mux, perm, branchGuard)
	assistantMod.RegisterRoutes(mux, perm, branchGuard)
	srv.MountStatic(cfg.PublicDir)
	srv.MountUploads(cfg.StorageDir)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdown)
	log.Info("api stopped")
}
