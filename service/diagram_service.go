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

// ── Conversational diagram generation ────────────────────────────────────────

// diagramConversationalSystemPrompt instructs the model to use a discriminated
// {"action":"ask"|"diagram"} schema with an explicit decision policy so the
// model reliably chooses to ask only when an answer would materially change
// the graph (not for cosmetic unknowns). Based on: CLAM framework, CIGAsk
// counterfactual information gain, and Eraser.io production patterns.
const diagramConversationalSystemPrompt = `You are a diagram architecture planner.

Return ONLY one JSON object — no prose, no markdown, no explanation outside the JSON.

OPTION A — need clarification:
{"action":"ask","question":"<single targeted question with 2-4 lettered options when possible>","missing":["<what is unknown>"]}

OPTION B — enough detail to generate:
{"action":"diagram","title":"...","nodes":[...],"edges":[...],"assumptions":["<assumed X>","<assumed Y>"]}

DECISION POLICY — before choosing, silently verify all 6:
1. Diagram type known? (system / data-flow / deployment / sequence / ER / workflow)
2. Major components and responsibilities known?
3. Request path or data flow known?
4. External systems and protocols known?
5. Deployment / cloud / storage constraints known?
6. Any non-functional constraints that change architecture? (scale, latency, security, offline, cost)

Choose "ask" ONLY if a missing answer would materially change nodes, edges, grouping, or deployment topology.
Choose "diagram" when remaining unknowns are cosmetic or can be safely assumed.

Rules for "ask":
- One question per turn, never two topics
- Offer 2-4 concrete mutually exclusive lettered options when the answer space is bounded:
  "What is the primary deployment shape?
   A. Single monolithic backend + managed database
   B. Modular services behind an API gateway
   C. Event-driven services with a message broker
   D. Serverless functions + managed services"
- Never ask about styling, colors, fonts, or team names
- Never ask a question whose answer is already present in the conversation
- Never ask more than 3 questions total across the entire conversation

Rules for "diagram":
- Max 15 nodes
- Node IDs: lowercase_snake_case (e.g. "api_gateway", "user_service")
- All edge source/target must exactly match a node ID
- Edge labels: 1-3 words
- Use real service names when user mentions a specific technology
- List every assumption made in the "assumptions" array (e.g. "PostgreSQL for persistence", "REST over HTTP", "AWS deployment")
- Shape guide:
  rectangle: services, APIs, apps, users, clients, servers, components
  cylinder: databases, caches (Redis), queues (Kafka/RabbitMQ), storage
  diamond: decisions, conditions, if/else, gateways
  circle: start/end points, events
  hexagon: external systems, third-party services
  rounded-rectangle: processes, jobs, workers
  parallelogram: data inputs/outputs only
  triangle: alerts, warnings only
  star: highlights only

Example — specific enough, generate immediately with assumptions:
User: "I need a backend for my SaaS"
Response: {"action":"diagram","title":"SaaS Backend","nodes":[{"id":"client","label":"Web Client","shape":"rectangle"},{"id":"api","label":"REST API","shape":"rectangle"},{"id":"db","label":"PostgreSQL","shape":"cylinder"},{"id":"cache","label":"Redis Cache","shape":"cylinder"},{"id":"auth","label":"Auth Service","shape":"rectangle"}],"edges":[{"source":"client","target":"api","label":"HTTPS"},{"source":"api","target":"db","label":"queries"},{"source":"api","target":"cache","label":"cache"},{"source":"api","target":"auth","label":"verify"}],"assumptions":["PostgreSQL for persistence","Redis for caching","REST over HTTP","single-region deployment"]}

Example — topology is ambiguous, ask first with options:
User: "I need a backend architecture diagram"
Response: {"action":"ask","question":"What is the primary deployment shape?\nA. Single monolithic backend + managed database\nB. Modular services behind an API gateway\nC. Event-driven services with a message broker\nD. Serverless functions + managed services","missing":["deployment topology"]}`

