package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/pkg/groq"
)

const validGraph = `{"title":"T","nodes":[{"id":"a","label":"A","shape":"rectangle"}],"edges":[]}`

// fakeGroq serves chat completions; respond decides each reply from the
// requested model and that model's call count (1-based).
type fakeGroq struct {
	mu      sync.Mutex
	calls   map[string]int
	bodies  []map[string]any
	respond func(model string, n int, w http.ResponseWriter)
}

func newFake(respond func(model string, n int, w http.ResponseWriter)) (*fakeGroq, *httptest.Server) {
	f := &fakeGroq{calls: map[string]int{}, respond: respond}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		f.mu.Lock()
		f.calls[model]++
		n := f.calls[model]
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		f.respond(model, n, w)
	}))
	return f, srv
}

func reply(w http.ResponseWriter, content, finish string) {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}, "finish_reason": finish}},
	})
	_, _ = w.Write(b)
}

func rateLimit(w http.ResponseWriter, retryAfter string) {
	w.Header().Set("retry-after", retryAfter)
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
}

func newTestService(url string) *DiagramService {
	c := groq.NewClientWithBaseURL("k", "fallback-a", url)
	return NewDiagramService(c, "openai/gpt-oss-primary", []string{"fallback-a", "qwen/fallback-b"}, true)
}

func TestGenerateDiagram_AllRateLimitedThenRecovers(t *testing.T) {
	// Every model is rate-limited on its first call (retry in 1s), then fine.
	f, srv := newFake(func(model string, n int, w http.ResponseWriter) {
		if n == 1 {
			rateLimit(w, "1")
			return
		}
		reply(w, validGraph, "stop")
	})
	defer srv.Close()

	start := time.Now()
	graph, err := newTestService(srv.URL).GenerateDiagram(context.Background(), "x", DiagramOptions{})
	if err != nil {
		t.Fatalf("want success after server-side wait, got %v", err)
	}
	if graph["title"] != "T" {
		t.Errorf("unexpected graph %v", graph)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("expected a ~1s server-side wait, took %v", waited)
	}
	if f.calls["openai/gpt-oss-primary"] != 2 {
		t.Errorf("primary should be tried twice (chain + retry), got %d", f.calls["openai/gpt-oss-primary"])
	}
}

func TestGenerateDiagram_LongRateLimitReturnsRetryAfter(t *testing.T) {
	// Hint above maxServerWait: no server-side wait, client gets retry_after.
	_, srv := newFake(func(model string, n int, w http.ResponseWriter) { rateLimit(w, "25") })
	defer srv.Close()

	start := time.Now()
	_, err := newTestService(srv.URL).GenerateDiagram(context.Background(), "x", DiagramOptions{})
	var appErr *errs.AppError
	if !errors.As(err, &appErr) || appErr.Code != "RATE_LIMITED" {
		t.Fatalf("want RATE_LIMITED, got %v", err)
	}
	if appErr.RetryAfter != 25 {
		t.Errorf("RetryAfter = %d, want 25", appErr.RetryAfter)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("should not wait server-side for a 25s hint")
	}
}

func TestGenerateDiagram_TruncatedReplyFallsThrough(t *testing.T) {
	// Primary returns JSON cut off at the token limit; next model succeeds.
	f, srv := newFake(func(model string, n int, w http.ResponseWriter) {
		if model == "openai/gpt-oss-primary" {
			reply(w, `{"title":"T","nodes":[{"id":"a"`, "length")
			return
		}
		reply(w, validGraph, "stop")
	})
	defer srv.Close()

	if _, err := newTestService(srv.URL).GenerateDiagram(context.Background(), "x", DiagramOptions{}); err != nil {
		t.Fatalf("want fallback success, got %v", err)
	}
	if f.calls["fallback-a"] != 1 {
		t.Errorf("fallback-a calls = %d, want 1", f.calls["fallback-a"])
	}
}

func TestGenerateDiagram_RequestShapeAndCache(t *testing.T) {
	f, srv := newFake(func(model string, n int, w http.ResponseWriter) { reply(w, validGraph, "stop") })
	defer srv.Close()
	svc := newTestService(srv.URL)

	// Search on: primary gets browser_search, no JSON mode, low reasoning, 8192 cap.
	if _, err := svc.GenerateDiagram(context.Background(), "Kafka  Pipeline", DiagramOptions{WebSearch: true}); err != nil {
		t.Fatal(err)
	}
	b := f.bodies[0]
	if b["tools"] == nil || b["response_format"] != nil {
		t.Errorf("search attempt: tools=%v response_format=%v", b["tools"], b["response_format"])
	}
	if b["reasoning_effort"] != "low" || b["max_completion_tokens"] != float64(diagramMaxTokens) {
		t.Errorf("reasoning_effort=%v max_completion_tokens=%v", b["reasoning_effort"], b["max_completion_tokens"])
	}

	// Same prompt modulo case/whitespace → cache hit, no new upstream call.
	if _, err := svc.GenerateDiagram(context.Background(), "kafka pipeline", DiagramOptions{WebSearch: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 1 {
		t.Errorf("expected cache hit, got %d upstream calls", len(f.bodies))
	}

	// Search off is a different cache key and uses JSON mode.
	if _, err := svc.GenerateDiagram(context.Background(), "kafka pipeline", DiagramOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 || f.bodies[1]["tools"] != nil || f.bodies[1]["response_format"] == nil {
		t.Errorf("non-search attempt should use JSON mode without tools: %v", f.bodies[len(f.bodies)-1])
	}
}

func TestReasoningEffortFor(t *testing.T) {
	for model, want := range map[string]string{
		"openai/gpt-oss-120b": "low", "openai/gpt-oss-20b": "low",
		"qwen/qwen3.8-27b": "none", "llama-3.3-70b": "",
	} {
		if got := reasoningEffortFor(model); got != want {
			t.Errorf("%s: got %q want %q", model, got, want)
		}
	}
}
