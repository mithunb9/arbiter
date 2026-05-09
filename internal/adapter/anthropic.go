package adapter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicAdapter struct {
	name              string
	model             string
	client            anthropic.Client
	inputCostPerToken float64 // USD per token; 0 means use built-in default
	outputCostPerToken float64
}

func NewAnthropicAdapter(name, apiKey, model string, inputCostPerMillionTokens, outputCostPerMillionTokens float64) (*AnthropicAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("adapter %q: api_key is required", name)
	}
	return &AnthropicAdapter{
		name:               name,
		model:              model,
		client:             anthropic.NewClient(option.WithAPIKey(apiKey)),
		inputCostPerToken:  inputCostPerMillionTokens / 1e6,
		outputCostPerToken: outputCostPerMillionTokens / 1e6,
	}, nil
}

func (a *AnthropicAdapter) Name() string { return a.name }
func (a *AnthropicAdapter) Type() string { return "anthropic" }

func (a *AnthropicAdapter) toParams(req *ChatRequest, maxTokens int64) anthropic.MessageNewParams {
	msgs := make([]anthropic.MessageParam, len(req.Messages))
	for i, m := range req.Messages {
		if m.Role == "user" {
			msgs[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		} else {
			msgs[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}
	return anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
}

func (a *AnthropicAdapter) wrapErr(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == 401 {
		return fmt.Errorf("adapter %q: invalid API key", a.name)
	}
	return fmt.Errorf("adapter %q: %w", a.name, err)
}

func (a *AnthropicAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	msg, err := a.client.Messages.New(ctx, a.toParams(req, 4096))
	if err != nil {
		return nil, a.wrapErr(err)
	}

	var sb strings.Builder
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return &ChatResponse{
		Content:      sb.String(),
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}, nil
}

func (a *AnthropicAdapter) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	stream := a.client.Messages.NewStreaming(ctx, a.toParams(req, 4096))

	ch := make(chan ChatChunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		for stream.Next() {
			event := stream.Current()
			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if ev.Delta.Type == "text_delta" {
					ch <- ChatChunk{Delta: ev.Delta.Text}
				}
			case anthropic.MessageStopEvent:
				ch <- ChatChunk{Done: true}
				return
			}
		}
		if err := stream.Err(); err != nil {
			ch <- ChatChunk{Err: a.wrapErr(err)}
		}
	}()

	return ch, nil
}

func (a *AnthropicAdapter) Health(ctx context.Context) error {
	_, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 1,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("ping"))},
	})
	if err != nil {
		return a.wrapErr(err)
	}
	return nil
}

var modelCosts = []struct {
	prefix string
	input  float64
	output float64
}{
	{"claude-haiku", 0.80 / 1e6, 4.00 / 1e6},
	{"claude-sonnet", 3.00 / 1e6, 15.00 / 1e6},
	{"claude-opus", 15.00 / 1e6, 75.00 / 1e6},
}

func (a *AnthropicAdapter) EstimateCost(_ *ChatRequest, resp *ChatResponse) float64 {
	in, out := a.inputCostPerToken, a.outputCostPerToken
	if in == 0 && out == 0 {
		for _, mc := range modelCosts {
			if strings.HasPrefix(a.model, mc.prefix) {
				in, out = mc.input, mc.output
				break
			}
		}
	}
	return float64(resp.InputTokens)*in + float64(resp.OutputTokens)*out
}
