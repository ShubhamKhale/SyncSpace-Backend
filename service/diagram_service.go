package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/pkg/aiclient"
	"syncspace-backend/pkg/groq"
	"syncspace-backend/pkg/utils"
)

// diagramUnavailableMsg is what users see when every model fails for a
// non-rate-limit reason; the underlying Groq error is logged, not returned,
// since it exposes provider details and isn't actionable for the user.
const diagramUnavailableMsg = "AI diagram generation is temporarily unavailable. Please try again in a minute."

// diagramSearchPrompt is prepended when web search is enabled. Wording matters:
// with tools enabled, gpt-oss models told to "output ONLY JSON" try to call a
// nonexistent "JSON" tool (Groq 400 tool_use_failed), so the prompt names
// browser_search as the only tool and asks for the answer as plain text.
const diagramSearchPrompt = `You may use the browser_search tool to confirm accurate, current details (real service names, typical component relationships, standard patterns). browser_search is the ONLY tool available — never call any other tool. When done, write your final answer as plain message text (not a tool call).

`

const diagramSystemPrompt = `You are a diagram generation engine.

When the user describes an architecture, system, or process, output ONLY a valid JSON object. No markdown. No explanation. No text outside the JSON.

Output schema:
{
  "title": "string",
  "nodes": [{ "id": "string", "label": "string", "shape": "rectangle|circle|diamond|cylinder|hexagon|parallelogram|triangle|rounded-rectangle|star" }],
  "edges": [{ "source": "string", "target": "string", "label": "string" }]
}

Shape guide — follow strictly:
- rectangle: services, APIs, applications, users, clients, servers, components
- cylinder: databases, caches (Redis, Memcached), message queues (Kafka, RabbitMQ), storage
- diamond: decisions, conditions, if/else, gateways
- circle: start points, end points, events
- hexagon: external systems, third-party services
- rounded-rectangle: processes, jobs, workers
- parallelogram: data inputs/outputs only
- triangle: alerts, warnings only
- star: highlights only

Rules:
- Node IDs: lowercase, no spaces, use underscores (e.g. "api_gateway", "user_service")
- All edge source/target must exactly match a node ID
- Max 15 nodes
- Keep edge labels short (1–3 words)
- Use real, accurate component/service names when the user names a specific technology or cloud provider
- Make diagrams meaningful: include realistic components for the architecture described

Example output for "Redis pub/sub":
{"title":"Redis Pub/Sub","nodes":[{"id":"publisher","label":"Publisher","shape":"rectangle"},{"id":"redis","label":"Redis","shape":"cylinder"},{"id":"sub1","label":"Subscriber 1","shape":"rectangle"},{"id":"sub2","label":"Subscriber 2","shape":"rectangle"}],"edges":[{"source":"publisher","target":"redis","label":"PUBLISH"},{"source":"redis","target":"sub1","label":"MESSAGE"},{"source":"redis","target":"sub2","label":"MESSAGE"}]}`

// Generation limits. MaxTokens must cover the gpt-oss models' hidden
// reasoning *and* the JSON answer: at Groq's default (2048) a 12-service
// diagram's reasoning alone used ~1800 tokens and the JSON was cut off
// mid-edge (finish_reason=length). Low reasoning effort roughly halves cost.
const (
	diagramMaxTokens = 8192
	// Groq 429s carry a retry hint; when every model is rate-limited and the
	// shortest wait is at most this long, wait server-side and retry once
	// rather than failing the request.
	maxServerWait = 10 * time.Second
	cacheTTL      = 10 * time.Minute
	cacheMax      = 100
)

// reasoningEffortFor picks the cheapest reasoning setting each model family
// accepts (verified against Groq): gpt-oss takes low|medium|high, qwen takes none.
func reasoningEffortFor(model string) string {
	switch {
	case strings.HasPrefix(model, "openai/gpt-oss"):
		return "low"
	case strings.HasPrefix(model, "qwen/"):
		return "none"
	}
	return ""
}

// DiagramService generates architecture-diagram JSON (nodes/edges) from a
// natural-language prompt by walking a chain of Groq models. Each model has
// its own tokens-per-minute budget (~8k on the free tier), so a rate-limited
// model is skipped and the next one tried:
//
//	primary (GROQ_DIAGRAM_MODEL, + browser_search when the user opts in)
//	→ fallbacks (GROQ_DIAGRAM_FALLBACKS), JSON mode, no tools
//
// If every model is rate-limited it waits for the shortest Groq retry hint
// (≤ maxServerWait) and retries once, else returns errs.RateLimited with a
// retry_after the client can count down. Successful results are cached
// briefly so retrying the same prompt costs no tokens.
//
// groqClient is nil when GROQ_API_KEY isn't configured — this feature has no
// offline fallback.
type DiagramService struct {
	groqClient    *groq.Client
	primary       string
	fallbacks     []string
	searchAllowed bool
	cache         *diagramCache
}

