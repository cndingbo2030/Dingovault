package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dingbo/dingovault/internal/bus"
)

// UpdateBlock replaces only the source lines for the given block and re-indexes the file.
// Child blocks live on separate lines and are not part of [LineStart, LineEnd], so they are preserved.
func (s *Service) UpdateBlock(ctx context.Context, blockID, newContent string) error {
	b, err := s.store.GetBlockByID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("lookup block: %w", err)
	}
	path := b.Metadata.SourcePath
	if s.bus != nil {
		nc, err := s.bus.BeforeBlockSave(ctx, bus.BeforeBlockSaveData{
			BlockID:    blockID,
			SourcePath: path,
			Content:    newContent,
		})
		if err != nil {
			return fmt.Errorf("before:block:save hook: %w", err)
		}
		newContent = nc
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	out, err := ReplaceBlockLineRange(data, b.Metadata.LineStart, b.Metadata.LineEnd, newContent)
	if err != nil {
		return fmt.Errorf("replace lines: %w", err)
	}
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	if err := s.ReindexFile(ctx, path); err != nil {
		return err
	}
	s.publish(ctx, bus.TopicBlockUpdated, bus.BlockUpdatedPayload{BlockID: blockID, Path: path})
	return nil
}

// InsertBlockAfter inserts a new Markdown line after the block's last line (sibling list item / paragraph).
func (s *Service) InsertBlockAfter(ctx context.Context, blockID, initialText string) error {
	b, err := s.store.GetBlockByID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("lookup block: %w", err)
	}
	path := b.Metadata.SourcePath
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines, _, _, err := splitFileLines(data)
	if err != nil {
		return err
	}
	ls, le := b.Metadata.LineStart, b.Metadata.LineEnd
	if ls < 1 || le > len(lines) || le < ls {
		return fmt.Errorf("invalid stored line range %d-%d", ls, le)
	}
	anchor := lines[ls-1]
	newLine := NewSiblingLine(anchor, initialText)

	out, err := InsertLinesAfter(data, le, []string{newLine})
	if err != nil {
		return fmt.Errorf("insert lines: %w", err)
	}
	if err := AtomicWriteFile(path, out); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	return s.ReindexFile(ctx, path)
}

// EnsurePage creates an empty Markdown page with a title heading if the file does not exist.
func (s *Service) EnsurePage(ctx context.Context, absPath string) error {
	absPath = filepath.Clean(absPath)
	if _, statErr := os.Stat(absPath); statErr == nil {
		return nil
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("stat: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	base := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	body := "# " + base + "\n\n"
	if err := AtomicWriteFile(absPath, []byte(body)); err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	return s.ReindexFile(ctx, absPath)
}

// ResolveVaultPath joins a user path (absolute or vault-relative) against notesRoot and rejects ".." escape.
func ResolveVaultPath(notesRoot, userPath string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(notesRoot))
	if err != nil {
		return "", fmt.Errorf("notes root: %w", err)
	}
	p := strings.TrimSpace(userPath)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	var joined string
	if filepath.IsAbs(p) {
		joined = filepath.Clean(p)
	} else {
		joined = filepath.Clean(filepath.Join(root, p))
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rootWithSep := root + string(filepath.Separator)
	if absJoined != root && !strings.HasPrefix(absJoined+string(filepath.Separator), rootWithSep) {
		return "", fmt.Errorf("path escapes vault root")
	}
	return absJoined, nil
}

// WikilinkToAbsPath maps a [[target]] title to a .md path inside the vault.
func WikilinkToAbsPath(notesRoot, target string) (string, error) {
	t := strings.TrimSpace(target)
	t = strings.ReplaceAll(t, "\\", "/")
	if strings.HasSuffix(strings.ToLower(t), ".md") {
		return ResolveVaultPath(notesRoot, t)
	}
	t = strings.TrimSuffix(t, "/")
	if strings.Contains(t, "/") {
		return ResolveVaultPath(notesRoot, t+".md")
	}
	return ResolveVaultPath(notesRoot, t+".md")
}
