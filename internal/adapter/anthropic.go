package adapter

import (
	"context"
	"fmt"
)

type AnthropicAdapter struct {
	name   string
	apiKey string
	model  string
}

func NewAnthropicAdapter(name, apiKey, model string) (*AnthropicAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("adapter %q: api_key is required", name)
	}
	return &AnthropicAdapter{
		name:   name,
		apiKey: apiKey,
		model:  model,
	}, nil
}

func (a *AnthropicAdapter) Name() string { return a.name }

func (a *AnthropicAdapter) Type() string { return "anthropic" }

func (a *AnthropicAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *AnthropicAdapter) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *AnthropicAdapter) Health(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (a *AnthropicAdapter) EstimateCost(req *ChatRequest, resp *ChatResponse) float64 {
	return 0
}
