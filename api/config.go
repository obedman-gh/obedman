package main

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

type config struct {
	DatabaseURL, ListenAddr, StaticDir string
	Production                         bool
	TrustedProxies                     []netip.Prefix
}

func loadConfig() (config, error) {
	c := config{DatabaseURL: os.Getenv("DATABASE_URL"), ListenAddr: os.Getenv("LISTEN_ADDR"), StaticDir: os.Getenv("STATIC_DIR")}
	env := os.Getenv("APP_ENV")
	if env != "development" && env != "production" {
		return c, errors.New("APP_ENV must be development or production")
	}
	c.Production = env == "production"
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.User == nil || strings.Trim(u.Path, "/") == "" {
		return c, errors.New("DATABASE_URL must be a PostgreSQL URL with user, host and database")
	}
	// Prevent driver environment and fallback hosts from overriding the reviewed URL.
	for _, key := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSERVICE", "PGSERVICEFILE", "PGSSLMODE", "PGSSLROOTCERT"} {
		if os.Getenv(key) != "" {
			return c, errors.New("use DATABASE_URL exclusively; clear PG connection environment variables")
		}
	}
	for key, values := range u.Query() {
		if len(values) != 1 {
			return c, errors.New("duplicate DATABASE_URL query option")
		}
		if key != "sslmode" && key != "sslrootcert" && key != "connect_timeout" {
			return c, errors.New("unsupported DATABASE_URL query option")
		}
	}
	if strings.Contains(u.Host, ",") {
		return c, errors.New("multiple database hosts are unsupported")
	}
	mode := u.Query().Get("sslmode")
	if mode == "" {
		return c, errors.New("DATABASE_URL must explicitly set sslmode")
	}
	if c.Production && mode != "verify-full" {
		return c, errors.New("production database requires sslmode=verify-full")
	}
	pc, err := pgx.ParseConfig(c.DatabaseURL)
	if err != nil || (c.Production && (pc.TLSConfig == nil || pc.TLSConfig.InsecureSkipVerify || len(pc.Fallbacks) != 0)) {
		return c, errors.New("database TLS configuration rejected")
	}
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:3030"
	}
	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return c, errors.New("LISTEN_ADDR must contain an IP and port")
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return c, errors.New("LISTEN_ADDR must contain an explicit IP")
	}
	if c.StaticDir == "" {
		c.StaticDir = "../dev/dist"
	}
	if proxies := os.Getenv("TRUSTED_PROXY_CIDRS"); proxies != "" {
		for _, item := range strings.Split(proxies, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(item))
			if err != nil || prefix.Bits() == 0 {
				return c, errors.New("TRUSTED_PROXY_CIDRS contains an invalid or universal network")
			}
			c.TrustedProxies = append(c.TrustedProxies, prefix)
		}
	}
	return c, nil
}
