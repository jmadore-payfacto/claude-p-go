// Command stream-json demonstrates --output-format stream-json: the
// transcript JSONL is replayed verbatim, then a single final `result`
// envelope is appended — matching `claude -p --output-format stream-json
// --verbose` byte-for-byte.
//
// Two consumer patterns are shown:
//
//  1. Stream the wire format to stdout, suitable for piping to `jq -c .`
//     or downstream processes that expect the claude-p JSONL contract.
//  2. Iterate the replay programmatically via result.Summary().JSONLReplay
//     to inspect each event in-process — useful when you want per-turn
//     telemetry or want to filter tool_use events.
//
// It requires `claude` on $PATH and a live Claude Code login.
//
// Usage:
//
//	go run ./examples/stream-json "explain quicksort"
//	go run ./examples/stream-json "..." | jq -c 'select(.type=="assistant")'
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	claudep "github.com/jmadore-payfacto/claude-p-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: stream-json <prompt>")
		os.Exit(2)
	}
	prompt := strings.Join(os.Args[1:], " ")

	result, err := claudep.Run(claudep.Options{
		Prompt:          prompt,
		OutputFormat:    claudep.FormatStreamJSON,
		SkipPermissions: true,
		Verbose:         true, // stream-json requires verbose, like claude -p
		TimeoutMs:       120_000,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-p: %v\n", err)
		os.Exit(1)
	}

	// Pattern 1: emit the wire format to stdout.
	if err := result.Write(os.Stdout, claudep.FormatStreamJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	// Pattern 2: walk the replay in-process. Count event types and emit a
	// terse breakdown to stderr.
	counts := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(result.Summary().JSONLReplay))
	for scanner.Scan() {
		var ev struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		counts[ev.Type]++
	}
	fmt.Fprintf(os.Stderr, "\n[events: %v]\n", counts)

	os.Exit(result.ExitCode())
}
