package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/kaffie-1517/provenn/internal/auth"
	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/invoice"
	"github.com/kaffie-1517/provenn/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := envOr("PORT", "8080")
	databaseURL := envOr("DATABASE_URL", "postgres://provenn:provenn@localhost:5432/provenn?sslmode=disable")
	jwtSecret := envOr("JWT_SECRET", "provenn-demo-secret-change-in-production")

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
		// Tolerate duplicate-key errors from concurrent API/worker startup.
		if strings.Contains(err.Error(), "duplicate key") {
			slog.Warn("river migration: already applied (concurrent startup)", "error", err)
		} else {
			slog.Error("river migration", "error", err)
			os.Exit(1)
		}
	}
	slog.Info("river migrations applied")

	// ── River client (insert-only — no workers in the API process) ──────
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		slog.Error("river client", "error", err)
		os.Exit(1)
	}

	// ── MinIO storage ───────────────────────────────────────────────────
	stor, err := storage.NewMinioStore(storage.MinioConfigFromEnv())
	if err != nil {
		slog.Error("minio storage", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to object storage")

	// ── Services & handlers ─────────────────────────────────────────────
	store := db.NewStore(pool)

	invoiceSvc := &invoice.Service{
		Store:   store,
		Storage: stor,
		EnqueueJob: func(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
			_, err := riverClient.Insert(ctx, args, opts)
			return err
		},
	}
	invoiceHandlers := &invoice.Handlers{Service: invoiceSvc}

	authHandlers := &auth.Handlers{
		Store:     store,
		JWTSecret: jwtSecret,
	}

	// ── Router ──────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Partner-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// ── Auth routes (public) ────────────────────────────────────────────
	r.Post("/api/v1/auth/register", authHandlers.Register)
	r.Post("/api/v1/auth/login", authHandlers.Login)

	// ── Public invoice lookup (no auth) ─────────────────────────────────
	r.Get("/api/v1/invoices/{referenceCode}", invoiceHandlers.GetStatus)
	r.Get("/api/v1/invoices/{referenceCode}/download", invoiceHandlers.Download)

	// ── JWT-protected routes ────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtSecret))

		// Provider: issue invoice
		r.With(auth.RequireRole("provider")).Post("/api/v1/invoices", invoiceHandlers.CreateFromProvider)

		// Employee: submit verification
		r.With(auth.RequireRole("employee")).Post("/api/v1/verifications", placeholderHandler("submit verification"))

		// Employee + company_admin: list verifications
		r.With(auth.RequireRole("employee", "company_admin")).Get("/api/v1/verifications", placeholderHandler("list verifications"))

		// Company admin: approve + export
		r.With(auth.RequireRole("company_admin")).Patch("/api/v1/verifications/{id}/approve", placeholderHandler("approve verification"))
		r.With(auth.RequireRole("company_admin")).Get("/api/v1/verifications/export", placeholderHandler("export verifications"))

		// Platform admin
		r.With(auth.RequireRole("platform_admin")).Get("/api/v1/admin/partners", placeholderHandler("list partners"))
		r.With(auth.RequireRole("platform_admin")).Get("/api/v1/admin/companies", placeholderHandler("list companies"))
	})

	// ── Partner API-key routes ──────────────────────────────────────────
	r.Route("/api/v1/partner", func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware(store))
		r.Post("/invoices", invoiceHandlers.CreateFromPartner)
	})

	// ── Server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("api server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

func placeholderHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"todo": name})
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
