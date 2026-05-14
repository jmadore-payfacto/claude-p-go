// Command json demonstrates --output-format json: a single result envelope
// is written to stdout, byte-identical to what `claude -p --output-format
// json` would produce. The same envelope is also accessible programmatically
// via result.Summary() — usage totals, cost, session id, etc.
//
// It requires `claude` on $PATH and a live Claude Code login.
//
// Usage:
//
//	go run ./examples/json "summarize Go's net/http package in 2 sentences"
//	go run ./examples/json "..." | jq .usage
package main

import (
	"fmt"
	"os"
	"strings"

	claudep "github.com/jmadore-payfacto/claude-p-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: json <prompt>")
		os.Exit(2)
	}
	prompt := strings.Join(os.Args[1:], " ")

	result, err := claudep.Run(claudep.Options{
		Prompt:          prompt,
		OutputFormat:    claudep.FormatJSON,
		SkipPermissions: true,
		TimeoutMs:       120_000,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-p: %v\n", err)
		os.Exit(1)
	}

	// Stream the result envelope to stdout. Pipe to `jq` to slice it.
	if err := result.Write(os.Stdout, claudep.FormatJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	// And/or grab the same fields programmatically.
	s := result.Summary()
	fmt.Fprintf(os.Stderr,
		"\n[in=%d out=%d cache_read=%d cache_creation=%d]\n",
		s.Usage.InputTokens, s.Usage.OutputTokens,
		s.Usage.CacheReadInputTokens, s.Usage.CacheCreationInputTokens,
	)

	os.Exit(result.ExitCode())
}
