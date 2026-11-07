package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	for _, key := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSERVICE", "PGSERVICEFILE", "PGSSLMODE", "PGSSLROOTCERT", "TRUSTED_PROXY_CIDRS", "LISTEN_ADDR"} {
		t.Setenv(key, "")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("missing database allowed")
	}
	t.Setenv("DATABASE_URL", "postgres://test@127.0.0.1/guestbook_test?sslmode=disable")
	if _, err := loadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := loadConfig(); err == nil {
		t.Fatal("plaintext production allowed")
	}
	t.Setenv("DATABASE_URL", "postgres://test@db.example.invalid/guestbook_test?sslmode=verify-full")
	if _, err := loadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://test@db.example.invalid/guestbook_test?sslmode=verify-full&host=127.0.0.1")
	if _, err := loadConfig(); err == nil {
		t.Fatal("URL override allowed")
	}
	t.Setenv("DATABASE_URL", "not-a-url")
	if _, err := loadConfig(); err == nil || strings.Contains(err.Error(), "not-a-url") {
		t.Fatal("configuration not rejected safely")
	}
}
func TestStaticCSP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><script>console.log('test')</script></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	csp, err := staticCSP(dir)
	if err != nil || !strings.Contains(csp, "'sha256-") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatal("script policy missing hashes")
	}
}