// NewDiagramService creates a DiagramService. searchAllowed is a server-side
// kill switch for web search; users still opt in per request.
func NewDiagramService(groqClient *groq.Client, primary string, fallbacks []string, searchAllowed bool) *DiagramService {
	return &DiagramService{
		groqClient:    groqClient,
		primary:       primary,
		fallbacks:     fallbacks,
		searchAllowed: searchAllowed,
		cache:         newDiagramCache(),
	}
}

// DiagramOptions are per-request generation options.
type DiagramOptions struct {
	// WebSearch grounds the diagram in live search results (real service
	// names). Costs ~3x the tokens, so it is opt-in.
	WebSearch bool
}

type diagramAttempt struct {
	model  string
	search bool
}

// GenerateDiagram returns the parsed {title, nodes, edges} graph for the
// prompt. Errors: BadRequest (empty prompt), RateLimited (every model busy —
// carries retry_after), Internal (not configured or all models failed).
func (s *DiagramService) GenerateDiagram(ctx context.Context, prompt string, opts DiagramOptions) (map[string]any, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errs.BadRequest("prompt is required")
	}
	if s.groqClient == nil {
		return nil, errs.Internal("diagram generation requires GROQ_API_KEY to be configured")
	}

	search := opts.WebSearch && s.searchAllowed
	key := cacheKey(prompt, search)
	if graph, ok := s.cache.get(key); ok {
		utils.Info("[AI]", "GenerateDiagram: cache hit")
		return graph, nil
	}

	attempts := []diagramAttempt{{model: s.primary, search: search}}
	for _, m := range s.fallbacks {
		if m != "" && m != s.primary {
			attempts = append(attempts, diagramAttempt{model: m})
		}
	}

	graph, minWait, allLimited, err := s.runChain(ctx, prompt, attempts)
	if err == nil {
		s.cache.put(key, graph)
		return graph, nil
	}

	// Every model was rate-limited. A short wait usually frees enough
	// tokens-per-minute budget, so retry once rather than failing — without
	// web search, which is the most token-hungry attempt.
	if allLimited && minWait > 0 && minWait <= maxServerWait {
		utils.Info("[AI]", fmt.Sprintf("GenerateDiagram: all models rate-limited, waiting %s and retrying", minWait.Round(100*time.Millisecond)))
		select {
		case <-ctx.Done():
			return nil, errs.Internal(diagramUnavailableMsg)
		case <-time.After(minWait + 300*time.Millisecond):
		}
		retry := make([]diagramAttempt, len(attempts))
		for i, a := range attempts {
			retry[i] = diagramAttempt{model: a.model}
		}
		graph, minWait, allLimited, err = s.runChain(ctx, prompt, retry)
		if err == nil {
			s.cache.put(key, graph)
			return graph, nil
		}
	}

	if allLimited {
		secs := int(math.Ceil(minWait.Seconds()))
		if secs < 3 {
			secs = 3 // unknown or very short hint — avoid a hot retry loop
		}
		return nil, errs.RateLimited("AI is busy right now — too many requests this minute.", secs)
	}
	return nil, errs.Internal(diagramUnavailableMsg)
}

// runChain tries each attempt in order until one yields a valid graph.
// allLimited reports whether every failure was a rate limit; minWait is the
// shortest retry hint among them (0 if none was given).
func (s *DiagramService) runChain(ctx context.Context, prompt string, attempts []diagramAttempt) (map[string]any, time.Duration, bool, error) {
	var minWait time.Duration
	allLimited := true
	var lastErr error

	for _, a := range attempts {
		graph, err := s.attempt(ctx, prompt, a)
		if err == nil {
			return graph, 0, false, nil
		}
		if ctx.Err() != nil {
			return nil, 0, false, ctx.Err()
		}
		lastErr = err
		utils.Error("[AI]", fmt.Sprintf("GenerateDiagram: %s (search=%t) failed: %s", a.model, a.search, failureKind(err)), err)

		var statusErr *groq.StatusError
		if errors.As(err, &statusErr) && (statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode == http.StatusRequestEntityTooLarge) {
			if statusErr.RetryAfter > 0 && (minWait == 0 || statusErr.RetryAfter < minWait) {
				minWait = statusErr.RetryAfter
			}
			continue
		}
		allLimited = false
	}
	return nil, minWait, allLimited, lastErr
}

