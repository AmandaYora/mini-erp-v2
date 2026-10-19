// Package guard holds the safety checks that run before the ETL touches
// anything (F5-1). The tool is destructive-adjacent (it bulk-writes the
// revamp database) and reads a production copy, so every check fails closed.
package guard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// Redact hides the password of a DSN so logs never leak it.
func Redact(dsn string) string {
	c, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return "(dsn tidak terbaca)"
	}
	return fmt.Sprintf("%s@%s/%s", c.User, c.Addr, c.DBName)
}

// DBName returns the database name of a DSN.
func DBName(dsn string) (string, error) {
	c, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return "", errors.New("DSN tidak terbaca")
	}
	if c.DBName == "" {
		return "", errors.New("DSN tanpa nama database")
	}
	return c.DBName, nil
}

// CheckTargetName refuses names that look like production. A target is allowed
// when it ends in _test or _rehearsal, or equals the explicitly allowed name.
func CheckTargetName(name, allow string) error {
	l := strings.ToLower(name)
	if strings.Contains(l, "prod") {
		return fmt.Errorf("database target %q ditolak: nama mengandung 'prod'", name)
	}
	if strings.HasSuffix(l, "_test") || strings.HasSuffix(l, "_rehearsal") {
		return nil
	}
	if allow != "" && allow == name {
		return nil
	}
	return fmt.Errorf("database target %q tidak ada di daftar putih (*_test, *_rehearsal, atau --allow-target-db=%s)", name, name)
}

// CheckLegacyHost requires a local legacy copy unless explicitly relaxed.
func CheckLegacyHost(dsn string, allowRemote bool) error {
	c, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return errors.New("DSN legacy tidak terbaca")
	}
	host := c.Addr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return nil
	}
	if allowRemote {
		return nil
	}
	return fmt.Errorf("host legacy %q bukan lokal (gunakan --allow-remote-legacy bila disengaja)", host)
}

var identRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// CheckIdent validates a table prefix or name so it can be embedded in SQL.
func CheckIdent(s string) error {
	if s != "" && !identRe.MatchString(s) {
		return fmt.Errorf("pengenal tidak valid: %q", s)
	}
	return nil
}

// LegacyMarker requires the owner-created marker table that proves the legacy
// connection points at a restored COPY, not at the live production database.
func LegacyMarker(ctx context.Context, db *sql.DB, markerTable string) error {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+markerTable+"`").Scan(&n); err != nil {
		return fmt.Errorf("penanda salinan legacy (%s) tidak ada: ETL menolak jalan (%v)", markerTable, err)
	}
	if n == 0 {
		return fmt.Errorf("penanda salinan legacy (%s) kosong", markerTable)
	}
	return nil
}

// SeedTables may legitimately hold rows before the ETL runs (reference data
// created by migrations or a bootstrap). Everything else must be empty.
var SeedTables = map[string]bool{
	"permissions": true, "roles": true, "role_permissions": true,
	"finance_accounts": true, "finance_account_mappings": true,
	"authz_seed_applied": true, "company_settings": true,
}

// TargetEmpty enforces the "empty target" rule over the tables the ETL writes.
func TargetEmpty(ctx context.Context, db *sql.DB, stagePrefix, runID string, tables []string, resume, allowNonEmpty bool) error {
	var dirty []string
	for _, t := range tables {
		if SeedTables[t] {
			continue
		}
		var n int64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+t+"`").Scan(&n); err != nil {
			return fmt.Errorf("tabel target %s tidak terbaca: %w", t, err)
		}
		if n > 0 {
			dirty = append(dirty, fmt.Sprintf("%s(%d)", t, n))
		}
	}
	if len(dirty) == 0 || allowNonEmpty {
		return nil
	}
	if resume {
		var one int
		err := db.QueryRowContext(ctx, "SELECT 1 FROM `"+stagePrefix+"run` WHERE run_id = ?", runID).Scan(&one)
		if err == nil {
			return nil
		}
		return fmt.Errorf("--resume: lari %q tidak tercatat di target (target berisi %s)", runID, strings.Join(dirty, ", "))
	}
	return fmt.Errorf("target tidak kosong: %s (gunakan --resume untuk lari yang sama)", strings.Join(dirty, ", "))
}

// Fingerprint hashes the column definitions of the given tables so a schema
// drift between the ETL build and the target is caught before loading.
func Fingerprint(ctx context.Context, db *sql.DB, tables []string) (string, error) {
	h := sha256.New()
	for _, t := range tables {
		rows, err := db.QueryContext(ctx,
			`SELECT column_name, column_type, is_nullable FROM information_schema.columns
			  WHERE table_schema = DATABASE() AND table_name = ? ORDER BY ordinal_position`, t)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "[%s]\n", t)
		for rows.Next() {
			var c, ty, nl string
			if err := rows.Scan(&c, &ty, &nl); err != nil {
				_ = rows.Close()
				return "", err
			}
			fmt.Fprintf(h, "%s %s %s\n", c, ty, nl)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return "", err
		}
		_ = rows.Close()
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
