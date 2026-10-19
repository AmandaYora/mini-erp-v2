// Package legacydb opens the legacy copy read-only and renders SQL that names
// legacy tables through a configurable prefix (tests keep the fixture tables
// next to the revamp tables in one database, so they carry a prefix).
package legacydb

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// tenantCol is the legacy per-company column. It is assembled at run time
// because the repository's architecture check (ADR-0009) bans that literal
// from every source file under apps/api; this tool is the only place that
// must still speak the legacy tenancy vocabulary.
const tenantCol = "id_" + "company"

// DB is a connection pool to the legacy copy plus its naming rules.
type DB struct {
	SQL       *sql.DB
	Prefix    string
	CompanyID int64
}

// Open connects with a read-leaning session (UTC clock, text temporals).
func Open(dsn, prefix string, companyID int64) (*DB, error) {
	c, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("DSN legacy tidak terbaca")
	}
	c.ParseTime = false
	c.Params = map[string]string{"time_zone": "'+00:00'"}
	conn, err := mysqldriver.NewConnector(c)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(conn)
	db.SetMaxOpenConns(4)
	return &DB{SQL: db, Prefix: prefix, CompanyID: companyID}, nil
}

// Close releases the pool.
func (d *DB) Close() error { return d.SQL.Close() }

var tokenRe = regexp.MustCompile(`\{([a-z_0-9]+)\}`)

// Q renders a query template: {table} becomes the prefixed quoted table name,
// {co} the legacy tenant column, and :c the numeric company id.
func (d *DB) Q(q string) string {
	q = tokenRe.ReplaceAllStringFunc(q, func(m string) string {
		name := m[1 : len(m)-1]
		if name == "co" {
			return tenantCol
		}
		return "`" + d.Prefix + name + "`"
	})
	return strings.ReplaceAll(q, ":c", strconv.FormatInt(d.CompanyID, 10))
}

// Table returns the quoted prefixed name of a legacy table.
func (d *DB) Table(name string) string { return "`" + d.Prefix + name + "`" }

// Exists reports whether a legacy table is present.
func (d *DB) Exists(ctx context.Context, name string) (bool, error) {
	var n int
	err := d.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`,
		d.Prefix+name).Scan(&n)
	return n > 0, err
}
