# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go port of the Zig [`claude-p`](https://github.com/smithersai/claude-p). It is a drop-in replacement for `claude -p` that drives the **interactive** `claude` TUI inside an in-process PTY rather than calling a public API. Stdout is intended to match upstream `claude -p` byte-for-byte across `text`, `json`, and `stream-json` formats.

Requires Go **1.25+** (see `go.mod`). The original Zig project is no longer source-compatible — do not try to mirror its layout file-for-file; behavior parity is the contract, not structure.

## Commands

```bash
go build ./cmd/claude-p          # build the CLI binary
go vet ./...                     # what CI runs
go test ./...                    # full test suite (CI-equivalent)
go test ./internal/driver -run TestHermeticE2E -v    # single test, verbose
go test -race ./...              # race detector — run before touching driver/

# Cross-build matrix (CI runs this on every push):
GOOS=windows go build ./...
GOOS=darwin  go build ./...
GOOS=linux   go build ./...
```

The CI workflow (`.github/workflows/ci.yml`) only runs on `ubuntu-latest` — Windows and Darwin are compile-checked but not test-executed. If you change PTY or platform-specific code, test locally on the affected OS.

### Running the binary against fake-claude

`cmd/fake-claude` is a stub `claude` binary used by the hermetic E2E tests. To drive `claude-p` against it manually:

```bash
go build -o ./bin/fake-claude ./cmd/fake-claude
go run ./cmd/claude-p --claude-path ./bin/fake-claude "hi"   # if exposed; otherwise use the driver test harness
```

Most contributors should never need real `claude` to develop — the hermetic tests cover the protocol surface.

## Architecture

The public surface is **two entry points** that share an engine:

- `claudep.Run(Options) (*Result, error)` — library API in [claudep.go](claudep.go)
- `cmd/claude-p` — CLI that parses argv, resolves the prompt source, and calls `claudep.Run`

Everything else is `internal/`. The package boundaries map to the lifecycle of one `claude -p` invocation:

| Package | Responsibility |
| --- | --- |
| `internal/args` | CLI flag parsing + the `OutputFormat` enum. Knows nothing about PTYs. |
| `internal/driver` | The engine. Spawns `claude` under a PTY, drives the TUI, waits for Stop, returns Result. Owns timeout policy and process termination (`terminate_unix.go` / `terminate_windows.go`). |
| `internal/hook` | Generates the per-run temp dir, the relay shell script, and the inline `--settings` JSON that registers `SessionStart` and `Stop` hooks. Hooks write tab-separated events to a file the driver tails. |
| `internal/terminal` | The ANSI scanner that answers DA1 / DA2 / DSR / XTVERSION / window-size queries Ink issues at startup. **Without these replies the TUI hangs forever** — this is the most fragile part of the project. |
| `internal/transcript` | Reads the JSONL transcript file (path comes from the Stop hook payload), extracts the final assistant message + usage block. |
| `internal/emit` | Formats a `Summary` into text / json / stream-json output. The byte-for-byte parity contract lives here. |

### Critical control flow

1. `driver.Run` calls `hook.Create()` → temp dir + relay script + settings JSON.
2. PTY spawns `claude` with `--settings '<inline-json>'`. **Never touches `~/.claude/`.**
3. A reader goroutine pumps PTY output into a bounded rolling buffer. `terminal` scans it and writes ANSI responses back through the PTY.
4. On `SessionStart` hook fire → driver types the prompt + Enter into the PTY.
5. On `Stop` hook fire → payload contains `transcript_path`. Driver reads the JSONL, parses via `transcript`, then terminates the child.
6. `emit` writes the chosen format to stdout.

### Why the indirection between `claudep.Options` and `driver.Options`

The public `Options` uses zero values to mean "not set" (`Model string`), but the driver needs to distinguish "not set" from "explicitly empty" to build argv correctly. `claudep.Run` translates by setting paired `HasX bool` fields (`HasModel`, `HasMaxTurns`, …). Preserve this pattern when adding new flags.

## Platform gotchas

- **Windows requires Git Bash.** The hook relay is a `#!/bin/sh` script; `sh` must be on `PATH`. PTY I/O uses ConPTY via `github.com/aymanbagabas/go-pty` (Windows 10 1809+).
- **Process termination is split** by build tag — `terminate_unix.go` sends SIGTERM/SIGKILL, `terminate_windows.go` uses Job Objects. Edits to one must be mirrored.
- The original Zig project was POSIX-only. Windows support is a port-only feature; see [docs/windows-support-plan.md](docs/windows-support-plan.md) for the rationale.

## When changing output formatting

`internal/emit` produces output that downstream tools (jq pipelines, scripts) parse positionally. The README claims byte-for-byte parity with upstream `claude -p`. Any change here needs golden-file test updates in `internal/emit/emit_test.go` **and** verification that the three `examples/` programs still produce identical output.

## Versioning

`claudep.Version` in [claudep.go](claudep.go) is the single source of truth — bump it there when tagging a release. Existing tags: `v0.0.1` through `v0.0.4`.