// attempt runs one model and parses its reply into a graph.
func (s *DiagramService) attempt(ctx context.Context, prompt string, a diagramAttempt) (map[string]any, error) {
	req := groq.Request{
		Model: a.model,
		Messages: []aiclient.Message{
			{Role: "system", Content: diagramSystemPrompt},
			{Role: "user", Content: prompt},
		},
		// Fail fast on 429 — the next model has its own rate limit, so
		// switching beats making the user wait through backoff retries.
		NoRetry:         true,
		MaxTokens:       diagramMaxTokens,
		ReasoningEffort: reasoningEffortFor(a.model),
	}
	if a.search {
		req.Messages[0].Content = diagramSearchPrompt + diagramSystemPrompt
		req.Tools = []groq.Tool{groq.ToolBrowserSearch}
	} else {
		// JSON mode guarantees syntactically valid JSON; it can't be
		// combined with tools, so only non-search attempts use it.
		req.JSONMode = true
	}

	res, err := s.groqClient.CompleteResult(ctx, req)
	if err != nil {
		return nil, err
	}
	graph, ok := parseGraph(res.Content)
	if !ok {
		// Log what came back so truncation vs. prose vs. empty is diagnosable.
		utils.Error("[AI]", fmt.Sprintf("GenerateDiagram: %s returned unusable output (finish=%s, completion_tokens=%d, len=%d, head=%q)",
			a.model, res.FinishReason, res.CompletionTokens, len(res.Content), truncate(res.Content, 80)))
		return nil, errInvalidDiagramJSON
	}
	return graph, nil
}

var errInvalidDiagramJSON = errors.New("model did not return valid diagram JSON")

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// failureKind labels an attempt error for logs.
func failureKind(err error) string {
	var statusErr *groq.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusNotFound:
			return "model not available to this API key"
		case http.StatusRequestEntityTooLarge, http.StatusTooManyRequests:
			if statusErr.RetryAfter > 0 {
				return fmt.Sprintf("rate limited (retry in %s)", statusErr.RetryAfter.Round(100*time.Millisecond))
			}
			return "rate limited"
		case http.StatusBadRequest:
			return "rejected request (e.g. bad tool call)"
		}
		return fmt.Sprintf("HTTP %d", statusErr.StatusCode)
	}
	if errors.Is(err, errInvalidDiagramJSON) {
		return "invalid JSON"
	}
	return "request error"
}

// parseGraph extracts the {title, nodes, edges} object from a model reply,
// tolerating code fences and reasoning prose before the JSON. Requires a
// non-empty nodes array so a stray {...} in prose isn't mistaken for a diagram.
func parseGraph(reply string) (map[string]any, bool) {
	valid := func(g map[string]any) bool {
		nodes, ok := g["nodes"].([]any)
		return ok && len(nodes) > 0
	}

	var graph map[string]any
	if err := json.Unmarshal([]byte(cleanJSONFences(reply)), &graph); err == nil && valid(graph) {
		return graph, true
	}
	// Agentic models can narrate before the JSON despite the system prompt;
	// the real answer is the last balanced {...} block.
	if block, ok := trailingJSONBlock(reply); ok {
		graph = nil
		if err := json.Unmarshal([]byte(block), &graph); err == nil && valid(graph) {
			return graph, true
		}
	}
	return nil, false
}

// cleanJSONFences strips a leading/trailing markdown code fence, if present.
func cleanJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// trailingJSONBlock finds the last balanced {...} block in s by scanning
// backward from the final '}' and tracking brace depth. Used when a model
// prepends prose/reasoning before its actual JSON answer.
func trailingJSONBlock(s string) (string, bool) {
	end := strings.LastIndex(s, "}")
	if end == -1 {
		return "", false
	}
	depth := 0
	for i := end; i >= 0; i-- {
		switch s[i] {
		case '}':
			depth++
		case '{':
			depth--
			if depth == 0 {
				return s[i : end+1], true
			}
		}
	}
	return "", false
}

// ── Short-lived result cache ──────────────────────────────────────────────────

// diagramCache keeps recent successful results so retrying the same prompt
// (e.g. after closing the panel) costs no tokens. In-memory, per instance.
type diagramCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	graph   map[string]any
	expires time.Time
}

func newDiagramCache() *diagramCache {
	return &diagramCache{entries: map[string]cacheEntry{}}
}

// cacheKey normalises case and whitespace so trivially different prompts hit.
func cacheKey(prompt string, search bool) string {
	norm := strings.Join(strings.Fields(strings.ToLower(prompt)), " ")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%t|%s", search, norm)))
	return hex.EncodeToString(sum[:])
}

func (c *diagramCache) get(key string) (map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.graph, true
}

func (c *diagramCache) put(key string, graph map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if len(c.entries) >= cacheMax {
		var oldestKey string
		var oldest time.Time
		for k, e := range c.entries {
			if now.After(e.expires) {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || e.expires.Before(oldest) {
				oldestKey, oldest = k, e.expires
			}
		}
		if len(c.entries) >= cacheMax {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = cacheEntry{graph: graph, expires: now.Add(cacheTTL)}
}
