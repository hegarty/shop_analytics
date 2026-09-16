// Command scheduler periodically checks job_schedules for due work and
// publishes it onto analytics.jobs. It never executes analytics itself —
// see ADR-0008.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hegarty/shop_platform/config"
	"github.com/hegarty/shop_platform/db"
	"github.com/hegarty/shop_platform/logging"
	"github.com/hegarty/shop_platform/otelx"
	"github.com/hegarty/shop_platform/redpanda"

	"github.com/hegarty/shop_analytics/internal/scheduler"
	"github.com/hegarty/shop_analytics/internal/store"
)

func main() {
	logger := logging.New("shop-scheduler", slog.LevelInfo)
	slog.SetDefault(logger)

	l := config.NewLoader()
	dbHost := l.String("DATABASE_HOST")
	dbPort := l.IntDefault("DATABASE_PORT", 5432)
	dbName := l.String("DATABASE_NAME")
	dbUser := l.String("DATABASE_USER")
	dbPassword := l.String("DATABASE_PASSWORD")
	redpandaBrokers := l.String("REDPANDA_BROKERS")
	tickInterval := l.IntDefault("SCHEDULER_TICK_SECONDS", 60)
	otelEndpoint := l.StringDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if err := l.Err(); err != nil {
		logger.Error("configuration error", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if otelEndpoint != "" {
		shutdown, err := otelx.Bootstrap(ctx, otelx.Config{
			ServiceName: "shop-scheduler", ServiceVersion: "dev",
			Endpoint: otelEndpoint, Insecure: true,
		})
		if err != nil {
			logger.Error("otel bootstrap failed", slog.Any("error", err))
			os.Exit(1)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdown(shutdownCtx)
		}()
	}

	pool, err := db.Connect(ctx, db.Config{Host: dbHost, Port: dbPort, Database: dbName, User: dbUser}, dbPassword)
	if err != nil {
		logger.Error("db connect failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	producer, err := redpanda.NewProducer([]string{redpandaBrokers})
	if err != nil {
		logger.Error("redpanda producer init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer producer.Close()

	s := &scheduler.Scheduler{
		Store:     store.NewScheduleStore(pool),
		Publisher: producer,
	}

	ticker := time.NewTicker(time.Duration(tickInterval) * time.Second)
	defer ticker.Stop()

	logger.Info("shop-scheduler started", slog.Int("tick_seconds", tickInterval))
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		case now := <-ticker.C:
			if err := s.Tick(ctx, now.UTC()); err != nil {
				logger.Error("tick failed", slog.Any("error", err))
			}
		}
	}
}
