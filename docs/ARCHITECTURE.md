# Architecture and function guide

This document explains how pairfs turns a CLI command into a bounded filesystem operation. It also records the responsibility of every function currently in the project, including production code, unit tests, and executable documentation examples.

## System overview

pairfs is a single-process Go CLI with three small layers:

1. `cmd/pairfs` parses CLI arguments, creates a workspace, and dispatches a command.
2. `internal/tools` implements read, discovery, preview, and mutation behavior.
3. `internal/workspace` owns workspace-relative path resolution and low-level file operations.

The dependency direction is inward: the CLI depends on tools and workspace, tools depend on workspace, and workspace depends only on the Go standard library.

```text
CLI arguments
    |
    v
cmd/pairfs
    |
    +---- read / grep / glob ------------------+
    |                                          |
    +---- edit / write / delete / move         |
                 |                             |
                 +---- preview ---- apply      |
                                   |           |
                                   v           v
                           internal/workspace
                                   |
                                   v
                           local filesystem
```

## Request lifecycle

Every invocation starts in `main`:

1. The first positional argument selects a command.
2. A shared `flag.FlagSet` parses paths, search options, content, mutation mode, apply state, and an optional expected hash.
3. `workspace.New` resolves the requested root to an absolute directory.
4. `main` dispatches to one function in `internal/tools`.
5. Read-only commands return text or JSON immediately.
6. Mutation commands return a preview unless `--apply` is present.
7. Apply operations optionally compare the current SHA-256 hash with `--expected-hash` before changing the filesystem.
8. Success results are emitted as JSON; errors are written to standard error and terminate the process with status 1.

Unknown commands and missing subcommands terminate without touching the workspace.

## Read-only operations

### Read

`read` calls `ReadFile`, which delegates file access to `Workspace.Read`. The content is scanned line by line, sliced using a one-based offset and a maximum line count, then returned as tab-separated line-numbered text.

### Grep

`grep` compiles the supplied regular expression, walks the workspace, optionally filters file basenames, and returns matching path/line/text records. `.git`, `.pairfs`, `vendor`, and symlink entries are skipped during discovery.

### Glob

`glob` converts the pairfs glob syntax into an anchored regular expression, walks the workspace, collects matching file paths, and sorts them before returning JSON. `**` matches recursively, while `*` remains within one path segment. Symlink entries and `.git`, `.pairfs`, and `vendor` directories are excluded.

## Mutation lifecycle

All four mutation families follow the same two-stage model:

```text
current file state
       |
       v
PreviewXxx
  - validate the operation
  - calculate the diff or rename description
  - calculate beforeHash when a source exists
  - do not modify the filesystem
       |
       v
user/agent reviews preview
       |
       v
ApplyXxx(expectedHash)
  - optionally reject stale state
  - validate again
  - perform one filesystem mutation
       |
       v
Result: applied or stale
```

### Edit

An edit requires `oldText` to occur exactly once. Preview produces the replacement content and a whole-file unified diff. Apply repeats validation and writes the new content atomically while preserving the original mode.

### Write

Write has two modes. `create` requires the destination not to exist and uses mode `0644`. `overwrite` requires an existing regular file and preserves its mode. Both paths use `Workspace.AtomicWrite` during apply.

### Delete

Delete is recoverable within the workspace. Preview renders the file as removed; apply renames it into `.pairfs/trash/<original-path>` instead of permanently deleting it. If that trash path already exists, apply fails and preserves both the source and the earlier recovery copy.

### Move

Move requires a readable source and a destination that does not exist. Preview returns a rename description and the source hash. Apply creates destination parents and renames the file.

## Workspace boundary

`Workspace` is the filesystem boundary used by tools:

- The root must exist, be a directory, and is canonicalized once during construction.
- Tool paths must be non-empty and workspace-relative.
- Absolute paths and lexical `..` escapes are rejected.
- Every existing component below the root is inspected with `Lstat`.
- All descendant symlinks are rejected, even when they point back inside the workspace or are broken.
- Missing nested paths are allowed so create and move operations can prepare normal destinations.
- Reads accept regular files only.
- Hashes are SHA-256 digests of file bytes.
- Atomic writes create parents, repeat path validation, then use a temporary sibling file, `fsync`, close, and rename.

These checks use portable Go APIs and prevent static symlink escapes. They do not provide descriptor-level isolation from another local process that maliciously replaces a checked component before it is used. OS-specific no-follow operations, hard-link policy, and mandatory mutation hashes remain separate hardening work.

## Output contracts

Read returns plain line-numbered text. Grep, glob, previews, and mutation results are JSON.

`Preview` describes an operation before execution:

- `operation`: edit, write, delete, or move
- `path`, or `from` and `to`: affected workspace-relative paths
- `diff`: whole-file diff or rename description
- `beforeHash`: source SHA-256 when applicable
- `canApply`: whether validation succeeded
- `message`: optional context

`Result` describes an apply attempt:

- `status`: normally `applied`, or `stale` when an expected hash differs
- `operation`: applied mutation family
- affected path fields
- optional human-readable message

## Production function inventory

### `cmd/pairfs/main.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `fail` | Centralizes fatal CLI errors. | `error` | Writes to stderr and exits with status 1. |
| `printJSON` | Formats machine-readable command results. | Any JSON-compatible value | Writes indented JSON to stdout. |
| `main` | Parses flags, creates the workspace, and dispatches commands. | Process arguments | Produces command output or terminates with an error. |

