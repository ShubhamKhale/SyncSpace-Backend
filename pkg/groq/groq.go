// Package groq is a thin client for the Groq cloud API
// (https://console.groq.com), used in production where the deployed backend
// has no local machine to run Ollama against. OpenAI-compatible chat shape.
package groq

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

const defaultBaseURL = "https://api.groq.com/openai/v1"

// Client talks to the Groq chat-completions API for a fixed model.
// Implements aiclient.ChatClient.
type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client authenticated with the given Groq API key,
// targeting the given model (e.g. "llama-3.1-8b-instant").
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey:     apiKey,
		model:      model,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model    string             `json:"model"`
	Messages []aiclient.Message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message aiclient.Message `json:"message"`
	} `json:"choices"`
}

// Chat sends a full message history to the client's configured model and
// returns its reply. Implements aiclient.ChatClient.
func (c *Client) Chat(ctx context.Context, messages []aiclient.Message) (string, error) {
	return c.ChatWithModel(ctx, c.model, messages)
}

// maxRetries and retryBackoff bound retries against transient 429s — models
// like "groq/compound" fan out to multiple sub-models internally, so their
// shared per-minute quota can be exhausted by unrelated traffic even when
// the caller's own request is small.
const maxRetries = 3

var retryBackoff = [maxRetries]time.Duration{3 * time.Second, 6 * time.Second, 10 * time.Second}

// ChatWithModel sends a full message history to the given model — used when a
// feature needs a specific model (e.g. "groq/compound" for web-search-backed
// generation) regardless of the client's configured default model. Retries a
// bounded number of times on HTTP 429 before giving up.
func (c *Client) ChatWithModel(ctx context.Context, model string, messages []aiclient.Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages})
	if err != nil {
		return "", fmt.Errorf("groq: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("groq: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("groq: request failed: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("groq: rate limited: %s", string(payload))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(retryBackoff[attempt]):
			}
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			payload, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("groq: unexpected status %d: %s", resp.StatusCode, string(payload))
		}

		var out chatResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", fmt.Errorf("groq: decode response: %w", err)
		}
		if len(out.Choices) == 0 {
			return "", fmt.Errorf("groq: empty choices in response")
		}
		return out.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("groq: exhausted retries: %w", lastErr)
}
