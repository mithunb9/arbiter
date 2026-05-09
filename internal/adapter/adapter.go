package adapter

import "context"

type Adapter interface {
	Name() string
	Type() string
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error)
	Health(ctx context.Context) error
	EstimateCost(req *ChatRequest, resp *ChatResponse) float64
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

type ChatChunk struct {
	Delta string
	Done  bool
	Err   error
}
