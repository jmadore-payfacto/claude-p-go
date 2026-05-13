package hook

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuildSettingsJSON(t *testing.T) {
	s, err := buildSettingsJSON("/tmp/hook.sh")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		t.Fatal("no hooks")
	}
	for _, evt := range []string{"SessionStart", "Stop"} {
		arr, ok := hooks[evt].([]any)
		if !ok || len(arr) == 0 {
			t.Fatalf("missing event %q", evt)
		}
		entry, _ := arr[0].(map[string]any)
		if entry["matcher"] != "*" {
			t.Fatalf("matcher: %v", entry["matcher"])
		}
		inner := entry["hooks"].([]any)[0].(map[string]any)
		if inner["type"] != "command" {
			t.Fatalf("type: %v", inner["type"])
		}
		cmd := inner["command"].(string)
		if !strings.Contains(cmd, "/tmp/hook.sh") || !strings.HasSuffix(cmd, " "+evt) {
			t.Fatalf("cmd: %q", cmd)
		}
	}
}

func TestParseLine(t *testing.T) {
	ln, ok := ParseLine("Stop\t{\"transcript_path\":\"/tmp/x.jsonl\"}\n")
	if !ok {
		t.Fatal("parse failed")
	}
	if ln.Event != EventStop {
		t.Fatalf("event: %v", ln.Event)
	}
	if ln.Payload != `{"transcript_path":"/tmp/x.jsonl"}` {
		t.Fatalf("payload: %q", ln.Payload)
	}
}

func TestParseLineUnknownEvent(t *testing.T) {
	ln, ok := ParseLine("PreFooBar\t{}")
	if !ok {
		t.Fatal("parse failed")
	}
	if ln.Event != EventUnknown {
		t.Fatalf("expected unknown: %v", ln.Event)
	}
}

func TestParseLineMalformed(t *testing.T) {
	if _, ok := ParseLine("nope-no-tab"); ok {
		t.Fatal("expected !ok")
	}
}

func TestExtractTranscriptPath(t *testing.T) {
	p, ok := ExtractTranscriptPath(`{"transcript_path":"/a/b.jsonl","session_id":"x"}`)
	if !ok || p != "/a/b.jsonl" {
		t.Fatalf("got %q ok=%v", p, ok)
	}
}

func TestExtractLastAssistantMessage(t *testing.T) {
	m, ok := ExtractLastAssistantMessage(`{"last_assistant_message":"OK","session_id":"x"}`)
	if !ok || m != "OK" {
		t.Fatalf("got %q ok=%v", m, ok)
	}
}

func TestExtractSessionID(t *testing.T) {
	s, ok := ExtractSessionID(`{"session_id":"abc-123"}`)
	if !ok || s != "abc-123" {
		t.Fatalf("got %q ok=%v", s, ok)
	}
}

func TestCreateRoundTrip(t *testing.T) {
	h, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	defer h.Cleanup()
	if _, err := os.Stat(h.ScriptPath); err != nil {
		t.Fatalf("script missing: %v", err)
	}
	if _, err := os.Stat(h.FifoPath); err != nil {
		t.Fatalf("fifo missing: %v", err)
	}
}
