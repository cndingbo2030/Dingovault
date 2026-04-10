package bridge

import (
	"context"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/config"
)

func (a *App) aiChatSemanticBlock(ctx context.Context, p ai.LLMProvider, userMsg, embeddingsModel string, topK int) string {
	if a.store == nil {
		return ""
	}
	qvec, err := p.Embed(ctx, userMsg)
	if err != nil || len(qvec) == 0 {
		return ""
	}
	hits, err := a.store.SearchSemantic(ctx, qvec, embeddingsModel, topK)
	if err != nil {
		return ""
	}
	return formatSemanticHits(a.notesRoot, hits)
}

func buildAIChatUserPayload(pageText, userMsg, semanticBlock string) string {
	var b strings.Builder
	b.WriteString("--- Current page (Markdown) ---\n")
	b.WriteString(pageText)
	if semanticBlock != "" {
		b.WriteString("\n\n--- Related vault blocks (vector similarity) ---\n")
		b.WriteString(semanticBlock)
	}
	b.WriteString("\n\n--- User question ---\n")
	b.WriteString(userMsg)
	return b.String()
}

func effectiveAISystemPrompt(c config.AISettings) string {
	sys := strings.TrimSpace(c.SystemPrompt)
	if sys == "" {
		return config.Default().AI.SystemPrompt
	}
	return sys
}
