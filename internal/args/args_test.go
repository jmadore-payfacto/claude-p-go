package args

import (
	"errors"
	"strings"
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

func TestParseModelEqualsForm(t *testing.T) {
	o, err := Parse([]string{"--model=opus"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Model != "opus" || !o.HasModel {
		t.Fatalf("got %+v", o)
	}
}

func TestParseAllowedToolsAlias(t *testing.T) {
	o, err := Parse([]string{"--allowed-tools", "Read,Edit"})
	if err != nil {
		t.Fatal(err)
	}
	if o.AllowedTools != "Read,Edit" || !o.HasAllowedTools {
		t.Fatalf("got %+v", o)
	}
}

func TestParseAllowedToolsCanonical(t *testing.T) {
	o, err := Parse([]string{"--allowedTools", "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	if o.AllowedTools != "Bash" {
		t.Fatalf("got %q", o.AllowedTools)
	}
}

func TestParseBadTimeoutInteger(t *testing.T) {
	_, err := Parse([]string{"--timeout", "soon"})
	if !errors.Is(err, ErrBadInteger) {
		t.Fatalf("expected ErrBadInteger, got %v", err)
	}
}

func TestParseShortContinueFlag(t *testing.T) {
	o, err := Parse([]string{"-c"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Continue {
		t.Fatal("expected Continue")
	}
}

func TestParseSessionID(t *testing.T) {
	o, err := Parse([]string{"--session-id", "abc-123"})
	if err != nil {
		t.Fatal(err)
	}
	if o.SessionID != "abc-123" || !o.HasSessionID {
		t.Fatalf("got %+v", o)
	}
}

func TestParseCwd(t *testing.T) {
	o, err := Parse([]string{"--cwd", "/work"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Cwd != "/work" || !o.HasCwd {
		t.Fatalf("got %+v", o)
	}
}

func TestParseVerboseDebug(t *testing.T) {
	o, err := Parse([]string{"--verbose", "--debug"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Verbose || !o.Debug {
		t.Fatalf("got %+v", o)
	}
}

func TestParseUnknownLongFlagNoArg(t *testing.T) {
	o, err := Parse([]string{"--solo-flag", "--known-next"})
	if err != nil {
		t.Fatal(err)
	}
	// Both should land in Passthrough since neither is consumed as a value.
	if len(o.Passthrough) != 2 {
		t.Fatalf("passthrough: %v", o.Passthrough)
	}
}

func TestParseShortPassthrough(t *testing.T) {
	o, err := Parse([]string{"-x", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Passthrough) != 1 || o.Passthrough[0] != "-x" {
		t.Fatalf("passthrough: %v", o.Passthrough)
	}
	if o.Prompt != "hi" {
		t.Fatalf("prompt: %q", o.Prompt)
	}
}

func TestParseBadEqualsOutputFormat(t *testing.T) {
	_, err := Parse([]string{"--output-format=yaml"})
	if !errors.Is(err, ErrBadOutputFormat) {
		t.Fatalf("expected ErrBadOutputFormat, got %v", err)
	}
}

func TestParseOutputFormatTextExplicit(t *testing.T) {
	f, ok := ParseOutputFormat("text")
	if !ok || f != FormatText {
		t.Fatalf("got %v ok=%v", f, ok)
	}
}

func TestParseOutputFormatInvalid(t *testing.T) {
	if _, ok := ParseOutputFormat("xml"); ok {
		t.Fatal("expected !ok")
	}
}

func TestOutputFormatString(t *testing.T) {
	cases := []struct {
		f    OutputFormat
		want string
	}{
		{FormatText, "text"},
		{FormatJSON, "json"},
		{FormatStreamJSON, "stream-json"},
	}
	for _, tt := range cases {
		if got := tt.f.String(); got != tt.want {
			t.Fatalf("%v: got %q want %q", tt.f, got, tt.want)
		}
	}
}

func TestHelpTextMentionsKeyFlags(t *testing.T) {
	h := HelpText()
	for _, want := range []string{"--output-format", "--model", "--max-turns", "claude-p"} {
		if !strings.Contains(h, want) {
			t.Fatalf("HelpText missing %q", want)
		}
	}
}
