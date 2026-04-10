package vaultsync

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const timeSkew = 2 * time.Second

type fileSnapshot struct {
	modTime time.Time
	size    int64
}

type localFile struct {
	abs  string
	snap fileSnapshot
}

// remoteFile describes one remote Markdown object (WebDAV path or S3 key).
type remoteFile struct {
	path string
	snap fileSnapshot
}

type syncAction int

const (
	syncSkip syncAction = iota
	syncPush
	syncPull
	syncConflict
)

func classifySync(local, remote *fileSnapshot) syncAction {
	switch {
	case local == nil && remote == nil:
		return syncSkip
	case local == nil && remote != nil:
		return syncPull
	case local != nil && remote == nil:
		return syncPush
	}
	if timesRoughlyEqual(local.modTime, remote.modTime) && local.size == remote.size {
		return syncSkip
	}
	timeDiffers := !timesRoughlyEqual(local.modTime, remote.modTime)
	sizeDiffers := local.size != remote.size
	if timeDiffers && sizeDiffers {
		return syncConflict
	}
	if local.modTime.After(remote.modTime) {
		return syncPush
	}
	if remote.modTime.After(local.modTime) {
		return syncPull
	}
	if sizeDiffers {
		return syncConflict
	}
	return syncSkip
}

func timesRoughlyEqual(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d <= timeSkew
}

func mergeRelKeys(local map[string]localFile, remote map[string]remoteFile) map[string]struct{} {
	all := make(map[string]struct{})
	for k := range local {
		all[k] = struct{}{}
	}
	for k := range remote {
		all[k] = struct{}{}
	}
	return all
}

func listLocalMarkdown(root string) (map[string]localFile, error) {
	out := make(map[string]localFile)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := filepath.Base(path)
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		fi, err := d.Info()
		if err != nil {
			return err
		}
		out[rel] = localFile{
			abs: path,
			snap: fileSnapshot{
				modTime: fi.ModTime(),
				size:    fi.Size(),
			},
		}
		return nil
	})
	return out, err
}

func conflictSiblingPath(abs string) string {
	ext := filepath.Ext(abs)
	base := strings.TrimSuffix(abs, ext)
	return base + ".conflict" + ext
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dv-sync-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
