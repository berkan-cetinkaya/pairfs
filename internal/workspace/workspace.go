package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe path")

type Workspace struct {
	root string
}

type FileEntry struct {
	Path string
	Size int64
}

// New creates a Workspace rooted at an existing directory and stores its canonical absolute path.
func New(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve root symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory")
	}
	return &Workspace{root: filepath.Clean(resolved)}, nil
}

// Root returns the canonical absolute path of the workspace root.
func (w *Workspace) Root() string { return w.root }

// ListFiles returns sorted metadata for regular, non-symlink files in the workspace.
func (w *Workspace) ListFiles(ctx context.Context) ([]FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var files []FileEntry
	err := filepath.WalkDir(w.root, func(full string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if full != w.root && (d.Name() == ".git" || d.Name() == ".pairfs" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(w.root, full)
		if err != nil {
			return err
		}
		files = append(files, FileEntry{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Resolve validates a workspace-relative path and returns its absolute path.
// It rejects empty, absolute, parent-escaping, and symlink-containing paths.
// Missing path components are allowed so callers can safely prepare create operations.
func (w *Workspace) Resolve(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", ErrUnsafePath
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	full := filepath.Join(w.root, clean)
	current := w.root
	parts := strings.Split(clean, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return full, nil
		}
		if err != nil {
			return "", fmt.Errorf("inspect path component %q: %w", filepath.ToSlash(filepath.Join(parts[:i+1]...)), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: symlink component %q", ErrUnsafePath, filepath.ToSlash(filepath.Join(parts[:i+1]...)))
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("path component %q is not a directory", filepath.ToSlash(filepath.Join(parts[:i+1]...)))
		}
	}
	return full, nil
}

// Read returns the contents and mode of a regular file inside the workspace.
func (w *Workspace) Read(rel string) ([]byte, fs.FileMode, error) {
	return w.ReadContext(context.Background(), rel)
}

// ReadContext returns the exact contents and mode of a regular workspace file.
func (w *Workspace) ReadContext(ctx context.Context, rel string) ([]byte, fs.FileMode, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	full, err := w.Resolve(rel)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file")
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	data := make([]byte, 0, info.Size())
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		n, readErr := file.Read(buf)
		data = append(data, buf[:n]...)
		if errors.Is(readErr, io.EOF) {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			return data, info.Mode(), nil
		}
		if readErr != nil {
			return nil, 0, readErr
		}
	}
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
	full, err = w.Resolve(rel)
	if err != nil {
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
