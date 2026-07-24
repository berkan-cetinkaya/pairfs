# pairfs

A minimal, deterministic, LLM-agnostic local filesystem tool runtime written in Go.

## Tools

- `read`: read a file with line numbers
- `grep`: regex content search
- `glob`: recursive glob search with `**`
- `edit`: exact one-file replacement with preview and explicit apply
- `write`: create or overwrite one file
- `delete`: safe delete by moving to `.pairfs/trash`
- `move`: move one file

Mutation commands are preview-only by default. Add `--apply` to execute. Preview responses include `beforeHash`; pass it back with `--expected-hash` to prevent stale writes.

## How it works

See [Architecture and function guide](docs/ARCHITECTURE.md) for the end-to-end execution flow, mutation lifecycle, package responsibilities, and a complete function inventory.

## Build

```bash
go build -o bin/pairfs ./cmd/pairfs
```

## Go library usage

Import the module directly:

```go
import "github.com/berkan-cetinkaya/pairfs"
```

Open a workspace, list its regular files deterministically, and read exact raw bytes:

```go
fs, err := pairfs.Open("/path/to/repository")
if err != nil {
	return err
}

files, err := fs.ListFiles(ctx)
if err != nil {
	return err
}

for _, file := range files {
	content, err := fs.ReadFile(ctx, file.Path)
	if err != nil {
		return err
	}
	fmt.Println(file.Path, file.Size, len(content))
}
```

Listed and read paths are workspace-relative. Discovery skips `.git`, `.pairfs`, and `vendor`; symlinks are skipped during listing. Reads reject absolute paths, workspace escapes, and paths containing symlinks. `ReadFile` returns unmodified bytes, including the original final-newline behavior.

The package is an independent filesystem facade. It does not know about pair-index or any consuming application's lifecycle.

The same `FS` also exposes every CLI operation directly:

```go
matches, err := fs.Grep(`type\s+Todo`, "*.go", 100)
paths, err := fs.Glob("**/*.go")
numbered, err := fs.Read("README.md", 1, 200)

preview, err := fs.PreviewEdit("README.md", "old", "new")
if err != nil {
	return err
}
result, err := fs.ApplyEdit("README.md", "old", "new", preview.BeforeHash)
```

Mutation pairs are available for `Edit`, `Write`, `Delete`, and `Move`. Apply methods accept the preview's `BeforeHash` to detect stale files. Writes use the typed `pairfs.WriteCreate` and `pairfs.WriteOverwrite` modes. The CLI is an adapter over this public API and retains the same flags and JSON contracts.

## Test everything

```bash
go test ./...
./scripts/smoke.sh
```

## Examples

```bash
# Read
./bin/pairfs read --root ./testdata/base --path internal/todo/model.go

# Grep
./bin/pairfs grep --root ./testdata/base --pattern 'type Todo' --include '*.go'

# Glob
./bin/pairfs glob --root ./testdata/base --pattern '**/*.go'

# Edit preview
./bin/pairfs edit \
  --root ./testdata/base \
  --path internal/todo/model.go \
  --old 'Title string' \
  --new $'Title string\n\tDone bool'

# Edit apply
./bin/pairfs edit \
  --root ./testdata/base \
  --path internal/todo/model.go \
  --old 'Title string' \
  --new $'Title string\n\tDone bool' \
  --expected-hash '<hash from preview>' \
  --apply

# Write create preview/apply
./bin/pairfs write --root . --path notes.txt --content 'hello' --mode create
./bin/pairfs write --root . --path notes.txt --content 'hello' --mode create --apply

# Move preview/apply
./bin/pairfs move --root . --from notes.txt --to docs/notes.txt
./bin/pairfs move --root . --from notes.txt --to docs/notes.txt --apply

# Delete preview/apply
./bin/pairfs delete --root . --path docs/notes.txt
./bin/pairfs delete --root . --path docs/notes.txt --apply
```

## Safety rules

- workspace-relative paths only
- blocks `../` path escapes and absolute paths
- canonicalizes the workspace root once at startup
- rejects every symlink below the workspace root, including broken and final-component symlinks
- mutation preview before apply
- optional SHA-256 stale check
- atomic writes
- delete moves files to `.pairfs/trash`
- preserves existing trash entries instead of overwriting them
- ignores `.git`, `.pairfs`, `vendor`, and symlink entries during discovery

Path checks use portable Go filesystem operations. They prevent static symlink escapes, but do not claim descriptor-level isolation against another local process that changes path components between validation and use.

## Current MVP limitations

- text files only
- one file per mutation
- edit uses exact unique text matching
- no fuzzy matching
- no arbitrary shell execution
- move does not edit file content
- no OS-specific protection against malicious concurrent path replacement
