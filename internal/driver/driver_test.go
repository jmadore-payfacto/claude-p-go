package driver

import (
	"errors"
	"slices"
	"testing"
)

func TestBuildArgvMinimal(t *testing.T) {
	argv := BuildArgv("/bin/claude", "{}", Options{Prompt: "hi"})
	if argv[0] != "/bin/claude" || argv[1] != "--settings" || argv[2] != "{}" {
		t.Fatalf("argv: %v", argv)
	}
}

func TestBuildArgvWithModelVerbose(t *testing.T) {
	argv := BuildArgv("claude", "{}", Options{
		Prompt:   "hi",
		Model:    "opus",
		HasModel: true,
		Verbose:  true,
	})
	if !slices.Contains(argv, "--model") {
		t.Fatalf("missing --model: %v", argv)
	}
	if !slices.Contains(argv, "--verbose") {
		t.Fatalf("missing --verbose: %v", argv)
	}
}

func TestBuildArgvSkipPermissions(t *testing.T) {
	argv := BuildArgv("claude", "{}", Options{Prompt: "x", SkipPermissions: true})
	if !slices.Contains(argv, "--dangerously-skip-permissions") {
		t.Fatalf("missing flag: %v", argv)
	}
}

func TestBuildArgvPassthrough(t *testing.T) {
	argv := BuildArgv("claude", "{}", Options{
		Prompt:    "x",
		ExtraArgs: []string{"--include-hook-events", "--bare"},
	})
	if !slices.Contains(argv, "--include-hook-events") || !slices.Contains(argv, "--bare") {
		t.Fatalf("missing pass-through: %v", argv)
	}
}

func TestRunEmptyPromptRejected(t *testing.T) {
	_, err := Run(Options{})
	if !errors.Is(err, ErrNoPromptSupplied) {
		t.Fatalf("expected ErrNoPromptSupplied, got %v", err)
	}
}

func TestStripCSI(t *testing.T) {
	// CSI sequences are removed; payload survives.
	in := []byte("Hello\x1b[1Cworld\x1b[2J!")
	got := string(stripCSI(in))
	if got != "Helloworld!" {
		t.Fatalf("got %q", got)
	}
}

func TestStripCSIOscBel(t *testing.T) {
	in := []byte("a\x1b]0;title\x07b")
	got := string(stripCSI(in))
	if got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestStripCSIDcsStTerminated(t *testing.T) {
	in := []byte("a\x1bPxyz\x1b\\b")
	got := string(stripCSI(in))
	if got != "ab" {
		t.Fatalf("got %q", got)
	}
}
