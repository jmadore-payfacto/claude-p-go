package emit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

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
