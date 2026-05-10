// Package main is the single-binary entry point for the LLMHub
// platform. Everything runs in one process:
//
//   - public API gateway (`/v1/...` OpenAI/Anthropic-compat)
//   - account / iam / wallet HTTP API (`/api/user/...`)
//   - admin ops API (`/api/admin/...`)
//   - scheduler (in-process, no RPC hop)
//   - billing (the wallet service is called directly)
//   - background workers (aggregator, hold-reaper, daily-recon)
//
// Default port: 8080. All routes share one chi router so the public
// edge, the console, and the admin can be served from the same listener
// behind one reverse proxy.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/account"
	"github.com/llmhub/llmhub/internal/admin"
	"github.com/llmhub/llmhub/internal/capability/chat"
	"github.com/llmhub/llmhub/internal/capability/embedding"
	"github.com/llmhub/llmhub/internal/catalog"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/gateway"
	"github.com/llmhub/llmhub/internal/iam"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/platform/cache"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/httpclient"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/platform/mq"
	"github.com/llmhub/llmhub/internal/platform/ratelimit"
	"github.com/llmhub/llmhub/internal/platform/vault"
	"github.com/llmhub/llmhub/internal/pool"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/internal/wallet"
	walletrepo "github.com/llmhub/llmhub/internal/wallet/repo"
	"github.com/llmhub/llmhub/internal/worker"

	"github.com/nats-io/nats.go"

	// Provider adapters register via init() on import.
	_ "github.com/llmhub/llmhub/internal/provider/anthropic"
	_ "github.com/llmhub/llmhub/internal/provider/dashscope"
	_ "github.com/llmhub/llmhub/internal/provider/deepgram"
	_ "github.com/llmhub/llmhub/internal/provider/deepl"
	_ "github.com/llmhub/llmhub/internal/provider/deepseek"
	_ "github.com/llmhub/llmhub/internal/provider/elevenlabs"
	_ "github.com/llmhub/llmhub/internal/provider/volc"
)

const serviceName = "llmhub"

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

	// Catalog reconcile from configs/providers/*.yaml at startup.
	if err := catalog.NewLoader(dbpool).Reconcile(ctx, "configs"); err != nil {
		logger.Warn("catalog reconcile skipped", "err", err)
	}

	// ---- domain services (in-process, shared across the router) ----
	iamSvc := iam.NewService(iamrepo.New(dbpool))
	walletSvc := wallet.NewService(walletrepo.New(dbpool))
	poolSvc := pool.New(poolrepo.New(dbpool))
	catalogSvc := catalog.NewService(catalogrepo.New(dbpool))
	schedSvc := scheduler.NewService(poolSvc, scheduler.NewPicker(poolSvc, scheduler.NewMemStickiness()))
	meterRepo := meteringrepo.New(dbpool)

	providerMgr := provider.NewManager()
	if err := providerMgr.LoadDir("configs/providers"); err != nil {
		logger.Warn("provider manager load failed (continuing without providers)", "err", err)
	}

	dispatcher := &gateway.ProviderDispatcher{
		Catalog:   catalogSvc,
		Pool:      poolSvc,
		Providers: providerMgr,
		Vault:     vault.NewCached(vault.DevInline{}),
		HTTP:      httpclient.New(),
	}

	var invoker chat.ProviderInvoker = gateway.RealProvider{D: dispatcher}
	if os.Getenv("LLMHUB_GATEWAY_MOCK") == "1" {
		invoker = gateway.MockProvider{}
		logger.Warn("gateway running with MockProvider (dev mode)")
	}

	// ---- platform: rate limiter (Redis-preferred, in-mem fallback) ----
	var limiter ratelimit.Limiter = ratelimit.NewMemory()
	if cfg.Redis.Addr != "" {
		if rc, err := cache.Open(ctx, cfg.Redis); err == nil {
			limiter = ratelimit.Fallback{
				Primary:   ratelimit.NewRedis(rc, "llmhub:rl:"),
				Secondary: ratelimit.NewMemory(),
			}
		} else {
			logger.Warn("redis unavailable; using in-memory rate limiter", "err", err)
		}
	}

	// ---- platform: NATS publisher (optional) ----
	var publisher chat.EventPublisher
	var natsConn *nats.Conn
	if cfg.NATS.URL != "" {
		if nc, err := mq.Open(cfg.NATS); err == nil {
			publisher = gateway.NATSPublisher{NC: nc}
			natsConn = nc
			logger.Info("call events: publishing to NATS", "url", cfg.NATS.URL)
		} else {
			logger.Warn("nats unavailable; call events will be dropped", "err", err)
		}
	}

	// ---- gateway HTTP handler (/v1/...) ----
	gatewayHandler := gateway.NewServer(logger, gateway.Deps{
		Logger:    logger,
		Scheduler: schedSvc,
		Billing:   gateway.WalletBilling{S: walletSvc},
		Auth:      gateway.IAMAuth{S: iamSvc},
		Provider:  invoker,
		Catalog:   catalogSvc,
		Publisher: publisher,
	}, gateway.Options{
		RateLimiter:        limiter,
		RateLimitPerSecond: 20,
		Embedding: &embedding.Deps{
			Logger:    logger,
			Scheduler: schedSvc,
			Billing:   gateway.WalletBilling{S: walletSvc},
			Auth:      gateway.IAMAuth{S: iamSvc},
			Provider:  gateway.EmbeddingInvoker{D: dispatcher},
			Catalog:   catalogSvc,
			Publisher: publisher,
		},
	})

	// ---- account + admin HTTP handler (/api/user/* + /api/admin/*) ----
	adminToken := os.Getenv("LLMHUB_ADMIN_TOKEN")
	if adminToken == "" {
		logger.Warn("admin token not set — /api/admin/* routes will reject all requests")
	}
	adminSrv := admin.New(logger, poolrepo.New(dbpool), adminToken).
		WithIAM(iamrepo.New(dbpool)).
		WithCatalog(catalogrepo.New(dbpool)).
		WithMetering(meterRepo).
		WithWallet(walletSvc)
	accountSrv := account.New(logger, iamSvc, walletSvc, adminSrv).
		WithMetering(meterRepo).
		WithPinger(dbpool)

	// ---- compose router: gateway under /v1, account elsewhere ----
	root := chi.NewRouter()
	root.Mount("/v1", gatewayHandler)
	root.Mount("/", accountSrv.Handler())

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":8080"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeoutSec) * time.Second,
		// SSE streams must not be killed by a write deadline.
		WriteTimeout: 0,
	}

	// ---- start background workers in-process ----
	if natsConn != nil {
		agg := &worker.Aggregator{Logger: logger, NC: natsConn, Sink: meterRepo}
		if _, err := agg.Subscribe(ctx); err != nil {
			logger.Error("aggregator subscribe failed", "err", err)
		}
	}
	reaper := &worker.HoldReaper{
		Logger:  logger,
		Holds:   meterHoldStore{repo: meterRepo},
		Billing: walletSvc,
		Tick:    30 * time.Second,
		Batch:   100,
	}
	go reaper.Run(ctx)
	recon := &worker.DailyRecon{Logger: logger, Store: meterRepo, HourUTC: 0, MinuteUTC: 5}
	go recon.Run(ctx)

	go func() {
		logger.Info("llmhub listening", "addr", addr, "env", cfg.Env)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("llmhub shutting down", "released", reaper.Released(), "recon_runs", recon.Runs())
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
	if natsConn != nil {
		_ = natsConn.Drain()
	}
}

// meterHoldStore adapts metering/repo into worker.HoldStore. Kept here
// so worker doesn't import the repo package directly.
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
