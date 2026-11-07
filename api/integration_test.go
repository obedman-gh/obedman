package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func integrationDB(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL missing; PostgreSQL integration not run")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(strings.Trim(u.Path, "/"), "_test") {
		t.Fatal("integration database name must end in _test")
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid test configuration")
	}
	admin := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal("cannot create isolated test schema")
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error("test schema cleanup failed")
		}
	})
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	return db
}
func TestIntegrationMigrationAndModeration(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	// Match the legacy GORM shape, including a row outside the new input limits.
	_, err := db.Exec(`CREATE TABLE guestbooks (id bigserial PRIMARY KEY,name text,message text,created_at timestamptz);
 INSERT INTO guestbooks(name,message,created_at) VALUES ('legacy','<b>synthetic</b>','2020-01-01T00:00:00Z'),('legacy oversized',repeat('x',2100),NULL)`)
	if err != nil {
		t.Fatal("legacy fixture failed")
	}
	if err = migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, db); err != nil {
		t.Fatal("migration is not repeatable")
	}
	var count, pending int
	if err = db.QueryRow(`SELECT count(*),count(*) FILTER (WHERE status='pending') FROM guestbooks`).Scan(&count, &pending); err != nil || count != 2 || pending != 2 {
		t.Fatal("legacy rows lost or exposed")
	}
	var body string
	var created time.Time
	if err = db.QueryRow(`SELECT message,created_at FROM guestbooks WHERE id=1`).Scan(&body, &created); err != nil || body != "<b>synthetic</b>" || created.Year() != 2020 {
		t.Fatal("legacy content changed")
	}
	s := &postgresStore{db: db}
	if err = s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx, nil, 10)
	if err != nil || len(entries) != 0 {
		t.Fatal("pending rows exposed")
	}
	e, err := s.Create(ctx, input{"synthetic visitor", "<script>window.injected=true</script>"})
	if err != nil || e.ID <= 2 || e.Status != "pending" || e.CreatedAt.IsZero() {
		t.Fatal("server fields failed")
	}
	if err = s.Moderate(ctx, "approve", e.ID, "test-operator"); err != nil {
		t.Fatal(err)
	}
	entries, err = s.List(ctx, nil, 10)
	if err != nil || len(entries) != 1 || entries[0].ID != e.ID {
		t.Fatal("approval not published")
	}
	if err = s.Moderate(ctx, "reject", e.ID, "test-operator"); err != nil {
		t.Fatal(err)
	}
	entries, err = s.List(ctx, nil, 10)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected entry exposed")
	}
	if err = s.Moderate(ctx, "delete", e.ID, "test-operator"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM moderation_audit WHERE entry_id=$1`, e.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("audit missing")
	}
	if err = s.Moderate(ctx, "approve", 99999, "test-operator"); err == nil {
		t.Fatal("nonexistent moderation succeeded")
	}
	if err = db.QueryRow(`SELECT count(*) FROM moderation_audit`).Scan(&count); err != nil || count != 3 {
		t.Fatal("failed action left audit row")
	}
	// Auditing and mutation must roll back together.
	if _, err = db.Exec(`ALTER TABLE moderation_audit ADD CONSTRAINT fail_audit CHECK (actor <> 'blocked')`); err != nil {
		t.Fatal(err)
	}
	if err = s.Moderate(ctx, "approve", 1, "blocked"); err == nil {
		t.Fatal("failed audit accepted")
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM guestbooks WHERE id=1`).Scan(&status); err != nil || status != "pending" {
		t.Fatal("failed audit did not roll back moderation")
	}
}
func TestIntegrationPaginationAndFailure(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO guestbooks(name,message,status,created_at) SELECT 'synthetic','safe','approved','2024-01-01T00:00:00Z' FROM generate_series(1,5)`); err != nil {
		t.Fatal(err)
	}
	s := &postgresStore{db: db}
	h := testHandler(s)
	ids := map[int64]bool{}
	next := ""
	for pageNumber := 0; pageNumber < 3; pageNumber++ {
		w := request(h, "GET", "/api/guestbook?limit=2&cursor="+next, "", "")
		var page struct {
			Items []entry `json:"items"`
			Next  string  `json:"next_cursor"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
			t.Fatal("page failed")
		}
		for _, e := range page.Items {
			if ids[e.ID] {
				t.Fatal("duplicate at timestamp tie")
			}
			ids[e.ID] = true
		}
		next = page.Next
	}
	if len(ids) != 5 || next != "" {
		t.Fatal("pagination skipped data")
	}
	if _, err := db.Exec(`ALTER TABLE guestbooks RENAME TO unavailable`); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		w := request(h, method, "/api/guestbook", `{"name":"synthetic","message":"safe"}`, "application/json")
		if w.Code != 503 {
			t.Fatal("real DB failure reported success")
		}
	}
}
func TestIntegrationMigrationConcurrency(t *testing.T) {
	db := integrationDB(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			errs <- migrate(ctx, db)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET checksum='tampered' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), db); err == nil {
		t.Fatal("changed migration checksum accepted")
	}
}

