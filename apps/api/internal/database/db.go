package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ensureParams guarantees the MySQL DSN parses temporal columns into
// time.Time (UTC). Without parseTime, DATETIME scans come back as []byte
// and every query breaks in a different place.
func ensureParams(dsn string) string {
	if strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}

// Open connects to MySQL and verifies the connection. Timestamps stored in
// the database are UTC (see shared/timeutil); conversion to Asia/Jakarta
// happens at the application layer.
func Open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	db, err := sql.Open("mysql", ensureParams(dsn))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
