// Package ollama is a thin client for a locally running Ollama server
// (https://ollama.com), used for on-device chat and summarization in local
// development so no external AI API key or network call is required.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"syncspace-backend/pkg/aiclient"
)

// Client talks to a local Ollama server for a fixed model.
// Implements aiclient.ChatClient.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient creates a Client targeting the given Ollama server and model.
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type chatRequest struct {
	Model    string             `json:"model"`
	Messages []aiclient.Message `json:"messages"`
	Stream   bool               `json:"stream"`
}

type chatResponse struct {
	Message aiclient.Message `json:"message"`
}

// Chat sends a full message history to the model and returns its reply.
// Uses non-streaming mode — acceptable for short chat/summarization replies
// on a local CPU model where the whole response is needed at once anyway.
func (c *Client) Chat(ctx context.Context, messages []aiclient.Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Stream: false})
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed (is `ollama serve` running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: unexpected status %d: %s", resp.StatusCode, string(payload))
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ollama: decode response: %w", err)
	}
	return out.Message.Content, nil
}
