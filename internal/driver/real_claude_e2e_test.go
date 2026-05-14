package driver

import (
	"os"
	"testing"
)

// TestRunRealClaude exercises Run against the actual `claude` binary on
// $PATH. Requires a live Claude Code login. Gated by CLAUDE_P_E2E=1 so
// `go test ./...` stays hermetic.
func TestRunRealClaude(t *testing.T) {
	if os.Getenv("CLAUDE_P_E2E") != "1" {
		t.Skip("set CLAUDE_P_E2E=1 to run against the real `claude` binary")
	}

	res, err := Run(Options{
		Prompt:          "Reply with exactly the word: pong",
		SkipPermissions: true,
		TimeoutMs:       120000,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary.FinalText == "" {
		t.Fatal("FinalText empty")
	}
	if res.Summary.IsError {
		t.Fatalf("IsError true: %q", res.Summary.FinalText)
	}
}
