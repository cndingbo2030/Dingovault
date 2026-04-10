package embeddings

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/bus"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/storage"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

const (
	embedDebounce   = 280 * time.Millisecond
	embedParallel   = 8
	maxEmbedChars   = 8000
	betweenEmbedGap = 12 * time.Millisecond
)

// Plugin indexes block embeddings after SQLite replace. Paths are debounced and coalesced;
// each file's blocks are embedded with a small parallel worker pool.
type Plugin struct {
	store storage.Provider
	llm   ai.LLMProvider

	mu      sync.Mutex
	pending map[string]struct{}
	timer   *time.Timer
}

// Register subscribes to after:block:indexed. llm may be nil (plugin does nothing).
func Register(b *bus.Bus, store storage.Provider, llm ai.LLMProvider) *Plugin {
	if b == nil || store == nil || llm == nil {
		return nil
	}
	if _, ok := store.(*storage.Store); !ok {
		return nil
	}
	p := &Plugin{
		store:   store,
		llm:     llm,
		pending: make(map[string]struct{}),
	}
	b.Subscribe(bus.TopicAfterBlockIndexed, p.onAfterBlockIndexed)
	return p
}

func (p *Plugin) onAfterBlockIndexed(ctx context.Context, payload any) {
	evt, ok := castPayload(payload)
	if !ok || strings.TrimSpace(evt.SourcePath) == "" {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		return
	}
	if cfg.AI.DisableEmbeddings {
		return
	}
	cfg.AI = config.NormalizeAISettings(cfg.AI)
	if strings.TrimSpace(cfg.AI.EmbeddingsModel) == "" {
		return
	}
	p.scheduleFlush(ctx, evt.SourcePath)
}

func (p *Plugin) scheduleFlush(ctx context.Context, path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == nil {
		p.pending = make(map[string]struct{})
	}
	p.pending[path] = struct{}{}
	if p.timer != nil {
		p.timer.Stop()
	}
	runCtx := ctx
	p.timer = time.AfterFunc(embedDebounce, func() {
		p.runFlush(runCtx)
	})
}

func (p *Plugin) runFlush(ctx context.Context) {
	p.mu.Lock()
	paths := make([]string, 0, len(p.pending))
	for k := range p.pending {
		paths = append(paths, k)
	}
	p.pending = make(map[string]struct{})
	p.timer = nil
	p.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return
	}
	if cfg.AI.DisableEmbeddings {
		return
	}
	cfg.AI = config.NormalizeAISettings(cfg.AI)
	if strings.TrimSpace(cfg.AI.EmbeddingsModel) == "" {
		return
	}
	for _, path := range paths {
		p.embedSourceParallel(ctx, path, cfg)
	}
}

func castPayload(v any) (bus.AfterBlockIndexedPayload, bool) {
	switch x := v.(type) {
	case bus.AfterBlockIndexedPayload:
		return x, true
	case *bus.AfterBlockIndexedPayload:
		if x == nil {
			return bus.AfterBlockIndexedPayload{}, false
		}
		return *x, true
	default:
		return bus.AfterBlockIndexedPayload{}, false
	}
}

type embedJob struct {
	blockID string
	text    string
}

func (p *Plugin) embedSourceParallel(ctx context.Context, abs string, cfg config.Config) {
	blocks, err := p.store.ListDomainBlocksBySourcePath(ctx, abs)
	if err != nil {
		log.Printf("embeddings: list blocks: %v", err)
		return
	}
	model := strings.TrimSpace(cfg.AI.EmbeddingsModel)
	uid := tenant.UserID(ctx)

	var jobs []embedJob
	for _, b := range blocks {
		text := strings.TrimSpace(b.Content)
		if len(text) > maxEmbedChars {
			text = text[:maxEmbedChars]
		}
		if text == "" {
			continue
		}
		jobs = append(jobs, embedJob{blockID: b.ID, text: text})
	}
	if len(jobs) == 0 {
		return
	}

	sem := make(chan struct{}, embedParallel)
	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			vec, err := p.llm.Embed(ctx, j.text)
			if err != nil {
				log.Printf("embeddings: embed block %s: %v", j.blockID, err)
				return
			}
			if err := p.store.UpsertBlockEmbedding(ctx, uid, j.blockID, model, vec); err != nil {
				log.Printf("embeddings: upsert %s: %v", j.blockID, err)
			}
			time.Sleep(betweenEmbedGap)
		}()
	}
	wg.Wait()
}