func TestIntegrationMigrationFailureIsAtomic(t *testing.T) {
	db := integrationDB(t)
	// An unexpected pre-existing column should fail safely, without partially upgrading.
	if _, err := db.Exec(`CREATE TABLE guestbooks(id bigserial PRIMARY KEY,name text,message text,created_at timestamptz,status text); INSERT INTO guestbooks(name,message) VALUES ('synthetic','retained')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), db); err == nil {
		t.Fatal("invalid legacy schema migration succeeded")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM guestbooks`).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed migration changed legacy rows")
	}
	var table *string
	if err := db.QueryRow(`SELECT to_regclass('schema_migrations')::text`).Scan(&table); err != nil || table != nil {
		t.Fatal("failed migration left partial metadata")
	}
}

func TestIntegrationRuntimePrivileges(t *testing.T) {
	db := integrationDB(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	role := fmt.Sprintf("runtime_test_%d", time.Now().UnixNano())
	if _, err := db.Exec(`CREATE ROLE ` + role + ` NOLOGIN`); err != nil {
		t.Fatal("isolated test administrator must be able to create test roles")
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`RESET ROLE`); err != nil {
			t.Error("test role reset failed")
		}
		if _, err := db.Exec(`DROP OWNED BY ` + role); err != nil {
			t.Error("test role grant cleanup failed")
		}
		if _, err := db.Exec(`DROP ROLE ` + role); err != nil {
			t.Error("test role cleanup failed")
		}
	})
	grants := `GRANT USAGE ON SCHEMA ` + schema + ` TO ` + role + `;
 GRANT SELECT ON guestbooks,schema_migrations TO ` + role + `;
 GRANT INSERT(name,message) ON guestbooks TO ` + role + `;
 GRANT USAGE ON SEQUENCE guestbooks_id_seq TO ` + role
	if _, err := db.Exec(grants); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`SET ROLE ` + role); err != nil {
		t.Fatal(err)
	}
	s := &postgresStore{db: db}
	if err := s.Ready(ctx); err != nil {
		t.Fatal("runtime readiness permission missing")
	}
	e, err := s.Create(ctx, input{"synthetic", "restricted role"})
	if err != nil || e.Status != "pending" || e.CreatedAt.IsZero() {
		t.Fatal("restricted insert/defaults failed")
	}
	if rows, err := s.List(ctx, nil, 10); err != nil || len(rows) != 0 {
		t.Fatal("restricted list failed")
	}
	for _, query := range []string{
		`INSERT INTO guestbooks(name,message,status) VALUES ('synthetic','blocked','approved')`,
		`UPDATE guestbooks SET status='approved'`,
		`DELETE FROM guestbooks`,
		`CREATE TABLE forbidden(id int)`,
		`INSERT INTO moderation_audit(entry_id,action,actor) VALUES (1,'approve','blocked')`,
	} {
		if _, err := db.Exec(query); err == nil {
			t.Fatal("runtime exceeded minimum privileges")
		}
	}
}
