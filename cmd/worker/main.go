package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/invoice"
	"github.com/kaffie-1517/provenn/internal/observability"
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

	// ── River migrations (with advisory lock to avoid race) ────────────
	for attempt := 1; attempt <= 3; attempt++ {
		_, _ = pool.Exec(ctx, "SELECT pg_advisory_lock(42)")
		riverMigrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
		if err != nil {
			_, _ = pool.Exec(ctx, "SELECT pg_advisory_unlock(42)")
			slog.Error("river migrator", "error", err)
			os.Exit(1)
		}
		_, err = riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
		_, _ = pool.Exec(ctx, "SELECT pg_advisory_unlock(42)")
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "duplicate key") && attempt < 3 {
			slog.Warn("river migration: race detected, retrying", "attempt", attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if err != nil {
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
		JobTimeout: 5 * time.Minute,
		WorkerMiddleware: []rivertype.WorkerMiddleware{
			&observability.RiverMiddleware{},
		},
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

	// ── Start metrics server ────────────────────────────────────────────
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("worker metrics server listening on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			slog.Error("worker metrics server failed", "error", err)
		}
	}()

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
