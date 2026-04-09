package summarizer

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/dingbo/dingovault/internal/bus"
	"github.com/dingbo/dingovault/internal/domain"
	"github.com/dingbo/dingovault/internal/parser"
	"github.com/dingbo/dingovault/internal/storage"
)

const (
	triggerTag    = "#summarize"
	summaryMarker = "<!-- dingovault:summarizer -->"
)

// Plugin listens to after:block:indexed and appends a generated child summary for
// blocks containing #summarize. It is a reference implementation for Go plugins.
type Plugin struct {
	store  storage.Provider
	engine *parser.Engine

	mu     sync.Mutex
	active map[string]bool
}

// Register subscribes the summarizer plugin to bus.TopicAfterBlockIndexed.
func Register(b *bus.Bus, store storage.Provider, engine *parser.Engine) *Plugin {
	if b == nil || store == nil || engine == nil {
		return nil
	}
	p := &Plugin{
		store:  store,
		engine: engine,
		active: map[string]bool{},
	}
	b.Subscribe(bus.TopicAfterBlockIndexed, p.onAfterBlockIndexed)
	return p
}

func (p *Plugin) onAfterBlockIndexed(ctx context.Context, payload any) {
	evt, ok := castPayload(payload)
	if !ok || strings.TrimSpace(evt.SourcePath) == "" {
		return
	}
	if !p.enter(evt.SourcePath) {
		return
	}
	defer p.leave(evt.SourcePath)

	if err := p.handleSource(ctx, evt.SourcePath); err != nil {
		log.Printf("summarizer plugin: %v", err)
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

func (p *Plugin) enter(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active[path] {
		return false
	}
	p.active[path] = true
	return true
}

func (p *Plugin) leave(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, path)
}

func (p *Plugin) handleSource(ctx context.Context, sourcePath string) error {
	blocks, err := p.store.ListDomainBlocksBySourcePath(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("list source blocks: %w", err)
	}
	target, found := findTriggerBlock(blocks)
	if !found {
		return nil
	}
	if hasSummaryChild(blocks, target.ID) {
		return nil
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	sum := mockSummarize(target.Content)
	updated, ok := appendSummaryChild(raw, target.Metadata.LineEnd, sum)
	if !ok {
		return nil
	}
	if bytes.Equal(updated, raw) {
		return nil
	}
	if err := os.WriteFile(sourcePath, updated, 0o644); err != nil {
		return fmt.Errorf("write source file: %w", err)
	}
	return p.reindex(ctx, sourcePath, updated)
}

func findTriggerBlock(blocks []domain.Block) (domain.Block, bool) {
	for _, b := range blocks {
		if strings.Contains(strings.ToLower(b.Content), triggerTag) {
			return b, true
		}
	}
	return domain.Block{}, false
}

func hasSummaryChild(blocks []domain.Block, parentID string) bool {
	for _, b := range blocks {
		if b.ParentID != parentID {
			continue
		}
		if strings.Contains(b.Content, summaryMarker) || strings.HasPrefix(strings.TrimSpace(b.Content), "Summary:") {
			return true
		}
	}
	return false
}

func mockSummarize(content string) string {
	clean := strings.ReplaceAll(content, triggerTag, "")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "Summary: key point captured. " + summaryMarker
	}
	words := strings.Fields(clean)
	if len(words) > 14 {
		words = words[:14]
	}
	return "Summary: " + strings.Join(words, " ") + ". " + summaryMarker
}

func appendSummaryChild(src []byte, lineEnd int, summary string) ([]byte, bool) {
	if lineEnd <= 0 {
		return src, false
	}
	text := string(src)
	lines := strings.Split(text, "\n")
	if lineEnd > len(lines) {
		return src, false
	}
	parentLine := lines[lineEnd-1]
	indent := leadingIndent(parentLine)
	childLine := indent + "  - " + summary

	insertAt := lineEnd
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertAt]...)
	newLines = append(newLines, childLine)
	newLines = append(newLines, lines[insertAt:]...)

	out := strings.Join(newLines, "\n")
	return []byte(out), true
}

func leadingIndent(s string) string {
	n := 0
	for n < len(s) {
		if s[n] != ' ' && s[n] != '\t' {
			break
		}
		n++
	}
	return s[:n]
}

func (p *Plugin) reindex(ctx context.Context, sourcePath string, src []byte) error {
	body := src
	var pageProps map[string]string
	var aliases []string
	if yml, b, ok := parser.SplitFrontmatter(src); ok {
		props, als, err := parser.ParseFrontmatterYAML(yml)
		if err != nil {
			return fmt.Errorf("frontmatter parse: %w", err)
		}
		pageProps = props
		aliases = als
		body = b
	}
	res, err := p.engine.ParseSource(body, sourcePath)
	if err != nil {
		return fmt.Errorf("markdown parse: %w", err)
	}
	if err := p.store.ReplaceIndexedSource(ctx, sourcePath, res, pageProps, aliases); err != nil {
		return fmt.Errorf("replace indexed source: %w", err)
	}
	return nil
}
