package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	claudep "github.com/jmadore-payfacto/claude-p-go"
	"github.com/jmadore-payfacto/claude-p-go/internal/args"
)

func TestRealMainHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout missing help: %q", stdout.String())
	}
}

func TestRealMainVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(stdout.String(), "claude-p ") {
		t.Fatalf("stdout: %q", stdout.String())
	}
}

func TestRealMainBadArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--output-format", "yaml"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitWrapperError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "bad arguments") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestRealMainEmptyPrompt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain(nil, strings.NewReader(""), &stdout, &stderr)
	if code != exitWrapperError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "empty prompt") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestRealMainInputFileMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--input-file", "/no/such/path/here"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitWrapperError {
		t.Fatalf("exit %d", code)
	}
}

func TestResolvePromptPositional(t *testing.T) {
	got, err := resolvePrompt(args.Options{HasPrompt: true, Prompt: "hi"}, strings.NewReader("from-stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePromptStdin(t *testing.T) {
	got, err := resolvePrompt(args.Options{}, strings.NewReader("from-stdin\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-stdin" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePromptStdinTrimsCRLF(t *testing.T) {
	got, err := resolvePrompt(args.Options{}, strings.NewReader("hello\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePromptInputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(path, []byte("file-prompt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePrompt(args.Options{InputFile: path}, strings.NewReader("ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "file-prompt" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePromptInputFileMissing(t *testing.T) {
	_, err := resolvePrompt(args.Options{InputFile: "/no/such/file"}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolvePromptHonorsByteLimit(t *testing.T) {
	// Build a payload larger than maxPromptBytes; expect truncation, not error.
	oversize := strings.Repeat("a", maxPromptBytes+1024)
	got, err := resolvePrompt(args.Options{}, strings.NewReader(oversize))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxPromptBytes {
		t.Fatalf("len=%d want %d", len(got), maxPromptBytes)
	}
}

func TestMapErrorExit(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"session start timeout", claudep.ErrSessionStartTimeout, exitTimeout},
		{"stop timeout", claudep.ErrStopTimeout, exitTimeout},
		{"spawn failed", claudep.ErrSpawnFailed, exitWrapperError},
		{"transcript unavailable", claudep.ErrTranscriptUnavailable, exitWrapperError},
		{"unknown", errors.New("anything else"), exitWrapperError},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapErrorExit(tt.err); got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}
