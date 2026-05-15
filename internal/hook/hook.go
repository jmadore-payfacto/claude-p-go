// Package hook generates the Stop/SessionStart hook plumbing for a `claude`
// invocation: a per-run temp dir, an append-only events file the parent
// tails, a tiny shell script that relays each hook payload to that file, and
// the inline --settings JSON that tells `claude` to call it.
package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Harness holds the lifetime of a hook environment. Call Cleanup when done.
type Harness struct {
	TmpDir       string
	EventsPath   string
	ScriptPath   string
	SettingsJSON string
}

// Cleanup removes the temp dir, events file, and script. Best-effort.
func (h *Harness) Cleanup() {
	if h == nil {
		return
	}
	_ = os.Remove(h.EventsPath)
	_ = os.Remove(h.ScriptPath)
	_ = os.Remove(h.TmpDir)
}

const scriptBody = `#!/bin/sh
# Relay a Claude Code hook event to claude-p's events file.
#   $1 = event name (e.g. "Stop", "SessionStart")
# stdin = the hook's JSON payload (single line, no embedded newlines).
set -eu
event="$1"
events="${CLAUDE_P_EVENTS:?missing CLAUDE_P_EVENTS}"
payload="$(cat)"
printf '%s\t%s\n' "$event" "$payload" >> "$events"
exit 0
`

// Create builds a harness with tmp dir, events file, relay script, and JSON.
func Create() (*Harness, error) {
	// MkdirTemp creates the directory atomically with O_EXCL semantics and
	// mode 0o700, sidestepping symlink races on shared /tmp directories.
	dir, err := os.MkdirTemp("", "claude-p-*")
	if err != nil {
		return nil, err
	}

	// Pre-create the events file empty so the parent can open it for reading
	// before the child's first hook fires.
	eventsPath := filepath.Join(dir, "events.log")
	if err := os.WriteFile(eventsPath, nil, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	scriptPath := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	settings, err := buildSettingsJSON(scriptPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	return &Harness{
		TmpDir:       dir,
		EventsPath:   eventsPath,
		ScriptPath:   scriptPath,
		SettingsJSON: settings,
	}, nil
}

type hookCommand struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type hookMatcher struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookCommand `json:"hooks"`
}

type settings struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
}

func buildSettingsJSON(scriptPath string) (string, error) {
	// Exec form (command + args) bypasses the system shell on every
	// platform — no whitespace-tokenisation surprises when the temp path
	// contains spaces (e.g. C:\Users\First Last\... on Windows). Forward
	// slashes keep the path well-formed for Git Bash.
	command := filepath.ToSlash(scriptPath)
	mk := func(event string) []hookMatcher {
		return []hookMatcher{{
			Matcher: "*",
			Hooks: []hookCommand{{
				Type:    "command",
				Command: command,
				Args:    []string{event},
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

// Line is a parsed "<event>\t<payload>" entry from the events file.
type Line struct {
	Event   Event
	Payload string
}

// ParseLine parses one events-file line. Returns false if malformed.
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
