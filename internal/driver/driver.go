// Package driver is the end-to-end engine: spawn `claude` under a PTY, drive
// the UI with our prompt, wait for the Stop hook, and return a Result.
package driver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/emit"
	"github.com/jmadore-payfacto/claude-p-go/internal/hook"
	"github.com/jmadore-payfacto/claude-p-go/internal/terminal"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

// Options mirrors the public claudep.Options 1:1.
type Options struct {
	Prompt           string
	OutputFormat     args.OutputFormat
	Model            string
	HasModel         bool
	MaxTurns         uint32
	HasMaxTurns      bool
	AllowedTools     string
	HasAllowedTools  bool
	SkipPermissions  bool
	ResumeSession    string
	HasResumeSession bool
	Continue         bool
	SessionID        string
	HasSessionID     bool
	Cwd              string
	HasCwd           bool
	ExtraArgs        []string
	Verbose          bool
	TimeoutMs        uint64
	ClaudePath       string // override `claude` (testing)
	Cols             uint16
	Rows             uint16
	Debug            bool
}

// Result is what Run returns on success.
type Result struct {
	Summary    transcript.Summary
	DurationMs uint64
}

// Write formats the result onto w.
func (r *Result) Write(w io.Writer, format args.OutputFormat) error {
	return emit.Emit(w, format, emit.Envelope{Summary: &r.Summary, DurationMs: r.DurationMs})
}

// ExitCode returns 1 if the result is an error, else 0.
func (r *Result) ExitCode() int {
	if r.Summary.IsError {
		return 1
	}
	return 0
}

var (
	ErrSessionStartTimeout  = errors.New("session-start timeout")
	ErrStopTimeout          = errors.New("stop timeout")
	ErrTranscriptUnavailable = errors.New("transcript unavailable")
	ErrSpawnFailed          = errors.New("spawn failed")
	ErrNoPromptSupplied     = errors.New("no prompt supplied")
)

// BuildArgv builds the argv passed to the child `claude` process.
func BuildArgv(binary, settingsJSON string, opts Options) []string {
	argv := []string{binary, "--settings", settingsJSON}
	if opts.HasModel {
		argv = append(argv, "--model", opts.Model)
	}
	if opts.HasMaxTurns {
		argv = append(argv, "--max-turns", strconv.FormatUint(uint64(opts.MaxTurns), 10))
	}
	if opts.HasAllowedTools {
		argv = append(argv, "--allowedTools", opts.AllowedTools)
	}
	if opts.SkipPermissions {
		argv = append(argv, "--dangerously-skip-permissions")
	}
	if opts.HasResumeSession {
		argv = append(argv, "--resume", opts.ResumeSession)
	}
	if opts.Continue {
		argv = append(argv, "--continue")
	}
	if opts.HasSessionID {
		argv = append(argv, "--session-id", opts.SessionID)
	}
	if opts.Verbose {
		argv = append(argv, "--verbose")
	}
	argv = append(argv, opts.ExtraArgs...)
	return argv
}

// shared state between PTY reader goroutine and main loop
type sharedState struct {
	debug bool

	writeMu        sync.Mutex
	pendingToPTY   []byte // bytes the DEC responder wants written back

	exited atomic.Bool

	recentMu sync.Mutex
	recent   []byte // rolling buffer for trust-dialog detection

	trustDismissed bool
}

const (
	recentCapacity          = 8192
	defaultCols             = 120
	defaultRows             = 40
	defaultTimeoutMs        = 300_000
	terminateGrace          = 200 * time.Millisecond
	inkInitWait             = 1500 * time.Millisecond
	promptSubmitGap         = 120 * time.Millisecond
	mainPollInterval        = 5 * time.Millisecond
	transcriptRetries       = 40
	transcriptRetryInterval = 50 * time.Millisecond
	ptyReadBufferSize       = 4096
	fifoReadBufferSize      = 4096
)

// Run spawns claude under a PTY, drives the UI, and returns a Result.
func Run(opts Options) (*Result, error) {
	if opts.Prompt == "" {
		return nil, ErrNoPromptSupplied
	}
	applyDefaults(&opts)

	harness, err := hook.Create()
	if err != nil {
		return nil, fmt.Errorf("hook setup: %w", err)
	}
	defer harness.Cleanup()

	fifoFD, err := openFifo(harness.FifoPath)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fifoFD)

	cmd, ptyFile, err := spawnClaude(harness, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ptyFile.Close() }()

	shared := &sharedState{debug: opts.Debug}
	go ptyReaderLoop(ptyFile, shared, opts.Debug)
	defer terminateChild(cmd)

	start := time.Now()
	transcriptPath, stopPayload, err := driveSession(ptyFile, fifoFD, shared, opts, start)
	if err != nil {
		return nil, err
	}

	summary, err := loadSummary(transcriptPath, stopPayload)
	if err != nil {
		return nil, err
	}

	return &Result{
		Summary:    summary,
		DurationMs: uint64(time.Since(start) / time.Millisecond),
	}, nil
}

