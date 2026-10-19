// Package dsnguard keeps destructive test harnesses (TRUNCATE, DROP) away from
// any database that is not a disposable test database on this machine.
//
// It is pure string handling with no database access, so it can be unit tested
// without MySQL. Test-only: nothing under internal/modules may import it.
package dsnguard

import (
	"fmt"
	"strings"
)

// Split returns the server part of a DSN (everything up to and including the
// last "/") and the database name (without query parameters).
func Split(dsn string) (server, name string) {
	name = dsn
	if i := strings.LastIndex(dsn, "/"); i >= 0 {
		server, name = dsn[:i+1], dsn[i+1:]
	}
	if i := strings.Index(name, "?"); i >= 0 {
		name = name[:i]
	}
	return server, name
}

// Host extracts the host from a go-sql-driver DSN of the form
// user:pass@tcp(host:port)/db. It returns "" when the DSN does not use tcp
// (for example a unix socket), so callers treat that as non-local.
func Host(dsn string) string {
	server, _ := Split(dsn)
	i := strings.Index(server, "tcp(")
	if i < 0 {
		return ""
	}
	rest := server[i+len("tcp("):]
	j := strings.Index(rest, ")")
	if j < 0 {
		return ""
	}
	hostport := rest[:j]
	if strings.HasPrefix(hostport, "[") {
		if k := strings.Index(hostport, "]"); k > 0 {
			return strings.ToLower(hostport[:k+1])
		}
		return ""
	}
	if k := strings.LastIndex(hostport, ":"); k >= 0 {
		hostport = hostport[:k]
	}
	return strings.ToLower(hostport)
}

func isLocal(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}

// Check rejects a DSN unless its database name ends in "_test" and its host is
// this machine. allowRemote relaxes the host rule only (CI service containers);
// the "_test" rule is never relaxed.
func Check(dsn string, allowRemote bool) error {
	_, name := Split(dsn)
	if name == "" {
		return fmt.Errorf("DSN tanpa nama database; test integrasi menolak berjalan")
	}
	if !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("database %q ditolak: nama database test harus berakhiran _test", name)
	}
	if !allowRemote && !isLocal(Host(dsn)) {
		return fmt.Errorf("host %q ditolak: test integrasi hanya boleh ke mesin ini (set ALLOW_REMOTE_TEST_DB=1 untuk service container CI)", Host(dsn))
	}
	return nil
}

// CheckName is the second line of defence used right before a destructive
// statement, with the name reported by the live connection (SELECT DATABASE()).
func CheckName(name string) error {
	if !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("koneksi menunjuk database %q, bukan *_test; operasi destruktif dibatalkan", name)
	}
	return nil
}
