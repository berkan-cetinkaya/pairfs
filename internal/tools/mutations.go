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

func PreviewWrite(ws *workspace.Workspace, path, content, mode string) (Preview, os.FileMode, error) {
	full, err := ws.Resolve(path)
	if err != nil {
		return Preview{}, 0, err
	}
	_, statErr := os.Stat(full)
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

func PreviewDelete(ws *workspace.Workspace, path string) (Preview, error) {
	data, _, err := ws.Read(path)
	if err != nil {
		return Preview{}, err
	}
	hash, _ := ws.Hash(path)
	return Preview{Operation: "delete", Path: path, Diff: UnifiedDiff(path, string(data), ""), BeforeHash: hash, CanApply: true}, nil
}

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
	trash := filepath.Join(ws.Root(), ".pairfs", "trash", path)
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return Result{}, err
	}
	_ = os.Remove(trash)
	if err := os.Rename(full, trash); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "delete", Path: path, Message: "moved to .pairfs/trash"}, nil
}

func PreviewMove(ws *workspace.Workspace, from, to string) (Preview, error) {
	data, _, err := ws.Read(from)
	if err != nil {
		return Preview{}, err
	}
	if _, err := ws.Resolve(to); err != nil {
		return Preview{}, err
	}
	toFull, _ := ws.Resolve(to)
	if _, err := os.Stat(toFull); err == nil {
		return Preview{}, fmt.Errorf("target already exists")
	}
	hash, _ := ws.Hash(from)
	diff := fmt.Sprintf("rename from %s\nrename to %s\n", from, to)
	return Preview{Operation: "move", From: from, To: to, Diff: diff, BeforeHash: hash, CanApply: true, Message: fmt.Sprintf("%d bytes", len(data))}, nil
}

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
	if err := os.Rename(fromFull, toFull); err != nil {
		return Result{}, err
	}
	return Result{Status: "applied", Operation: "move", From: from, To: to}, nil
}
