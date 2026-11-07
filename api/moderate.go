package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

func moderate(s *postgresStore, args []string, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	if len(args) == 1 && args[0] == "list" {
		// Metadata only; inspect message text using a restricted database session.
		rows, err := s.db.QueryContext(ctx, `SELECT id,status FROM guestbooks WHERE status='pending' ORDER BY id LIMIT 100`)
		if err != nil {
			return errors.New("moderation list failed")
		}
		defer rows.Close()
		type metadata struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		result := []metadata{}
		for rows.Next() {
			var m metadata
			if err := rows.Scan(&m.ID, &m.Status); err != nil {
				return errors.New("moderation list failed")
			}
			result = append(result, m)
		}
		if rows.Err() != nil {
			return errors.New("moderation list failed")
		}
		if json.NewEncoder(os.Stdout).Encode(result) != nil {
			return errors.New("moderation output failed")
		}
		return nil
	}
	if len(args) != 2 {
		return errors.New("usage: moderate list | moderate approve|reject|delete ID; set MODERATOR_ID")
	}
	action := args[0]
	id, err := strconv.ParseInt(args[1], 10, 64)
	actor := strings.TrimSpace(os.Getenv("MODERATOR_ID"))
	if err != nil || id < 1 || actor == "" || utf8.RuneCountInString(actor) > 80 || strings.ContainsAny(actor, "\r\n\t") {
		return errors.New("invalid ID or MODERATOR_ID")
	}
	if err = s.Moderate(ctx, action, id, actor); err != nil {
		return errors.New("moderation failed; verify ID and database availability")
	}
	logger.Info("moderation complete", "entry_id", id, "action", action)
	return nil
}
func (s *postgresStore) Moderate(ctx context.Context, action string, id int64, actor string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result sql.Result
	switch action {
	case "approve":
		result, err = tx.ExecContext(ctx, `UPDATE guestbooks SET status='approved',created_at=COALESCE(created_at,CURRENT_TIMESTAMP) WHERE id=$1`, id)
	case "reject":
		result, err = tx.ExecContext(ctx, `UPDATE guestbooks SET status='rejected' WHERE id=$1`, id)
	case "delete":
		result, err = tx.ExecContext(ctx, `DELETE FROM guestbooks WHERE id=$1`, id)
	default:
		return errors.New("invalid moderation action")
	}
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO moderation_audit(entry_id,action,actor) VALUES ($1,$2,$3)`, id, action, actor); err != nil {
		return err
	}
	return tx.Commit()
}
