package onboarding

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const DemoBundleVersion = "1"

// EnsureDemoVaultFromFS extracts embedded demo-vault files into the user cache directory when
// missing or when the bundle version changed. rootInFS is the top-level folder name inside the embed FS (e.g. "demo-vault").
func EnsureDemoVaultFromFS(fsys fs.FS, rootInFS string) (absDest string, err error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(base, "dingovault", "demo-vault")
	verPath := filepath.Join(dest, ".bundle-version")

	if shouldReuse(dest, verPath) {
		return dest, nil
	}

	if err := os.RemoveAll(dest); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("clear demo cache: %w", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}

	err = fs.WalkDir(fsys, rootInFS, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, ok := strings.CutPrefix(path, rootInFS+"/")
		if !ok {
			if path == rootInFS {
				return nil
			}
			return fmt.Errorf("unexpected embed path %q", path)
		}
		if rel == "" {
			return nil
		}
		out := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, rerr := fs.ReadFile(fsys, path)
		if rerr != nil {
			return rerr
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	if err := os.WriteFile(verPath, []byte(DemoBundleVersion+"\n"), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func shouldReuse(dest, verPath string) bool {
	data, err := os.ReadFile(verPath)
	if err != nil || strings.TrimSpace(string(data)) != DemoBundleVersion {
		return false
	}
	readme := filepath.Join(dest, "README.md")
	st, err := os.Stat(readme)
	return err == nil && !st.IsDir()
}

// DemoVaultRootName is the top-level directory name inside the embedded FS.
const DemoVaultRootName = "demo-vault"
