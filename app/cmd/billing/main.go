// Package main is the entry point for the LLMHub billing service.
//
// The billing service handles balance freeze/settle/release, pricing
// lookup, usage aggregation, and reconciliation against upstream bills.
// M7 exposes an HTTP/JSON RPC; M8 adds the gRPC transport on the same
// service surface.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/llmhub/llmhub/internal/billing/server"
	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/wallet"
	walletrepo "github.com/llmhub/llmhub/internal/wallet/repo"
)

const serviceName = "billing"

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

	walletSvc := wallet.NewService(walletrepo.New(dbpool))

	token := os.Getenv("LLMHUB_INTERNAL_TOKEN")
	if token == "" {
		logger.Warn("internal token not set — billing will reject all requests")
	}
	srv := server.New(logger, walletSvc, token)

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":8082"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		logger.Info("billing listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("billing shutting down")
	shutdownCtx, sc := context.WithTimeout(context.Background(), 10*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
}
