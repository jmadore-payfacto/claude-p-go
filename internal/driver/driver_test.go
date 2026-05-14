package driver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
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

func TestStripCSILoneEscAtEnd(t *testing.T) {
	in := []byte("abc\x1b")
	got := string(stripCSI(in))
	if got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestStripCSIOscStTerminated(t *testing.T) {
	in := []byte("a\x1b]0;title\x1b\\b")
	got := string(stripCSI(in))
	if got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyDefaultsZeroValuesFilled(t *testing.T) {
	o := Options{Prompt: "x"}
	applyDefaults(&o)
	if o.Cols != defaultCols || o.Rows != defaultRows {
		t.Fatalf("cols=%d rows=%d", o.Cols, o.Rows)
	}
	if o.TimeoutMs != defaultTimeoutMs {
		t.Fatalf("timeout=%d", o.TimeoutMs)
	}
}

func TestApplyDefaultsPreservesNonZero(t *testing.T) {
	o := Options{Cols: 80, Rows: 24, TimeoutMs: 5000}
	applyDefaults(&o)
	if o.Cols != 80 || o.Rows != 24 || o.TimeoutMs != 5000 {
		t.Fatalf("clobbered: %+v", o)
	}
}

func TestExitCodeMaps(t *testing.T) {
	r := &Result{Summary: transcript.Summary{IsError: false}}
	if r.ExitCode() != 0 {
		t.Fatalf("expected 0")
	}
	r = &Result{Summary: transcript.Summary{IsError: true}}
	if r.ExitCode() != 1 {
		t.Fatalf("expected 1")
	}
}

func TestResultWriteText(t *testing.T) {
	r := &Result{Summary: transcript.Summary{FinalText: "ok"}, DurationMs: 5}
	var buf bytes.Buffer
	if err := r.Write(&buf, args.FormatText); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "ok\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func writeTranscriptFile(t *testing.T, jsonl string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(jsonl), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadTranscriptWithBudgetValid(t *testing.T) {
	path := writeTranscriptFile(t, `{"type":"assistant","session_id":"s","message":{"content":[{"type":"text","text":"hello"}]}}`+"\n")
	s, ok := readTranscriptWithBudget(path, 1, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	if s.FinalText != "hello" {
		t.Fatalf("got %q", s.FinalText)
	}
}

func TestReadTranscriptWithBudgetMissing(t *testing.T) {
	_, ok := readTranscriptWithBudget("/no/such/transcript.jsonl", 1, 0)
	if ok {
		t.Fatal("expected !ok")
	}
}

func TestReadTranscriptWithBudgetEmptyContent(t *testing.T) {
	// Valid JSONL but no assistant text and no error → keeps retrying, fails.
	path := writeTranscriptFile(t, `{"type":"system","subtype":"init"}`+"\n")
	_, ok := readTranscriptWithBudget(path, 2, time.Millisecond)
	if ok {
		t.Fatal("expected !ok")
	}
}

func TestReadTranscriptWithBudgetErrorOnly(t *testing.T) {
	// is_error: true should be treated as success-to-return even without text.
	path := writeTranscriptFile(t, `{"type":"assistant","session_id":"s","message":{"content":[{"type":"text","text":""}]}}`+"\n"+
		`{"type":"result","is_error":true}`+"\n")
	s, ok := readTranscriptWithBudget(path, 1, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	if !s.IsError {
		t.Fatal("expected IsError")
	}
}

func TestLoadSummaryFromTranscript(t *testing.T) {
	path := writeTranscriptFile(t, `{"type":"assistant","session_id":"sid","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n")
	s, err := loadSummaryBudgeted(path, "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "hi" || s.SessionID != "sid" {
		t.Fatalf("got %+v", s)
	}
}

func TestLoadSummaryFallbackToPayload(t *testing.T) {
	payload := `{"last_assistant_message":"fallback-text","session_id":"abc"}`
	s, err := loadSummaryBudgeted("/no/such/transcript.jsonl", payload, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "fallback-text" || s.SessionID != "abc" {
		t.Fatalf("got %+v", s)
	}
	if s.NumTurns != 1 {
		t.Fatalf("NumTurns=%d", s.NumTurns)
	}
}

func TestLoadSummaryNoFallbackPayloadEmpty(t *testing.T) {
	_, err := loadSummaryBudgeted("/no/such/path.jsonl", "", 1, 0)
	if !errors.Is(err, ErrTranscriptUnavailable) {
		t.Fatalf("expected ErrTranscriptUnavailable, got %v", err)
	}
}

func TestLoadSummaryNoFallbackPayloadMissingMessage(t *testing.T) {
	payload := `{"session_id":"xyz"}` // no last_assistant_message
	_, err := loadSummaryBudgeted("/no/such/path.jsonl", payload, 1, 0)
	if !errors.Is(err, ErrTranscriptUnavailable) {
		t.Fatalf("expected ErrTranscriptUnavailable, got %v", err)
	}
}
