package service

import (
	"context"
	"encoding/json"
	"strings"

	"syncspace-backend/errs"
	"syncspace-backend/pkg/aiclient"
	"syncspace-backend/pkg/groq"
)

// diagramModel is fixed to Groq's web-search-backed compound model — this
// feature has no local/Ollama equivalent, since it depends on live web
// search to confirm real service names and architecture patterns.
const diagramModel = "groq/compound"

const diagramSystemPrompt = `You are a diagram generation engine with web search access.

When the user describes an architecture, system, or process, search the web if needed to confirm accurate, current details (real service names, typical component relationships, standard patterns) — then output ONLY valid JSON. No markdown. No explanation. No text outside the JSON.

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
- Use real, accurate component/service names when the user names a specific technology or cloud provider — verify via search rather than guessing
- Make diagrams meaningful: include realistic components for the architecture described

Example output for "Redis pub/sub":
{"title":"Redis Pub/Sub","nodes":[{"id":"publisher","label":"Publisher","shape":"rectangle"},{"id":"redis","label":"Redis","shape":"cylinder"},{"id":"sub1","label":"Subscriber 1","shape":"rectangle"},{"id":"sub2","label":"Subscriber 2","shape":"rectangle"}],"edges":[{"source":"publisher","target":"redis","label":"PUBLISH"},{"source":"redis","target":"sub1","label":"MESSAGE"},{"source":"redis","target":"sub2","label":"MESSAGE"}]}`

// DiagramService generates architecture-diagram JSON (nodes/edges) from a
// natural-language prompt, grounded by live web search via Groq's compound
// model. groqClient is nil when GROQ_API_KEY isn't configured — this feature
// has no offline fallback.
type DiagramService struct {
	groqClient *groq.Client
}

// NewDiagramService creates a DiagramService. Pass nil for groqClient when
// GROQ_API_KEY isn't configured — GenerateDiagram then returns errs.Internal.
func NewDiagramService(groqClient *groq.Client) *DiagramService {
	return &DiagramService{groqClient: groqClient}
}

// GenerateDiagram returns the parsed {title, nodes, edges} graph for the
// given prompt. Returns errs.BadRequest for an empty prompt, errs.Internal
// if Groq isn't configured, the request fails, or the model's output isn't
// valid JSON.
func (s *DiagramService) GenerateDiagram(ctx context.Context, prompt string) (map[string]any, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errs.BadRequest("prompt is required")
	}
	if s.groqClient == nil {
		return nil, errs.Internal("diagram generation requires GROQ_API_KEY to be configured")
	}

	reply, err := s.groqClient.ChatWithModel(ctx, diagramModel, []aiclient.Message{
		{Role: "system", Content: diagramSystemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, errs.Internal("diagram generation failed: " + err.Error())
	}

	var graph map[string]any
	if err := json.Unmarshal([]byte(cleanJSONFences(reply)), &graph); err == nil {
		return graph, nil
	}

	// groq/compound is agentic — it narrates its reasoning in prose before the
	// JSON despite the system prompt saying not to. Fall back to extracting the
	// last balanced {...} block, which is where the real answer ends up.
	if block, ok := trailingJSONBlock(reply); ok {
		if err := json.Unmarshal([]byte(block), &graph); err == nil {
			return graph, nil
		}
	}

	return nil, errs.Internal("model did not return valid JSON")
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
