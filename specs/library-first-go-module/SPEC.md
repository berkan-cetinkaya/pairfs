# Overview

Expose all pairfs filesystem behavior as a public Go library and make the existing CLI a thin adapter over that API.

# Requirements

- REQ-001: `Open` accepts an existing directory and returns an `FS` whose root is the canonical absolute workspace path.
- REQ-002: `Open` rejects missing paths and non-directory paths through the existing workspace implementation.
- REQ-003: `ListFiles` recursively returns regular files as slash-normalized workspace-relative paths with `int64` sizes, sorted by path.
- REQ-004: `ListFiles` excludes `.git`, `.pairfs`, and `vendor` directories and all symlink entries.
- REQ-005: `ListFiles` checks context cancellation before and during traversal and returns the context error.
- REQ-006: `ReadFile` returns exact bytes for regular files without transforming newline behavior.
- REQ-007: `ReadFile` accepts only safe workspace-relative, non-symlink paths by delegating path validation to the workspace implementation.
- REQ-008: `ReadFile` checks context cancellation before and during reading and returns the context error.
- REQ-009: Existing CLI commands, output contracts, and mutation safety remain unchanged.
- REQ-010: The public package remains independent of pair-index, application lifecycle frameworks, SQLite, and LLM dependencies.
- REQ-011: Public documentation and an executable example demonstrate opening, listing, and reading.
- REQ-012: The public `FS` exposes the CLI's line-numbered read, grep, and glob operations.
- REQ-013: The public `FS` exposes preview and apply operations for edit, write, delete, and move.
- REQ-014: Public mutation types preserve operation, diff, hash, applicability, status, path, and message data used by the CLI JSON contract.
- REQ-015: Write modes are represented by a public named type and constants.
- REQ-016: The CLI delegates every filesystem operation to the public package without changing its flags or output.

# User stories

- As a Go application author, I can enumerate repository files deterministically without spawning the pairfs CLI.
- As an indexer author, I can obtain exact raw bytes and file sizes using workspace-relative paths.
- As a caller, I can cancel long traversal and read operations through a context.
- As a Go application author, I can use every CLI capability without spawning a subprocess.
- As a mutation client, I can preserve preview/apply and stale-hash protection through typed results.

# Edge cases

- Missing roots and file roots are rejected.
- Root symlinks are canonicalized.
- Absolute paths, parent traversal, broken symlinks, internal symlinks, and external symlinks are rejected for reads.
- Symlinked files and directories are omitted from listings.
- Files with and without final newlines retain their exact bytes.
- Already-canceled contexts return immediately.
- Files that cease to exist or change type during an operation return the underlying filesystem error.

# Non-functional

- Results are deterministic across repeated traversals of unchanged workspace contents.
- File size discovery uses metadata and does not read file contents.
- The public facade is thin and exposes stable public representations rather than internal mutation types.
- Existing repository tests, vet, CLI build, and smoke tests must pass.

# Technical notes

- The public package is the module root, `github.com/berkan-cetinkaya/pairfs`.
- Root canonicalization and path safety remain owned by `internal/workspace`.
- Context-aware traversal and raw reads are implemented in `internal/workspace` and wrapped by the public package.
- Existing implementations remain in `internal/tools`; public methods convert their results into public types.
- The CLI imports only the public root package for filesystem operations.

# Risks

- Filesystem path validation and later open operations retain the repository's documented time-of-check/time-of-use limitation against malicious concurrent path replacement.
- Context cancellation cannot interrupt a single operating-system read already in progress, but is checked between bounded read calls.

# Open questions

- None.
