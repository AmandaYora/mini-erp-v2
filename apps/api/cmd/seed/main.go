package main

// Dev seed: owner role with the L1 permission catalog, one head branch, one
// admin user, and the singleton company profile. Idempotent — skips whatever
// already exists. Composition-root privilege: it drives application services
// directly, exactly like main.go wires modules. NEVER run against production.

import (
	"context"
	"fmt"
	"os"

	"mini-erp/internal/config"
	"mini-erp/internal/database"
	assistantcontracts "mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/audit"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/branch"
	branchapp "mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/company"
	companycontracts "mini-erp/internal/modules/company/contracts"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	reportingcontracts "mini-erp/internal/modules/reporting/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/user"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/logger"
)

func check(err error) {
	if err != nil {
		fmt.Println("seed failed:", err)
		os.Exit(1)
	}
}

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	check(err)
	db, err := database.Open(cfg.DBDSN)
	check(err)
	defer func() { _ = db.Close() }()

	// Seed drives services directly with a real audit client (actor 0 =
	// system): bootstrap writes are trailed like any other mutation.
	auditMod := audit.NewModule(db, logger.New())
	userMod := user.NewModule(db, auditMod.Audit())
	branchMod := branch.NewModule(db, userMod.Users(), auditMod.Audit())
	companyMod := company.NewModule(db, auditMod.Audit())
	users := userMod.Service()
	branches := branchMod.Service()
	companySvc := companyMod.Service()

	check(users.EnsurePermissionCatalog(ctx))
	check(users.EnsurePermissions(ctx, productcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, partycontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, stockcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, purchasingcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, salescontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, goodsreceiptcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, deliverycontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, paymentcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, salesreturncontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, purchasereturncontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, financecontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, auditcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, assistantcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, dashboardcontracts.PermissionCatalog()))
	check(users.EnsurePermissions(ctx, reportingcontracts.PermissionCatalog()))

	owner, err := users.GetRoleByCode(ctx, "owner")
	check(err)
	if owner == nil {
		owner, err = users.CreateRole(ctx, 0, userapp.CreateRoleInput{
			Code: "owner", Name: "Owner", Description: "Akses penuh",
		})
		check(err)
		fmt.Println("role owner created")
	}
	// Owner always holds every known permission (re-synced each run so new
	// modules' codes attach automatically).
	allCodes, err := users.AllPermissionCodes(ctx)
	check(err)
	check(users.SetRolePermissions(ctx, 0, owner.ID, allCodes))

	branchID, err := ensureBranch(ctx, branches)
	check(err)

	admin, err := users.GetByUsername(ctx, "admin")
	check(err)
	if admin == nil {
		admin, err = users.CreateUser(ctx, 0, userapp.CreateUserInput{
			Username: "admin", FullName: "Administrator", Password: "Admin123!",
			RoleIDs:  []int64{owner.ID},
			Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
		})
		check(err)
		fmt.Println("user admin created (password: Admin123! — ganti segera)")
	}

	profile, err := companySvc.GetProfile(ctx)
	check(err)
	if profile == nil {
		_, err = companySvc.SaveProfile(ctx, 0, companycontracts.Profile{Name: "Perusahaan Demo"})
		check(err)
		fmt.Println("company profile created")
	}

	// Default chart of accounts + mappings. Provider clients are nil: seed
	// calls EnsureChart only, which touches the finance tables alone.
	financeMod := finance.NewModule(db, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, auditMod.Audit())
	check(financeMod.Service().EnsureChart(ctx, 0))

	fmt.Println("seed ok")
}

func ensureBranch(ctx context.Context, branches *branchapp.Service) (int64, error) {
	list, err := branches.List(ctx, "", "", 1, 10)
	if err != nil {
		return 0, err
	}
	if len(list.Branches) > 0 {
		return list.Branches[0].ID, nil
	}
	b, err := branches.CreateBranch(ctx, 0, branchapp.CreateBranchInput{
		Code: "UTM", Name: "Utama", IsHead: true,
	})
	if err != nil {
		return 0, err
	}
	return b.ID, nil
}
