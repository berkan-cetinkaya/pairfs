#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin/pairfs"
WORK="$ROOT/testdata/workspace"
rm -rf "$WORK"
cp -R "$ROOT/testdata/base" "$WORK"
mkdir -p "$ROOT/bin"
go build -o "$BIN" "$ROOT/cmd/pairfs"

echo '== read =='
"$BIN" read --root "$WORK" --path internal/todo/model.go --offset 1 --limit 20

echo '== glob =='
"$BIN" glob --root "$WORK" --pattern '**/*.go'

echo '== grep =='
"$BIN" grep --root "$WORK" --pattern 'type Todo' --include '*.go'

echo '== edit preview =='
EDIT_JSON=$("$BIN" edit --root "$WORK" --path internal/todo/model.go --old 'Title string' --new $'Title string\n\tDone bool')
echo "$EDIT_JSON"
EDIT_HASH=$(printf '%s' "$EDIT_JSON" | sed -n 's/.*"beforeHash": "\([^"]*\)".*/\1/p')

echo '== edit apply =='
"$BIN" edit --root "$WORK" --path internal/todo/model.go --old 'Title string' --new $'Title string\n\tDone bool' --expected-hash "$EDIT_HASH" --apply

echo '== write preview/apply =='
WRITE_JSON=$("$BIN" write --root "$WORK" --path internal/todo/store.go --content $'package todo\n' --mode create)
echo "$WRITE_JSON"
"$BIN" write --root "$WORK" --path internal/todo/store.go --content $'package todo\n' --mode create --apply

echo '== move preview/apply =='
MOVE_JSON=$("$BIN" move --root "$WORK" --from internal/todo/store.go --to internal/todo/memory_store.go)
echo "$MOVE_JSON"
MOVE_HASH=$(printf '%s' "$MOVE_JSON" | sed -n 's/.*"beforeHash": "\([^"]*\)".*/\1/p')
"$BIN" move --root "$WORK" --from internal/todo/store.go --to internal/todo/memory_store.go --expected-hash "$MOVE_HASH" --apply

echo '== delete preview/apply =='
DELETE_JSON=$("$BIN" delete --root "$WORK" --path internal/todo/memory_store.go)
echo "$DELETE_JSON"
DELETE_HASH=$(printf '%s' "$DELETE_JSON" | sed -n 's/.*"beforeHash": "\([^"]*\)".*/\1/p')
"$BIN" delete --root "$WORK" --path internal/todo/memory_store.go --expected-hash "$DELETE_HASH" --apply

echo '== final tree =='
find "$WORK" -maxdepth 6 -type f | sort

echo 'Smoke tests passed.'
