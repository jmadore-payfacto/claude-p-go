# Windows support plan

## Scope

Add Windows support to `claude-p-go`, **scoped to Windows environments that have
Git Bash installed**. Native Windows Claude Code runs without Git Bash, so this
is an explicit, documented limitation — a Windows user without Git Bash gets
broken hooks.

## Goal

Replace the FIFO + Unix-syscall hook relay with a plain append-file relay, swap
`creack/pty` for a cross-platform PTY library (Unix PTY + Windows ConPTY), and
isolate the one remaining OS-specific call behind build tags. The wire protocol
(`<event>\t<payload>\n` lines, `hook.ParseLine`) is **unchanged** throughout.

The work is sequenced so macOS/Linux stay green at every step and Windows
compiles clean by Step 3.

## Background

All platform-specific code lives in two files: `internal/driver/driver.go` and
`internal/hook/hook.go`. Everything else (`args`, `emit`, `transcript`,
`terminal`, `claudep.go`, `main.go`) is portable Go.

Two real blockers:

1. **PTY** — `github.com/creack/pty` has no Windows support. Windows has ConPTY
   (Win10 1809+); a cross-platform library abstracts both.
2. **FIFO + sh relay** — `unix.Mkfifo` won't compile for `GOOS=windows`, and
   MSYS2 FIFOs only interoperate between MSYS2 processes (not a native
   `claude-p.exe`). Git Bash lets the `#!/bin/sh` hook script run unchanged, but
   the FIFO itself must become a plain append-file the native parent can tail.

Doc check confirmed: shell-form hooks spawn `sh -c` on Unix and **Git Bash on
Windows by default**; hook payload is JSON on stdin on every OS. Sources:
- https://code.claude.com/docs/en/hooks-guide.md
- https://code.claude.com/docs/en/hooks.md

## Steps

### Step 1 — Portability nits (no behavior change, all platforms)

- `hook.go` `tmpRoot()` → `os.TempDir()` (handles Windows `TEMP`/`TMP`).
- Split `terminateChild` into build-tagged files:
  - `internal/driver/terminate_unix.go` (`//go:build !windows`) — current
    SIGTERM-then-Kill logic.
  - `internal/driver/terminate_windows.go` (`//go:build windows`) —
    `cmd.Process.Kill()` (no console SIGTERM on Windows).
- **Verify:** `go test ./...` green on macOS; `GOOS=windows go build ./...` gets
  further (still fails on `unix`/`creack`).

### Step 2 — FIFO → append-file relay (Unix-first; keeps macOS working)

**hook package** (`internal/hook/hook.go`):

- Drop `unix.Mkfifo` and the `golang.org/x/sys/unix` import.
- `Create()` makes the events file empty up front:
  `os.WriteFile(eventsPath, nil, 0o600)` — so the parent can open it before any
  hook fires.
- `scriptBody`: `>> "$fifo"` → `>> "$file"` to a regular file. Otherwise
  identical — `printf >> file` works in every `sh`, Git Bash included.
- `filepath.ToSlash` the script/events paths baked into the script and
  `--settings` (backslashes are escape chars in `sh`).
- Rename `Harness.FifoPath` → `EventsPath`, env var `CLAUDE_P_FIFO` →
  `CLAUDE_P_EVENTS`, update `Cleanup()`.

**driver package** (`internal/driver/driver.go`):

- `openFifo()` → `openEventsFile()`: `os.Open(path)` instead of
  `syscall.Open(O_RDONLY|O_NONBLOCK)`. Returns `*os.File`.
- `drainFIFO()` → `drainEvents()`: `eventsFile.Read(buf)` instead of
  `syscall.Read(fd, buf)`. Repeated `Read` on a held `*os.File` returns new
  bytes as the file grows (standard tail-poll pattern) — `n == 0` / `io.EOF`
  just means "caught up", same as today's `n <= 0`. Line-buffering logic
  unchanged.
- Drop `syscall.Open/Read/Close` and the `syscall.Close` defer.
- This relay code is now cross-platform and shared — no build tags.

*Note:* `SessionStart` and `Stop` are temporally separated, so concurrent
appends aren't a practical concern; single-line `printf` appends are effectively
atomic for our payload sizes. Won't over-engineer this.

- **Verify:** `go test ./...` green on macOS — hermetic e2e passing proves the
  protocol survives the FIFO→file swap. `GOOS=windows go build ./...` now fails
  *only* on `creack/pty`.

### Step 3 — Swap PTY library → cross-platform ConPTY

- `go get github.com/aymanbagabas/go-pty`, drop `creack/pty`. Single dependency,
  covers Unix PTY + Windows ConPTY — no build tags needed for spawn itself.
- Rewrite `spawnClaude`: `go-pty` uses `pty.New()` → `pty.Command(name, args...)`
  → `Start()`, with `Env`/`Dir` on its own `Cmd` type. **API differs from
  `creack/pty` — exact surface to be confirmed at implementation.**
- Change `ptyFile *os.File` params in `ptyReaderLoop`, `flushPendingToPTY`,
  `sendPrompt`, `driveSession` to the `pty.Pty` type (an `io.ReadWriteCloser`)
  or a narrow `io.ReadWriter` interface.
- Point `terminateChild` at the go-pty `Cmd`'s process handle.
- **Verify:** `go test ./...` green on macOS; `GOOS=windows go build ./...`
  compiles clean.

### Step 4 — Docs + CI guard

- README: add a "Windows support" section stating the hard Git Bash requirement.
- Add `GOOS=windows go build ./...` to CI as a cross-compile regression guard
  (check whether CI config exists first).
- **Verify:** cross-build passes in CI.

### Step 5 — Real Windows smoke test (manual / Windows runner)

- On a Windows box with Git Bash: `claude-p -p "ping"` end-to-end. Validates
  ConPTY drives the Ink UI, Git Bash runs the hook script, and the native parent
  can tail the events file while Git Bash appends to it (Windows file-sharing).
- Cannot be fully automated without a Windows CI runner.

## Files touched

- `internal/hook/hook.go`
- `internal/driver/driver.go` (+ new `terminate_unix.go`, `terminate_windows.go`)
- `go.mod` / `go.sum`
- `README.md`
- CI config
- Test fixups in `hook_test.go`, `driver_test.go`, `hermetic_e2e_test.go` for
  renamed symbols.
- `cmd/fake-claude/main.go` already runs the hook via `sh -c` — consistent with
  the Git Bash policy, no change needed.

## Open items

- `go-pty`'s exact API (Step 3) — verify against its docs when implementing.
