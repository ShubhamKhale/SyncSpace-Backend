package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"syncspace-backend/errs"
	"syncspace-backend/pkg/aiclient"
	"syncspace-backend/pkg/groq"
	"syncspace-backend/pkg/utils"
)

// diagramUnavailableMsg is what users see when both the primary and fallback
// models fail; the underlying Groq error is logged, not returned, since it
// exposes provider details and isn't actionable for the user.
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
- Use real, accurate component/service names when the user names a specific technology or cloud provider
- Make diagrams meaningful: include realistic components for the architecture described

Example output for "Redis pub/sub":
{"title":"Redis Pub/Sub","nodes":[{"id":"publisher","label":"Publisher","shape":"rectangle"},{"id":"redis","label":"Redis","shape":"cylinder"},{"id":"sub1","label":"Subscriber 1","shape":"rectangle"},{"id":"sub2","label":"Subscriber 2","shape":"rectangle"}],"edges":[{"source":"publisher","target":"redis","label":"PUBLISH"},{"source":"redis","target":"sub1","label":"MESSAGE"},{"source":"redis","target":"sub2","label":"MESSAGE"}]}`

// DiagramService generates architecture-diagram JSON (nodes/edges) from a
// natural-language prompt. The primary model (GROQ_DIAGRAM_MODEL) runs with
// Groq's built-in browser_search tool when webSearch is on, so diagrams use
// real service names. Any primary failure — retired/disabled model (404),
// tokens-per-minute limit (413/429: one searched diagram uses ~10k tokens,
// most of the free tier's per-minute budget), bad tool call (400), or
// non-JSON output — falls back once to the client's default chat model with
// no tools, which has its own separate rate limit.
//
// groqClient is nil when GROQ_API_KEY isn't configured — this feature has no
// offline fallback.
type DiagramService struct {
	groqClient *groq.Client
	model      string
	webSearch  bool
}

// NewDiagramService creates a DiagramService. Pass nil for groqClient when
// GROQ_API_KEY isn't configured — GenerateDiagram then returns errs.Internal.
func NewDiagramService(groqClient *groq.Client, model string, webSearch bool) *DiagramService {
	return &DiagramService{groqClient: groqClient, model: model, webSearch: webSearch}
}

// GenerateDiagram returns the parsed {title, nodes, edges} graph for the
// given prompt. Returns errs.BadRequest for an empty prompt, errs.Internal
// if Groq isn't configured or both the primary and fallback attempts fail.
func (s *DiagramService) GenerateDiagram(ctx context.Context, prompt string) (map[string]any, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errs.BadRequest("prompt is required")
	}
	if s.groqClient == nil {
		return nil, errs.Internal("diagram generation requires GROQ_API_KEY to be configured")
	}

	primary := groq.Request{
		Model: s.model,
		Messages: []aiclient.Message{
			{Role: "system", Content: diagramSystemPrompt},
			{Role: "user", Content: prompt},
		},
		// Fail fast on 429 — the fallback model has its own rate limit, so
		// switching beats making the user wait through backoff retries.
		NoRetry: true,
	}
	if s.webSearch {
		primary.Messages[0].Content = diagramSearchPrompt + diagramSystemPrompt
		primary.Tools = []groq.Tool{groq.ToolBrowserSearch}
	}

	graph, err := s.attempt(ctx, primary)
	if err == nil {
		return graph, nil
	}
	if ctx.Err() != nil {
		return nil, errs.Internal(diagramUnavailableMsg)
	}
	utils.Error("[AI]", fmt.Sprintf("GenerateDiagram: primary model %s failed (%s), falling back to %s",
		s.model, failureKind(err), s.groqClient.Model()), err)

	fallback := groq.Request{
		Model: s.groqClient.Model(),
		Messages: []aiclient.Message{
			{Role: "system", Content: diagramSystemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	graph, err = s.attempt(ctx, fallback)
	if err != nil {
		utils.Error("[AI]", "GenerateDiagram: fallback model "+fallback.Model+" failed ("+failureKind(err)+")", err)
		return nil, errs.Internal(diagramUnavailableMsg)
	}
	return graph, nil
}

// attempt runs one request and parses the reply into a graph.
func (s *DiagramService) attempt(ctx context.Context, req groq.Request) (map[string]any, error) {
	reply, err := s.groqClient.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	graph, ok := parseGraph(reply)
	if !ok {
		return nil, errInvalidDiagramJSON
	}
	return graph, nil
}

var errInvalidDiagramJSON = errors.New("model did not return valid diagram JSON")

// failureKind labels an attempt error for logs.
func failureKind(err error) string {
	var statusErr *groq.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusNotFound:
			return "model not available to this API key"
		case http.StatusRequestEntityTooLarge, http.StatusTooManyRequests:
			return "rate limited"
		case http.StatusBadRequest:
			return "rejected request (e.g. bad tool call)"
		}
		return fmt.Sprintf("HTTP %d", statusErr.StatusCode)
	}
	if errors.Is(err, errInvalidDiagramJSON) {
		return "invalid JSON"
	}
	if strings.Contains(err.Error(), "rate limited") {
		return "rate limited"
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
