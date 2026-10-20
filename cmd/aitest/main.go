// aitest is a diagnostic / stress CLI for AI diagram generation.
//
// It checks which configured Groq models this API key can use, then runs the
// real DiagramService (model chain, fallbacks, rate-limit wait, result
// cache) and reports each result. Reads .env / environment exactly like the
// server does. Each run spends Groq tokens (≈2–4k, ≈10k with web search).
//
// Usage:
//
//	go run ./cmd/aitest                                  # one run, default prompt
//	go run ./cmd/aitest -prompt "Redis pub/sub" -search  # with web search
//	go run ./cmd/aitest -n 8                             # stress: 8 varied prompts back to back
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"syncspace-backend/config"
	"syncspace-backend/pkg/groq"
	"syncspace-backend/service"
)

const defaultPrompt = "Architecture for a mobile app offline sync: local SQLite cache, sync queue, " +
	"conflict resolver using last-write-wins with vector clocks, REST API gateway, PostgreSQL, " +
	"and push notification service."

// stressPrompts cover small, large and vague requests — the 12-service Kafka
// prompt is the one that used to hit the 2048-token truncation.
var stressPrompts = []string{
	defaultPrompt,
	"User signup flow with email verification and OAuth via Google",
	"AWS serverless e-commerce checkout: API Gateway, Lambda, DynamoDB, SQS, Stripe, SES",
	"CI/CD pipeline from git push to Kubernetes production with staging, tests and approvals",
	"Kafka based event-driven microservices for an online food delivery platform with 12 services",
	"Netflix-style video streaming platform architecture with CDN, transcoding and recommendations",
	"RAG chatbot architecture with vector database, embeddings and LLM",
	"login",
}

func main() {
	prompt := flag.String("prompt", defaultPrompt, "diagram prompt to generate (ignored with -n > 1)")
	search := flag.Bool("search", false, "opt in to web search, like the panel's toggle")
	n := flag.Int("n", 1, "number of back-to-back runs (cycles through varied prompts when > 1)")
	flag.Parse()

	cfg := config.Load()
	if cfg.GroqAPIKey == "" {
		fmt.Println("✗ GROQ_API_KEY is empty — diagram generation is disabled")
		os.Exit(1)
	}

	chain := append([]string{cfg.GroqDiagramModel}, cfg.GroqDiagramFallbacks...)
	fmt.Printf("model chain: %s (web search allowed: %t, requested: %t)\n\n", strings.Join(chain, " → "), cfg.GroqDiagramWebSearch, *search)

	client := groq.NewClient(cfg.GroqAPIKey, cfg.GroqModel)
	listCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	available, err := client.ListModels(listCtx)
	cancel()
	if err != nil {
		fmt.Println("✗ could not list models:", err)
		os.Exit(1)
	}
	have := map[string]bool{}
	for _, id := range available {
		have[id] = true
	}
	for _, m := range chain {
		mark := "✓"
		if !have[m] {
			mark = "✗ NOT AVAILABLE"
		}
		fmt.Printf("  %s %s\n", mark, m)
	}

	svc := service.NewDiagramService(client, cfg.GroqDiagramModel, cfg.GroqDiagramFallbacks, cfg.GroqDiagramWebSearch)
	fmt.Println("\ngenerating (per-model failures and fallbacks are logged below)...")

	failures := 0
	for i := 0; i < *n; i++ {
		p := *prompt
		if *n > 1 {
			p = stressPrompts[i%len(stressPrompts)]
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		start := time.Now()
		graph, err := svc.GenerateDiagram(ctx, p, service.DiagramOptions{WebSearch: *search})
		cancel()
		dur := time.Since(start).Round(100 * time.Millisecond)
		short := p
		if len(short) > 50 {
			short = short[:50] + "…"
		}
		if err != nil {
			failures++
			fmt.Printf("✗ #%d %-52q %6s  %v\n", i+1, short, dur, err)
			continue
		}
		nodes, _ := graph["nodes"].([]any)
		edges, _ := graph["edges"].([]any)
		fmt.Printf("✓ #%d %-52q %6s  %2d nodes %2d edges\n", i+1, short, dur, len(nodes), len(edges))
		if *n == 1 {
			for _, nd := range nodes {
				if m, ok := nd.(map[string]any); ok {
					fmt.Printf("    - %v (%v)\n", m["label"], m["shape"])
				}
			}
		}
	}
	if *n > 1 {
		fmt.Printf("\n%d/%d succeeded\n", *n-failures, *n)
	}
	if failures > 0 {
		os.Exit(1)
	}
}
