package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OllamaAdapter struct {
	name    string
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaAdapter(name, baseURL, model string) (*OllamaAdapter, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("adapter %q: base_url is required", name)
	}
	return &OllamaAdapter{
		name:    name,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}, nil
}

func (a *OllamaAdapter) Name() string { return a.name }
func (a *OllamaAdapter) Type() string { return "ollama" }

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

func (a *OllamaAdapter) post(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("adapter %q: %w", a.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adapter %q: %w", a.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("adapter %q: unexpected status %d", a.name, resp.StatusCode)
	}
	return resp, nil
}

func (a *OllamaAdapter) buildMessages(req *ChatRequest) []ollamaMessage {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *OllamaAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(ollamaChatRequest{
		Model:    a.model,
		Messages: a.buildMessages(req),
		Stream:   false,
	})
	if err != nil {
		return nil, fmt.Errorf("adapter %q: marshal request: %w", a.name, err)
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("adapter %q: decode response: %w", a.name, err)
	}

	return &ChatResponse{
		Content:      ollamaResp.Message.Content,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
	}, nil
}

func (a *OllamaAdapter) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error) {
	body, err := json.Marshal(ollamaChatRequest{
		Model:    a.model,
		Messages: a.buildMessages(req),
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("adapter %q: marshal request: %w", a.name, err)
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan ChatChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var chunk ollamaChatResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				ch <- ChatChunk{Err: fmt.Errorf("adapter %q: decode chunk: %w", a.name, err)}
				return
			}
			if chunk.Done {
				ch <- ChatChunk{Done: true}
				return
			}
			ch <- ChatChunk{Delta: chunk.Message.Content}
		}
		if err := scanner.Err(); err != nil {
			ch <- ChatChunk{Err: fmt.Errorf("adapter %q: read stream: %w", a.name, err)}
		}
	}()

	return ch, nil
}

func (a *OllamaAdapter) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("adapter %q: %w", a.name, err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("adapter %q: %w", a.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("adapter %q: unexpected status %d", a.name, resp.StatusCode)
	}
	return nil
}

func (a *OllamaAdapter) EstimateCost(_ *ChatRequest, _ *ChatResponse) float64 {
	return 0
}
