package pairfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

func TestOpenAndRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if fs.Root() != want {
		t.Fatalf("Root() = %q, want %q", fs.Root(), want)
	}
}

func TestOpenRejectsInvalidRoots(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{filepath.Join(t.TempDir(), "missing"), file} {
		if _, err := Open(root); err == nil {
			t.Errorf("Open(%q) succeeded, want error", root)
		}
	}
}

func TestListFiles(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("z.txt", "123")
	write("nested/a.txt", "hello")
	write(".git/ignored", "x")
	write(".pairfs/ignored", "x")
	write("vendor/ignored", "x")
	if err := os.Symlink(filepath.Join(root, "z.txt"), filepath.Join(root, "file-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "nested"), filepath.Join(root, "dir-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs.ListFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []FileEntry{{Path: "nested/a.txt", Size: 5}, {Path: "z.txt", Size: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFiles() = %#v, want %#v", got, want)
	}
}

func TestListFilesCanceled(t *testing.T) {
	fs, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fs.ListFiles(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListFiles() error = %v, want context.Canceled", err)
	}
}

func TestReadFileExactBytes(t *testing.T) {
	root := t.TempDir()
	for name, want := range map[string][]byte{
		"with-newline":    []byte("hello\n"),
		"without-newline": []byte("hello"),
		"binary":          {0, 1, 2, '\n'},
	} {
		if err := os.WriteFile(filepath.Join(root, name), want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{
		"with-newline":    []byte("hello\n"),
		"without-newline": []byte("hello"),
		"binary":          {0, 1, 2, '\n'},
	} {
		got, err := fs.ReadFile(context.Background(), name)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReadFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestReadFileRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "absolute"), "../escape"} {
		if _, err := fs.ReadFile(context.Background(), path); !errors.Is(err, workspace.ErrUnsafePath) {
			t.Errorf("ReadFile(%q) error = %v, want ErrUnsafePath", path, err)
		}
	}
}

func TestReadFileRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(context.Background(), "link"); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("ReadFile() error = %v, want ErrUnsafePath", err)
	}
}

func TestReadFileCanceled(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fs.ReadFile(ctx, "file"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadFile() error = %v, want context.Canceled", err)
	}
}

func TestPublicReadGrepAndGlob(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "todo.go"), []byte("package todo\n\ntype Todo struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	numbered, err := fs.Read("nested/todo.go", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(numbered, "3\ttype Todo struct{}") {
		t.Fatalf("Read() = %q", numbered)
	}
	matches, err := fs.Grep("type Todo", "*.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Path != "nested/todo.go" || matches[0].Line != 3 {
		t.Fatalf("Grep() = %#v", matches)
	}
	paths, err := fs.Glob("**/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{"nested/todo.go"}) {
		t.Fatalf("Glob() = %#v", paths)
	}
}

func TestPublicMutationOperations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "todo.txt"), []byte("status: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	edit, err := fs.PreviewEdit("todo.txt", "open", "done")
	if err != nil {
		t.Fatal(err)
	}
	if edit.Operation != "edit" || edit.BeforeHash == "" || !edit.CanApply {
		t.Fatalf("PreviewEdit() = %#v", edit)
	}
	if result, err := fs.ApplyEdit("todo.txt", "open", "done", edit.BeforeHash); err != nil || result.Status != "applied" {
		t.Fatalf("ApplyEdit() = %#v, %v", result, err)
	}

	write, err := fs.PreviewWrite("new.txt", "new\n", WriteCreate)
	if err != nil {
		t.Fatal(err)
	}
	if write.Operation != "write" || !write.CanApply {
		t.Fatalf("PreviewWrite() = %#v", write)
	}
	if result, err := fs.ApplyWrite("new.txt", "new\n", WriteCreate, ""); err != nil || result.Status != "applied" {
		t.Fatalf("ApplyWrite() = %#v, %v", result, err)
	}

	move, err := fs.PreviewMove("new.txt", "archive/new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := fs.ApplyMove("new.txt", "archive/new.txt", move.BeforeHash); err != nil || result.Status != "applied" {
		t.Fatalf("ApplyMove() = %#v, %v", result, err)
	}

	deletePreview, err := fs.PreviewDelete("archive/new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := fs.ApplyDelete("archive/new.txt", deletePreview.BeforeHash); err != nil || result.Status != "applied" {
		t.Fatalf("ApplyDelete() = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pairfs", "trash", "archive", "new.txt")); err != nil {
		t.Fatalf("trash file: %v", err)
	}
}

func TestPublicMutationErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "todo.txt"), []byte("same same"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.PreviewEdit("todo.txt", "missing", "new"); !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("PreviewEdit missing error = %v", err)
	}
	if _, err := fs.PreviewEdit("todo.txt", "same", "new"); !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("PreviewEdit ambiguous error = %v", err)
	}
	if _, err := fs.PreviewWrite("new.txt", "new", WriteMode("invalid")); err == nil {
		t.Fatal("PreviewWrite invalid mode succeeded")
	}
}
