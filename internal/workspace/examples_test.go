package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExampleWorkspace_Hash demonstrates hashing a workspace-relative file.
func ExampleWorkspace_Hash() {
	root, err := os.MkdirTemp("", "pairfs-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	if err := os.WriteFile(root+"/hello.txt", []byte("hello\n"), 0o644); err != nil {
		panic(err)
	}

	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		panic(err)
	}
	ws, err := New(canonicalRoot)
	if err != nil {
		panic(err)
	}
	hash, err := ws.Hash("hello.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(hash))
	// Output:
	// 64
}

// ExampleWorkspace_AtomicWrite demonstrates an atomic write with the default file mode.
func ExampleWorkspace_AtomicWrite() {
	root, err := os.MkdirTemp("", "pairfs-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)

	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		panic(err)
	}
	ws, err := New(canonicalRoot)
	if err != nil {
		panic(err)
	}
	if err := ws.AtomicWrite("docs/hello.txt", []byte("hello\n"), 0); err != nil {
		panic(err)
	}
	content, mode, err := ws.Read("docs/hello.txt")
	if err != nil {
		panic(err)
	}
	fmt.Print(string(content))
	fmt.Printf("%#o\n", mode.Perm())
	// Output:
	// hello
	// 0644
}
