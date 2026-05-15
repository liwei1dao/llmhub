// Package main is the single-binary entry point for the LLMHub
// platform. The platform follows a「聚合 SDK」mode (see
// docs/03-核心数据结构设计-v2.md):
//
//   - SDK clients (downloaded by users) call POST /sdk/credentials/issue
//     to exchange (id, key) + sku_id for a short-lived lease + the real
//     upstream credential.
//   - SDK clients then call upstream (火山 / DeepSeek / ...) directly,
//     and asynchronously POST /sdk/usage/report so the platform can
//     decrement quotas and adjust binding health.
//   - The platform itself is NOT in the upstream request path.
//
// Default port: 8080. The single chi router fans out to:
//
//   - /sdk/*        — SDK API (credential issue + usage report)
//   - /api/user/*   — end-user console (signup / subscriptions / wallet / SDK download)
//   - /api/admin/*  — operator admin (vendor pool / SKUs / users / observability)
//   - /health, /ready
//
// The pre-v0.2 OpenAI-compatible /v1/chat/completions HTTP gateway has
// been removed: the platform never proxies upstream calls in the
// 聚合 SDK mode.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/llmhub/llmhub/internal/account"
	"github.com/llmhub/llmhub/internal/admin"
	"github.com/llmhub/llmhub/internal/adminauth"
	adminauthrepo "github.com/llmhub/llmhub/internal/adminauth/repo"
	"github.com/llmhub/llmhub/internal/audit"
	"github.com/llmhub/llmhub/internal/catalog"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/iam"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/platform/cache"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/platform/mail"
	"github.com/llmhub/llmhub/internal/platform/mq"
	"github.com/llmhub/llmhub/internal/platform/vault"
	"github.com/llmhub/llmhub/internal/pool"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/sdkapi"
	_ "github.com/llmhub/llmhub/internal/vendors" // 触发所有 vendor 适配器 init()，
	// 把自己登记到 catalog.adapterRegistry（admin「服务列表」的"代码已实现"靠这条链路）
	"github.com/llmhub/llmhub/internal/wallet"
	walletrepo "github.com/llmhub/llmhub/internal/wallet/repo"
	"github.com/llmhub/llmhub/internal/worker"

	"github.com/nats-io/nats.go"
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

	// ---- domain services ----
	iamSvc := iam.NewService(iamrepo.New(dbpool))
	walletSvc := wallet.NewService(walletrepo.New(dbpool))
	poolSvc := pool.New(poolrepo.New(dbpool))
	catalogSvc := catalog.NewService(catalogrepo.New(dbpool))
	meterRepo := meteringrepo.New(dbpool)

	// Optional NATS connection for usage-event fanout. The aggregator
	// worker subscribes to it; if unavailable, usage still gets recorded
	// directly into metering.call_logs by the SDK report path — NATS
	// only adds out-of-process aggregation.
	var natsConn *nats.Conn
	if cfg.NATS.URL != "" {
		if nc, err := mq.Open(cfg.NATS); err == nil {
			natsConn = nc
			logger.Info("nats connected", "url", cfg.NATS.URL)
		} else {
			logger.Warn("nats unavailable; usage events stay in-process", "err", err)
		}
	}

	// ---- admin auth (后台管理员独立用户体系) ----
	adminAuthRepo := adminauthrepo.New(dbpool)
	adminAuthSvc := adminauth.New(adminAuthRepo)
	if bootAcct, bootPass := os.Getenv("LLMHUB_ADMIN_BOOTSTRAP_ACCOUNT"), os.Getenv("LLMHUB_ADMIN_BOOTSTRAP_PASSWORD"); bootAcct != "" && bootPass != "" {
		bootName := os.Getenv("LLMHUB_ADMIN_BOOTSTRAP_NAME")
		if err := adminAuthSvc.EnsureBootstrap(ctx, bootAcct, bootPass, bootName); err != nil {
			logger.Warn("admin bootstrap skipped", "err", err)
		} else {
			logger.Info("admin bootstrap checked", "account", bootAcct)
		}
	} else {
		logger.Warn("LLMHUB_ADMIN_BOOTSTRAP_ACCOUNT/PASSWORD not set — first admin must be inserted manually")
	}

	// ---- admin server ----
	adminSrv := admin.New(logger, poolrepo.New(dbpool)).
		WithAuth(adminAuthSvc).
		WithIAM(iamrepo.New(dbpool)).
		WithCatalog(catalogrepo.New(dbpool)).
		WithMetering(meterRepo).
		WithWallet(walletSvc).
		WithPool(poolSvc).
		WithAudit(audit.NewPgRecorder(dbpool, logger))

	// ---- SDK API: the only entry point user binaries hit ----
	// SDK does (id, key) + sku_id → short-lived lease + real upstream
	// auth_payload. Platform is NOT on the upstream request path.
	sdkSrv := sdkapi.New(sdkapi.Deps{
		Logger:   logger,
		Auth:     sdkAuthAdapter{s: iamSvc},
		Catalog:  catalogSvc,
		Pool:     poolSvc,
		Subs:     iamrepo.New(dbpool).Subscriptions(),
		Metering: meterRepo,
		Vault:    vault.NewCached(vault.DevInline{}),
	})
	// SDK needs upstream endpoint + protocol_family hints so its in-binary
	// adapter can pick the right wire path; both come from the static
	// catalog dictionary.
	sdkapi.SetProductHinter(func(productID string) (string, string, bool) {
		p, ok := catalog.LookupProduct(productID)
		if !ok {
			return "", "", false
		}
		return p.EndpointTemplate, p.ProtocolFamily, true
	})

	// ---- redis + mailer (for email verification on user signup) ----
	// Both are optional in dev: when Redis isn't configured we skip the
	// verifier entirely and registration falls back to legacy mode (no
	// email verification). When SMTP isn't configured but Redis is, the
	// mailer downgrades to a dev mailer that logs codes to stdout.
	var verifier *account.Verifier
	if cfg.Redis.Addr != "" {
		rds, err := cache.Open(ctx, cfg.Redis)
		if err != nil {
			logger.Warn("redis unavailable; email verification disabled", "err", err)
		} else {
			mailer := mail.New(cfg.SMTP, logger)
			verifier = account.NewVerifier(rds, mailer)
			logger.Info("email verifier ready", "smtp_enabled", mailer.Enabled())
		}
	} else {
		logger.Warn("LLMHUB_REDIS_ADDR not set; email verification disabled")
	}

	// ---- compose: account-server hosts /api/user/*, /api/admin/*, /sdk/* ----
	accountSrv := account.New(logger, iamSvc, walletSvc, adminSrv).
		WithMetering(meterRepo).
		WithSDKAPI(sdkSrv).
		WithSubscriptions(iamrepo.New(dbpool).Subscriptions()).
		WithCatalog(catalogSvc).
		WithPinger(dbpool)
	if verifier != nil {
		accountSrv = accountSrv.WithVerifier(verifier)
	}

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":8080"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           accountSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout:      0,
	}

	// ---- background workers ----
	if natsConn != nil {
		agg := &worker.Aggregator{Logger: logger, NC: natsConn, Sink: meterRepo}
		if _, err := agg.Subscribe(ctx); err != nil {
			logger.Error("aggregator subscribe failed", "err", err)
		}
	}
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
	logger.Info("llmhub shutting down", "recon_runs", recon.Runs())
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
	if natsConn != nil {
		_ = natsConn.Drain()
	}
}

// sdkAuthAdapter bridges *iam.Service to sdkapi.AuthResolver. The iam
// service returns the full APIKey row; the SDK API only needs (user_id,
// api_key_id), so we adapt down here rather than widening the SDK
// interface to import iam directly.
type sdkAuthAdapter struct{ s *iam.Service }

func (a sdkAuthAdapter) AuthenticateAPIKey(ctx context.Context, plaintext string) (int64, int64, error) {
	k, err := a.s.AuthenticateAPIKey(ctx, plaintext)
	if err != nil {
		return 0, 0, err
	}
	return k.UserID, k.ID, nil
}
