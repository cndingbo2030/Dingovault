package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/graph"
	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/cndingbo2030/dingovault/internal/storage"
)

// AISettingsDTO is persisted AI configuration exposed to the Svelte UI.
type AISettingsDTO struct {
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	Endpoint          string  `json:"endpoint"`
	APIKey            string  `json:"apiKey"`
	Temperature       float64 `json:"temperature"`
	EmbeddingsModel   string  `json:"embeddingsModel"`
	DisableEmbeddings bool    `json:"disableEmbeddings"`
	SystemPrompt      string  `json:"systemPrompt"`
	SemanticTopK      int     `json:"semanticTopK"`
}

func dtoFromAI(a config.AISettings) AISettingsDTO {
	a = config.NormalizeAISettings(a)
	return AISettingsDTO{
		Provider:          a.Provider,
		Model:             a.Model,
		Endpoint:          a.Endpoint,
		APIKey:            a.APIKey,
		Temperature:       a.Temperature,
		EmbeddingsModel:   a.EmbeddingsModel,
		DisableEmbeddings: a.DisableEmbeddings,
		SystemPrompt:      a.SystemPrompt,
		SemanticTopK:      a.SemanticTopK,
	}
}

// GetAISettings returns persisted AI options (API key is local-only, like cloudToken).
func (a *App) GetAISettings() (AISettingsDTO, error) {
	c, err := config.Load()
	if err != nil {
		return AISettingsDTO{}, err
	}
	return dtoFromAI(c.AI), nil
}

// SetAISettings saves AI options. Empty apiKey preserves the previous key when provider stays OpenAI.
func (a *App) SetAISettings(dto AISettingsDTO) error {
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	prev := c.AI
	c.AI.Provider = strings.TrimSpace(dto.Provider)
	if c.AI.Provider == "" {
		c.AI.Provider = config.Default().AI.Provider
	}
	c.AI.Model = strings.TrimSpace(dto.Model)
	c.AI.Endpoint = strings.TrimSpace(dto.Endpoint)
	c.AI.Temperature = dto.Temperature
	c.AI.EmbeddingsModel = strings.TrimSpace(dto.EmbeddingsModel)
	c.AI.DisableEmbeddings = dto.DisableEmbeddings
	c.AI.SystemPrompt = dto.SystemPrompt
	c.AI.SemanticTopK = dto.SemanticTopK
	key := strings.TrimSpace(dto.APIKey)
	if key == "" && strings.EqualFold(strings.TrimSpace(c.AI.Provider), "openai") {
		c.AI.APIKey = prev.APIKey
	} else {
		c.AI.APIKey = key
	}
	c.AI = config.NormalizeAISettings(c.AI)
	if strings.EqualFold(strings.TrimSpace(c.AI.Provider), "openai") && strings.TrimSpace(c.AI.APIKey) == "" {
		return fmt.Errorf("%s", a.t(locale.ErrAIKeyRequired))
	}
	return config.Save(c)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "\n\n[… truncated for context limit …]"
}

func formatSemanticHits(notesRoot string, hits []storage.SemanticSearchHit) string {
	if len(hits) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	var b strings.Builder
	for _, h := range hits {
		if _, ok := seen[h.BlockID]; ok {
			continue
		}
		seen[h.BlockID] = struct{}{}
		rel := strings.TrimSpace(h.SourcePath)
		if notesRoot != "" && rel != "" {
			if r, err := graph.VaultRelativePath(notesRoot, h.SourcePath); err == nil && strings.TrimSpace(r) != "" {
				rel = r
			}
		}
		body := strings.TrimSpace(h.Content)
		body = truncateRunes(body, 520)
		if body == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[similarity=%.3f] %s\n%s\n\n", h.Score, rel, body))
	}
	return strings.TrimSpace(b.String())
}

// AIChat uses RAG: current page Markdown plus top semantic hits from the vault.
func (a *App) AIChat(pagePath, userMessage string) (string, error) {
	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return "", fmt.Errorf("%s", a.t(locale.ErrAIEmptyMessage))
	}
	if a.notesRoot == "" {
		return "", fmt.Errorf("%s", a.t(locale.ErrNotesRootNotSet))
	}
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	p, err := ai.NewProvider(c.AI)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	abs, err := a.resolveVaultMarkdownAbs(ctx, pagePath)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.t(locale.ErrReadPage), err)
	}
	pageText := truncateRunes(string(raw), 14000)
	sys := effectiveAISystemPrompt(c.AI)
	semanticBlock := a.aiChatSemanticBlock(ctx, p, msg, c.AI.EmbeddingsModel, c.AI.SemanticTopK)
	userPayload := buildAIChatUserPayload(pageText, msg, semanticBlock)
	msgs := []ai.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: userPayload},
	}
	out, err := p.Complete(ctx, msgs)
	if err != nil {
		return "", err
	}
	return out, nil
}