// ConversationTurn is one message in a diagram conversation.
type ConversationTurn struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // raw text or raw JSON from assistant
}

// ConversationResponse is returned by GenerateDiagramConversational each turn.
type ConversationResponse struct {
	Action      string         `json:"action"`                // "ask" or "diagram"
	Question    string         `json:"question,omitempty"`    // set when action=="ask"
	Graph       map[string]any `json:"graph,omitempty"`       // set when action=="diagram"
	Assumptions []string       `json:"assumptions,omitempty"` // model's stated assumptions (action=="diagram")
}

// GenerateDiagramConversational handles one turn of a multi-turn diagram session.
// history is the full conversation so far (user + assistant turns). Each call
// returns either a clarifying question or the final diagram JSON.
func (s *DiagramService) GenerateDiagramConversational(
	ctx context.Context,
	history []ConversationTurn,
	opts DiagramOptions,
) (*ConversationResponse, error) {
	if s.groqClient == nil {
		return nil, errs.Internal("diagram generation requires GROQ_API_KEY to be configured")
	}
	if len(history) == 0 {
		return nil, errs.BadRequest("conversation history cannot be empty")
	}

	// Count prior "ask" turns to enforce the 3-question hard cap.
	askCount := 0
	for _, t := range history {
		if t.Role == "assistant" {
			var m map[string]any
			if json.Unmarshal([]byte(t.Content), &m) == nil {
				if m["action"] == "ask" {
					askCount++
				}
			}
		}
	}

	// Build Groq messages: system prompt + full history.
	sysPrompt := diagramConversationalSystemPrompt
	if askCount >= 3 {
		sysPrompt += "\n\nYou have already asked 3 questions. Generate the diagram NOW. Do not ask any more questions."
	}
	messages := make([]aiclient.Message, 0, len(history)+1)
	messages = append(messages, aiclient.Message{Role: "system", Content: sysPrompt})
	for _, t := range history {
		messages = append(messages, aiclient.Message{Role: t.Role, Content: t.Content})
	}

	search := opts.WebSearch && s.searchAllowed
	attempts := []diagramAttempt{{model: s.primary, search: search}}
	for _, m := range s.fallbacks {
		if m != "" && m != s.primary {
			attempts = append(attempts, diagramAttempt{model: m})
		}
	}

	rawContent, minWait, allLimited, err := s.runChainMessages(ctx, messages, attempts)
	if err != nil {
		// Same rate-limit retry logic as GenerateDiagram.
		if allLimited && minWait > 0 && minWait <= maxServerWait {
			utils.Info("[AI]", fmt.Sprintf("GenerateDiagramConversational: all models rate-limited, waiting %s", minWait.Round(100*time.Millisecond)))
			select {
			case <-ctx.Done():
				return nil, errs.Internal(diagramUnavailableMsg)
			case <-time.After(minWait + 300*time.Millisecond):
			}
			retry := make([]diagramAttempt, len(attempts))
			for i, a := range attempts {
				retry[i] = diagramAttempt{model: a.model}
			}
			rawContent, minWait, allLimited, err = s.runChainMessages(ctx, messages, retry)
		}
		if err != nil {
			if allLimited {
				secs := int(math.Ceil(minWait.Seconds()))
				if secs < 3 {
					secs = 3
				}
				return nil, errs.RateLimited("AI is busy right now — too many requests this minute.", secs)
			}
			return nil, errs.Internal(diagramUnavailableMsg)
		}
	}

	resp, err := parseConversationResponse(rawContent)
	if err != nil {
		return nil, errs.Internal(diagramUnavailableMsg)
	}

	// For diagram responses, run server-side graph validation and silently
	// repair up to maxRepairRounds if the model produced invalid edges/IDs.
	if resp.Action == "diagram" && resp.Graph != nil {
		if validationErrs := validateGraph(resp.Graph); len(validationErrs) > 0 {
			resp = s.repairDiagram(ctx, messages, resp, 1)
		}
	}

	return resp, nil
}

