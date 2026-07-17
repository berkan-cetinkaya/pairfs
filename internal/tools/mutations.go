package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

var (
	ErrMatchNotFound  = errors.New("old text not found")
	ErrAmbiguousMatch = errors.New("old text matched more than once")
)

// PreviewEdit prepares an exact, unique text replacement without modifying the file.
// It returns the preview, resulting content, and original file mode for a later apply.
func PreviewEdit(ws *workspace.Workspace, path, oldText, newText string) (Preview, string, os.FileMode, error) {
	data, mode, err := ws.Read(path)
	if err != nil {
		return Preview{}, "", 0, err
	}
	current := string(data)
	count := strings.Count(current, oldText)
	if count == 0 {
		return Preview{}, "", 0, ErrMatchNotFound
	}
	if count > 1 {
		return Preview{}, "", 0, ErrAmbiguousMatch
	}
	next := strings.Replace(current, oldText, newText, 1)
	hash, _ := ws.Hash(path)
	return Preview{Operation: "edit", Path: path, Diff: UnifiedDiff(path, current, next), BeforeHash: hash, CanApply: true}, next, mode, nil
}

// ApplyEdit performs an exact, unique text replacement using an atomic file write.
// When expectedHash is non-empty, a changed source returns a stale result without writing.
func ApplyEdit(ws *workspace.Workspace, path, oldText, newText, expectedHash string) (Result, error) {
	if expectedHash != "" {
		h, err := ws.Hash(path)
		if err != nil {
			return Result{}, err
		}
		if h != expectedHash {
			return Result{Status: "stale", Operation: "edit", Path: path, Message: "file changed after preview"}, nil
		}
	}
	_, next, mode, err := PreviewEdit(ws, path, oldText, newText)
	if err != nil {
		return Result{}, err
	}
	if err := ws.AtomicWrite(path, []byte(next), mode); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "edit", Path: path}, nil
}

// PreviewWrite validates a create or overwrite operation without modifying the workspace.
// It returns the mode that ApplyWrite should preserve or use for the destination.
func PreviewWrite(ws *workspace.Workspace, path, content, mode string) (Preview, os.FileMode, error) {
	full, err := ws.Resolve(path)
	if err != nil {
		return Preview{}, 0, err
	}
	_, statErr := os.Lstat(full)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return Preview{}, 0, statErr
	}
	switch mode {
	case "create":
		if statErr == nil {
			return Preview{}, 0, fmt.Errorf("file already exists")
		}
		return Preview{Operation: "write", Path: path, Diff: UnifiedDiff(path, "", content), CanApply: true}, 0o644, nil
	case "overwrite":
		if statErr != nil {
			return Preview{}, 0, fmt.Errorf("file does not exist")
		}
		data, fileMode, err := ws.Read(path)
		if err != nil {
			return Preview{}, 0, err
		}
		hash, _ := ws.Hash(path)
		return Preview{Operation: "write", Path: path, Diff: UnifiedDiff(path, string(data), content), BeforeHash: hash, CanApply: true}, fileMode, nil
	default:
		return Preview{}, 0, fmt.Errorf("mode must be create or overwrite")
	}
}

// ApplyWrite creates or atomically overwrites a workspace file after validating the operation.
// For overwrite mode, a non-empty expectedHash detects changes made after preview.
func ApplyWrite(ws *workspace.Workspace, path, content, mode, expectedHash string) (Result, error) {
	if mode == "overwrite" && expectedHash != "" {
		h, err := ws.Hash(path)
		if err != nil {
			return Result{}, err
		}
		if h != expectedHash {
			return Result{Status: "stale", Operation: "write", Path: path, Message: "file changed after preview"}, nil
		}
	}
	_, fileMode, err := PreviewWrite(ws, path, content, mode)
	if err != nil {
		return Result{}, err
	}
	if err := ws.AtomicWrite(path, []byte(content), fileMode); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "write", Path: path}, nil
}

// PreviewDelete returns the deletion diff and current hash without modifying the file.
func PreviewDelete(ws *workspace.Workspace, path string) (Preview, error) {
	data, _, err := ws.Read(path)
	if err != nil {
		return Preview{}, err
	}
	hash, _ := ws.Hash(path)
	return Preview{Operation: "delete", Path: path, Diff: UnifiedDiff(path, string(data), ""), BeforeHash: hash, CanApply: true}, nil
}

// ApplyDelete moves a workspace file into .pairfs/trash instead of removing it permanently.
// When expectedHash is non-empty, a changed source returns a stale result without moving it.
// An existing trash target is preserved and causes the operation to fail.
func ApplyDelete(ws *workspace.Workspace, path, expectedHash string) (Result, error) {
	if expectedHash != "" {
		h, err := ws.Hash(path)
		if err != nil {
			return Result{}, err
		}
		if h != expectedHash {
			return Result{Status: "stale", Operation: "delete", Path: path, Message: "file changed after preview"}, nil
		}
	}
	full, err := ws.Resolve(path)
	if err != nil {
		return Result{}, err
	}
	trashRel := filepath.Join(".pairfs", "trash", filepath.Clean(path))
	trash, err := ws.Resolve(trashRel)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return Result{}, err
	}
	full, err = ws.Resolve(path)
	if err != nil {
		return Result{}, err
	}
	trash, err = ws.Resolve(trashRel)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Lstat(trash); err == nil {
		return Result{}, fmt.Errorf("trash target already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	if err := os.Rename(full, trash); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "delete", Path: path, Message: "moved to .pairfs/trash"}, nil
}

// PreviewMove validates a file move and returns a rename preview without changing the workspace.
func PreviewMove(ws *workspace.Workspace, from, to string) (Preview, error) {
	data, _, err := ws.Read(from)
	if err != nil {
		return Preview{}, err
	}
	if _, err := ws.Resolve(to); err != nil {
		return Preview{}, err
	}
	toFull, _ := ws.Resolve(to)
	if _, err := os.Lstat(toFull); err == nil {
		return Preview{}, fmt.Errorf("target already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Preview{}, err
	}
	hash, _ := ws.Hash(from)
	diff := fmt.Sprintf("rename from %s\nrename to %s\n", from, to)
	return Preview{Operation: "move", From: from, To: to, Diff: diff, BeforeHash: hash, CanApply: true, Message: fmt.Sprintf("%d bytes", len(data))}, nil
}

// ApplyMove renames a workspace file after validating that the destination is available.
// When expectedHash is non-empty, a changed source returns a stale result without moving it.
func ApplyMove(ws *workspace.Workspace, from, to, expectedHash string) (Result, error) {
	if expectedHash != "" {
		h, err := ws.Hash(from)
		if err != nil {
			return Result{}, err
		}
		if h != expectedHash {
			return Result{Status: "stale", Operation: "move", From: from, To: to, Message: "file changed after preview"}, nil
		}
	}
	_, err := PreviewMove(ws, from, to)
	if err != nil {
		return Result{}, err
	}
	fromFull, _ := ws.Resolve(from)
	toFull, _ := ws.Resolve(to)
	if err := os.MkdirAll(filepath.Dir(toFull), 0o755); err != nil {
		return Result{}, err
	}
	fromFull, err = ws.Resolve(from)
	if err != nil {
		return Result{}, err
	}
	toFull, err = ws.Resolve(to)
	if err != nil {
		return Result{}, err
	}
	if err := os.Rename(fromFull, toFull); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "move", From: from, To: to}, nil
}
