package ai

import (
	"fmt"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/config"
)

// NewProvider builds an LLMProvider from persisted AI settings.
func NewProvider(s config.AISettings) (LLMProvider, error) {
	s = config.NormalizeAISettings(s)
	switch strings.ToLower(strings.TrimSpace(s.Provider)) {
	case "", "ollama":
		return newOllama(s), nil
	case "openai":
		if strings.TrimSpace(s.APIKey) == "" {
			return nil, fmt.Errorf("openai: apiKey is required")
		}
		return newOpenAI(s), nil
	default:
		return nil, fmt.Errorf("unknown ai provider %q", s.Provider)
	}
}
