package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// requireSymlink creates a symlink or skips the test when the platform denies it.
func requireSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
}

// newTestWorkspace creates a workspace with its root and outside directory as siblings.
func newTestWorkspace(t *testing.T) (*Workspace, string, string) {
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
	ws, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return ws, root, outside
}

// TestNewCanonicalizesRoot verifies that Root reports the resolved root directory.
func TestNewCanonicalizesRoot(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	linkRoot := filepath.Join(base, "link")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	requireSymlink(t, realRoot, linkRoot)

	ws, err := New(linkRoot)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(realRoot)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Root() != want {
		t.Fatalf("Root() = %q, want %q", ws.Root(), want)
	}
}

// TestResolveAcceptsNormalPaths verifies existing and missing nested paths remain usable.
func TestResolveAcceptsNormalPaths(t *testing.T) {
	ws, root, _ := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"existing.txt", "missing/nested/file.txt"} {
		resolved, err := ws.Resolve(path)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", path, err)
		}
		if !filepath.IsAbs(resolved) {
			t.Fatalf("Resolve(%q) returned non-absolute path %q", path, resolved)
		}
	}
}

// TestResolveRejectsUnsafeSyntax verifies lexical workspace escapes are rejected.
func TestResolveRejectsUnsafeSyntax(t *testing.T) {
	ws, _, _ := newTestWorkspace(t)
	for _, path := range []string{"", ".", "..", "../secret", filepath.Join(t.TempDir(), "absolute")} {
		if _, err := ws.Resolve(path); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("Resolve(%q) error = %v, want ErrUnsafePath", path, err)
		}
	}
}

// TestResolveRejectsEverySymlinkShape verifies no descendant symlink is accepted.
func TestResolveRejectsEverySymlinkShape(t *testing.T) {
	tests := map[string]func(t *testing.T, root, outside string) string{
		"final external": func(t *testing.T, root, outside string) string {
			target := filepath.Join(outside, "secret.txt")
			if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
				t.Fatal(err)
			}
			requireSymlink(t, target, filepath.Join(root, "link"))
			return "link"
		},
		"final internal": func(t *testing.T, root, _ string) string {
			if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("ok"), 0o644); err != nil {
				t.Fatal(err)
			}
			requireSymlink(t, "target.txt", filepath.Join(root, "link"))
			return "link"
		},
		"broken": func(t *testing.T, root, _ string) string {
			requireSymlink(t, "missing.txt", filepath.Join(root, "link"))
			return "link"
		},
		"intermediate": func(t *testing.T, root, outside string) string {
			requireSymlink(t, outside, filepath.Join(root, "escape"))
			return filepath.Join("escape", "new", "proof.txt")
		},
	}

	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			ws, root, outside := newTestWorkspace(t)
			path := setup(t, root, outside)
			if _, err := ws.Resolve(path); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("Resolve(%q) error = %v, want ErrUnsafePath", path, err)
			}
		})
	}
}

// TestReadAndHashRejectFinalSymlink verifies read operations cannot escape the root.
func TestReadAndHashRejectFinalSymlink(t *testing.T) {
	ws, root, outside := newTestWorkspace(t)
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireSymlink(t, target, filepath.Join(root, "secret-link"))

	if _, _, err := ws.Read("secret-link"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Read error = %v, want ErrUnsafePath", err)
	}
	if _, err := ws.Hash("secret-link"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Hash error = %v, want ErrUnsafePath", err)
	}
}

// TestAtomicWriteRejectsSymlinkParent verifies writes cannot create files outside the root.
func TestAtomicWriteRejectsSymlinkParent(t *testing.T) {
	ws, root, outside := newTestWorkspace(t)
	requireSymlink(t, outside, filepath.Join(root, "escape"))

	err := ws.AtomicWrite(filepath.Join("escape", "new", "proof.txt"), []byte("escaped"), 0o644)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("AtomicWrite error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "new", "proof.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", err)
	}
}
