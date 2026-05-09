package router

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/mithunb9/arbiter/internal/adapter"
	"github.com/mithunb9/arbiter/internal/config"
)

type RouteResult struct {
	Response     *adapter.ChatResponse
	AdapterType  string
	AdapterName  string
	TierName     string
	FallbackUsed bool
}

type Router struct {
	tiers    []config.TierConfig
	adapters map[string]adapter.Adapter
	logger   *zap.Logger
}

func New(tiers []config.TierConfig, adapters map[string]adapter.Adapter, logger *zap.Logger) *Router {
	return &Router{
		tiers:    tiers,
		adapters: adapters,
		logger:   logger,
	}
}

func (r *Router) Route(ctx context.Context, tierName string, req *adapter.ChatRequest) (*RouteResult, error) {
	tier, err := r.findTier(tierName)
	if err != nil {
		return nil, err
	}

	var lastErr error
	fallbackUsed := false

	for i, adapterName := range tier.Adapters {
		a, ok := r.adapters[adapterName]
		if !ok {
			return nil, fmt.Errorf("adapter %q not found", adapterName)
		}

		if i > 0 {
			fallbackUsed = true
			r.logger.Warn("falling back to next adapter",
				zap.String("tier", tierName),
				zap.String("adapter", adapterName),
				zap.Error(lastErr),
			)
		}

		r.logger.Info("routing request",
			zap.String("tier", tierName),
			zap.String("adapter", adapterName),
		)

		resp, err := a.Chat(ctx, req)
		if err != nil {
			lastErr = err
			if tier.Fallback && i < len(tier.Adapters)-1 {
				continue
			}
			return nil, fmt.Errorf("adapter %q failed: %w", adapterName, err)
		}

		return &RouteResult{
			Response:     resp,
			AdapterType:  a.Type(),
			AdapterName:  adapterName,
			TierName:     tierName,
			FallbackUsed: fallbackUsed,
		}, nil
	}

	return nil, fmt.Errorf("all adapters failed for tier %q: %w", tierName, lastErr)
}

func (r *Router) findTier(name string) (*config.TierConfig, error) {
	for i := range r.tiers {
		if r.tiers[i].Name == name {
			return &r.tiers[i], nil
		}
	}
	return nil, fmt.Errorf("tier %q not found, check your config.yaml", name)
}
