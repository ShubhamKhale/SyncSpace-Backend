package service

import (
	"context"
	"fmt"
	"strings"

	"syncspace-backend/errs"
	"syncspace-backend/pkg/aiclient"
	"syncspace-backend/service/database"
)

// AIService provides board-content chat and text summarization backed by an
// aiclient.ChatClient — a local Ollama model in dev, or a cloud provider
// (Groq) in prod, chosen by whichever client is injected at startup.
type AIService struct {
	llm                aiclient.ChatClient
	boardRepo         *database.BoardRepo
	taskRepo          *database.TaskRepo
	linkedResourceRepo *database.LinkedResourceRepo
}

// NewAIService creates an AIService backed by the given chat client.
func NewAIService(
	llm aiclient.ChatClient,
	boardRepo *database.BoardRepo,
	taskRepo *database.TaskRepo,
	linkedResourceRepo *database.LinkedResourceRepo,
) *AIService {
	return &AIService{
		llm:                llm,
		boardRepo:          boardRepo,
		taskRepo:           taskRepo,
		linkedResourceRepo: linkedResourceRepo,
	}
}

// ChatAboutBoard answers a question grounded in the board's current title,
// description, tasks, and linked resources. Returns errs.NotFound if the
// board doesn't exist, errs.Internal if the local model call fails.
func (s *AIService) ChatAboutBoard(ctx context.Context, boardID, question string) (string, error) {
	board, err := s.boardRepo.GetBoardByID(ctx, boardID)
	if err != nil {
		return "", err
	}

	tasks, err := s.taskRepo.GetTasksByBoard(ctx, boardID, database.TaskFilter{})
	if err != nil {
		return "", err
	}

	resources, err := s.linkedResourceRepo.GetByBoard(ctx, boardID)
	if err != nil {
		return "", err
	}

	var ctxBuilder strings.Builder
	fmt.Fprintf(&ctxBuilder, "Board: %s\nDescription: %s\n\n", board.Title, board.Description)

	ctxBuilder.WriteString("Tasks:\n")
	if len(tasks) == 0 {
		ctxBuilder.WriteString("(none)\n")
	}
	for _, t := range tasks {
		fmt.Fprintf(&ctxBuilder, "- [%s/%s] %s: %s\n", t.Stage, t.Priority, t.Title, t.Description)
	}

	ctxBuilder.WriteString("\nLinked resources:\n")
	if len(resources) == 0 {
		ctxBuilder.WriteString("(none)\n")
	}
	for _, r := range resources {
		fmt.Fprintf(&ctxBuilder, "- %s: %s\n", r.Label, r.URL)
	}

	reply, err := s.llm.Chat(ctx, []aiclient.Message{
		{Role: "system", Content: "You are an assistant answering questions about a project board. " +
			"Use only the board context provided below; if the answer isn't in it, say so.\n\n" + ctxBuilder.String()},
		{Role: "user", Content: question},
	})
	if err != nil {
		return "", errs.Internal("AI chat failed: " + err.Error())
	}
	return reply, nil
}

// Summarize returns a concise summary of the given text.
// Returns errs.BadRequest if text is empty, errs.Internal if the model call fails.
func (s *AIService) Summarize(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", errs.BadRequest("text is required")
	}

	reply, err := s.llm.Chat(ctx, []aiclient.Message{
		{Role: "system", Content: "Summarize the following document concisely, in plain prose, capturing the key points."},
		{Role: "user", Content: text},
	})
	if err != nil {
		return "", errs.Internal("AI summarization failed: " + err.Error())
	}
	return reply, nil
}
