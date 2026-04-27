// Package main is the entry point for the LLMHub account service.
//
// The account service hosts user management, API key management, wallet
// (balance + recharge + invoice), admin APIs, and the user-facing
// console REST APIs. Exposes HTTP for UIs and gRPC for internal auth.
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
	"github.com/llmhub/llmhub/internal/catalog"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/iam"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/log"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/wallet"
	walletrepo "github.com/llmhub/llmhub/internal/wallet/repo"
)

const serviceName = "account"

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

	// Reconcile declarative catalog YAML into DB on startup.
	if err := catalog.NewLoader(dbpool).Reconcile(ctx, "configs"); err != nil {
		logger.Warn("catalog reconcile skipped", "err", err)
	}

	iamSvc := iam.NewService(iamrepo.New(dbpool))
	walletSvc := wallet.NewService(walletrepo.New(dbpool))

	adminToken := os.Getenv("LLMHUB_ADMIN_TOKEN")
	if adminToken == "" {
		logger.Warn("admin token not set — /api/admin/* routes will reject all requests")
	}
	adminSrv := admin.New(logger, poolrepo.New(dbpool), adminToken).
		WithIAM(iamrepo.New(dbpool)).
		WithCatalog(catalogrepo.New(dbpool)).
		WithMetering(meteringrepo.New(dbpool)).
		WithWallet(walletSvc)

	srv := account.New(logger, iamSvc, walletSvc, adminSrv).
		WithMetering(meteringrepo.New(dbpool)).
		WithPinger(dbpool)

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":8081"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout:      time.Duration(cfg.HTTP.WriteTimeoutSec) * time.Second,
	}

	go func() {
		logger.Info("account listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("account shutting down")
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
}
