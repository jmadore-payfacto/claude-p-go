// Package hook generates the Stop/SessionStart hook plumbing for a `claude`
// invocation: a per-run temp dir, a FIFO the parent reads, a tiny shell
// script that relays the hook payload to the FIFO, and the inline --settings
// JSON that tells `claude` to call it.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Harness holds the lifetime of a hook environment. Call Cleanup when done.
type Harness struct {
	TmpDir       string
	FifoPath     string
	ScriptPath   string
	SettingsJSON string
}

// Cleanup removes the temp dir, FIFO, and script. Best-effort.
func (h *Harness) Cleanup() {
	if h == nil {
		return
	}
	_ = os.Remove(h.FifoPath)
	_ = os.Remove(h.ScriptPath)
	_ = os.Remove(h.TmpDir)
}

const scriptBody = `#!/bin/sh
# Relay a Claude Code hook event to claude-p's FIFO.
#   $1 = event name (e.g. "Stop", "SessionStart")
# stdin = the hook's JSON payload (single line, no embedded newlines).
set -eu
event="$1"
fifo="${CLAUDE_P_FIFO:?missing CLAUDE_P_FIFO}"
payload="$(cat)"
printf '%s\t%s\n' "$event" "$payload" >> "$fifo"
exit 0
`

func tmpRoot() string {
	if v := os.Getenv("TMPDIR"); v != "" {
		return v
	}
	return "/tmp"
}

// Create builds a harness with tmp dir, FIFO, relay script, and inline JSON.
func Create() (*Harness, error) {
	pid := os.Getpid()
	suffix := rand.Uint32()
	dir := filepath.Join(tmpRoot(), fmt.Sprintf("claude-p-%d-%x", pid, suffix))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	fifoPath := filepath.Join(dir, "events.fifo")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("mkfifo: %w", err)
	}

	scriptPath := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o700); err != nil {
		_ = os.Remove(fifoPath)
		_ = os.RemoveAll(dir)
		return nil, err
	}

	settings, err := buildSettingsJSON(scriptPath)
	if err != nil {
		_ = os.Remove(fifoPath)
		_ = os.Remove(scriptPath)
		_ = os.RemoveAll(dir)
		return nil, err
	}

	return &Harness{
		TmpDir:       dir,
		FifoPath:     fifoPath,
		ScriptPath:   scriptPath,
		SettingsJSON: settings,
	}, nil
}

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookMatcher struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookCommand `json:"hooks"`
}

type settings struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
}

func buildSettingsJSON(scriptPath string) (string, error) {
	mk := func(event string) []hookMatcher {
		return []hookMatcher{{
			Matcher: "*",
			Hooks: []hookCommand{{
				Type:    "command",
				Command: scriptPath + " " + event,
			}},
		}}
	}
	s := settings{Hooks: map[string][]hookMatcher{
		"SessionStart": mk("SessionStart"),
		"Stop":         mk("Stop"),
	}}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type Event int

const (
	EventUnknown Event = iota
	EventSessionStart
	EventStop
)

func ParseEvent(s string) Event {
	switch s {
	case "SessionStart":
		return EventSessionStart
	case "Stop":
		return EventStop
	}
	return EventUnknown
}

func (e Event) String() string {
	switch e {
	case EventSessionStart:
		return "SessionStart"
	case EventStop:
		return "Stop"
	}
	return "Unknown"
}

// Line is a parsed "<event>\t<payload>" entry from the FIFO.
type Line struct {
	Event   Event
	Payload string
}

// ParseLine parses one FIFO line. Returns false if malformed.
func ParseLine(raw string) (Line, bool) {
	raw = strings.TrimRight(raw, "\r\n")
	name, payload, ok := strings.Cut(raw, "\t")
	if !ok {
		return Line{}, false
	}
	return Line{Event: ParseEvent(name), Payload: payload}, true
}

// ExtractTranscriptPath pulls `transcript_path` from a Stop hook payload.
func ExtractTranscriptPath(payload string) (string, bool) {
	return extractStringField(payload, "transcript_path")
}

// ExtractLastAssistantMessage pulls `last_assistant_message`. Newer Claude
// Code versions include this directly in Stop payloads.
func ExtractLastAssistantMessage(payload string) (string, bool) {
	return extractStringField(payload, "last_assistant_message")
}

// ExtractSessionID pulls `session_id`.
func ExtractSessionID(payload string) (string, bool) {
	return extractStringField(payload, "session_id")
}

func extractStringField(payload, field string) (string, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return "", false
	}
	v, ok := m[field]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// ErrMkfifo is returned when mkfifo fails.
var ErrMkfifo = errors.New("mkfifo failed")
