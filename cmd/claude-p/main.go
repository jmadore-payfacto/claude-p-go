// Command claude-p is a drop-in replacement for `claude -p` that drives the
// interactive `claude` binary under an in-process PTY session.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	claudep "github.com/jmadore-payfacto/claude-p-go"
	"github.com/jmadore-payfacto/claude-p-go/internal/args"
)

const (
	exitSuccess      = 0
	exitWrapperError = 2
	exitTimeout      = 124
	msPerSecond      = 1000
	maxPromptBytes   = 16 * 1024 * 1024
)

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func realMain(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := args.Parse(argv)
	if err != nil {
		fmt.Fprintf(stderr, "claude-p: bad arguments: %v\n", err)
		return exitWrapperError
	}

	if opts.ShowHelp {
		fmt.Fprint(stdout, args.HelpText())
		return exitSuccess
	}
	if opts.ShowVersion {
		fmt.Fprintf(stdout, "claude-p %s\n", claudep.Version)
		return exitSuccess
	}

	prompt, err := resolvePrompt(opts, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "claude-p: %v\n", err)
		return exitWrapperError
	}
	if prompt == "" {
		fmt.Fprintln(stderr, "error: empty prompt (positional, --input-file, or stdin required)")
		return exitWrapperError
	}

	result, err := claudep.Run(claudep.Options{
		Prompt:          prompt,
		OutputFormat:    opts.OutputFormat,
		Model:           opts.Model,
		MaxTurns:        opts.MaxTurns,
		AllowedTools:    opts.AllowedTools,
		SkipPermissions: opts.DangerouslySkipPermissions,
		ResumeSession:   opts.ResumeSession,
		Continue:        opts.Continue,
		SessionID:       opts.SessionID,
		Cwd:             opts.Cwd,
		ExtraArgs:       opts.Passthrough,
		Verbose:         opts.Verbose,
		TimeoutMs:       uint64(opts.TimeoutSeconds) * msPerSecond,
		Debug:           opts.Debug,
	})
	if err != nil {
		fmt.Fprintf(stderr, "claude-p: %v\n", err)
		return mapErrorExit(err)
	}

	if err := result.Write(stdout, opts.OutputFormat); err != nil {
		fmt.Fprintf(stderr, "claude-p: %v\n", err)
		return exitWrapperError
	}
	return result.ExitCode()
}

func resolvePrompt(opts args.Options, stdin io.Reader) (string, error) {
	if opts.HasPrompt {
		return opts.Prompt, nil
	}
	var src io.Reader
	if opts.InputFile != "" {
		f, err := os.Open(opts.InputFile)
		if err != nil {
			return "", err
		}
		defer f.Close()
		src = f
	} else {
		src = stdin
	}
	data, err := io.ReadAll(io.LimitReader(src, maxPromptBytes))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func mapErrorExit(err error) int {
	if errors.Is(err, claudep.ErrSessionStartTimeout) || errors.Is(err, claudep.ErrStopTimeout) {
		return exitTimeout
	}
	return exitWrapperError
}
