// Package main is the entry point for the LLMHub scheduler service.
//
// The scheduler owns the upstream account pool: it selects accounts for
// incoming calls (using health + tier + quota scoring), enforces session
// stickiness, tracks account health from call feedback, and drives the
// account lifecycle state machine. Exposed via HTTP/JSON RPC today; the
// wire schema mirrors proto/scheduler/v1 so the gRPC swap is mechanical.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/llmhub/llmhub/internal/platform/config"
	"github.com/llmhub/llmhub/internal/platform/db"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/pool"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/scheduler"
	schedserver "github.com/llmhub/llmhub/internal/scheduler/server"
)

const serviceName = "scheduler"

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

	poolSvc := pool.New(poolrepo.New(dbpool))
	schedSvc := scheduler.NewService(poolSvc, scheduler.NewPicker(poolSvc, scheduler.NewMemStickiness()))

	token := os.Getenv("LLMHUB_INTERNAL_TOKEN")
	if token == "" {
		logger.Warn("internal token not set — scheduler will reject all RPC requests")
	}
	srv := schedserver.New(logger, schedSvc, token)

	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = ":9001"
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		logger.Info("scheduler listening", "addr", addr, "env", cfg.Env)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("scheduler shutting down")
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
}
