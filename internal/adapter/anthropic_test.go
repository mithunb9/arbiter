package adapter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func newTestAnthropic(t *testing.T, handler http.HandlerFunc) *AnthropicAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	a, err := NewAnthropicAdapter("test", "test-key", "claude-haiku-4-5", 0, 0)
	if err != nil {
		t.Fatalf("NewAnthropicAdapter: %v", err)
	}
	a.client = anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(srv.URL),
		option.WithHTTPClient(srv.Client()),
	)
	return a
}

const validChatResponse = `{
	"id": "msg_test",
	"type": "message",
	"role": "assistant",
	"content": [{"type": "text", "text": "Hello!"}],
	"model": "claude-haiku-4-5",
	"stop_reason": "end_turn",
	"usage": {"input_tokens": 10, "output_tokens": 5}
}`

func TestChat_Success(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, validChatResponse)
	})

	resp, err := a.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("content: got %q, want %q", resp.Content, "Hello!")
	}
	if resp.InputTokens != 10 || resp.OutputTokens != 5 {
		t.Errorf("tokens: got %d/%d, want 10/5", resp.InputTokens, resp.OutputTokens)
	}
}

func TestChat_AuthError(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})

	_, err := a.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("expected 'invalid API key' in error, got: %v", err)
	}
}

// streamResponse returns a complete Anthropic SSE stream for the given text chunks.
func streamResponse(chunks ...string) string {
	var sb strings.Builder
	sb.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-haiku-4-5\",\"stop_reason\":null,\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n")
	sb.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
	for _, chunk := range chunks {
		escaped := strings.ReplaceAll(chunk, `"`, `\"`)
		fmt.Fprintf(&sb, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"%s\"}}\n\n", escaped)
	}
	sb.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
	sb.WriteString("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n")
	sb.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	return sb.String()
}

func TestChatStream_Success(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, streamResponse("Hello", " world"))
	})

	ch, err := a.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parts []string
	var done bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("stream error: %v", chunk.Err)
		}
		if chunk.Done {
			done = true
		} else {
			parts = append(parts, chunk.Delta)
		}
	}
	if !done {
		t.Error("expected Done=true chunk")
	}
	if got := strings.Join(parts, ""); got != "Hello world" {
		t.Errorf("content: got %q, want %q", got, "Hello world")
	}
}

func TestChatStream_AuthError(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})

	// With the SDK, auth errors surface via the channel (streaming is lazy).
	ch, err := a.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected immediate error: %v", err)
	}

	var streamErr error
	for chunk := range ch {
		if chunk.Err != nil {
			streamErr = chunk.Err
		}
	}
	if streamErr == nil {
		t.Fatal("expected error from stream, got nil")
	}
	if !strings.Contains(streamErr.Error(), "invalid API key") {
		t.Errorf("expected 'invalid API key' in error, got: %v", streamErr)
	}
}

func TestHealth_Success(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, validChatResponse)
	})
	if err := a.Health(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealth_AuthError(t *testing.T) {
	a := newTestAnthropic(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})
	err := a.Health(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("expected 'invalid API key' in error, got: %v", err)
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		model      string
		in, out    int
		inputCPM   float64 // cost per million input tokens (0 = use built-in)
		outputCPM  float64
		wantCost   float64
	}{
		// built-in defaults
		{"claude-haiku-4-5", 1000, 500, 0, 0, 1000*0.80/1e6 + 500*4.00/1e6},
		{"claude-sonnet-4-6", 1000, 500, 0, 0, 1000*3.00/1e6 + 500*15.00/1e6},
		{"claude-opus-4-7", 1000, 500, 0, 0, 1000*15.00/1e6 + 500*75.00/1e6},
		{"unknown-model", 1000, 500, 0, 0, 0},
		// config override
		{"claude-haiku-4-5", 1000, 500, 1.00, 5.00, 1000*1.00/1e6 + 500*5.00/1e6},
		{"unknown-model", 1000, 500, 2.00, 8.00, 1000*2.00/1e6 + 500*8.00/1e6},
	}
	for _, tc := range tests {
		a := &AnthropicAdapter{
			name:               "test",
			model:              tc.model,
			inputCostPerToken:  tc.inputCPM / 1e6,
			outputCostPerToken: tc.outputCPM / 1e6,
		}
		got := a.EstimateCost(nil, &ChatResponse{InputTokens: tc.in, OutputTokens: tc.out})
		if got != tc.wantCost {
			t.Errorf("model %q (cpm %.2f/%.2f): got %v, want %v", tc.model, tc.inputCPM, tc.outputCPM, got, tc.wantCost)
		}
	}
}