### `internal/workspace/workspace.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `New` | Constructs a workspace from an existing directory. | Root path | `*Workspace` with a canonical absolute root, or an error. |
| `(*Workspace).Root` | Exposes the workspace root. | Receiver | Canonical absolute root string; no side effect. |
| `(*Workspace).Resolve` | Validates syntax and rejects every existing descendant symlink. | Relative path | Absolute target path or `ErrUnsafePath`. |
| `(*Workspace).Read` | Reads a regular workspace file. | Relative path | Bytes, file mode, or an error. |
| `(*Workspace).Hash` | Hashes the current file bytes. | Relative path | Lowercase hexadecimal SHA-256 digest. |
| `(*Workspace).AtomicWrite` | Writes through a synced temporary sibling and rename. | Relative path, bytes, mode | Creates parents and atomically replaces the destination. |

### `internal/tools/read.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `ReadFile` | Produces a bounded, line-numbered file view. | Workspace, path, offset, limit | Plain text; no filesystem mutation. |

### `internal/tools/grep.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `Grep` | Searches non-symlink, non-ignored file lines using a regular expression. | Workspace, regex, basename filter, maximum matches | Ordered `[]GrepMatch`; no filesystem mutation. |

### `internal/tools/glob.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `Glob` | Finds and sorts non-symlink, non-ignored files matching pairfs glob syntax. | Workspace and glob pattern | Sorted relative paths; no filesystem mutation. |
| `globToRegexp` | Compiles pairfs glob syntax into a path regex. | Slash-separated glob | Anchored `*regexp.Regexp`; no side effect. |

### `internal/tools/diff.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `UnifiedDiff` | Renders a deterministic whole-file diff. | Path, old text, new text | Unified diff string; no side effect. |
| `splitLines` | Normalizes CRLF and removes a trailing empty line. | Text | Line slice; no side effect. |

### `internal/tools/mutations.go`

| Function | Responsibility | Input | Output or side effect |
| --- | --- | --- | --- |
| `PreviewEdit` | Validates one exact replacement and builds its diff. | Workspace, path, old text, new text | Preview, next content, original mode; no mutation. |
| `ApplyEdit` | Revalidates and atomically writes an edit. | Preview inputs plus optional expected hash | Applied/stale result; may replace one file. |
| `PreviewWrite` | Validates create/overwrite and builds its diff. | Workspace, path, content, mode | Preview and destination mode; no mutation. |
| `ApplyWrite` | Creates or atomically overwrites a file. | Preview inputs plus optional expected hash | Applied/stale result; may write one file. |
| `PreviewDelete` | Builds a deletion diff and captures the source hash. | Workspace and path | Preview; no mutation. |
| `ApplyDelete` | Moves a file to pairfs trash without overwriting an earlier recovery copy. | Workspace, path, optional expected hash | Applied/stale result; may rename one file. |
| `PreviewMove` | Validates source and destination and describes a rename. | Workspace, source path, destination path | Preview; no mutation. |
| `ApplyMove` | Creates destination parents and renames the source. | Preview inputs plus optional expected hash | Applied/stale result; may rename one file. |

## Unit-test function inventory

These functions live in `internal/tools/tools_test.go` and verify behavior rather than participating in runtime execution.

| Function | Verified behavior |
| --- | --- |
| `fixture` | Creates an isolated workspace and representative Go file. |
| `TestRead` | One-based line-numbered read output. |
| `TestGlob` | Recursive Go-file glob matching. |
| `TestGrep` | Regex matching, basename filtering, and line numbers. |
| `TestEditPreviewAndApply` | Edit diff plus hash-guarded application. |
| `TestWriteCreate` | Create preview and application. |
| `TestDeleteMovesToTrash` | Recoverable deletion under `.pairfs/trash`. |
| `TestMove` | Hash-guarded file rename. |
| `TestStaleHash` | Rejection after an intervening file change. |
| `TestUnsafePath` | Rejection of parent-directory traversal. |

## Executable-example function inventory

Go executes examples containing `// Output:` during `go test`, so these functions are both documentation and regression checks.

### `internal/tools/examples_test.go`

| Function | Demonstrated behavior |
| --- | --- |
| `exampleWorkspace` | Builds and cleans up disposable example workspaces. |
| `ExampleReadFile` | Bounded line-numbered reading. |
| `ExampleGrep` | Regex search with a basename filter. |
| `ExampleGlob` | Recursive glob matching and sorted output. |
| `ExampleUnifiedDiff` | Added and removed diff lines. |
| `ExamplePreviewEdit` | Previewed content versus unchanged disk content. |
| `ExampleApplyWrite` | Creating and reading a file. |
| `ExampleApplyDelete` | Moving a file into pairfs trash. |
| `ExampleApplyMove` | Hash-guarded move into a new directory. |

### `internal/workspace/examples_test.go`

| Function | Demonstrated behavior |
| --- | --- |
| `ExampleWorkspace_Hash` | Stable hexadecimal SHA-256 output shape. |
| `ExampleWorkspace_AtomicWrite` | Nested atomic write and default `0644` mode. |

## Extending pairfs

When adding a command:

1. Add CLI flags and dispatch in `cmd/pairfs/main.go`.
2. Keep filesystem policy in `internal/workspace`; do not bypass it from tools.
3. Put operation semantics in `internal/tools`.
4. Use preview/apply separation for mutations.
5. Return structured `Preview` and `Result` values for machine consumers.
6. Add unit tests for validation and errors.
7. Add an executable example when the public behavior is not obvious from the signature.
8. Update this guide when a new function or lifecycle is introduced.

## Known boundaries

The current MVP operates on one text file per mutation and does not provide fuzzy edits, arbitrary shell execution, multi-file transactions, or automatic reference rewriting after moves. README safety rules and the open security-hardening work define the intended boundary for the first release.
