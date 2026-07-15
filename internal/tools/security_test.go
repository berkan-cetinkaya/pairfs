package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

// requireToolSymlink creates a symlink or skips when the platform denies it.
func requireToolSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
}

// securityFixture creates a workspace with an adjacent outside directory.
func securityFixture(t *testing.T) (*workspace.Workspace, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return ws, root, outside
}

// TestDiscoverySkipsSymlinksAndPairFS verifies discovery never exposes ignored content.
func TestDiscoverySkipsSymlinksAndPairFS(t *testing.T) {
	ws, root, outside := securityFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "visible.go"), []byte("package visible\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".pairfs", "trash"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pairfs", "trash", "hidden.go"), []byte("package secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(outside, "external.go")
	if err := os.WriteFile(external, []byte("package secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireToolSymlink(t, external, filepath.Join(root, "src", "link.go"))

	paths, err := Glob(ws, "**/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "src/visible.go" {
		t.Fatalf("Glob returned %v", paths)
	}
	matches, err := Grep(ws, "package secret", "*.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("Grep exposed ignored content: %+v", matches)
	}
}

// TestSourceOperationsRejectFinalSymlink verifies read and mutation sources stay bounded.
func TestSourceOperationsRejectFinalSymlink(t *testing.T) {
	ws, root, outside := securityFixture(t)
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("status: open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireToolSymlink(t, target, filepath.Join(root, "link.txt"))

	tests := map[string]func() error{
		"read":           func() error { _, err := ReadFile(ws, "link.txt", 1, 10); return err },
		"preview edit":   func() error { _, _, _, err := PreviewEdit(ws, "link.txt", "open", "done"); return err },
		"apply edit":     func() error { _, err := ApplyEdit(ws, "link.txt", "open", "done", ""); return err },
		"preview delete": func() error { _, err := PreviewDelete(ws, "link.txt"); return err },
		"apply delete":   func() error { _, err := ApplyDelete(ws, "link.txt", ""); return err },
		"preview move":   func() error { _, err := PreviewMove(ws, "link.txt", "moved.txt"); return err },
		"apply move":     func() error { _, err := ApplyMove(ws, "link.txt", "moved.txt", ""); return err },
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			if err := run(); !errors.Is(err, workspace.ErrUnsafePath) {
				t.Fatalf("error = %v, want ErrUnsafePath", err)
			}
		})
	}
}

// TestDestinationOperationsRejectSymlinkParent verifies write and move cannot escape.
func TestDestinationOperationsRejectSymlinkParent(t *testing.T) {
	ws, root, outside := securityFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireToolSymlink(t, outside, filepath.Join(root, "escape"))
	target := filepath.Join("escape", "new", "proof.txt")

	if _, _, err := PreviewWrite(ws, target, "escaped", "create"); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("PreviewWrite error = %v, want ErrUnsafePath", err)
	}
	if _, err := ApplyWrite(ws, target, "escaped", "create", ""); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("ApplyWrite error = %v, want ErrUnsafePath", err)
	}
	if _, err := PreviewMove(ws, "source.txt", target); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("PreviewMove error = %v, want ErrUnsafePath", err)
	}
	if _, err := ApplyMove(ws, "source.txt", target, ""); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("ApplyMove error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "new", "proof.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", err)
	}
}

// TestDeleteRejectsSymlinkTrash verifies trash cannot redirect deletion outside the root.
func TestDeleteRejectsSymlinkTrash(t *testing.T) {
	ws, root, outside := securityFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".pairfs"), 0o755); err != nil {
		t.Fatal(err)
	}
	requireToolSymlink(t, outside, filepath.Join(root, ".pairfs", "trash"))
	preview, err := PreviewDelete(ws, "source.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDelete(ws, "source.txt", preview.BeforeHash); !errors.Is(err, workspace.ErrUnsafePath) {
		t.Fatalf("ApplyDelete error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(root, "source.txt")); err != nil {
		t.Fatalf("source was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "source.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", err)
	}
}

// TestDeletePreservesTrashCollision verifies existing recovery data is never overwritten.
func TestDeletePreservesTrashCollision(t *testing.T) {
	ws, root, _ := securityFixture(t)
	source := filepath.Join(root, "source.txt")
	trash := filepath.Join(root, ".pairfs", "trash", "source.txt")
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(trash, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewDelete(ws, "source.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDelete(ws, "source.txt", preview.BeforeHash); err == nil {
		t.Fatal("ApplyDelete succeeded despite trash collision")
	}
	sourceData, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	trashData, err := os.ReadFile(trash)
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceData) != "new" || string(trashData) != "old" {
		t.Fatalf("source=%q trash=%q", sourceData, trashData)
	}
}
