// Command worker consumes analytics.jobs and executes the registered Job
// implementations — see ADR-0008. Adding a new analytics capability means
// registering a new job.Job here, not building a new deployable service.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/hegarty/shop_platform/config"
	"github.com/hegarty/shop_platform/db"
	"github.com/hegarty/shop_platform/logging"
	"github.com/hegarty/shop_platform/otelx"
	"github.com/hegarty/shop_platform/redpanda"

	"github.com/hegarty/shop_analytics/internal/job"
	"github.com/hegarty/shop_analytics/internal/jobs"
	"github.com/hegarty/shop_analytics/internal/scheduler"
	"github.com/hegarty/shop_analytics/internal/store"
	"github.com/hegarty/shop_analytics/internal/worker"
)

func main() {
	logger := logging.New("shop-worker", slog.LevelInfo)
	slog.SetDefault(logger)

	l := config.NewLoader()
	dbHost := l.String("DATABASE_HOST")
	dbPort := l.IntDefault("DATABASE_PORT", 5432)
	dbName := l.String("DATABASE_NAME")
	dbUser := l.String("DATABASE_USER")
	dbPassword := l.String("DATABASE_PASSWORD")
	redpandaBrokers := l.String("REDPANDA_BROKERS")
	otelEndpoint := l.StringDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if err := l.Err(); err != nil {
		logger.Error("configuration error", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if otelEndpoint != "" {
		shutdown, err := otelx.Bootstrap(ctx, otelx.Config{
			ServiceName: "shop-worker", ServiceVersion: "dev",
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

	registry := job.NewRegistry(
		&jobs.SalesChannelBreakdown{Pool: pool},
		// Future jobs (inventory.low_stock, orders.unfulfilled, etc.)
		// register here — not as new deployable services. See ADR-0008.
	)

	w := &worker.Worker{
		Registry: registry,
		Store:    store.NewResultStore(pool),
		Notifier: producer,
	}

	consumer, err := redpanda.NewConsumer([]string{redpandaBrokers}, "shop-analytics-worker", []string{scheduler.AnalyticsJobsTopic})
	if err != nil {
		logger.Error("redpanda consumer init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer consumer.Close()

	logger.Info("shop-worker started")
	err = consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
		var req job.Request
		if err := json.Unmarshal(record.Value, &req); err != nil {
			logger.Error("failed to decode job request", slog.Any("error", err))
			return err
		}
		if err := w.HandleRequest(ctx, req); err != nil {
			logger.Error("job execution failed",
				slog.String("tenant_id", req.TenantID), slog.String("job", req.Job), slog.Any("error", err))
			return err
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		logger.Error("worker consumer stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
