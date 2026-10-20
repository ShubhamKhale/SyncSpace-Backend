package groq

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name, header, body string
		want               time.Duration
	}{
		{"header seconds", "7", "", 7 * time.Second},
		{"header fractional", "2.5", "", 2500 * time.Millisecond},
		{"body seconds", "", `Rate limit reached ... Please try again in 4.2s. Need more tokens?`, 4200 * time.Millisecond},
		{"body millis", "", `Please try again in 820ms.`, 820 * time.Millisecond},
		{"body minutes", "", `Please try again in 1m2.5s.`, 62500 * time.Millisecond},
		{"header wins over body", "3", `try again in 9s`, 3 * time.Second},
		{"nothing", "", "rate limited", 0},
	}
	for _, c := range cases {
		if got := parseRetryAfter(c.header, c.body); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompleteResult_RateLimitCarriesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("retry-after", "6")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limit reached. Please try again in 5.5s."}}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURL("k", "m", srv.URL)
	_, err := c.CompleteResult(context.Background(), Request{Model: "m", NoRetry: true})
	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want 429 StatusError, got %v", err)
	}
	if se.RetryAfter != 6*time.Second {
		t.Errorf("RetryAfter = %v, want 6s (header)", se.RetryAfter)
	}
}

func TestCompleteResult_ReturnsFinishReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"a\":"},"finish_reason":"length"}],"usage":{"completion_tokens":8192}}`))
	}))
	defer srv.Close()

	res, err := NewClientWithBaseURL("k", "m", srv.URL).CompleteResult(context.Background(), Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinishReason != "length" || res.CompletionTokens != 8192 {
		t.Errorf("got finish=%q tokens=%d", res.FinishReason, res.CompletionTokens)
	}
}
