package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// ensureParams guarantees the MySQL DSN parses temporal columns into
// time.Time (UTC). Without parseTime, DATETIME scans come back as []byte
// and every query breaks in a different place.
//
// Only first-class driver params go here. Session attributes (time_zone,
// isolation) are applied per connection by initConnector below: the driver
// forwards unknown DSN params as UNQUOTED `SET key=value`, so
// `time_zone=+00:00` arrives as invalid SQL and the server refuses to start
// (caught by the seed smoke path, F1-6).
func ensureParams(dsn string) string {
	add := func(key, val string) {
		if strings.Contains(dsn, key+"=") {
			return
		}
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + key + "=" + val
	}
	add("parseTime", "true")
	add("loc", "UTC")
	add("timeout", "5s")
	add("readTimeout", "30s")
	add("writeTimeout", "30s")
	return dsn
}

// sessionInit runs on EVERY pooled connection (F4-12): UTC session clock so
// DATETIME(6) reads/writes never depend on the host's zone, and READ
// COMMITTED isolation. Quoted explicitly — never via DSN system vars.
var sessionInit = []string{
	"SET time_zone = '+00:00'",
	"SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED",
}

// initConnector wraps the MySQL connector so each new connection starts
// with the session attributes above, not just the first Ping.
type initConnector struct {
	driver.Connector
}

func (c *initConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	execStmt := func(query string) error {
		if ex, ok := conn.(driver.ExecerContext); ok {
			_, err := ex.ExecContext(ctx, query, nil)
			return err
		}
		if ex, ok := conn.(driver.Execer); ok {
			_, err := ex.Exec(query, nil)
			return err
		}
		return fmt.Errorf("koneksi tidak mendukung exec")
	}
	for _, stmt := range sessionInit {
		if err := execStmt(stmt); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("sesi database: %w", err)
		}
	}
	return conn, nil
}

// Open connects to MySQL and verifies the connection. Timestamps stored in
// the database are UTC (see shared/timeutil); conversion to Asia/Jakarta
// happens at the application layer.
func Open(dsn string) (*sql.DB, error) {
	return OpenWithPool(dsn, 25, 10, 5*time.Minute, 2*time.Minute)
}

// OpenWithPool connects with explicit pool bounds (F6-4, from config).
func OpenWithPool(dsn string, maxOpen, maxIdle int, lifetime, idleTime time.Duration) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	cfg, err := mysqldriver.ParseDSN(ensureParams(dsn))
	if err != nil {
		return nil, err
	}
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(&initConnector{Connector: connector})
	if maxOpen <= 0 {
		maxOpen = 25
	}
	if maxIdle <= 0 {
		maxIdle = 10
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	if lifetime <= 0 {
		lifetime = 5 * time.Minute
	}
	if idleTime <= 0 {
		idleTime = 2 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(idleTime)
	// Verify reachability and the session attributes on a dedicated handle.
	conn, err := db.Conn(context.Background())
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	defer func() { _ = conn.Close() }()
	for _, stmt := range sessionInit {
		if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sesi database: %w", err)
		}
	}
	var tz, iso string
	if err := conn.QueryRowContext(context.Background(),
		"SELECT @@session.time_zone, @@session.transaction_isolation").Scan(&tz, &iso); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("verifikasi sesi database: %w", err)
	}
	if tz != "+00:00" {
		_ = db.Close()
		return nil, fmt.Errorf("zona sesi database = %q, want +00:00", tz)
	}
	return db, nil
}
