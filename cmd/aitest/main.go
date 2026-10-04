// aitest is a diagnostic CLI for AI diagram generation.
//
// It checks which configured Groq models this API key can use, then runs the
// real DiagramService (primary model with web search, fallback on failure)
// against a prompt and prints the result. Reads .env / environment exactly
// like the server does. Each run spends Groq tokens (~10k with web search).
//
// Usage:
//
//	go run ./cmd/aitest
//	go run ./cmd/aitest -prompt "Redis pub/sub with two subscribers"
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"syncspace-backend/config"
	"syncspace-backend/pkg/groq"
	"syncspace-backend/service"
)

const defaultPrompt = "Architecture for a mobile app offline sync: local SQLite cache, sync queue, " +
	"conflict resolver using last-write-wins with vector clocks, REST API gateway, PostgreSQL, " +
	"and push notification service."

func main() {
	prompt := flag.String("prompt", defaultPrompt, "diagram prompt to generate")
	flag.Parse()

	cfg := config.Load()
	if cfg.GroqAPIKey == "" {
		fmt.Println("✗ GROQ_API_KEY is empty — diagram generation is disabled")
		os.Exit(1)
	}

	fmt.Printf("primary : %s (web search: %t)\n", cfg.GroqDiagramModel, cfg.GroqDiagramWebSearch)
	fmt.Printf("fallback: %s\n\n", cfg.GroqModel)

	client := groq.NewClient(cfg.GroqAPIKey, cfg.GroqModel)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	available, err := client.ListModels(ctx)
	if err != nil {
		fmt.Println("✗ could not list models:", err)
		os.Exit(1)
	}
	have := map[string]bool{}
	for _, id := range available {
		have[id] = true
	}
	for _, m := range []string{cfg.GroqDiagramModel, cfg.GroqModel} {
		mark := "✓"
		if !have[m] {
			mark = "✗ NOT AVAILABLE"
		}
		fmt.Printf("  %s %s\n", mark, m)
	}

	fmt.Println("\ngenerating (failures and fallbacks are logged below)...")
	svc := service.NewDiagramService(client, cfg.GroqDiagramModel, cfg.GroqDiagramWebSearch)
	start := time.Now()
	graph, err := svc.GenerateDiagram(ctx, *prompt)
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}

	nodes, _ := graph["nodes"].([]any)
	edges, _ := graph["edges"].([]any)
	fmt.Printf("\n✓ %q — %d nodes, %d edges in %s\n", graph["title"], len(nodes), len(edges), time.Since(start).Round(100*time.Millisecond))
	for _, n := range nodes {
		if m, ok := n.(map[string]any); ok {
			fmt.Printf("    - %v (%v)\n", m["label"], m["shape"])
		}
	}
}
