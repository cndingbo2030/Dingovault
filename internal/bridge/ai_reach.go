package bridge

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/cndingbo2030/dingovault/internal/config"
)

// IsAIReachable returns true when the configured LLM endpoint responds (Ollama /api/tags or OpenAI root ping).
func (a *App) IsAIReachable() bool {
	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}

	switch strings.ToLower(strings.TrimSpace(c.AI.Provider)) {
	case "openai":
		u := strings.TrimSpace(c.AI.Endpoint)
		if u == "" {
			u = "https://api.openai.com/v1"
		}
		u = strings.TrimRight(u, "/") + "/models"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return false
		}
		if strings.TrimSpace(c.AI.APIKey) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.AI.APIKey))
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	default:
		base := strings.TrimRight(strings.TrimSpace(c.AI.Endpoint), "/")
		if base == "" {
			base = "http://127.0.0.1:11434"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/tags", nil)
		if err != nil {
			return false
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}
}
