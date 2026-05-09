package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mithunb9/arbiter/internal/config"
	"github.com/mithunb9/arbiter/internal/health"
)

func main() {
	// Healthcheck subcommand — used by Docker HEALTHCHECK instruction
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://localhost:9099/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load("/app/config.yaml")
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	logger.Info("arbiter starting", zap.Int("port", cfg.Server.Port))

	router := gin.New()
	router.Use(gin.Recovery())

	health.RegisterRoutes(router)
	// TODO: proxy.RegisterRoutes(router, arbiterRouter)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
