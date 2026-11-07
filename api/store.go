package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const queryTimeout = 3 * time.Second
const schemaVersion = 2

type entry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}
type input struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}
type cursor struct {
	CreatedAt time.Time `json:"t"`
	ID        int64     `json:"i"`
}
type store interface {
	Create(context.Context, input) (entry, error)
	List(context.Context, *cursor, int) ([]entry, error)
	Ready(context.Context) error
}
type postgresStore struct{ db *sql.DB }

func (s *postgresStore) Create(ctx context.Context, in input) (entry, error) {
	var e entry
	// Database defaults own ID, timestamp and pending state; runtime only needs name/message INSERT rights.
	err := s.db.QueryRowContext(ctx, `INSERT INTO guestbooks (name,message) VALUES ($1,$2) RETURNING id,name,message,created_at,status`, in.Name, in.Message).Scan(&e.ID, &e.Name, &e.Message, &e.CreatedAt, &e.Status)
	return e, err
}
func (s *postgresStore) List(ctx context.Context, after *cursor, limit int) ([]entry, error) {
	var t any
	var id int64
	if after != nil {
		t = after.CreatedAt
		id = after.ID
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,message,created_at,status FROM guestbooks WHERE status='approved' AND ($1::timestamptz IS NULL OR (created_at,id)<($1,$2)) ORDER BY created_at DESC,id DESC LIMIT $3`, t, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]entry, 0)
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.Name, &e.Message, &e.CreatedAt, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
func (s *postgresStore) Ready(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version != schemaVersion {
		return errors.New("schema version mismatch")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,message,created_at,status FROM guestbooks LIMIT 0`)
	if err != nil {
		return err
	}
	return rows.Close()
}
