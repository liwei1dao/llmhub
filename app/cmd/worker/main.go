// Package main is the entry point for the LLMHub worker service.
//
// The worker consumes NATS events (call.completed for usage rollup)
// and runs periodic jobs (HoldReaper releases expired billing freezes;
// later milestones add daily reconciliation, account warmup, quota
// sync). All side-effects are best-effort and log-failure-only — the
// authoritative state lives in the wallet/metering tables.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	billingclient "github.com/llmhub/llmhub/internal/billing/client"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/platform/mq"
	"github.com/llmhub/llmhub/internal/wallet"
	walletrepo "github.com/llmhub/llmhub/internal/wallet/repo"
	"github.com/llmhub/llmhub/internal/worker"
)

const serviceName = "worker"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(serviceName)
	cfg, err := config.Load(serviceName)
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}

	dbpool, err := db.Open(ctx, cfg.DB)
	if err != nil {
		logger.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	meterRepo := meteringrepo.New(dbpool)
	walletSvc := wallet.NewService(walletrepo.New(dbpool))

	// --- NATS aggregator (call.completed → metering.call_logs) ---
	if cfg.NATS.URL != "" {
		if nc, err := mq.Open(cfg.NATS); err == nil {
			defer nc.Drain() //nolint:errcheck
			agg := &worker.Aggregator{Logger: logger, NC: nc, Sink: meterRepo}
			if _, err := agg.Subscribe(ctx); err != nil {
				logger.Error("aggregator subscribe failed", "err", err)
				os.Exit(1)
			}
		} else {
			logger.Warn("nats unavailable; aggregator disabled", "err", err)
		}
	} else {
		logger.Warn("NATS URL not configured; aggregator disabled")
	}

	// --- HoldReaper (release expired freezes) ---
	var releaser worker.HoldReleaser = walletSvc
	if addr := os.Getenv("LLMHUB_BILLING_ADDR"); addr != "" {
		releaser = billingclient.New(addr, os.Getenv("LLMHUB_INTERNAL_TOKEN"))
		logger.Info("hold reaper: using remote billing", "addr", addr)
	}
	reaper := &worker.HoldReaper{
		Logger:  logger,
		Holds:   meterHoldStore{repo: meterRepo},
		Billing: releaser,
		Tick:    30 * time.Second,
		Batch:   100,
	}
	go reaper.Run(ctx)

	// --- DailyRecon cron (00:05 UTC) ---
	recon := &worker.DailyRecon{
		Logger:    logger,
		Store:     meterRepo,
		HourUTC:   0,
		MinuteUTC: 5,
	}
	go recon.Run(ctx)

	logger.Info("worker started", "env", cfg.Env)
	<-ctx.Done()
	logger.Info("worker shutting down", "released", reaper.Released(), "recon_runs", recon.Runs())
}

// meterHoldStore adapts metering/repo into worker.HoldStore. Kept here
// so worker doesn't import the repo package directly (preserves the
// hexagonal boundary).
type meterHoldStore struct{ repo *meteringrepo.Repo }

func (m meterHoldStore) ListExpiredHolds(ctx context.Context, limit int) ([]worker.ExpiredHold, error) {
	rows, err := m.repo.ListExpiredHolds(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]worker.ExpiredHold, len(rows))
	for i, r := range rows {
		out[i] = worker.ExpiredHold{
			RequestID: r.RequestID, UserID: r.UserID,
			AccountID: r.AccountID, AmountCents: r.AmountCents,
		}
	}
	return out, nil
}

func (m meterHoldStore) MarkHoldExpired(ctx context.Context, requestID string) error {
	return m.repo.MarkHoldExpired(ctx, requestID)
}
