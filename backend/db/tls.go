package db

import (
	"fmt"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// tlsWarning returns a description of how the connection string leaves the
// database connection open to interception, or "" when the server's
// certificate is verified (or the database is on this machine).
//
// With pgx, sslmode=require encrypts but does not verify the certificate
// unless sslrootcert is set, so only verify-full gives real protection.
func tlsWarning(connString string) (string, error) {
	cfg, err := pgconn.ParseConfig(connString)
	if err != nil {
		return "", fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if isLocal(cfg.Host) {
		return "", nil
	}
	switch {
	case cfg.TLSConfig == nil:
		return "database connection is not encrypted; use sslmode=verify-full", nil
	case cfg.TLSConfig.InsecureSkipVerify:
		return "database server certificate is not verified; use sslmode=verify-full", nil
	}
	for _, fallback := range cfg.Fallbacks {
		if fallback.TLSConfig == nil || fallback.TLSConfig.InsecureSkipVerify {
			return "database connection may fall back to an unverified one; use sslmode=verify-full", nil
		}
	}
	return "", nil
}

func isLocal(host string) bool {
	if host == "localhost" || host == "" || host[0] == '/' {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
