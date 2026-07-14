package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berkan-cetinkaya/pairfs/internal/workspace"
)

// exampleWorkspace creates a disposable workspace for executable examples.
func exampleWorkspace(files map[string]string) (*workspace.Workspace, func()) {
	root, err := os.MkdirTemp("", "pairfs-example-")
	if err != nil {
		panic(err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			cleanup()
			panic(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			cleanup()
			panic(err)
		}
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		cleanup()
		panic(err)
	}
	ws, err := workspace.New(canonicalRoot)
	if err != nil {
		cleanup()
		panic(err)
	}
	return ws, cleanup
}

// ExampleReadFile demonstrates one-based, line-numbered file reads.
func ExampleReadFile() {
	ws, cleanup := exampleWorkspace(map[string]string{
		"notes.txt": "first\nsecond\nthird\n",
	})
	defer cleanup()

	out, err := ReadFile(ws, "notes.txt", 2, 2)
	if err != nil {
		panic(err)
	}
	fmt.Print(out)
	// Output:
	// 2	second
	// 3	third
}

// ExampleGrep demonstrates regex search with a basename filter.
func ExampleGrep() {
	ws, cleanup := exampleWorkspace(map[string]string{
		"todo.go":  "package todo\n\ntype Todo struct{}\n",
		"notes.md": "type Todo is documented here\n",
	})
	defer cleanup()

	matches, err := Grep(ws, `type\s+Todo`, "*.go", 10)
	if err != nil {
		panic(err)
	}
	for _, match := range matches {
		fmt.Printf("%s:%d: %s\n", match.Path, match.Line, match.Text)
	}
	// Output:
	// todo.go:3: type Todo struct{}
}

// ExampleGlob demonstrates recursive glob matching and sorted output.
func ExampleGlob() {
	ws, cleanup := exampleWorkspace(map[string]string{
		"main.go":          "package main\n",
		"internal/todo.go": "package internal\n",
		"README.md":        "# Example\n",
	})
	defer cleanup()

	paths, err := Glob(ws, "**/*.go")
	if err != nil {
		panic(err)
	}
	fmt.Println(paths)
	// Output:
	// [internal/todo.go]
}

// ExampleUnifiedDiff demonstrates a deterministic single-file diff.
func ExampleUnifiedDiff() {
	diff := UnifiedDiff("notes.txt", "old\n", "new\n")
	fmt.Println(strings.Contains(diff, "-old"))
	fmt.Println(strings.Contains(diff, "+new"))
	// Output:
	// true
	// true
}

// ExamplePreviewEdit demonstrates an edit preview without a filesystem change.
func ExamplePreviewEdit() {
	ws, cleanup := exampleWorkspace(map[string]string{"todo.txt": "status: open\n"})
	defer cleanup()

	preview, next, _, err := PreviewEdit(ws, "todo.txt", "open", "done")
	if err != nil {
		panic(err)
	}
	current, _, err := ws.Read("todo.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(preview.Operation, preview.CanApply)
	fmt.Print(next)
	fmt.Print(string(current))
	// Output:
	// edit true
	// status: done
	// status: open
}

// ExampleApplyWrite demonstrates creating a file after a write preview.
func ExampleApplyWrite() {
	ws, cleanup := exampleWorkspace(nil)
	defer cleanup()

	result, err := ApplyWrite(ws, "notes.txt", "hello\n", "create", "")
	if err != nil {
		panic(err)
	}
	content, _, err := ws.Read("notes.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Status)
	fmt.Print(string(content))
	// Output:
	// applied
	// hello
}

// ExampleApplyDelete demonstrates recoverable deletion into .pairfs/trash.
func ExampleApplyDelete() {
	ws, cleanup := exampleWorkspace(map[string]string{"notes.txt": "hello\n"})
	defer cleanup()

	preview, err := PreviewDelete(ws, "notes.txt")
	if err != nil {
		panic(err)
	}
	result, err := ApplyDelete(ws, "notes.txt", preview.BeforeHash)
	if err != nil {
		panic(err)
	}
	_, sourceErr := os.Stat(filepath.Join(ws.Root(), "notes.txt"))
	_, trashErr := os.Stat(filepath.Join(ws.Root(), ".pairfs", "trash", "notes.txt"))
	fmt.Println(result.Status)
	fmt.Println(os.IsNotExist(sourceErr), trashErr == nil)
	// Output:
	// applied
	// true true
}

// ExampleApplyMove demonstrates a hash-guarded file move.
func ExampleApplyMove() {
	ws, cleanup := exampleWorkspace(map[string]string{"draft.txt": "hello\n"})
	defer cleanup()

	preview, err := PreviewMove(ws, "draft.txt", "archive/final.txt")
	if err != nil {
		panic(err)
	}
	result, err := ApplyMove(ws, "draft.txt", "archive/final.txt", preview.BeforeHash)
	if err != nil {
		panic(err)
	}
	content, _, err := ws.Read("archive/final.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Status)
	fmt.Print(string(content))
	// Output:
	// applied
	// hello
}
