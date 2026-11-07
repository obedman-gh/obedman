package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrations embed.FS

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize concurrent migrators, including first installation.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(726433019)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	var latest int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&latest); err != nil {
		return err
	}
	if latest > schemaVersion {
		return fmt.Errorf("database schema is newer than binary")
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for i, file := range files {
		body, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}
		version := i + 1
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		var prior string
		err = tx.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, version).Scan(&prior)
		if err == nil {
			if prior != checksum {
				return fmt.Errorf("migration checksum mismatch")
			}
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES ($1,$2)`, version, checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}
