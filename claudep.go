// Package claudep is a drop-in replacement for `claude -p` that drives the
// interactive `claude` UI inside an in-process PTY session.
//
// Architecture is documented in SPEC.md (in the original Zig project). The
// high-level API is Run(opts), which spawns `claude`, feeds it the prompt,
// waits for the Stop hook, and returns a Result containing the assistant's
// final message plus telemetry.
package claudep

import (
	"io"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/driver"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

// OutputFormat selects the wire format used by Result.Write.
type OutputFormat = args.OutputFormat

const (
	FormatText       = args.FormatText
	FormatJSON       = args.FormatJSON
	FormatStreamJSON = args.FormatStreamJSON
)

// Usage mirrors the transcript usage block.
type Usage = transcript.Usage

// Summary is the parsed view of a single claude-p run.
type Summary = transcript.Summary

// Options configures Run.
type Options struct {
	// Prompt is the prompt text sent to claude.
	Prompt string
	// OutputFormat selects how Result.Write emits.
	OutputFormat OutputFormat
	// Model — forwarded to `claude --model` when set.
	Model string
	// MaxTurns — forwarded to `claude --max-turns` when set.
	MaxTurns uint32
	// AllowedTools — forwarded to `claude --allowedTools` when set.
	AllowedTools string
	// SkipPermissions forwards --dangerously-skip-permissions.
	SkipPermissions bool
	// ResumeSession — forwarded to `claude --resume` when set.
	ResumeSession string
	// Continue forwards --continue.
	Continue bool
	// SessionID — forwarded to `claude --session-id` when set.
	SessionID string
	// Cwd sets the child working directory.
	Cwd string
	// ExtraArgs are forwarded verbatim to claude after the known flags.
	ExtraArgs []string
	// Verbose forwards --verbose.
	Verbose bool
	// TimeoutMs caps wall time waiting for the Stop hook. Default 300_000.
	TimeoutMs uint64
	// ClaudePath overrides the `claude` binary used (testing).
	ClaudePath string
	// Cols and Rows control the PTY size. Default 120x40.
	Cols uint16
	Rows uint16
	// Debug enables debug logs on stderr.
	Debug bool
}

// Result is the output of Run.
type Result struct {
	r *driver.Result
}

// Summary returns the parsed transcript summary.
func (r *Result) Summary() Summary { return r.r.Summary }

// DurationMs returns wall-clock duration from Run entry to Stop hook.
func (r *Result) DurationMs() uint64 { return r.r.DurationMs }

// Write formats the result onto w using the requested format.
func (r *Result) Write(w io.Writer, format OutputFormat) error {
	return r.r.Write(w, format)
}

// ExitCode is 0 on success, 1 if the model reported an error.
func (r *Result) ExitCode() int { return r.r.ExitCode() }

// Version is the package version.
const Version = "0.0.4"

// Sentinel errors mirror driver's so callers can errors.Is them.
var (
	ErrSessionStartTimeout   = driver.ErrSessionStartTimeout
	ErrStopTimeout           = driver.ErrStopTimeout
	ErrTranscriptUnavailable = driver.ErrTranscriptUnavailable
	ErrSpawnFailed           = driver.ErrSpawnFailed
	ErrNoPromptSupplied      = driver.ErrNoPromptSupplied
)

// Run spawns claude under a PTY, sends the prompt, waits for Stop, and
// returns a Result.
func Run(opts Options) (*Result, error) {
	dopts := driver.Options{
		Prompt:           opts.Prompt,
		OutputFormat:     opts.OutputFormat,
		Model:            opts.Model,
		HasModel:         opts.Model != "",
		MaxTurns:         opts.MaxTurns,
		HasMaxTurns:      opts.MaxTurns != 0,
		AllowedTools:     opts.AllowedTools,
		HasAllowedTools:  opts.AllowedTools != "",
		SkipPermissions:  opts.SkipPermissions,
		ResumeSession:    opts.ResumeSession,
		HasResumeSession: opts.ResumeSession != "",
		Continue:         opts.Continue,
		SessionID:        opts.SessionID,
		HasSessionID:     opts.SessionID != "",
		Cwd:              opts.Cwd,
		HasCwd:           opts.Cwd != "",
		ExtraArgs:        opts.ExtraArgs,
		Verbose:          opts.Verbose,
		TimeoutMs:        opts.TimeoutMs,
		ClaudePath:       opts.ClaudePath,
		Cols:             opts.Cols,
		Rows:             opts.Rows,
		Debug:            opts.Debug,
	}
	r, err := driver.Run(dopts)
	if err != nil {
		return nil, err
	}
	return &Result{r: r}, nil
}
