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

	"github.com/mithunb9/arbiter/internal/adapter"
	"github.com/mithunb9/arbiter/internal/config"
	"github.com/mithunb9/arbiter/internal/health"
	"github.com/mithunb9/arbiter/internal/proxy"
	"github.com/mithunb9/arbiter/internal/router"
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

	adapters, err := buildAdapters(cfg, logger)
	if err != nil {
		logger.Fatal("failed to init adapters", zap.Error(err))
	}

	ar := router.New(cfg.Tiers, adapters, logger)
	proxyHandler := proxy.New(ar, logger)

	mux := gin.New()
	mux.Use(gin.Recovery())

	health.RegisterRoutes(mux)
	proxy.RegisterRoutes(mux, proxyHandler)

	logger.Info("arbiter starting", zap.Int("port", cfg.Server.Port))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: mux,
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

func buildAdapters(cfg *config.Config, logger *zap.Logger) (map[string]adapter.Adapter, error) {
	adapters := make(map[string]adapter.Adapter, len(cfg.Adapters))
	for _, ac := range cfg.Adapters {
		var a adapter.Adapter
		var err error
		switch ac.Type {
		case "anthropic":
			a, err = adapter.NewAnthropicAdapter(ac.Name, ac.APIKey, ac.Model,
				ac.CostPerMillionInputTokens, ac.CostPerMillionOutputTokens)
		case "ollama":
			a, err = adapter.NewOllamaAdapter(ac.Name, ac.BaseURL, ac.Model)
		default:
			return nil, fmt.Errorf("unknown adapter type %q for adapter %q", ac.Type, ac.Name)
		}
		if err != nil {
			return nil, fmt.Errorf("adapter %q: %w", ac.Name, err)
		}
		logger.Info("registered adapter", zap.String("name", ac.Name), zap.String("type", ac.Type))
		adapters[ac.Name] = a
	}
	return adapters, nil
}
