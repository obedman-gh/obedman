package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		// Driver errors can contain credentials; run returns only controlled messages.
		logger.Error("application stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration rejected")
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	err = db.PingContext(ctx)
	cancel()
	if err != nil {
		return errors.New("database unavailable")
	}
	store := &postgresStore{db: db}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			if len(os.Args) != 2 {
				return errors.New("usage: obedman-api migrate")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := migrate(ctx, db); err != nil {
				return errors.New("migration failed; inspect schema with restricted operator access")
			}
			logger.Info("migrations complete")
			return nil
		case "moderate":
			return moderate(store, os.Args[2:], logger)
		default:
			return errors.New("unknown command; use migrate or moderate")
		}
	}
	ctx, cancel = context.WithTimeout(context.Background(), queryTimeout)
	err = store.Ready(ctx)
	cancel()
	if err != nil {
		return errors.New("database schema unavailable; run migrate with migration role")
	}
	csp, err := staticCSP(cfg.StaticDir)
	if err != nil {
		return errors.New("static build unavailable; build frontend and check STATIC_DIR")
	}
	server := &http.Server{Addr: cfg.ListenAddr, Handler: newHandler(store, cfg, logger, csp),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	logger.Info("server listening", "address", cfg.ListenAddr)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP listener failed")
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown deadline exceeded")
		}
	}
	return nil
}
