package adapter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestOllama(t *testing.T, handler http.HandlerFunc) *OllamaAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	a, err := NewOllamaAdapter("test", srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaAdapter: %v", err)
	}
	a.client = srv.Client()
	return a
}

func TestOllamaChat_Success(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"Hello!"},"done":true,"prompt_eval_count":10,"eval_count":5}`)
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

func TestOllamaChat_ErrorStatus(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := a.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestOllamaChat_Unreachable(t *testing.T) {
	a, err := NewOllamaAdapter("test", "http://127.0.0.1:19999", "llama3")
	if err != nil {
		t.Fatalf("NewOllamaAdapter: %v", err)
	}
	_, err = a.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestOllamaChatStream_Success(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"Hello"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":" world"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":5}`+"\n")
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

func TestOllamaChatStream_ErrorStatus(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := a.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestOllamaChatStream_Unreachable(t *testing.T) {
	a, err := NewOllamaAdapter("test", "http://127.0.0.1:19999", "llama3")
	if err != nil {
		t.Fatalf("NewOllamaAdapter: %v", err)
	}
	_, err = a.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestOllamaHealth_Success(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"models":[]}`)
	})
	if err := a.Health(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOllamaHealth_ErrorStatus(t *testing.T) {
	a := newTestOllama(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if err := a.Health(context.Background()); err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
}

func TestOllamaHealth_Unreachable(t *testing.T) {
	a, err := NewOllamaAdapter("test", "http://127.0.0.1:19999", "llama3")
	if err != nil {
		t.Fatalf("NewOllamaAdapter: %v", err)
	}
	if err := a.Health(context.Background()); err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}
