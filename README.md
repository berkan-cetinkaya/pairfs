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
- blocks symlink-parent escapes
- mutation preview before apply
- optional SHA-256 stale check
- atomic writes
- delete moves files to `.pairfs/trash`
- ignores `.git` and `vendor` during discovery

## Current MVP limitations

- text files only
- one file per mutation
- edit uses exact unique text matching
- no fuzzy matching
- no arbitrary shell execution
- move does not edit file content
