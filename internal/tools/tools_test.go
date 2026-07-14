package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

// fixture creates an isolated workspace containing a representative Go source file.
func fixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "todo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "todo", "model.go"), []byte("package todo\n\ntype Todo struct {\n\tTitle string\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

// TestRead verifies that file output contains one-based line numbers.
func TestRead(t *testing.T) {
	ws := fixture(t)
	out, err := ReadFile(ws, "internal/todo/model.go", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1\tpackage todo") {
		t.Fatalf("unexpected: %s", out)
	}
}

// TestGlob verifies recursive glob matching for Go source files.
func TestGlob(t *testing.T) {
	ws := fixture(t)
	out, err := Glob(ws, "**/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %v", out)
	}
}

// TestGrep verifies regex matching, basename filtering, and reported line numbers.
func TestGrep(t *testing.T) {
	ws := fixture(t)
	out, err := Grep(ws, "type Todo", "*.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Line != 3 {
		t.Fatalf("got %+v", out)
	}
}

// TestEditPreviewAndApply verifies an edit preview and hash-guarded application.
func TestEditPreviewAndApply(t *testing.T) {
	ws := fixture(t)
	p, _, _, err := PreviewEdit(ws, "internal/todo/model.go", "Title string", "Title string\n\tDone bool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Diff, "+\tDone bool") {
		t.Fatal(p.Diff)
	}
	r, err := ApplyEdit(ws, "internal/todo/model.go", "Title string", "Title string\n\tDone bool", p.BeforeHash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "applied" {
		t.Fatal(r)
	}
}

// TestWriteCreate verifies previewing and applying a new file creation.
func TestWriteCreate(t *testing.T) {
	ws := fixture(t)
	p, _, err := PreviewWrite(ws, "internal/todo/store.go", "package todo\n", "create")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ApplyWrite(ws, "internal/todo/store.go", "package todo\n", "create", p.BeforeHash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "applied" {
		t.Fatal(r)
	}
}

// TestDeleteMovesToTrash verifies that deletion preserves the file under pairfs trash.
func TestDeleteMovesToTrash(t *testing.T) {
	ws := fixture(t)
	p, err := PreviewDelete(ws, "internal/todo/model.go")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ApplyDelete(ws, "internal/todo/model.go", p.BeforeHash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "applied" {
		t.Fatal(r)
	}
	if _, err := os.Stat(filepath.Join(ws.Root(), ".pairfs", "trash", "internal", "todo", "model.go")); err != nil {
		t.Fatal(err)
	}
}

// TestMove verifies previewing and applying a hash-guarded file rename.
func TestMove(t *testing.T) {
	ws := fixture(t)
	p, err := PreviewMove(ws, "internal/todo/model.go", "internal/todo/entity.go")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ApplyMove(ws, "internal/todo/model.go", "internal/todo/entity.go", p.BeforeHash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "applied" {
		t.Fatal(r)
	}
}

// TestStaleHash verifies that an intervening file change prevents an edit.
func TestStaleHash(t *testing.T) {
	ws := fixture(t)
	p, _, _, err := PreviewEdit(ws, "internal/todo/model.go", "Title string", "Name string")
	if err != nil {
		t.Fatal(err)
	}
	full, _ := ws.Resolve("internal/todo/model.go")
	if err := os.WriteFile(full, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ApplyEdit(ws, "internal/todo/model.go", "Title string", "Name string", p.BeforeHash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "stale" {
		t.Fatalf("got %+v", r)
	}
}

// TestUnsafePath verifies that parent-directory traversal is rejected.
func TestUnsafePath(t *testing.T) {
	ws := fixture(t)
	if _, err := ReadFile(ws, "../secret", 1, 10); err == nil {
		t.Fatal("expected error")
	}
}