// runChainMessages is like runChain but takes a pre-built messages slice instead
// of a single prompt string. Used by the conversational flow.
func (s *DiagramService) runChainMessages(
	ctx context.Context,
	messages []aiclient.Message,
	attempts []diagramAttempt,
) (string, time.Duration, bool, error) {
	var minWait time.Duration
	allLimited := true
	var lastErr error

	for _, a := range attempts {
		content, err := s.attemptMessages(ctx, messages, a)
		if err == nil {
			return content, 0, false, nil
		}
		if ctx.Err() != nil {
			return "", 0, false, ctx.Err()
		}
		lastErr = err
		utils.Error("[AI]", fmt.Sprintf("GenerateDiagramConversational: %s (search=%t) failed: %s", a.model, a.search, failureKind(err)), err)

		var statusErr *groq.StatusError
		if errors.As(err, &statusErr) && (statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode == http.StatusRequestEntityTooLarge) {
			if statusErr.RetryAfter > 0 && (minWait == 0 || statusErr.RetryAfter < minWait) {
				minWait = statusErr.RetryAfter
			}
			continue
		}
		allLimited = false
	}
	return "", minWait, allLimited, lastErr
}

// attemptMessages runs one model attempt with a pre-built messages slice.
func (s *DiagramService) attemptMessages(ctx context.Context, messages []aiclient.Message, a diagramAttempt) (string, error) {
	req := groq.Request{
		Model:           a.model,
		Messages:        messages,
		NoRetry:         true,
		MaxTokens:       diagramMaxTokens,
		ReasoningEffort: reasoningEffortFor(a.model),
		JSONMode:        !a.search, // JSON mode incompatible with tools
	}
	if a.search {
		msgs := make([]aiclient.Message, len(messages))
		copy(msgs, messages)
		msgs[0].Content = diagramSearchPrompt + msgs[0].Content
		req.Messages = msgs
		req.Tools = []groq.Tool{groq.ToolBrowserSearch}
	}
	res, err := s.groqClient.CompleteResult(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// parseConversationResponse parses a discriminated {"action":"ask"|"diagram"} reply.
// Falls back to treating unrecognised plain-text output as an implicit question so
// the conversation keeps moving rather than hard-failing.
func parseConversationResponse(raw string) (*ConversationResponse, error) {
	clean := cleanJSONFences(raw)
	var m map[string]any
	if err := json.Unmarshal([]byte(clean), &m); err != nil {
		if block, ok := trailingJSONBlock(raw); ok {
			if err2 := json.Unmarshal([]byte(block), &m); err2 != nil {
				// Plain-text fallback — keep conversation moving.
				return &ConversationResponse{Action: "ask", Question: strings.TrimSpace(raw)}, nil
			}
		} else {
			return &ConversationResponse{Action: "ask", Question: strings.TrimSpace(raw)}, nil
		}
	}

	action, _ := m["action"].(string)
	switch action {
	case "ask":
		q, _ := m["question"].(string)
		if q == "" {
			return &ConversationResponse{Action: "ask", Question: strings.TrimSpace(raw)}, nil
		}
		return &ConversationResponse{Action: "ask", Question: q}, nil
	case "diagram":
		g, ok := parseGraph(raw)
		if !ok {
			return nil, errInvalidDiagramJSON
		}
		resp := &ConversationResponse{Action: "diagram", Graph: g}
		// Extract model-stated assumptions (new field in updated prompt).
		if assumptionsRaw, ok := m["assumptions"].([]any); ok {
			for _, a := range assumptionsRaw {
				if s, ok := a.(string); ok && s != "" {
					resp.Assumptions = append(resp.Assumptions, s)
				}
			}
		}
		return resp, nil
	default:
		return &ConversationResponse{Action: "ask", Question: strings.TrimSpace(raw)}, nil
	}
}

// validateGraph runs deterministic checks on a parsed diagram graph and returns
// a list of human-readable error strings. Empty slice means valid.
func validateGraph(graph map[string]any) []string {
	var errsOut []string

	nodesRaw, _ := graph["nodes"].([]any)
	if len(nodesRaw) == 0 {
		return []string{"nodes array is empty"}
	}

	nodeIDs := make(map[string]bool, len(nodesRaw))
	for i, nr := range nodesRaw {
		n, ok := nr.(map[string]any)
		if !ok {
			errsOut = append(errsOut, fmt.Sprintf("node[%d] is not an object", i))
			continue
		}
		id, _ := n["id"].(string)
		if id == "" {
			errsOut = append(errsOut, fmt.Sprintf("node[%d] has empty id", i))
			continue
		}
		if nodeIDs[id] {
			errsOut = append(errsOut, fmt.Sprintf("duplicate node id %q", id))
		}
		nodeIDs[id] = true
	}

	edgesRaw, _ := graph["edges"].([]any)
	for i, er := range edgesRaw {
		e, ok := er.(map[string]any)
		if !ok {
			continue
		}
		src, _ := e["source"].(string)
		tgt, _ := e["target"].(string)
		if src != "" && !nodeIDs[src] {
			errsOut = append(errsOut, fmt.Sprintf("edge[%d] source %q references unknown node", i, src))
		}
		if tgt != "" && !nodeIDs[tgt] {
			errsOut = append(errsOut, fmt.Sprintf("edge[%d] target %q references unknown node", i, tgt))
		}
		if src != "" && src == tgt {
			errsOut = append(errsOut, fmt.Sprintf("edge[%d] is a self-loop on %q", i, src))
		}
	}

	return errsOut
}

// repairDiagram sends a repair prompt containing the specific validation errors
// and returns the corrected ConversationResponse (action=="diagram" guaranteed).
// At most maxRepairRounds calls are made; on failure the original resp is returned.
const maxRepairRounds = 2

func (s *DiagramService) repairDiagram(
	ctx context.Context,
	messages []aiclient.Message,
	resp *ConversationResponse,
	round int,
) *ConversationResponse {
	validationErrs := validateGraph(resp.Graph)
	if len(validationErrs) == 0 || round > maxRepairRounds {
		return resp
	}

	utils.Info("[AI]", fmt.Sprintf("repairDiagram: round %d, %d error(s): %v", round, len(validationErrs), validationErrs))

	repairPrompt := fmt.Sprintf(
		"The diagram JSON you returned has validation errors. Fix them.\n\nErrors:\n%s\n\nReturn ONLY the corrected diagram JSON. Preserve all valid nodes and edges. Do not rename existing IDs. Make the smallest possible correction.",
		strings.Join(validationErrs, "\n"),
	)

	repairMessages := append(messages,
		aiclient.Message{Role: "assistant", Content: fmt.Sprintf(`{"action":"diagram",%s}`, mustJSON(resp.Graph))},
		aiclient.Message{Role: "user", Content: repairPrompt},
	)

	req := groq.Request{
		Model:           s.primary,
		Messages:        repairMessages,
		NoRetry:         true,
		MaxTokens:       diagramMaxTokens,
		ReasoningEffort: reasoningEffortFor(s.primary),
		JSONMode:        true,
	}
	res, err := s.groqClient.CompleteResult(ctx, req)
	if err != nil {
		utils.Error("[AI]", fmt.Sprintf("repairDiagram round %d failed: %v", round, err))
		return resp
	}

	repaired, err := parseConversationResponse(res.Content)
	if err != nil || repaired.Action != "diagram" {
		utils.Error("[AI]", fmt.Sprintf("repairDiagram round %d: could not parse repaired output", round))
		return resp
	}

	// Recurse if still invalid (capped by maxRepairRounds).
	return s.repairDiagram(ctx, messages, repaired, round+1)
}

// mustJSON marshals v to a JSON string fragment for embedding in repair prompts.
// Panics only on types that can't be marshalled (safe here — v is map[string]any).
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	// Trim leading '{' and trailing '}' so caller can embed inside a wrapper object.
	s := string(b)
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}

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
