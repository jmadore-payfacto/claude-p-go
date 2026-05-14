package driver

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	fakeClaudeOnce sync.Once
	fakeClaudePath string
	fakeClaudeErr  error
)

// buildFakeClaude compiles cmd/fake-claude into a temp binary the test can
// pass as opts.ClaudePath. Built once per test run.
func buildFakeClaude(t *testing.T) string {
	t.Helper()
	fakeClaudeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fake-claude-build-*")
		if err != nil {
			fakeClaudeErr = err
			return
		}
		out := filepath.Join(dir, "fake-claude")
		cmd := exec.Command("go", "build", "-o", out, "github.com/jmadore-payfacto/claude-p-go/cmd/fake-claude")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fakeClaudeErr = err
			return
		}
		fakeClaudePath = out
	})
	if fakeClaudeErr != nil {
		t.Fatalf("build fake-claude: %v", fakeClaudeErr)
	}
	return fakeClaudePath
}

func TestRunEndToEndHermetic(t *testing.T) {
	bin := buildFakeClaude(t)
	t.Setenv("FAKE_CLAUDE_REPLY", "pong")
	t.Setenv("FAKE_CLAUDE_SESSION_ID", "hermetic-1")

	res, err := Run(Options{
		Prompt:     "ping",
		ClaudePath: bin,
		TimeoutMs:  15000,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary.FinalText != "pong" {
		t.Fatalf("FinalText=%q want %q", res.Summary.FinalText, "pong")
	}
	if res.Summary.SessionID != "hermetic-1" {
		t.Fatalf("SessionID=%q", res.Summary.SessionID)
	}
	if res.Summary.IsError {
		t.Fatal("IsError true")
	}
	if res.ExitCode() != 0 {
		t.Fatalf("ExitCode=%d", res.ExitCode())
	}
}

func TestRunEndToEndTrustDialog(t *testing.T) {
	bin := buildFakeClaude(t)
	t.Setenv("FAKE_CLAUDE_TRUST_PROMPT", "1")
	t.Setenv("FAKE_CLAUDE_REPLY", "trusted")

	res, err := Run(Options{
		Prompt:     "hi",
		ClaudePath: bin,
		TimeoutMs:  15000,
	})
	if err != nil {
		t.Fatalf("Run with trust prompt: %v", err)
	}
	if res.Summary.FinalText != "trusted" {
		t.Fatalf("FinalText=%q", res.Summary.FinalText)
	}
}

func TestRunEndToEndReplyContainsPrompt(t *testing.T) {
	// With no FAKE_CLAUDE_REPLY override, fake-claude echoes the prompt
	// back. This verifies the parent→PTY prompt write path end-to-end.
	bin := buildFakeClaude(t)
	res, err := Run(Options{
		Prompt:     "echo-this-back",
		ClaudePath: bin,
		TimeoutMs:  15000,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Summary.FinalText, "echo-this-back") {
		t.Fatalf("reply missing prompt echo: %q", res.Summary.FinalText)
	}
}
