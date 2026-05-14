// Command fake-claude is a hermetic stand-in for the real `claude` binary,
// used to drive claude-p's orchestration paths in e2e tests without needing
// a live Claude Code login or `claude` on $PATH.
//
// It speaks the parts of the protocol claude-p actually depends on:
//
//   - Parses --settings <json> to find the SessionStart and Stop hook
//     commands.
//   - Optionally prints a synthetic trust-dialog line first (controlled by
//     the FAKE_CLAUDE_TRUST_PROMPT env var) and waits for the parent's
//     dismissal \r.
//   - Fires SessionStart, then reads the prompt from stdin (terminated by
//     the parent's submit \r).
//   - Writes a minimal valid transcript JSONL file to disk.
//   - Fires Stop with `transcript_path` (and `last_assistant_message`) in
//     the payload, then exits.
//
// The reply text can be overridden via FAKE_CLAUDE_REPLY. The session id
// can be overridden via FAKE_CLAUDE_SESSION_ID.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func main() {
	settingsJSON := findSettings(os.Args[1:])
	if settingsJSON == "" {
		die("missing --settings")
	}

	sessionStartCmd, stopCmd, err := parseHookCommands(settingsJSON)
	if err != nil {
		die("parse settings: %v", err)
	}

	if os.Getenv("FAKE_CLAUDE_TRUST_PROMPT") == "1" {
		// Print a line containing both "trust" and "folder" so the parent's
		// stripCSI + Contains check fires. Wait for the parent's \r.
		fmt.Fprintln(os.Stdout, "Is this a project you trust? Yes, I trust this folder")
		_, _ = bufio.NewReader(os.Stdin).ReadByte()
	}

	sessionID := envOr("FAKE_CLAUDE_SESSION_ID", "fake-session")

	if err := fireHook(sessionStartCmd, fmt.Sprintf(`{"session_id":%q,"source":"startup"}`, sessionID)); err != nil {
		die("fire SessionStart: %v", err)
	}

	prompt := readUntilSubmit(os.Stdin)

	reply := envOr("FAKE_CLAUDE_REPLY", "fake reply for: "+prompt)
	transcriptPath, err := writeTranscript(sessionID, reply)
	if err != nil {
		die("write transcript: %v", err)
	}

	stopPayload, err := json.Marshal(map[string]any{
		"session_id":             sessionID,
		"transcript_path":        transcriptPath,
		"last_assistant_message": reply,
	})
	if err != nil {
		die("marshal stop payload: %v", err)
	}
	if err := fireHook(stopCmd, string(stopPayload)); err != nil {
		die("fire Stop: %v", err)
	}
}

func findSettings(argv []string) string {
	for i, a := range argv {
		if a == "--settings" && i+1 < len(argv) {
			return argv[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--settings="); ok {
			return v
		}
	}
	return ""
}

func parseHookCommands(jsonStr string) (sessionStart, stop string, err error) {
	var s settings
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return "", "", err
	}
	pick := func(event string) string {
		matchers := s.Hooks[event]
		if len(matchers) == 0 || len(matchers[0].Hooks) == 0 {
			return ""
		}
		return matchers[0].Hooks[0].Command
	}
	sessionStart, stop = pick("SessionStart"), pick("Stop")
	if sessionStart == "" || stop == "" {
		return "", "", fmt.Errorf("missing SessionStart or Stop hook")
	}
	return sessionStart, stop, nil
}

// readUntilSubmit reads from r until the parent's terminating submit byte
// arrives. The PTY is typically in cooked mode (ICRNL), which translates
// the parent's \r to \n on input — we accept either as the terminator.
func readUntilSubmit(r *os.File) string {
	br := bufio.NewReader(r)
	var b strings.Builder
	for {
		c, err := br.ReadByte()
		if err != nil {
			break
		}
		if c == '\r' || c == '\n' {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

func writeTranscript(sessionID, reply string) (string, error) {
	dir, err := os.MkdirTemp("", "fake-claude-transcript-*")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "session.jsonl")
	lines := []map[string]any{
		{"type": "system", "subtype": "init", "session_id": sessionID},
		{
			"type":       "assistant",
			"session_id": sessionID,
			"message": map[string]any{
				"content": []map[string]any{{"type": "text", "text": reply}},
				"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
			},
		},
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, ln := range lines {
		if err := enc.Encode(ln); err != nil {
			return "", err
		}
	}
	return path, nil
}

func fireHook(command, payload string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = strings.NewReader(payload)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fake-claude: "+format+"\n", args...)
	os.Exit(1)
}
