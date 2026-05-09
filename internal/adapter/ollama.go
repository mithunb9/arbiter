package adapter

import (
	"context"
	"fmt"
)

type OllamaAdapter struct {
	name    string
	baseURL string
	model   string
}

func NewOllamaAdapter(name, baseURL, model string) (*OllamaAdapter, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("adapter %q: base_url is required", name)
	}
	return &OllamaAdapter{
		name:    name,
		baseURL: baseURL,
		model:   model,
	}, nil
}

func (a *OllamaAdapter) Name() string { return a.name }

func (a *OllamaAdapter) Type() string { return "ollama" }

func (a *OllamaAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *OllamaAdapter) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *OllamaAdapter) Health(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (a *OllamaAdapter) EstimateCost(_ *ChatRequest, _ *ChatResponse) float64 {
	return 0
}
