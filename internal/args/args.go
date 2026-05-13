// Package args parses the claude-p CLI surface. It mirrors a useful subset of
// `claude -p` and forwards unknown flags through to the child invocation.
package args

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
	FormatStreamJSON
)

func ParseOutputFormat(s string) (OutputFormat, bool) {
	switch s {
	case "text":
		return FormatText, true
	case "json":
		return FormatJSON, true
	case "stream-json":
		return FormatStreamJSON, true
	}
	return 0, false
}

func (f OutputFormat) String() string {
	switch f {
	case FormatJSON:
		return "json"
	case FormatStreamJSON:
		return "stream-json"
	default:
		return "text"
	}
}

var (
	ErrBadOutputFormat = errors.New("bad output format")
	ErrMissingValue    = errors.New("missing value for flag")
	ErrUnknownFlag     = errors.New("unknown flag")
	ErrBadInteger      = errors.New("bad integer")
)

const defaultTimeoutSeconds = 300

type Options struct {
	Prompt                       string
	HasPrompt                    bool
	InputFile                    string
	OutputFormat                 OutputFormat
	Model                        string
	HasModel                     bool
	MaxTurns                     uint32
	HasMaxTurns                  bool
	AllowedTools                 string
	HasAllowedTools              bool
	DangerouslySkipPermissions   bool
	ResumeSession                string
	HasResumeSession             bool
	Continue                     bool
	SessionID                    string
	HasSessionID                 bool
	Cwd                          string
	HasCwd                       bool
	Verbose                      bool
	TimeoutSeconds               uint32
	Debug                        bool
	ShowHelp                     bool
	ShowVersion                  bool
	Passthrough                  []string
}

const helpText = `Usage: claude-p [OPTIONS] [PROMPT]

Emulates ` + "`claude -p`" + ` by driving the interactive ` + "`claude`" + ` binary inside
an in-process PTY session and capturing the final assistant message via a
Stop hook.

Options:
  --output-format <fmt>           text | json | stream-json (default: text)
  --model <name>                  Forwarded to ` + "`claude --model`" + `
  --max-turns <N>                 Abort after N assistant turns
  --allowedTools <list>           Permission-rule list
  --dangerously-skip-permissions  Bypass permission prompts
  --resume <id>                   Resume a session
  --continue, -c                  Continue the most recent session
  --session-id <uuid>             Use a specific session UUID
  --cwd <path>                    Working directory for ` + "`claude`" + `
  --input-file <path>             Read prompt from a file
  --verbose                       Verbose output (required for stream-json)
  --timeout <seconds>             Wrapper wall-time cap (default: 300)
  --debug                         Wrapper debug logs to stderr
  -h, --help                      Print this help
  -v, --version                   Print version

Unrecognized flags are forwarded verbatim to ` + "`claude`" + `.
`

func HelpText() string { return helpText }

// Parse argv (already stripped of argv[0]).
func Parse(argv []string) (Options, error) {
	opts := Options{
		OutputFormat:   FormatText,
		TimeoutSeconds: defaultTimeoutSeconds,
	}

	needValue := func(i int, flag string) (string, error) {
		if i >= len(argv) {
			return "", fmt.Errorf("%w: %s", ErrMissingValue, flag)
		}
		return argv[i], nil
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			opts.ShowHelp = true
		case a == "-v" || a == "--version":
			opts.ShowVersion = true
		case a == "--output-format":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			f, ok := ParseOutputFormat(v)
			if !ok {
				return opts, fmt.Errorf("%w: %s", ErrBadOutputFormat, v)
			}
			opts.OutputFormat = f
		case strings.HasPrefix(a, "--output-format="):
			f, ok := ParseOutputFormat(a[len("--output-format="):])
			if !ok {
				return opts, fmt.Errorf("%w: %s", ErrBadOutputFormat, a)
			}
			opts.OutputFormat = f
		case a == "--model":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.Model, opts.HasModel = v, true
		case strings.HasPrefix(a, "--model="):
			opts.Model, opts.HasModel = a[len("--model="):], true
		case a == "--max-turns":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return opts, fmt.Errorf("%w: %s", ErrBadInteger, v)
			}
			opts.MaxTurns, opts.HasMaxTurns = uint32(n), true
		case a == "--allowedTools" || a == "--allowed-tools":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.AllowedTools, opts.HasAllowedTools = v, true
		case a == "--dangerously-skip-permissions":
			opts.DangerouslySkipPermissions = true
		case a == "--resume":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.ResumeSession, opts.HasResumeSession = v, true
		case a == "--continue" || a == "-c":
			opts.Continue = true
		case a == "--session-id":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.SessionID, opts.HasSessionID = v, true
		case a == "--cwd":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.Cwd, opts.HasCwd = v, true
		case a == "--input-file":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			opts.InputFile = v
		case a == "--verbose":
			opts.Verbose = true
		case a == "--debug":
			opts.Debug = true
		case a == "--timeout":
			i++
			v, err := needValue(i, a)
			if err != nil {
				return opts, err
			}
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return opts, fmt.Errorf("%w: %s", ErrBadInteger, v)
			}
			opts.TimeoutSeconds = uint32(n)
		case strings.HasPrefix(a, "--"):
			opts.Passthrough = append(opts.Passthrough, a)
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				opts.Passthrough = append(opts.Passthrough, argv[i])
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			opts.Passthrough = append(opts.Passthrough, a)
		case !opts.HasPrompt:
			opts.Prompt, opts.HasPrompt = a, true
		default:
			return opts, fmt.Errorf("%w: %s", ErrUnknownFlag, a)
		}
	}
	return opts, nil
}
