// Package main is the entry point for the LLMHub API gateway service.
//
// The gateway exposes the public HTTP/SSE/WebSocket surface to end users
// (OpenAI + Anthropic compatible), authenticates requests, performs
// billing pre-authorization, calls the scheduler to pick an upstream
// account, and proxies / streams traffic to the selected provider.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	billingclient "github.com/llmhub/llmhub/internal/billing/client"
	"github.com/llmhub/llmhub/internal/capability/chat"
	"github.com/llmhub/llmhub/internal/capability/embedding"
	schedclient "github.com/llmhub/llmhub/internal/scheduler/client"
	"github.com/llmhub/llmhub/internal/catalog"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/gateway"
	"github.com/llmhub/llmhub/internal/iam"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/httpclient"
	"github.com/llmhub/llmhub/internal/platform/cache"
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

	// Provider adapters are registered via init() on import.
	_ "github.com/llmhub/llmhub/internal/provider/anthropic"
	_ "github.com/llmhub/llmhub/internal/provider/dashscope"
	_ "github.com/llmhub/llmhub/internal/provider/deepgram"
	_ "github.com/llmhub/llmhub/internal/provider/deepl"
	_ "github.com/llmhub/llmhub/internal/provider/deepseek"
	_ "github.com/llmhub/llmhub/internal/provider/elevenlabs"
	_ "github.com/llmhub/llmhub/internal/provider/volc"
)

const serviceName = "gateway"

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

	iamSvc := iam.NewService(iamrepo.New(dbpool))
	walletSvc := wallet.NewService(walletrepo.New(dbpool))
	poolSvc := pool.New(poolrepo.New(dbpool))
	catalogSvc := catalog.NewService(catalogrepo.New(dbpool))
	schedSvc := scheduler.NewService(poolSvc, scheduler.NewPicker(poolSvc, scheduler.NewMemStickiness()))

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

	// Prefer Redis-backed rate limiting when configured; fall back to
	// in-process if Redis is absent so local dev still works.
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

	// Billing backend: prefer a remote billing service when configured
	// (LLMHUB_BILLING_ADDR), fall back to the in-process wallet service
	// for single-binary dev runs.
	var billing chat.BillingClient = gateway.WalletBilling{S: walletSvc}
	if addr := os.Getenv("LLMHUB_BILLING_ADDR"); addr != "" {
		billing = billingclient.New(addr, os.Getenv("LLMHUB_INTERNAL_TOKEN"))
		logger.Info("billing: using remote service", "addr", addr)
	}

	// Scheduler backend: same swap pattern. The in-process Service is
	// the default; LLMHUB_SCHEDULER_ADDR routes to a remote scheduler.
	var sched chat.SchedulerClient = schedSvc
	if addr := os.Getenv("LLMHUB_SCHEDULER_ADDR"); addr != "" {
		sched = schedclient.New(addr, os.Getenv("LLMHUB_INTERNAL_TOKEN"))
		logger.Info("scheduler: using remote service", "addr", addr)
	}

	// NATS publisher for call.completed events. Optional — if NATS is
	// unreachable we let the gateway run without metering rather than
	// fail closed (the data is recoverable from upstream bills).
	var publisher chat.EventPublisher
	if cfg.NATS.URL != "" {
		if nc, err := mq.Open(cfg.NATS); err == nil {
			publisher = gateway.NATSPublisher{NC: nc}
			logger.Info("call events: publishing to NATS", "url", cfg.NATS.URL)
		} else {
			logger.Warn("nats unavailable; call events will be dropped", "err", err)
		}
	}

	handler := gateway.NewServer(logger, gateway.Deps{
		Logger:    logger,
		Scheduler: sched,
		Billing:   billing,
		Auth:      gateway.IAMAuth{S: iamSvc},
		Provider:  invoker,
		Catalog:   catalogSvc,
		Publisher: publisher,
	}, gateway.Options{
		RateLimiter:        limiter,
		RateLimitPerSecond: 20,
		Embedding: &embedding.Deps{
			Logger:    logger,
			Scheduler: sched,
			Billing:   billing,
			Auth:      gateway.IAMAuth{S: iamSvc},
			Provider:  gateway.EmbeddingInvoker{D: dispatcher},
			Catalog:   catalogSvc,
			Publisher: publisher,
		},
	})

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":8080"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout:      0, // long-lived SSE streams
	}

	go func() {
		logger.Info("gateway listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("gateway shutting down")
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
}
