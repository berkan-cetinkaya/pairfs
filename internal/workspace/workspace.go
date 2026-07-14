package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe path")

type Workspace struct {
	root string
}

// New creates a Workspace rooted at an existing directory and stores its absolute path.
func New(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory")
	}
	return &Workspace{root: abs}, nil
}

// Root returns the absolute path of the workspace root.
func (w *Workspace) Root() string { return w.root }

// Resolve validates a workspace-relative path and returns its absolute path.
// It rejects empty, absolute, parent-escaping, and symlink-parent-escaping paths.
func (w *Workspace) Resolve(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", ErrUnsafePath
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	full := filepath.Join(w.root, clean)
	resolvedRoot, err := filepath.EvalSymlinks(w.root)
	if err != nil {
		return "", fmt.Errorf("resolve root symlinks: %w", err)
	}

	parent := filepath.Dir(full)
	resolvedParent := parent
	if p, err := filepath.EvalSymlinks(parent); err == nil {
		resolvedParent = p
	}
	relToRoot, err := filepath.Rel(resolvedRoot, resolvedParent)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	return full, nil
}

// Read returns the contents and mode of a regular file inside the workspace.
func (w *Workspace) Read(rel string) ([]byte, fs.FileMode, error) {
	full, err := w.Resolve(rel)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file")
	}
	data, err := os.ReadFile(full)
	return data, info.Mode(), err
}

// Hash returns the lowercase hexadecimal SHA-256 digest of a workspace file.
func (w *Workspace) Hash(rel string) (string, error) {
	data, _, err := w.Read(rel)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// AtomicWrite replaces a workspace file by syncing and renaming a temporary sibling file.
// A zero mode defaults to 0644, and missing parent directories are created with mode 0755.
func (w *Workspace) AtomicWrite(rel string, data []byte, mode fs.FileMode) error {
	full, err := w.Resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".pairfs-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if mode == 0 {
		mode = 0o644
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, full)
}
