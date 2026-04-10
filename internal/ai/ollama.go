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

type ollamaProvider struct {
	cfg    config.AISettings
	client *http.Client
}

func newOllama(s config.AISettings) *ollamaProvider {
	base := strings.TrimRight(strings.TrimSpace(s.Endpoint), "/")
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	return &ollamaProvider{
		cfg: s,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (o *ollamaProvider) base() string {
	b := strings.TrimRight(strings.TrimSpace(o.cfg.Endpoint), "/")
	if b == "" {
		return "http://127.0.0.1:11434"
	}
	return b
}

func (o *ollamaProvider) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	model := strings.TrimSpace(o.cfg.Model)
	if model == "" {
		model = "llama3.2"
	}
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	if o.cfg.Temperature > 0 {
		body["options"] = map[string]any{"temperature": o.cfg.Temperature}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
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
		return "", fmt.Errorf("ollama chat: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("ollama chat decode: %w", err)
	}
	return strings.TrimSpace(out.Message.Content), nil
}

func (o *ollamaProvider) StreamComplete(ctx context.Context, messages []ChatMessage, onChunk func(string) error) error {
	model := strings.TrimSpace(o.cfg.Model)
	if model == "" {
		model = "llama3.2"
	}
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}
	if o.cfg.Temperature > 0 {
		body["options"] = map[string]any{"temperature": o.cfg.Temperature}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("ollama stream: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	br := bufio.NewReader(resp.Body)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := br.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		s := strings.TrimSpace(string(line))
		done, err := ollamaHandleStreamJSONLine(s, onChunk)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func ollamaHandleStreamJSONLine(s string, onChunk func(string) error) (done bool, err error) {
	if s == "" {
		return false, nil
	}
	var ev struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done  bool   `json:"done"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(s), &ev); err != nil {
		return false, nil
	}
	if ev.Error != "" {
		return false, fmt.Errorf("ollama: %s", ev.Error)
	}
	if ev.Message.Content != "" {
		if err := onChunk(ev.Message.Content); err != nil {
			return false, err
		}
	}
	if ev.Done {
		return true, nil
	}
	return false, nil
}

func (o *ollamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	model := strings.TrimSpace(o.cfg.EmbeddingsModel)
	if model == "" {
		model = "nomic-embed-text"
	}
	body, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": text,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("ollama embeddings: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("ollama embeddings decode: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embeddings: empty vector")
	}
	v := make([]float32, len(out.Embedding))
	for i, x := range out.Embedding {
		v[i] = float32(x)
	}
	return v, nil
}
