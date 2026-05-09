package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mithunb9/arbiter/internal/adapter"
	"github.com/mithunb9/arbiter/internal/router"
)

type Handler struct {
	router *router.Router
	logger *zap.Logger
}

func New(r *router.Router, logger *zap.Logger) *Handler {
	return &Handler{router: r, logger: logger}
}

func RegisterRoutes(r *gin.Engine, h *Handler) {
	r.POST("/v1/chat/completions", h.chatCompletions)
}

type chatRequest struct {
	Model    string            `json:"model" binding:"required"`
	Messages []adapter.Message `json:"messages" binding:"required,min=1"`
	Stream   bool              `json:"stream"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
}

type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func (h *Handler) chatCompletions(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Stream {
		h.handleStream(c, &req)
	} else {
		h.handleChat(c, &req)
	}
}

func (h *Handler) handleChat(c *gin.Context, req *chatRequest) {
	adapterReq := &adapter.ChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
	}

	result, err := h.router.Route(c.Request.Context(), req.Model, adapterReq)
	if err != nil {
		h.routeError(c, req.Model, err)
		return
	}

	setArbiterHeaders(c, result.AdapterType, result.AdapterName, result.TierName, result.FallbackUsed)

	c.JSON(http.StatusOK, chatResponse{
		ID:      newID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []chatChoice{{
			Index:        0,
			Message:      chatMessage{Role: "assistant", Content: result.Response.Content},
			FinishReason: "stop",
		}},
		Usage: chatUsage{
			PromptTokens:     result.Response.InputTokens,
			CompletionTokens: result.Response.OutputTokens,
			TotalTokens:      result.Response.InputTokens + result.Response.OutputTokens,
		},
	})
}

func (h *Handler) handleStream(c *gin.Context, req *chatRequest) {
	adapterReq := &adapter.ChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   true,
	}

	result, err := h.router.RouteStream(c.Request.Context(), req.Model, adapterReq)
	if err != nil {
		h.routeError(c, req.Model, err)
		return
	}

	setArbiterHeaders(c, result.AdapterType, result.AdapterName, result.TierName, result.FallbackUsed)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, canFlush := c.Writer.(http.Flusher)

	id := newID()
	created := time.Now().Unix()

	for chunk := range result.Channel {
		if chunk.Err != nil {
			h.logger.Error("stream error", zap.String("tier", req.Model), zap.Error(chunk.Err))
			break
		}

		var finishReason *string
		delta := streamDelta{}
		if chunk.Done {
			stop := "stop"
			finishReason = &stop
		} else {
			delta.Content = chunk.Delta
		}

		sc := streamChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []streamChoice{{Index: 0, Delta: delta, FinishReason: finishReason}},
		}
		data, _ := json.Marshal(sc)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if canFlush {
			flusher.Flush()
		}
	}

	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}

func (h *Handler) routeError(c *gin.Context, tierName string, err error) {
	if errors.Is(err, router.ErrTierNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("unknown tier %q", tierName)})
		return
	}
	h.logger.Error("routing failed", zap.String("tier", tierName), zap.Error(err))
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

func setArbiterHeaders(c *gin.Context, adapterType, adapterName, tierName string, fallbackUsed bool) {
	c.Header("X-Arbiter-Adapter", adapterType)
	c.Header("X-Arbiter-Model", adapterName)
	c.Header("X-Arbiter-Tier", tierName)
	if fallbackUsed {
		c.Header("X-Arbiter-Fallback", "true")
	}
}

func newID() string {
	return fmt.Sprintf("chatcmpl-%x", time.Now().UnixNano())
}
