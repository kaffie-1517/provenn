package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/invoice"
	"github.com/kaffie-1517/provenn/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	databaseURL := envOr("DATABASE_URL", "postgres://provenn:provenn@localhost:5432/provenn?sslmode=disable")

	// ── Postgres ────────────────────────────────────────────────────────
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// ── River migrations (idempotent) ───────────────────────────────────
	riverMigrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		slog.Error("river migrator", "error", err)
		os.Exit(1)
	}
	if _, err := riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			slog.Warn("river migration: already applied (concurrent startup)", "error", err)
		} else {
			slog.Error("river migration", "error", err)
			os.Exit(1)
		}
	}
	slog.Info("river migrations applied")

	// ── MinIO storage ───────────────────────────────────────────────────
	stor, err := storage.NewMinioStore(storage.MinioConfigFromEnv())
	if err != nil {
		slog.Error("minio storage", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to object storage")

	// ── Repository ──────────────────────────────────────────────────────
	store := db.NewStore(pool)

	// ── River workers ───────────────────────────────────────────────────
	workers := river.NewWorkers()
	river.AddWorker(workers, &invoice.StampAndHashWorker{
		Store:   store,
		Storage: stor,
	})

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		slog.Error("river client", "error", err)
		os.Exit(1)
	}

	// Start processing jobs.
	if err := riverClient.Start(ctx); err != nil {
		slog.Error("river start", "error", err)
		os.Exit(1)
	}
	slog.Info("worker started — processing stamp_and_hash jobs")

	// ── Block until signal ──────────────────────────────────────────────
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	<-done

	slog.Info("worker shutting down")

	// Stop River gracefully (finishes in-progress jobs).
	if err := riverClient.Stop(ctx); err != nil {
		slog.Error("river stop", "error", err)
	}
	slog.Info("worker stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
