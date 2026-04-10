package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cndingbo2030/dingovault/internal/config"
)

type openAIProvider struct {
	cfg    config.AISettings
	client *http.Client
}

func newOpenAI(s config.AISettings) *openAIProvider {
	base := strings.TrimRight(strings.TrimSpace(s.Endpoint), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &openAIProvider{
		cfg: s,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (o *openAIProvider) base() string {
	b := strings.TrimRight(strings.TrimSpace(o.cfg.Endpoint), "/")
	if b == "" {
		return "https://api.openai.com/v1"
	}
	return b
}

func (o *openAIProvider) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	model := strings.TrimSpace(o.cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": o.cfg.Temperature,
	}
	if o.cfg.Temperature <= 0 {
		body["temperature"] = 0.7
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(o.cfg.APIKey))
	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai chat: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("openai chat decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai chat: no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (o *openAIProvider) StreamComplete(ctx context.Context, messages []ChatMessage, onChunk func(string) error) error {
	model := strings.TrimSpace(o.cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	temp := o.cfg.Temperature
	if temp <= 0 {
		temp = 0.7
	}
	body, err := json.Marshal(map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": temp,
		"stream":      true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(o.cfg.APIKey))
	req.Header.Set("Accept", "text/event-stream")
	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("openai stream: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	sc := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	return scanOpenAIChatStream(ctx, sc, onChunk)
}

func scanOpenAIChatStream(ctx context.Context, sc *bufio.Scanner, onChunk func(string) error) error {
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		done, err := openAIHandleSSEDataPayload(data, onChunk)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	return sc.Err()
}

func openAIHandleSSEDataPayload(data string, onChunk func(string) error) (done bool, err error) {
	if data == "[DONE]" {
		return true, nil
	}
	var ev struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return false, nil
	}
	if ev.Error != nil && ev.Error.Message != "" {
		return false, fmt.Errorf("openai: %s", ev.Error.Message)
	}
	if len(ev.Choices) == 0 {
		return false, nil
	}
	c := ev.Choices[0].Delta.Content
	if c == "" {
		return false, nil
	}
	if err := onChunk(c); err != nil {
		return false, err
	}
	return false, nil
}

func (o *openAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	model := strings.TrimSpace(o.cfg.EmbeddingsModel)
	if model == "" {
		model = "text-embedding-3-small"
	}
	body, err := json.Marshal(map[string]any{
		"model": model,
		"input": text,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(o.cfg.APIKey))
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai embeddings: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("openai embeddings decode: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embeddings: empty vector")
	}
	e := out.Data[0].Embedding
	v := make([]float32, len(e))
	for i, x := range e {
		v[i] = float32(x)
	}
	return v, nil
}
