package emit

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

// failingWriter returns errFailingWriter after `okBytes` bytes have been
// successfully written. Used to exercise the error paths in emit*.
type failingWriter struct {
	okBytes int
	written int
}

var errFailingWriter = errors.New("failing-writer")

func (w *failingWriter) Write(p []byte) (int, error) {
	remaining := w.okBytes - w.written
	if remaining <= 0 {
		return 0, errFailingWriter
	}
	if len(p) <= remaining {
		w.written += len(p)
		return len(p), nil
	}
	w.written += remaining
	return remaining, errFailingWriter
}

func parseAndEmit(t *testing.T, fmt args.OutputFormat, jsonl string) string {
	t.Helper()
	s, err := transcript.Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Emit(&buf, fmt, Envelope{Summary: &s, DurationMs: 100}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestEmitText(t *testing.T) {
	out := parseAndEmit(t, args.FormatText,
		`{"type":"assistant","session_id":"s","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n")
	if out != "hi\n" {
		t.Fatalf("got %q", out)
	}
}

func TestEmitTextNoDoubleNewline(t *testing.T) {
	out := parseAndEmit(t, args.FormatText,
		`{"type":"assistant","session_id":"s","message":{"content":[{"type":"text","text":"hi\n"}]}}`+"\n")
	if out != "hi\n" {
		t.Fatalf("got %q", out)
	}
}

func TestEmitJSON(t *testing.T) {
	out := parseAndEmit(t, args.FormatJSON,
		`{"type":"assistant","session_id":"sid","message":{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":1}}}`+"\n")
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "result" || m["subtype"] != "success" {
		t.Fatalf("bad: %+v", m)
	}
	if m["session_id"] != "sid" || m["result"] != "ok" {
		t.Fatalf("bad: %+v", m)
	}
	if m["is_error"].(bool) {
		t.Fatal("is_error")
	}
	if int(m["duration_ms"].(float64)) != 100 {
		t.Fatalf("duration: %v", m["duration_ms"])
	}
	usage := m["usage"].(map[string]any)
	if int(usage["input_tokens"].(float64)) != 2 {
		t.Fatalf("usage: %v", usage)
	}
}

func TestEmitJSONError(t *testing.T) {
	out := parseAndEmit(t, args.FormatJSON, `{"type":"assistant","session_id":"e","message":{"content":[{"type":"text","text":"boom"}]}}
{"type":"result","subtype":"error","session_id":"e","result":"boom","is_error":true}
`)
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["subtype"] != "error" || !m["is_error"].(bool) {
		t.Fatalf("bad: %+v", m)
	}
}

func TestEmitUnknownFormatIsNoop(t *testing.T) {
	var buf bytes.Buffer
	s := transcript.Summary{FinalText: "x"}
	if err := Emit(&buf, args.OutputFormat(99), Envelope{Summary: &s}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestEmitTextWriterError(t *testing.T) {
	s := transcript.Summary{FinalText: "hello"}
	err := Emit(&failingWriter{okBytes: 0}, args.FormatText, Envelope{Summary: &s})
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("expected errFailingWriter, got %v", err)
	}
}

func TestEmitTextNewlineWriterError(t *testing.T) {
	// FinalText has no trailing newline → emitText writes the text, then "\n".
	// Let the body succeed but the trailing newline fail.
	s := transcript.Summary{FinalText: "hi"}
	err := Emit(&failingWriter{okBytes: 2}, args.FormatText, Envelope{Summary: &s})
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("expected errFailingWriter, got %v", err)
	}
}

func TestEmitJSONWriterError(t *testing.T) {
	s := transcript.Summary{FinalText: "hi", SessionID: "s"}
	err := Emit(&failingWriter{okBytes: 0}, args.FormatJSON, Envelope{Summary: &s})
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("expected errFailingWriter, got %v", err)
	}
}

func TestEmitStreamJSONReplayWriterError(t *testing.T) {
	s := transcript.Summary{FinalText: "hi", JSONLReplay: `{"x":1}` + "\n"}
	err := Emit(&failingWriter{okBytes: 0}, args.FormatStreamJSON, Envelope{Summary: &s})
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("expected errFailingWriter, got %v", err)
	}
}

func TestEmitStreamJSON(t *testing.T) {
	out := parseAndEmit(t, args.FormatStreamJSON, `{"type":"system","subtype":"init","session_id":"s"}
{"type":"assistant","session_id":"s","message":{"content":[{"type":"text","text":"go"}]}}
`)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], `"subtype":"init"`) {
		t.Fatalf("line1: %q", lines[0])
	}
	if !strings.Contains(lines[1], `"type":"assistant"`) {
		t.Fatalf("line2: %q", lines[1])
	}
	if !strings.Contains(lines[2], `"type":"result"`) {
		t.Fatalf("line3: %q", lines[2])
	}
}
