// Command text demonstrates programmatic access to a Run result, printing
// just the assistant's final text plus a one-line telemetry summary to
// stderr.
//
// It requires `claude` on $PATH and a live Claude Code login.
//
// Usage:
//
//	go run ./examples/text "what is the capital of France?"
package main

import (
	"fmt"
	"os"
	"strings"

	claudep "github.com/jmadore-payfacto/claude-p-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: text <prompt>")
		os.Exit(2)
	}
	prompt := strings.Join(os.Args[1:], " ")

	result, err := claudep.Run(claudep.Options{
		Prompt:          prompt,
		OutputFormat:    claudep.FormatText,
		SkipPermissions: true,
		TimeoutMs:       120_000,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-p: %v\n", err)
		os.Exit(1)
	}

	summary := result.Summary()
	fmt.Println(summary.FinalText)

	fmt.Fprintf(os.Stderr,
		"\n[turns=%d  cost=$%.4f  duration=%dms  session=%s]\n",
		summary.NumTurns, summary.TotalCostUSD, result.DurationMs(), summary.SessionID,
	)

	os.Exit(result.ExitCode())
}
