package ai

import "context"

// ChatMessage is one turn for chat-style completion APIs.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMProvider backs chat completion and text embeddings (RAG groundwork).
type LLMProvider interface {
	Complete(ctx context.Context, messages []ChatMessage) (string, error)
	StreamComplete(ctx context.Context, messages []ChatMessage, onChunk func(string) error) error
	Embed(ctx context.Context, text string) ([]float32, error)
}
