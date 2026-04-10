// Package vaultsync mirrors Markdown vaults to WebDAV and S3 remotes using timestamp + size rules.
package vaultsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	webdav "github.com/studio-b12/gowebdav"
)

// WebDAVConfig is the remote endpoint and optional path prefix under the WebDAV root URL.
type WebDAVConfig struct {
	URL      string
	User     string
	Password string
	// RemoteRoot is a path segment under the WebDAV base (no leading slash), e.g. "notes/vault".
	RemoteRoot string
}

// SyncMarkdownVault performs a bidirectional sync of all .md files under localRoot.
// When local and remote both differ in modification time and size, the local file is copied to
// stem.conflict.ext and the remote content replaces the local file.
func SyncMarkdownVault(ctx context.Context, localRoot string, cfg WebDAVConfig) error {
	localRoot = filepath.Clean(localRoot)
	if localRoot == "" {
		return fmt.Errorf("empty local root")
	}
	client, err := dialWebDAV(cfg)
	if err != nil {
		return err
	}
	localFiles, err := listLocalMarkdown(localRoot)
	if err != nil {
		return err
	}
	remoteFiles, err := listRemoteMarkdown(ctx, client, cfg.RemoteRoot)
	if err != nil {
		return fmt.Errorf("list remote: %w", err)
	}
	for rel := range mergeRelKeys(localFiles, remoteFiles) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncOneMarkdown(ctx, client, cfg, localRoot, localFiles, remoteFiles, rel); err != nil {
			return err
		}
	}
	return nil
}

func dialWebDAV(cfg WebDAVConfig) (*webdav.Client, error) {
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return nil, fmt.Errorf("empty WebDAV URL")
	}
	client := webdav.NewClient(cfg.URL, cfg.User, cfg.Password)
	client.SetTimeout(120 * time.Second)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("webdav connect: %w", err)
	}
	return client, nil
}

func syncOneMarkdown(ctx context.Context, client *webdav.Client, cfg WebDAVConfig, localRoot string, localFiles map[string]localFile, remoteFiles map[string]remoteFile, rel string) error {
	var lp, rp *fileSnapshot
	if v, ok := localFiles[rel]; ok {
		s := v.snap
		lp = &s
	}
	if v, ok := remoteFiles[rel]; ok {
		s := v.snap
		rp = &s
	}
	switch classifySync(lp, rp) {
	case syncSkip:
		return nil
	case syncPush:
		loc := localFiles[rel]
		if err := pushLocalToRemote(ctx, client, cfg.RemoteRoot, rel, loc.abs); err != nil {
			return fmt.Errorf("push %s: %w", rel, err)
		}
	case syncPull:
		rem := remoteFiles[rel]
		if err := pullRemoteToLocal(ctx, client, cfg.RemoteRoot, rel, rem.path, localRoot); err != nil {
			return fmt.Errorf("pull %s: %w", rel, err)
		}
	case syncConflict:
		loc := localFiles[rel]
		rem := remoteFiles[rel]
		if err := resolveConflict(ctx, client, cfg.RemoteRoot, rel, loc.abs, rem.path, localRoot); err != nil {
			return fmt.Errorf("conflict %s: %w", rel, err)
		}
	default:
		return fmt.Errorf("internal: unknown sync action for %s", rel)
	}
	return nil
}

func listRemoteMarkdown(ctx context.Context, c *webdav.Client, remoteRoot string) (map[string]remoteFile, error) {
	out := make(map[string]remoteFile)
	root := strings.Trim(strings.TrimSpace(remoteRoot), "/")
	listPath := "/"
	if root != "" {
		listPath = "/" + root + "/"
	}
	err := walkRemoteMD(ctx, c, listPath, root, out)
	return out, err
}

func walkRemoteMD(ctx context.Context, c *webdav.Client, dirPath string, remoteRoot string, acc map[string]remoteFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := c.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ingestRemoteEntry(ctx, c, remoteRoot, acc, e); err != nil {
			return err
		}
	}
	return nil
}

func ingestRemoteEntry(ctx context.Context, c *webdav.Client, remoteRoot string, acc map[string]remoteFile, e os.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fv, ok := e.(webdav.File)
	if !ok {
		return nil
	}
	name := fv.Name()
	if name == "." || name == ".." {
		return nil
	}
	full := strings.TrimSuffix(fv.Path(), "/")
	if fv.IsDir() {
		return descendRemoteDir(ctx, c, fv, remoteRoot, acc)
	}
	if !strings.EqualFold(pathpkg.Ext(name), ".md") {
		return nil
	}
	fi, err := c.Stat(full)
	if err != nil {
		return nil
	}
	rel := remoteToRel(full, remoteRoot)
	if rel == "" {
		return nil
	}
	acc[rel] = remoteFile{
		path: full,
		snap: fileSnapshot{modTime: fi.ModTime(), size: fi.Size()},
	}
	return nil
}

func descendRemoteDir(ctx context.Context, c *webdav.Client, fv webdav.File, remoteRoot string, acc map[string]remoteFile) error {
	name := fv.Name()
	if strings.HasPrefix(name, ".") {
		return nil
	}
	sub := fv.Path()
	if !strings.HasSuffix(sub, "/") {
		sub += "/"
	}
	return walkRemoteMD(ctx, c, sub, remoteRoot, acc)
}

func remoteToRel(davPath, remoteRoot string) string {
	p := strings.TrimPrefix(strings.TrimSuffix(davPath, "/"), "/")
	r := strings.Trim(strings.TrimSpace(remoteRoot), "/")
	if r == "" {
		return p
	}
	prefix := r + "/"
	if strings.HasPrefix(p, prefix) {
		return strings.TrimPrefix(p, prefix)
	}
	if p == r {
		return ""
	}
	return p
}

func remoteJoin(remoteRoot, rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	r := strings.Trim(strings.TrimSpace(remoteRoot), "/")
	switch {
	case r == "" && rel == "":
		return "/"
	case r == "":
		return "/" + rel
	case rel == "":
		return "/" + r
	default:
		return "/" + r + "/" + rel
	}
}

func pushLocalToRemote(ctx context.Context, c *webdav.Client, remoteRoot, rel, localAbs string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(localAbs)
	if err != nil {
		return err
	}
	rpath := remoteJoin(remoteRoot, rel)
	if err := ensureRemoteDir(c, rpath); err != nil {
		return err
	}
	return c.WriteStream(rpath, bytes.NewReader(data), 0o644)
}

func ensureRemoteDir(c *webdav.Client, filePath string) error {
	dir := pathpkg.Dir(filePath)
	dir = strings.TrimPrefix(dir, "/")
	if dir == "." || dir == "" {
		return nil
	}
	parts := strings.Split(dir, "/")
	acc := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		acc += "/" + p
		_ = c.Mkdir(acc, 0o755)
	}
	return nil
}

func pullRemoteToLocal(ctx context.Context, c *webdav.Client, remoteRoot, rel, remotePath, localRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rc, err := c.ReadStream(remotePath)
	if err != nil {
		return err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	localAbs := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(localAbs), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(localAbs, data)
}

func resolveConflict(ctx context.Context, c *webdav.Client, remoteRoot, rel, localAbs, remotePath, localRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	localData, err := os.ReadFile(localAbs)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	conflictAbs := conflictSiblingPath(localAbs)
	if len(localData) > 0 {
		if err := atomicWriteFile(conflictAbs, localData); err != nil {
			return err
		}
	}
	rc, err := c.ReadStream(remotePath)
	if err != nil {
		return err
	}
	defer rc.Close()
	remoteData, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	dest := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(dest, remoteData)
}