func applyDefaults(opts *Options) {
	if opts.Cols == 0 {
		opts.Cols = defaultCols
	}
	if opts.Rows == 0 {
		opts.Rows = defaultRows
	}
	if opts.TimeoutMs == 0 {
		opts.TimeoutMs = defaultTimeoutMs
	}
}

// openFifo opens the FIFO for reading BEFORE spawning the child so its hook
// never blocks trying to open the write side.
func openFifo(path string) (int, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return 0, fmt.Errorf("%w: open fifo: %v", ErrSpawnFailed, err)
	}
	return fd, nil
}

func spawnClaude(harness *hook.Harness, opts Options) (*exec.Cmd, *os.File, error) {
	binary := opts.ClaudePath
	if binary == "" {
		binary = "claude"
	}
	argv := BuildArgv(binary, harness.SettingsJSON, opts)

	env := append(os.Environ(),
		"CLAUDE_P_FIFO="+harness.FifoPath,
		"TERM=xterm-256color",
	)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	if opts.HasCwd {
		cmd.Dir = opts.Cwd
	}

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: opts.Rows,
		Cols: opts.Cols,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrSpawnFailed, err)
	}
	return cmd, ptyFile, nil
}

// ptyReaderLoop scans incoming PTY bytes for DEC queries, queues responses,
// and maintains a rolling buffer of recent output for trust-dialog detection.
func ptyReaderLoop(ptyFile *os.File, shared *sharedState, debug bool) {
	buf := make([]byte, ptyReadBufferSize)
	for {
		n, err := ptyFile.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			var resp []byte
			terminal.RespondToDecQueries(chunk, &resp)
			if len(resp) > 0 {
				shared.writeMu.Lock()
				shared.pendingToPTY = append(shared.pendingToPTY, resp...)
				shared.writeMu.Unlock()
			}
			shared.recentMu.Lock()
			shared.recent = append(shared.recent, chunk...)
			if len(shared.recent) > recentCapacity {
				drop := len(shared.recent) - recentCapacity
				shared.recent = shared.recent[drop:]
			}
			shared.recentMu.Unlock()
			if debug {
				fmt.Fprintf(os.Stderr, "pty read: %d bytes\n", n)
			}
		}
		if err != nil {
			shared.exited.Store(true)
			return
		}
	}
}

func terminateChild(cmd *exec.Cmd) {
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(terminateGrace):
		_ = cmd.Process.Kill()
		<-done
	}
}

// driveSession runs the main loop until a Stop hook delivers a transcript
// path (or its `last_assistant_message` payload). Returns the transcript
// path and the raw Stop payload (either may be empty if the other isn't).
func driveSession(ptyFile *os.File, fifoFD int, shared *sharedState, opts Options, start time.Time) (string, string, error) {
	state := stateWaitingForReady
	var (
		fifoBuf          []byte
		fifoRead         [fifoReadBufferSize]byte
		transcriptPath   string
		stopPayloadOwned string
	)

	for {
		if time.Since(start) > time.Duration(opts.TimeoutMs)*time.Millisecond {
			if state == stateWaitingForReady {
				return "", "", ErrSessionStartTimeout
			}
			return "", "", ErrStopTimeout
		}
		if shared.exited.Load() && state == stateWaitingForReady {
			return "", "", ErrSpawnFailed
		}

		flushPendingToPTY(ptyFile, shared)
		checkTrustDialog(ptyFile, shared, state, opts.Debug)

		newState, path, payload := drainFIFO(ptyFile, fifoFD, fifoRead[:], &fifoBuf, state, opts)
		state = newState
		if path != "" {
			transcriptPath = path
		}
		if payload != "" {
			stopPayloadOwned = payload
		}

		if transcriptPath != "" {
			return transcriptPath, stopPayloadOwned, nil
		}
		time.Sleep(mainPollInterval)
	}
}

func flushPendingToPTY(ptyFile *os.File, shared *sharedState) {
	shared.writeMu.Lock()
	var toWrite []byte
	if len(shared.pendingToPTY) > 0 {
		toWrite = append([]byte(nil), shared.pendingToPTY...)
		shared.pendingToPTY = shared.pendingToPTY[:0]
	}
	shared.writeMu.Unlock()
	if len(toWrite) > 0 {
		_, _ = ptyFile.Write(toWrite)
	}
}

// checkTrustDialog dismisses claude's workspace-trust prompt with Enter.
// The dialog appears in unfamiliar directories *before* SessionStart hooks
// register and is not bypassed by --dangerously-skip-permissions. Default
// selection is "Yes, I trust this folder".
func checkTrustDialog(ptyFile *os.File, shared *sharedState, state sessionState, debug bool) {
	if shared.trustDismissed || state != stateWaitingForReady {
		return
	}
	shared.recentMu.Lock()
	stripped := stripCSI(shared.recent)
	shared.recentMu.Unlock()
	// After stripping CSI, words are concatenated (the dialog pads with
	// `\033[1C` cursor-move, not real spaces). Search for two distinct
	// single-word markers being present together.
	if !bytes.Contains(stripped, []byte("trust")) || !bytes.Contains(stripped, []byte("folder")) {
		return
	}
	if debug {
		fmt.Fprintln(os.Stderr, "trust dialog detected — sending Enter")
	}
	_, _ = ptyFile.Write([]byte("\r"))
	shared.trustDismissed = true
}

