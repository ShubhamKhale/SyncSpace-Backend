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

// StatusError is returned when Groq responds with a non-200, non-429 status.
// Callers can check StatusCode (e.g. errors.As) to react to specific cases,
// such as 413 request_too_large from token-per-minute limits.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("groq: unexpected status %d: %s", e.StatusCode, e.Body)
}

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

// Tool is a Groq built-in (server-side) tool, e.g. {Type: "browser_search"}
// on the openai/gpt-oss models. Groq runs the tool itself — no client loop.
type Tool struct {
	Type string `json:"type"`
}

// ToolBrowserSearch enables Groq's built-in web search on supported models.
var ToolBrowserSearch = Tool{Type: "browser_search"}

// Request is a full chat-completion request for Complete.
type Request struct {
	Model    string
	Messages []aiclient.Message
	Tools    []Tool
	// NoRetry fails immediately on 429 instead of backing off — for callers
	// that have their own fallback and shouldn't make the user wait.
	NoRetry bool
}

type chatRequest struct {
	Model    string             `json:"model"`
	Messages []aiclient.Message `json:"messages"`
	Tools    []Tool             `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message aiclient.Message `json:"message"`
	} `json:"choices"`
}

// Model returns the client's default model.
func (c *Client) Model() string { return c.model }

// Chat sends a full message history to the client's configured model and
// returns its reply. Implements aiclient.ChatClient.
func (c *Client) Chat(ctx context.Context, messages []aiclient.Message) (string, error) {
	return c.ChatWithModel(ctx, c.model, messages)
}

// ListModels returns the IDs of the models this API key can use.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("groq: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return nil, &StatusError{StatusCode: resp.StatusCode, Body: string(payload)}
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("groq: decode models: %w", err)
	}
	ids := make([]string, len(out.Data))
	for i, m := range out.Data {
		ids[i] = m.ID
	}
	return ids, nil
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
	return c.Complete(ctx, Request{Model: model, Messages: messages})
}

// Complete sends a chat-completion request with optional built-in tools.
// Retries 429s with backoff unless req.NoRetry is set.
func (c *Client) Complete(ctx context.Context, r Request) (string, error) {
	body, err := json.Marshal(chatRequest{Model: r.Model, Messages: r.Messages, Tools: r.Tools})
	if err != nil {
		return "", fmt.Errorf("groq: marshal request: %w", err)
	}

	retries := maxRetries
	if r.NoRetry {
		retries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
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

		if resp.StatusCode == http.StatusTooManyRequests && attempt < retries {
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
			return "", &StatusError{StatusCode: resp.StatusCode, Body: string(payload)}
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
