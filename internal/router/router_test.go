package router

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/mithunb9/arbiter/internal/adapter"
	"github.com/mithunb9/arbiter/internal/config"
)

type mockAdapter struct {
	name      string
	typ       string
	chatResp  *adapter.ChatResponse
	chatErr   error
	streamCh  <-chan adapter.ChatChunk
	streamErr error
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Type() string { return m.typ }
func (m *mockAdapter) Chat(_ context.Context, _ *adapter.ChatRequest) (*adapter.ChatResponse, error) {
	return m.chatResp, m.chatErr
}
func (m *mockAdapter) ChatStream(_ context.Context, _ *adapter.ChatRequest) (<-chan adapter.ChatChunk, error) {
	return m.streamCh, m.streamErr
}
func (m *mockAdapter) Health(_ context.Context) error { return nil }
func (m *mockAdapter) EstimateCost(_ *adapter.ChatRequest, _ *adapter.ChatResponse) float64 {
	return 0
}

func newRouter(tiers []config.TierConfig, adapters map[string]adapter.Adapter) *Router {
	return New(tiers, adapters, zap.NewNop())
}

var req = &adapter.ChatRequest{Messages: []adapter.Message{{Role: "user", Content: "hi"}}}

func TestRoute_Success(t *testing.T) {
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{name: "a1", typ: "ollama", chatResp: &adapter.ChatResponse{Content: "hello"}},
		},
	)
	result, err := r.Route(context.Background(), "fast", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response.Content != "hello" {
		t.Errorf("content: got %q, want %q", result.Response.Content, "hello")
	}
	if result.AdapterName != "a1" || result.AdapterType != "ollama" || result.TierName != "fast" {
		t.Errorf("unexpected result metadata: %+v", result)
	}
	if result.FallbackUsed {
		t.Error("FallbackUsed should be false")
	}
}

func TestRoute_TierNotFound(t *testing.T) {
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
		map[string]adapter.Adapter{"a1": &mockAdapter{}},
	)
	_, err := r.Route(context.Background(), "unknown", req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTierNotFound) {
		t.Errorf("expected ErrTierNotFound, got: %v", err)
	}
}

func TestRoute_Fallback(t *testing.T) {
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1", "a2"}, Fallback: true}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{name: "a1", typ: "ollama", chatErr: errors.New("down")},
			"a2": &mockAdapter{name: "a2", typ: "anthropic", chatResp: &adapter.ChatResponse{Content: "fallback"}},
		},
	)
	result, err := r.Route(context.Background(), "fast", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdapterName != "a2" {
		t.Errorf("expected fallback to a2, got %q", result.AdapterName)
	}
	if !result.FallbackUsed {
		t.Error("FallbackUsed should be true")
	}
}

func TestRoute_AllAdaptersDown(t *testing.T) {
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1", "a2"}, Fallback: true}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{chatErr: errors.New("down")},
			"a2": &mockAdapter{chatErr: errors.New("also down")},
		},
	)
	_, err := r.Route(context.Background(), "fast", req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRoute_NoFallbackOnError(t *testing.T) {
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1", "a2"}, Fallback: false}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{chatErr: errors.New("down")},
			"a2": &mockAdapter{chatResp: &adapter.ChatResponse{Content: "should not reach"}},
		},
	)
	_, err := r.Route(context.Background(), "fast", req)
	if err == nil {
		t.Fatal("expected error when fallback disabled")
	}
}

func makeChunkCh(chunks ...adapter.ChatChunk) <-chan adapter.ChatChunk {
	ch := make(chan adapter.ChatChunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch
}

func TestRouteStream_Success(t *testing.T) {
	ch := makeChunkCh(
		adapter.ChatChunk{Delta: "hello"},
		adapter.ChatChunk{Done: true},
	)
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{name: "a1", typ: "ollama", streamCh: ch},
		},
	)
	result, err := r.RouteStream(context.Background(), "fast", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdapterName != "a1" || result.TierName != "fast" {
		t.Errorf("unexpected metadata: %+v", result)
	}
	var got []string
	for chunk := range result.Channel {
		if chunk.Delta != "" {
			got = append(got, chunk.Delta)
		}
	}
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("unexpected chunks: %v", got)
	}
}

func TestRouteStream_TierNotFound(t *testing.T) {
	r := newRouter(nil, nil)
	_, err := r.RouteStream(context.Background(), "nope", req)
	if !errors.Is(err, ErrTierNotFound) {
		t.Errorf("expected ErrTierNotFound, got: %v", err)
	}
}

func TestRouteStream_Fallback(t *testing.T) {
	ch := makeChunkCh(adapter.ChatChunk{Delta: "ok"}, adapter.ChatChunk{Done: true})
	r := newRouter(
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1", "a2"}, Fallback: true}},
		map[string]adapter.Adapter{
			"a1": &mockAdapter{streamErr: errors.New("down")},
			"a2": &mockAdapter{name: "a2", typ: "anthropic", streamCh: ch},
		},
	)
	result, err := r.RouteStream(context.Background(), "fast", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.FallbackUsed {
		t.Error("FallbackUsed should be true")
	}
}
