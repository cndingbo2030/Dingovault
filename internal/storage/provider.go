package storage

import (
	"context"

	"github.com/dingbo/dingovault/internal/domain"
	"github.com/dingbo/dingovault/internal/parser"
)

// Provider abstracts vault index persistence so the local SQLite implementation can be swapped
// for a remote SaaS backend (PostgreSQL/REST) without changing graph or bridge call sites.
type Provider interface {
	Close() error

	// Block reads
	GetBlockByID(ctx context.Context, id string) (domain.Block, error)
	ListDomainBlocksBySourcePath(ctx context.Context, sourcePath string) ([]domain.Block, error)
	GetBlocksByIDs(ctx context.Context, ids []string) ([]domain.Block, error)
	ListSourcePathsByRecency(ctx context.Context, limit int) ([]string, error)

	// Search & query
	QueryBlocksByProperty(ctx context.Context, key, value string) ([]domain.Block, error)
	BlockIDsFromFTS(ctx context.Context, ftsMatch string, limit int) ([]string, error)
	BlocksWithWikilinksToTargets(ctx context.Context, targets []string) ([]domain.Block, error)
	SearchBlocksFTS(ctx context.Context, query string, limit int) ([]BlockSearchHit, error)
	SearchBlocksFTSWithAliases(ctx context.Context, query string, limit int) ([]BlockSearchHit, error)

	// Page metadata (YAML frontmatter / aliases)
	ResolveAliasToPath(ctx context.Context, notesRoot, target string) (abs string, ok bool, err error)
	ListSourcePathsByPageProperty(ctx context.Context, key, value string) ([]string, error)

	// Writes — replace all indexed data for one markdown file, or remove it.
	ReplaceIndexedSource(ctx context.Context, absSourcePath string, res parser.ParseResult, pageProps map[string]string, aliases []string) error
	DeleteIndexedSource(ctx context.Context, absSourcePath string) error
}

// Compile-time check: local SQLite store satisfies Provider.
var _ Provider = (*Store)(nil)

// RemoteStore satisfies Provider via the SaaS HTTP API (see remote.go).
var _ Provider = (*RemoteStore)(nil)
