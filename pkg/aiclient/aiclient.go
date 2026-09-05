// Package aiclient defines the shared chat-completion contract implemented
// by every LLM backend SyncSpace can use (local Ollama, Groq, ...), so the
// service layer can depend on one interface regardless of which is active.
package aiclient

import "context"

// Message is one turn in a chat conversation.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// ChatClient sends a message history to a model and returns its reply.
type ChatClient interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}
