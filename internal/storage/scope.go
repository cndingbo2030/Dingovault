package storage

import (
	"context"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/domain"
	"github.com/cndingbo2030/dingovault/internal/parser"
	"github.com/cndingbo2030/dingovault/internal/tenant"
)

const idSep = "\x1e" // ASCII record separator — unlikely in vault paths / hashes

func storeUserID(ctx context.Context) string {
	return tenant.UserID(ctx)
}

// physicalBlockID maps a logical block id as produced by the parser into the DB primary key for this tenant.
func physicalBlockID(ctx context.Context, logicalID string) string {
	if logicalID == "" {
		return ""
	}
	u := storeUserID(ctx)
	if u == tenant.LocalUserID {
		return logicalID
	}
	return u + idSep + logicalID
}

// logicalBlockID maps a DB block id back to the parser-facing id.
func logicalBlockID(ctx context.Context, physicalID string) string {
	if physicalID == "" {
		return ""
	}
	u := storeUserID(ctx)
	if u == tenant.LocalUserID {
		return physicalID
	}
	p := u + idSep
	if strings.HasPrefix(physicalID, p) {
		return strings.TrimPrefix(physicalID, p)
	}
	return physicalID
}

func decodeBlock(ctx context.Context, b domain.Block) domain.Block {
	b.ID = logicalBlockID(ctx, b.ID)
	b.ParentID = logicalBlockID(ctx, b.ParentID)
	return b
}

// scopeParseResult rewrites block, wikilink, and tag ids for multi-tenant storage (non-local users only).
func scopeParseResult(ctx context.Context, res parser.ParseResult) parser.ParseResult {
	if storeUserID(ctx) == tenant.LocalUserID {
		return res
	}
	out := res
	out.Blocks = append([]domain.Block(nil), res.Blocks...)
	for i := range out.Blocks {
		out.Blocks[i].ID = physicalBlockID(ctx, out.Blocks[i].ID)
		out.Blocks[i].ParentID = physicalBlockID(ctx, out.Blocks[i].ParentID)
	}
	out.Wikilinks = append([]parser.WikilinkRef(nil), res.Wikilinks...)
	for i := range out.Wikilinks {
		out.Wikilinks[i].SourceBlockID = physicalBlockID(ctx, out.Wikilinks[i].SourceBlockID)
	}
	out.Tags = append([]parser.TagRef(nil), res.Tags...)
	for i := range out.Tags {
		out.Tags[i].BlockID = physicalBlockID(ctx, out.Tags[i].BlockID)
	}
	return out
}
