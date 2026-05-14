package claudep

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jmadore-payfacto/claude-p-go/internal/driver"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

func TestRunRejectsEmptyPrompt(t *testing.T) {
	_, err := Run(Options{})
	if !errors.Is(err, ErrNoPromptSupplied) {
		t.Fatalf("expected ErrNoPromptSupplied, got %v", err)
	}
}

func TestResultExitCodeAndAccessors(t *testing.T) {
	r := &Result{r: &driver.Result{
		Summary:    transcript.Summary{FinalText: "hi", SessionID: "s", IsError: false},
		DurationMs: 42,
	}}
	if r.ExitCode() != 0 {
		t.Fatalf("ExitCode=%d", r.ExitCode())
	}
	if r.Summary().FinalText != "hi" {
		t.Fatalf("Summary.FinalText=%q", r.Summary().FinalText)
	}
	if r.DurationMs() != 42 {
		t.Fatalf("DurationMs=%d", r.DurationMs())
	}
}

func TestResultExitCodeIsError(t *testing.T) {
	r := &Result{r: &driver.Result{Summary: transcript.Summary{IsError: true}}}
	if r.ExitCode() != 1 {
		t.Fatalf("expected 1")
	}
}

func TestResultWriteText(t *testing.T) {
	r := &Result{r: &driver.Result{Summary: transcript.Summary{FinalText: "out"}, DurationMs: 1}}
	var buf bytes.Buffer
	if err := r.Write(&buf, FormatText); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "out") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestVersionNotEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty")
	}
}
