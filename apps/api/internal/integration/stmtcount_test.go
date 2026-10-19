package integration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/shared/signedurl"
)

// F4-7/F4-17: a deterministic statement counter. The production module graph
// (bootstrap.Wire) is built a second time over a *sql.DB whose driver
// connector wraps the MySQL driver and counts every statement that actually
// reaches the server (Query/Exec, plain or prepared, inside or outside a
// transaction). Counting happens in-process, per connection, so other
// processes on the shared server cannot disturb it, and the same request
// always yields the same number.

type stmtCounter struct {
	mu   sync.Mutex
	stmt []string
}

func (c *stmtCounter) add(q string) {
	c.mu.Lock()
	c.stmt = append(c.stmt, q)
	c.mu.Unlock()
}

// Reset starts a fresh measurement window.
func (c *stmtCounter) Reset() {
	c.mu.Lock()
	c.stmt = nil
	c.mu.Unlock()
}

// Statements returns a copy of the statements seen since Reset.
func (c *stmtCounter) Statements() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.stmt...)
}

// Count is the number of statements since Reset.
func (c *stmtCounter) Count() int { return len(c.Statements()) }

// CountMatching counts statements whose text matches re.
func (c *stmtCounter) CountMatching(re *regexp.Regexp) int {
	n := 0
	for _, q := range c.Statements() {
		if re.MatchString(q) {
			n++
		}
	}
	return n
}

type countingConnector struct {
	inner driver.Connector
	c     *stmtCounter
}

func (cc *countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := cc.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn, c: cc.c}, nil
}

func (cc *countingConnector) Driver() driver.Driver { return cc.inner.Driver() }

type countingConn struct {
	driver.Conn
	c *stmtCounter
}

func (cn *countingConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := cn.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := qc.QueryContext(ctx, q, args)
	if err != driver.ErrSkip { // ErrSkip: database/sql re-issues it through Prepare (counted there)
		cn.c.add(q)
	}
	return rows, err
}

func (cn *countingConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := cn.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := ec.ExecContext(ctx, q, args)
	if err != driver.ErrSkip {
		cn.c.add(q)
	}
	return res, err
}

func (cn *countingConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	var st driver.Stmt
	var err error
	if pc, ok := cn.Conn.(driver.ConnPrepareContext); ok {
		st, err = pc.PrepareContext(ctx, q)
	} else {
		st, err = cn.Conn.Prepare(q)
	}
	if err != nil {
		return nil, err
	}
	return &countingStmt{Stmt: st, c: cn.c, q: q}, nil
}

func (cn *countingConn) Prepare(q string) (driver.Stmt, error) {
	return cn.PrepareContext(context.Background(), q)
}

func (cn *countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := cn.Conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	return cn.Conn.Begin() //nolint:staticcheck // fallback for drivers without ConnBeginTx
}

func (cn *countingConn) CheckNamedValue(nv *driver.NamedValue) error {
	if nvc, ok := cn.Conn.(driver.NamedValueChecker); ok {
		return nvc.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

func (cn *countingConn) ResetSession(ctx context.Context) error {
	if sr, ok := cn.Conn.(driver.SessionResetter); ok {
		return sr.ResetSession(ctx)
	}
	return nil
}

func (cn *countingConn) IsValid() bool {
	if v, ok := cn.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

type countingStmt struct {
	driver.Stmt
	c *stmtCounter
	q string
}

func (s *countingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	s.c.add(s.q)
	if se, ok := s.Stmt.(driver.StmtExecContext); ok {
		return se.ExecContext(ctx, args)
	}
	return s.Stmt.Exec(namedToValues(args)) //nolint:staticcheck
}

func (s *countingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	s.c.add(s.q)
	if sq, ok := s.Stmt.(driver.StmtQueryContext); ok {
		return sq.QueryContext(ctx, args)
	}
	return s.Stmt.Query(namedToValues(args)) //nolint:staticcheck
}

func namedToValues(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}

var (
	countOnce sync.Once
	countMods *mods
	countCtr  *stmtCounter
	countErr  error
)

// countedMods returns the production wiring over a counting connection pool
// (same database, same pool bounds, same isolation and time zone as TestMain).
func countedMods(t *testing.T) (*mods, *stmtCounter) {
	t.Helper()
	countOnce.Do(func() {
		cfg, err := mysql.ParseDSN(testDBDSN())
		if err != nil {
			countErr = err
			return
		}
		cfg.ParseTime = true
		if cfg.Params == nil {
			cfg.Params = map[string]string{}
		}
		cfg.Params["transaction_isolation"] = "'READ-COMMITTED'"
		cfg.Params["time_zone"] = "'+00:00'"
		inner, err := mysql.NewConnector(cfg)
		if err != nil {
			countErr = err
			return
		}
		countCtr = &stmtCounter{}
		db := sql.OpenDB(&countingConnector{inner: inner, c: countCtr})
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(5 * time.Minute)
		sessionDir, err := os.MkdirTemp("", "wa-count-*")
		if err != nil {
			countErr = err
			return
		}
		w, err := bootstrap.Wire(db, bootstrap.Options{
			Log: slog.New(slog.NewTextHandler(io.Discard, nil)), JWTSecret: "integration-test-jwt-secret-minimal-32-karakter",
			AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
			Storage: config.Storage{Driver: "local", SignedURLTTLSecs: 300}, MediaDir: os.TempDir(),
			Signer: signedurl.New([]byte("integration-test-key")), SessionDir: sessionDir,
		})
		if err != nil {
			countErr = err
			return
		}
		countMods = &mods{db: db, audit: w.Audit, user: w.User, branch: w.Branch,
			company: w.Company, auth: w.Auth, assistant: w.Assistant, media: w.Media, product: w.Product, party: w.Party,
			stock: w.Stock, purchasing: w.Purchasing, sales: w.Sales,
			goodsreceipt: w.GoodsReceipt, delivery: w.Delivery, salesreturn: w.SalesReturn,
			purchasereturn: w.PurchaseReturn, payment: w.Payment, finance: w.Finance,
			dashboard: w.Dashboard, reporting: w.Reporting}
	})
	if countErr != nil {
		t.Fatalf("counting graph: %v", countErr)
	}
	return countMods, countCtr
}

// TestStatementCounterCounts proves the counter itself: plain, parameterised
// (prepared) and transactional statements are each counted exactly once.
func TestStatementCounterCounts(t *testing.T) {
	cm, ctr := countedMods(t)
	ctx := context.Background()
	ctr.Reset()
	var one int
	if err := cm.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		t.Fatal(err)
	}
	if err := cm.db.QueryRowContext(ctx, "SELECT ? + 1", 1).Scan(&one); err != nil {
		t.Fatal(err)
	}
	tx, err := cm.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "DO 1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT ?", 3).Scan(&one); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := ctr.Count(); got != 4 {
		t.Fatalf("counted %d statements (%s), want 4", got, strings.Join(ctr.Statements(), " | "))
	}
}
