package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mithunb9/arbiter/internal/adapter"
	"github.com/mithunb9/arbiter/internal/config"
	"github.com/mithunb9/arbiter/internal/router"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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
func (m *mockAdapter) Health(_ context.Context) error                                       { return nil }
func (m *mockAdapter) EstimateCost(_ *adapter.ChatRequest, _ *adapter.ChatResponse) float64 { return 0 }

func makeChunkCh(chunks ...adapter.ChatChunk) <-chan adapter.ChatChunk {
	ch := make(chan adapter.ChatChunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch
}

func newTestEngine(adapters map[string]adapter.Adapter, tiers []config.TierConfig) *gin.Engine {
	r := router.New(tiers, adapters, zap.NewNop())
	h := New(r, zap.NewNop())
	eng := gin.New()
	RegisterRoutes(eng, h)
	return eng
}

func post(eng *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	eng.ServeHTTP(w, req)
	return w
}

func TestChat_Success(t *testing.T) {
	eng := newTestEngine(
		map[string]adapter.Adapter{
			"a1": &mockAdapter{name: "a1", typ: "anthropic", chatResp: &adapter.ChatResponse{
				Content: "Hello!", InputTokens: 10, OutputTokens: 5,
			}},
		},
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
	)

	w := post(eng, `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp chatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content: got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("usage: got %+v", resp.Usage)
	}
	if w.Header().Get("X-Arbiter-Adapter") != "anthropic" {
		t.Errorf("X-Arbiter-Adapter: got %q", w.Header().Get("X-Arbiter-Adapter"))
	}
	if w.Header().Get("X-Arbiter-Tier") != "fast" {
		t.Errorf("X-Arbiter-Tier: got %q", w.Header().Get("X-Arbiter-Tier"))
	}
}

func TestChat_FallbackHeader(t *testing.T) {
	eng := newTestEngine(
		map[string]adapter.Adapter{
			"a1": &mockAdapter{chatErr: errors.New("down")},
			"a2": &mockAdapter{name: "a2", typ: "ollama", chatResp: &adapter.ChatResponse{Content: "ok"}},
		},
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1", "a2"}, Fallback: true}},
	)

	w := post(eng, `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if w.Header().Get("X-Arbiter-Fallback") != "true" {
		t.Errorf("expected X-Arbiter-Fallback: true, got %q", w.Header().Get("X-Arbiter-Fallback"))
	}
}

func TestChat_BadRequest(t *testing.T) {
	eng := newTestEngine(nil, nil)
	w := post(eng, `{"messages":[{"role":"user","content":"hi"}]}`) // missing model
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestChat_UnknownTier(t *testing.T) {
	eng := newTestEngine(nil, nil)
	w := post(eng, `{"model":"nope","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestChat_AllAdaptersDown(t *testing.T) {
	eng := newTestEngine(
		map[string]adapter.Adapter{
			"a1": &mockAdapter{chatErr: errors.New("down")},
		},
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
	)
	w := post(eng, `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", w.Code)
	}
}

func TestStream_Success(t *testing.T) {
	ch := makeChunkCh(
		adapter.ChatChunk{Delta: "Hello"},
		adapter.ChatChunk{Delta: " world"},
		adapter.ChatChunk{Done: true},
	)
	eng := newTestEngine(
		map[string]adapter.Adapter{
			"a1": &mockAdapter{name: "a1", typ: "anthropic", streamCh: ch},
		},
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
	)

	w := post(eng, `{"model":"fast","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Errorf("missing 'Hello' in stream body: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("missing [DONE] in stream body: %s", body)
	}
	if !strings.Contains(body, `"stop"`) {
		t.Errorf("missing finish_reason stop in stream body: %s", body)
	}
	if w.Header().Get("X-Arbiter-Adapter") != "anthropic" {
		t.Errorf("X-Arbiter-Adapter: got %q", w.Header().Get("X-Arbiter-Adapter"))
	}
}

func TestStream_UnknownTier(t *testing.T) {
	eng := newTestEngine(nil, nil)
	w := post(eng, `{"model":"nope","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestStream_AdapterError(t *testing.T) {
	eng := newTestEngine(
		map[string]adapter.Adapter{
			"a1": &mockAdapter{streamErr: errors.New("down")},
		},
		[]config.TierConfig{{Name: "fast", Adapters: []string{"a1"}}},
	)
	w := post(eng, `{"model":"fast","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", w.Code)
	}
}
