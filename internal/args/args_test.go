package args

import (
	"errors"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	o, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.HasPrompt {
		t.Fatal("expected no prompt")
	}
	if o.OutputFormat != FormatText {
		t.Fatal("expected text default")
	}
}

func TestParsePositionalPrompt(t *testing.T) {
	o, err := Parse([]string{"hello world"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Prompt != "hello world" {
		t.Fatalf("got %q", o.Prompt)
	}
}

func TestParseOutputFormatJSON(t *testing.T) {
	o, err := Parse([]string{"--output-format", "json", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if o.OutputFormat != FormatJSON {
		t.Fatal("expected json")
	}
	if o.Prompt != "hi" {
		t.Fatalf("got %q", o.Prompt)
	}
}

func TestParseOutputFormatEqualsStreamJSON(t *testing.T) {
	o, err := Parse([]string{"--output-format=stream-json"})
	if err != nil {
		t.Fatal(err)
	}
	if o.OutputFormat != FormatStreamJSON {
		t.Fatal("expected stream-json")
	}
}

func TestParseBadOutputFormat(t *testing.T) {
	_, err := Parse([]string{"--output-format", "yaml"})
	if !errors.Is(err, ErrBadOutputFormat) {
		t.Fatalf("expected ErrBadOutputFormat, got %v", err)
	}
}

func TestParseMissingValue(t *testing.T) {
	_, err := Parse([]string{"--model"})
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestParseMaxTurns(t *testing.T) {
	o, err := Parse([]string{"--max-turns", "7"})
	if err != nil {
		t.Fatal(err)
	}
	if o.MaxTurns != 7 {
		t.Fatalf("got %d", o.MaxTurns)
	}
}

func TestParseBadInteger(t *testing.T) {
	_, err := Parse([]string{"--max-turns", "seven"})
	if !errors.Is(err, ErrBadInteger) {
		t.Fatalf("expected ErrBadInteger, got %v", err)
	}
}

func TestParseSkipPermissions(t *testing.T) {
	o, err := Parse([]string{"--dangerously-skip-permissions"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.DangerouslySkipPermissions {
		t.Fatal("expected true")
	}
}

func TestParseContinue(t *testing.T) {
	o, err := Parse([]string{"--continue"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Continue {
		t.Fatal("expected true")
	}
}

func TestParseUnknownLongFlagForwarded(t *testing.T) {
	o, err := Parse([]string{"--frobnitz", "bar", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Passthrough) != 2 || o.Passthrough[0] != "--frobnitz" || o.Passthrough[1] != "bar" {
		t.Fatalf("passthrough: %v", o.Passthrough)
	}
	if o.Prompt != "hello" {
		t.Fatalf("prompt: %q", o.Prompt)
	}
}

func TestParseHelp(t *testing.T) {
	o, err := Parse([]string{"--help"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.ShowHelp {
		t.Fatal("expected help")
	}
}

func TestParseVersion(t *testing.T) {
	o, err := Parse([]string{"-v"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.ShowVersion {
		t.Fatal("expected version")
	}
}

func TestParseTimeout(t *testing.T) {
	o, err := Parse([]string{"--timeout", "60", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if o.TimeoutSeconds != 60 {
		t.Fatalf("got %d", o.TimeoutSeconds)
	}
}

func TestParseResume(t *testing.T) {
	o, err := Parse([]string{"--resume", "550e8400-e29b-41d4-a716-446655440000"})
	if err != nil {
		t.Fatal(err)
	}
	if o.ResumeSession != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("got %q", o.ResumeSession)
	}
}

func TestParseInputFile(t *testing.T) {
	o, err := Parse([]string{"--input-file", "/tmp/p.md"})
	if err != nil {
		t.Fatal(err)
	}
	if o.InputFile != "/tmp/p.md" {
		t.Fatalf("got %q", o.InputFile)
	}
}
