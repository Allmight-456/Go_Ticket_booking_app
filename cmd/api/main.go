package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Allmight-456/ticketflow/internal/config"
	"github.com/Allmight-456/ticketflow/internal/repository/cache"
	"github.com/Allmight-456/ticketflow/internal/repository/postgres"
	"github.com/Allmight-456/ticketflow/internal/router"
	"github.com/Allmight-456/ticketflow/internal/service"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// ── Logging ────────────────────────────────────────────────────────────
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("APP_ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// ── Config ─────────────────────────────────────────────────────────────
	_ = godotenv.Load() // silently ignore missing .env in production

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Database ───────────────────────────────────────────────────────────
	pool, err := postgres.NewPool(ctx,
		cfg.Database.URL,
		cfg.Database.MaxConns,
		cfg.Database.MinConns,
		cfg.Database.MaxConnLifetime,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	migrationsPath := migrationsDir()
	if err := postgres.RunMigrations(cfg.Database.URL, migrationsPath); err != nil {
		log.Fatal().Err(err).Str("path", migrationsPath).Msg("migrations failed")
	}

	// ── Redis ──────────────────────────────────────────────────────────────
	redisRepo, err := cache.NewRedisRepo(cfg.Redis.URL, cfg.Redis.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("redis connection failed")
	}
	defer redisRepo.Close()

	// ── Repositories ───────────────────────────────────────────────────────
	userRepo := postgres.NewUserRepo(pool)
	eventRepo := postgres.NewEventRepo(pool)
	bookingRepo := postgres.NewBookingRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)

	// ── Services ───────────────────────────────────────────────────────────
	auditSvc := service.NewAuditService(auditRepo)
	authSvc := service.NewAuthService(userRepo, cfg.JWT.Secret, cfg.JWT.Expiration)
	eventSvc := service.NewEventService(eventRepo, redisRepo, auditSvc)
	bookingSvc := service.NewBookingService(bookingRepo, eventRepo, redisRepo, auditSvc)

	// ── Router ─────────────────────────────────────────────────────────────
	r := router.New(authSvc, eventSvc, bookingSvc, auditSvc, redisRepo)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// ── Start server ───────────────────────────────────────────────────────
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────────────────────
	<-ctx.Done()
	log.Info().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
	log.Info().Msg("server stopped")
}

// migrationsDir returns the migrations directory path.
// Override with the MIGRATIONS_PATH env var; defaults to "./migrations"
// (works both locally from project root and inside the Docker container where
// the WORKDIR is /app and migrations are copied to /app/migrations).
func migrationsDir() string {
	if p := os.Getenv("MIGRATIONS_PATH"); p != "" {
		return p
	}
	return "migrations"
}
