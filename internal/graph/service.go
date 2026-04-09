package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dingbo/dingovault/internal/bus"
	"github.com/dingbo/dingovault/internal/parser"
	"github.com/dingbo/dingovault/internal/storage"
)

// Service applies parse results to a storage Provider (blocks + link/tag edges).
type Service struct {
	store  storage.Provider
	engine *parser.Engine
	bus    *bus.Bus
}

// NewService wires storage and a shared parser engine.
func NewService(store storage.Provider, engine *parser.Engine) *Service {
	return &Service{store: store, engine: engine}
}

// SetBus attaches an optional event bus (plugins, sync hooks).
func (s *Service) SetBus(b *bus.Bus) {
	s.bus = b
}

func (s *Service) publish(ctx context.Context, topic string, payload any) {
	if s.bus != nil {
		s.bus.Publish(ctx, topic, payload)
	}
}

// ReindexFile reads path (read-only), parses Markdown, then replaces all blocks (and derived
// edges) for that source_path inside a single transaction.
func (s *Service) ReindexFile(ctx context.Context, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return s.ReindexMarkdownBytes(ctx, abs, src)
}

// ReindexMarkdownBytes parses in-memory markdown for an absolute vault path and applies it to the store
// (same indexing rules as ReindexFile). Used by the SaaS HTTP API and RemoteStore sync.
func (s *Service) ReindexMarkdownBytes(ctx context.Context, absPath string, src []byte) error {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}
	body := src
	var pageProps map[string]string
	var aliases []string
	if yml, b, ok := parser.SplitFrontmatter(src); ok {
		var perr error
		pageProps, aliases, perr = parser.ParseFrontmatterYAML(yml)
		if perr != nil {
			return fmt.Errorf("frontmatter: %w", perr)
		}
		body = b
	}
	res, err := s.engine.ParseSource(body, abs)
	if err != nil {
		return fmt.Errorf("parse markdown: %w", err)
	}
	return s.applyParseResult(ctx, abs, res, pageProps, aliases)
}

// ReindexBlocks upserts using an in-memory parse result (e.g. tests).
func (s *Service) ReindexBlocks(ctx context.Context, sourcePath string, res parser.ParseResult) error {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}
	for i := range res.Blocks {
		res.Blocks[i].Metadata.SourcePath = abs
	}
	return s.applyParseResult(ctx, abs, res, nil, nil)
}

func (s *Service) applyParseResult(ctx context.Context, abs string, res parser.ParseResult, pageProps map[string]string, aliases []string) error {
	if err := s.store.ReplaceIndexedSource(ctx, abs, res, pageProps, aliases); err != nil {
		return err
	}
	s.publish(ctx, bus.TopicAfterBlockIndexed, bus.AfterBlockIndexedPayload{
		SourcePath: abs,
		BlockCount: len(res.Blocks),
	})
	s.publish(ctx, bus.TopicFileReindexed, bus.FileReindexedPayload{Path: abs})
	return nil
}

// DeleteFile removes blocks, page metadata, and aliases for a path.
func (s *Service) DeleteFile(ctx context.Context, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}
	return s.store.DeleteIndexedSource(ctx, abs)
}
