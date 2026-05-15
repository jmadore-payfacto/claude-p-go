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
	if _, err := os.Stat(h.EventsPath); err != nil {
		t.Fatalf("events file missing: %v", err)
	}
}

func TestCreateFilePermissions(t *testing.T) {
	h, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	defer h.Cleanup()
	scriptInfo, err := os.Stat(h.ScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if mode := scriptInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("script perm: %o", mode)
	}
	dirInfo, err := os.Stat(h.TmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("dir perm: %o", mode)
	}
}

func TestCleanupRemovesArtifacts(t *testing.T) {
	h, err := Create()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir, events, script := h.TmpDir, h.EventsPath, h.ScriptPath
	h.Cleanup()
	for _, p := range []string{events, script, tmpDir} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("still present: %s (err=%v)", p, err)
		}
	}
}

func TestCleanupNilHarnessSafe(t *testing.T) {
	var h *Harness
	h.Cleanup() // must not panic
}

func TestEventString(t *testing.T) {
	cases := []struct {
		e    Event
		want string
	}{
		{EventSessionStart, "SessionStart"},
		{EventStop, "Stop"},
		{EventUnknown, "Unknown"},
	}
	for _, tt := range cases {
		if got := tt.e.String(); got != tt.want {
			t.Fatalf("%v: got %q want %q", tt.e, got, tt.want)
		}
	}
}

func TestParseEventUnknown(t *testing.T) {
	if ParseEvent("Wat") != EventUnknown {
		t.Fatal("expected EventUnknown")
	}
}

func TestExtractStringFieldMissing(t *testing.T) {
	if _, ok := ExtractTranscriptPath(`{"other":"x"}`); ok {
		t.Fatal("expected !ok")
	}
}

func TestExtractStringFieldBadJSON(t *testing.T) {
	if _, ok := ExtractSessionID(`not-json`); ok {
		t.Fatal("expected !ok")
	}
}

func TestExtractStringFieldNonString(t *testing.T) {
	if _, ok := ExtractSessionID(`{"session_id":42}`); ok {
		t.Fatal("expected !ok for non-string value")
	}
}
