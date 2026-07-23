// Package pairfs provides safe, read-only access to a local workspace.
package pairfs

import (
	"context"

	"github.com/berkan-cetinkaya/pairfs/internal/tools"
	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

var (
	// ErrMatchNotFound indicates that an edit's old text was not present.
	ErrMatchNotFound = tools.ErrMatchNotFound
	// ErrAmbiguousMatch indicates that an edit's old text occurred more than once.
	ErrAmbiguousMatch = tools.ErrAmbiguousMatch
)

// WriteMode controls whether a write creates a new file or replaces an existing one.
type WriteMode string

const (
	WriteCreate    WriteMode = "create"
	WriteOverwrite WriteMode = "overwrite"
)

// FileEntry describes a regular file using its workspace-relative path and size.
type FileEntry struct {
	Path string
	Size int64
}

// GrepMatch describes one matching source line.
type GrepMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Preview describes a mutation without applying it.
type Preview struct {
	Operation  string `json:"operation"`
	Path       string `json:"path,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Diff       string `json:"diff,omitempty"`
	BeforeHash string `json:"beforeHash,omitempty"`
	CanApply   bool   `json:"canApply"`
	Message    string `json:"message,omitempty"`
}

// Result describes the outcome of an applied mutation.
type Result struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	Path      string `json:"path,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Message   string `json:"message,omitempty"`
}

// FS provides read-only access to a canonicalized workspace.
type FS struct {
	workspace *workspace.Workspace
}

// Open validates root and opens it as a pairfs workspace.
func Open(root string) (*FS, error) {
	ws, err := workspace.New(root)
	if err != nil {
		return nil, err
	}
	return &FS{workspace: ws}, nil
}

// Root returns the canonical absolute workspace root.
func (fs *FS) Root() string {
	return fs.workspace.Root()
}

// ListFiles returns sorted metadata for regular files below the workspace root.
func (fs *FS) ListFiles(ctx context.Context) ([]FileEntry, error) {
	entries, err := fs.workspace.ListFiles(ctx)
	if err != nil {
		return nil, err
	}
	files := make([]FileEntry, len(entries))
	for i, entry := range entries {
		files[i] = FileEntry{Path: entry.Path, Size: entry.Size}
	}
	return files, nil
}

// ReadFile returns the exact raw bytes of a regular workspace-relative file.
func (fs *FS) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, _, err := fs.workspace.ReadContext(ctx, path)
	return data, err
}

// Read returns the CLI-compatible, line-numbered view of a file.
func (fs *FS) Read(path string, offset, limit int) (string, error) {
	return tools.ReadFile(fs.workspace, path, offset, limit)
}

// Grep searches workspace files for regular-expression matches.
func (fs *FS) Grep(pattern, include string, max int) ([]GrepMatch, error) {
	matches, err := tools.Grep(fs.workspace, pattern, include, max)
	if err != nil {
		return nil, err
	}
	out := make([]GrepMatch, len(matches))
	for i, match := range matches {
		out[i] = GrepMatch(match)
	}
	return out, nil
}

// Glob returns sorted workspace-relative paths matching pattern.
func (fs *FS) Glob(pattern string) ([]string, error) {
	return tools.Glob(fs.workspace, pattern)
}

// PreviewEdit prepares an exact unique text replacement without changing the file.
func (fs *FS) PreviewEdit(path, oldText, newText string) (Preview, error) {
	preview, _, _, err := tools.PreviewEdit(fs.workspace, path, oldText, newText)
	return previewFromInternal(preview), err
}

// ApplyEdit performs an atomic edit, optionally guarded by a preview hash.
func (fs *FS) ApplyEdit(path, oldText, newText, expectedHash string) (Result, error) {
	result, err := tools.ApplyEdit(fs.workspace, path, oldText, newText, expectedHash)
	return resultFromInternal(result), err
}

// PreviewWrite validates a create or overwrite without changing the workspace.
func (fs *FS) PreviewWrite(path, content string, mode WriteMode) (Preview, error) {
	preview, _, err := tools.PreviewWrite(fs.workspace, path, content, string(mode))
	return previewFromInternal(preview), err
}

// ApplyWrite creates or atomically overwrites a file.
func (fs *FS) ApplyWrite(path, content string, mode WriteMode, expectedHash string) (Result, error) {
	result, err := tools.ApplyWrite(fs.workspace, path, content, string(mode), expectedHash)
	return resultFromInternal(result), err
}

// PreviewDelete prepares a recoverable deletion without changing the workspace.
func (fs *FS) PreviewDelete(path string) (Preview, error) {
	preview, err := tools.PreviewDelete(fs.workspace, path)
	return previewFromInternal(preview), err
}

// ApplyDelete moves a file to pairfs trash, optionally guarded by a preview hash.
func (fs *FS) ApplyDelete(path, expectedHash string) (Result, error) {
	result, err := tools.ApplyDelete(fs.workspace, path, expectedHash)
	return resultFromInternal(result), err
}

// PreviewMove validates a file move without changing the workspace.
func (fs *FS) PreviewMove(from, to string) (Preview, error) {
	preview, err := tools.PreviewMove(fs.workspace, from, to)
	return previewFromInternal(preview), err
}

// ApplyMove renames a file, optionally guarded by a preview hash.
func (fs *FS) ApplyMove(from, to, expectedHash string) (Result, error) {
	result, err := tools.ApplyMove(fs.workspace, from, to, expectedHash)
	return resultFromInternal(result), err
}

func previewFromInternal(preview tools.Preview) Preview {
	return Preview(preview)
}

func resultFromInternal(result tools.Result) Result {
	return Result(result)
}