// drainFIFO reads pending hook events and processes them. Returns the
// (possibly updated) session state, transcript path, and Stop payload.
func drainFIFO(ptyFile *os.File, fifoFD int, readBuf []byte, fifoBuf *[]byte, state sessionState, opts Options) (sessionState, string, string) {
	n, _ := syscall.Read(fifoFD, readBuf)
	if n == 0 {
		return state, "", ""
	}
	*fifoBuf = append(*fifoBuf, readBuf[:n]...)
	var (
		transcriptPath string
		stopPayload    string
	)
	for {
		nl := bytes.IndexByte(*fifoBuf, '\n')
		if nl < 0 {
			break
		}
		line := string((*fifoBuf)[:nl])
		*fifoBuf = (*fifoBuf)[nl+1:]
		ev, ok := hook.ParseLine(line)
		if !ok {
			continue
		}
		if opts.Debug {
			fmt.Fprintf(os.Stderr, "hook: %s payload=%s\n", ev.Event, ev.Payload)
		}
		switch ev.Event {
		case hook.EventSessionStart:
			if state == stateWaitingForReady {
				state = sendPrompt(ptyFile, opts)
			}
		case hook.EventStop:
			if p, ok := hook.ExtractTranscriptPath(ev.Payload); ok {
				transcriptPath = p
			}
			stopPayload = ev.Payload
		}
		if transcriptPath != "" {
			break
		}
	}
	return state, transcriptPath, stopPayload
}

// sendPrompt types the prompt body, pauses, then sends Enter. Ink applies
// bracketed-paste / burst-input heuristics: if `\r` arrives in the same
// burst as the prompt body, it lands in the input buffer instead of
// triggering submit. The gap makes Ink see two events.
func sendPrompt(ptyFile *os.File, opts Options) sessionState {
	// Give Ink time to finish initialising.
	time.Sleep(inkInitWait)
	if opts.Debug {
		fmt.Fprintf(os.Stderr, "typing prompt (%d bytes)\n", len(opts.Prompt))
	}
	_, _ = ptyFile.Write([]byte(opts.Prompt))
	time.Sleep(promptSubmitGap)
	_, _ = ptyFile.Write([]byte("\r"))
	return stateAwaitingStop
}

// loadSummary reads the transcript with retry, falling back to the Stop
// payload's `last_assistant_message` if the transcript never materialises.
// The Stop hook can fire a few milliseconds before claude flushes the
// assistant message line into the transcript.
func loadSummary(transcriptPath, stopPayload string) (transcript.Summary, error) {
	if s, ok := readTranscriptWithRetry(transcriptPath); ok {
		return s, nil
	}
	if stopPayload == "" {
		return transcript.Summary{}, ErrTranscriptUnavailable
	}
	text, ok := hook.ExtractLastAssistantMessage(stopPayload)
	if !ok {
		return transcript.Summary{}, ErrTranscriptUnavailable
	}
	sid, _ := hook.ExtractSessionID(stopPayload)
	return transcript.Summary{
		FinalText: text,
		SessionID: sid,
		NumTurns:  1,
	}, nil
}

type sessionState int

const (
	stateWaitingForReady sessionState = iota
	stateAwaitingStop
)

func readTranscriptWithRetry(path string) (transcript.Summary, bool) {
	for range transcriptRetries {
		s, err := transcript.ParseFile(path)
		if err == nil && (len(s.FinalText) > 0 || s.IsError) {
			return s, true
		}
		time.Sleep(transcriptRetryInterval)
	}
	return transcript.Summary{}, false
}

// stripCSI strips CSI / OSC / DCS escape sequences, leaving only literal
// payload. Used to make plain-text substring matching robust against cursor-
// positioning escapes that pad words with `\033[1C`.
func stripCSI(in []byte) []byte {
	out := make([]byte, 0, len(in))
	i := 0
	for i < len(in) {
		b := in[i]
		if b != 0x1b {
			out = append(out, b)
			i++
			continue
		}
		if i+1 >= len(in) {
			break
		}
		next := in[i+1]
		switch next {
		case '[':
			i += 2
			for i < len(in) && in[i] >= 0x30 && in[i] <= 0x3f {
				i++
			}
			for i < len(in) && in[i] >= 0x20 && in[i] <= 0x2f {
				i++
			}
			if i < len(in) {
				i++ // final byte
			}
		case ']':
			i += 2
			for i < len(in) {
				if in[i] == 0x07 {
					i++
					break
				}
				if in[i] == 0x1b && i+1 < len(in) && in[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case 'P', 'X', '^', '_':
			i += 2
			for i < len(in) {
				if in[i] == 0x1b && i+1 < len(in) && in[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			i += 2
		}
	}
	return out
}
