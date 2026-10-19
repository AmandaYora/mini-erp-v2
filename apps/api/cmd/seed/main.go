package main

// Dev seed (F1-6): subcommands for local bootstrapping. NEVER run against
// production with demo data. Composition-root privilege: it drives
// application services directly, exactly like main.go wires modules.
//
//   - `seed init`  : catalog + system/default roles + chart + head branch +
//     minimal company profile. Idempotent; also runs on every server start.
//   - `seed admin` : first owner user. Only when the users table is empty
//     (or --force-new with --username). Password from
//     BOOTSTRAP_ADMIN_PASSWORD (>=12 chars) or generated and printed ONCE.
//   - `seed demo`  : dev/demo dataset. Refused in staging/production unless
//     --i-know-this-is-not-production.
//
// APP_ENV is required (config.Load enforces it).

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/database"
	"mini-erp/internal/modules/audit"
	"mini-erp/internal/modules/branch"
	branchapp "mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/company"
	companyapp "mini-erp/internal/modules/company/application"
	companycontracts "mini-erp/internal/modules/company/contracts"
	"mini-erp/internal/modules/finance"
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

type seedCtx struct {
	ctx         context.Context
	users       *userapp.Service
	branches    *branchapp.Service
	company     *companyapp.Service
	ensureChart func(ctx context.Context, actorID int64) error
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cfg, err := config.Load()
	check(err)
	if os.Args[1] == "-profile=e2e" || os.Args[1] == "--profile=e2e" {
		runE2E(cfg) // own connection pool (needs multiStatements for migrations)
		return
	}
	db, err := database.Open(cfg.DBDSN)
	check(err)
	defer func() { _ = db.Close() }()

	log := logger.New()
	auditMod := audit.NewModule(db, log)
	userMod := user.NewModule(db, auditMod.Audit(), nil, nil)
	branchMod := branch.NewModule(db, userMod.Users(), auditMod.Audit(), nil)
	companyMod := company.NewModule(db, auditMod.Audit())
	financeMod := finance.NewModule(db, branchMod.Branches(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, auditMod.Audit(), nil, nil, nil, nil)

	sc := &seedCtx{
		ctx:         context.Background(),
		users:       userMod.Service(),
		branches:    branchMod.Service(),
		company:     companyMod.Service(),
		ensureChart: financeMod.Service().EnsureChart,
	}

	switch os.Args[1] {
	case "init":
		runInit(sc)
	case "admin":
		runAdmin(sc, os.Args[2:])
	case "demo":
		runDemo(sc, cfg.Env, os.Args[2:])
	default:
		fmt.Printf("subperintah tak dikenal: %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`pakai: seed <init|admin|demo> [opsi]

  init   katalog izin + role sistem/default + chart + cabang kepala + profil
  admin  user owner pertama [--force-new --username=NAMA]
  demo   dataset demo/dev [--i-know-this-is-not-production]`)
}

// runInit is idempotent: every step skips what already exists.
func runInit(sc *seedCtx) {
	ctx, cancel := context.WithTimeout(sc.ctx, 60*time.Second)
	defer cancel()
	check(bootstrap.EnsureAuthz(ctx, sc.users))
	check(sc.ensureChart(ctx, 0))
	branchID, err := ensureBranch(sc)
	check(err)
	profile, err := sc.company.GetProfile(ctx)
	check(err)
	if profile == nil {
		_, err = sc.company.SaveProfile(ctx, 0, companycontracts.Profile{Name: "Perusahaan Demo"})
		check(err)
		fmt.Println("company profile created")
	}
	fmt.Println("seed init ok (branch:", branchID, ")")
}

// runAdmin creates the first owner user. Password policy: explicit
// BOOTSTRAP_ADMIN_PASSWORD (>=12 chars) wins; otherwise a 20-char password
// is generated and printed ONCE to stdout (never to logs/audit).
func runAdmin(sc *seedCtx, args []string) {
	forceNew := false
	username := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	for i, a := range args {
		switch {
		case a == "--force-new":
			forceNew = true
		case strings.HasPrefix(a, "--username="):
			username = strings.TrimSpace(strings.TrimPrefix(a, "--username="))
		case a == "--username" && i+1 < len(args):
			username = strings.TrimSpace(args[i+1])
		}
	}
	if username == "" {
		check(fmt.Errorf("--username tidak boleh kosong"))
	}
	page, err := sc.users.ListUsers(sc.ctx, 0, "", "", 1, 1)
	check(err)
	if page.Total > 0 && !forceNew {
		if existing, err := sc.users.GetByUsername(sc.ctx, username); err == nil && existing != nil {
			fmt.Println("user", username, "sudah ada; tidak dibuat ulang")
			return
		}
		if !forceNew {
			check(fmt.Errorf("sudah ada %d user; pakai --force-new --username=NAMA untuk menambah", page.Total))
		}
	}
	branchID, err := ensureBranch(sc)
	check(err)
	owner, err := sc.users.GetRoleByCode(sc.ctx, "owner")
	check(err)
	if owner == nil {
		check(fmt.Errorf("role owner belum ada; jalankan `seed init` dulu"))
	}
	password := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	generated := false
	if password == "" {
		password, err = randomPassword(20)
		check(err)
		generated = true
	} else if len([]byte(password)) < 12 {
		check(fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD minimal 12 karakter"))
	}
	u, err := sc.users.CreateUser(sc.ctx, 0, userapp.CreateUserInput{
		Username: username, FullName: "Administrator", Password: password,
		RoleIDs:  []int64{owner.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
	})
	check(err)
	if generated {
		fmt.Printf("user %s dibuat (id %d) dengan password acak:\n%s\nSimpan sekarang — tidak ditampilkan lagi.\n", u.Username, u.ID, password)
	} else {
		fmt.Printf("user %s dibuat (id %d)\n", u.Username, u.ID)
	}
}

// runDemo builds the dev/demo dataset on top of init. Production and
// staging refuse unless explicitly overridden.
func runDemo(sc *seedCtx, env string, args []string) {
	if env == "production" || env == "staging" {
		ack := false
		for _, a := range args {
			if a == "--i-know-this-is-not-production" {
				ack = true
			}
		}
		if !ack {
			check(fmt.Errorf("seed demo menolak berjalan di APP_ENV=%s (tambah --i-know-this-is-not-production bila benar-benar dev)", env))
		}
	}
	runInit(sc)
	fmt.Println("seed demo ok")
}

// randomPassword generates n unambiguous alphanumeric characters.
func randomPassword(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

func ensureBranch(sc *seedCtx) (int64, error) {
	code := strings.TrimSpace(os.Getenv("BOOTSTRAP_BRANCH_CODE"))
	if code == "" {
		code = "UTM"
	}
	name := strings.TrimSpace(os.Getenv("BOOTSTRAP_BRANCH_NAME"))
	if name == "" {
		name = "Utama"
	}
	list, err := sc.branches.List(sc.ctx, "", "", 1, 10)
	if err != nil {
		return 0, err
	}
	if len(list.Branches) > 0 {
		return list.Branches[0].ID, nil
	}
	head := true
	if strings.TrimSpace(os.Getenv("BOOTSTRAP_BRANCH_HEAD")) == "0" {
		head = false
	}
	b, err := sc.branches.CreateBranch(sc.ctx, 0, branchapp.CreateBranchInput{
		Code: code, Name: name, IsHead: head,
	})
	if err != nil {
		return 0, err
	}
	fmt.Println("branch", code, "created")
	return b.ID, nil
}
