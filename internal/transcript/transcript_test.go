package transcript

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestSingleAssistant(t *testing.T) {
	jsonl := `{"type":"assistant","session_id":"abc","message":{"id":"m1","role":"assistant","content":[{"type":"text","text":"Hello, world."}],"usage":{"input_tokens":10,"output_tokens":3}}}` + "\n"
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "Hello, world." {
		t.Fatalf("text: %q", s.FinalText)
	}
	if s.SessionID != "abc" {
		t.Fatalf("session: %q", s.SessionID)
	}
	if s.NumTurns != 1 {
		t.Fatalf("turns: %d", s.NumTurns)
	}
	if s.Usage.InputTokens != 10 || s.Usage.OutputTokens != 3 {
		t.Fatalf("usage: %+v", s.Usage)
	}
	if s.IsError {
		t.Fatal("unexpected error")
	}
}

func TestMultipleTurnsLastWins(t *testing.T) {
	jsonl := `{"type":"system","subtype":"init","session_id":"xyz"}
{"type":"assistant","session_id":"xyz","message":{"content":[{"type":"text","text":"first"}],"usage":{"input_tokens":5,"output_tokens":2}}}
{"type":"user","session_id":"xyz","message":{"content":[{"type":"tool_result","content":"ok"}]}}
{"type":"assistant","session_id":"xyz","message":{"content":[{"type":"text","text":"final answer"}],"usage":{"input_tokens":7,"output_tokens":4}}}
`
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "final answer" {
		t.Fatalf("text: %q", s.FinalText)
	}
	if s.SessionID != "xyz" {
		t.Fatalf("session: %q", s.SessionID)
	}
	if s.Usage.InputTokens != 12 || s.Usage.OutputTokens != 6 {
		t.Fatalf("usage: %+v", s.Usage)
	}
}

func TestResultEventWins(t *testing.T) {
	jsonl := `{"type":"assistant","session_id":"r","message":{"content":[{"type":"text","text":"draft"}]}}
{"type":"result","subtype":"success","session_id":"r","result":"final","is_error":false,"num_turns":2,"total_cost_usd":0.0421,"duration_api_ms":9120}
`
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "final" {
		t.Fatalf("text: %q", s.FinalText)
	}
	if s.IsError {
		t.Fatal("is_error")
	}
	if s.NumTurns != 2 {
		t.Fatalf("turns: %d", s.NumTurns)
	}
	if math.Abs(s.TotalCostUSD-0.0421) > 1e-9 {
		t.Fatalf("cost: %f", s.TotalCostUSD)
	}
	if s.DurationAPIMs != 9120 {
		t.Fatalf("dur: %d", s.DurationAPIMs)
	}
}

func TestErrorResult(t *testing.T) {
	jsonl := `{"type":"assistant","session_id":"e","message":{"content":[{"type":"text","text":"oops"}]}}
{"type":"result","subtype":"error","session_id":"e","result":"failure detail","is_error":true}
`
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "failure detail" {
		t.Fatalf("text: %q", s.FinalText)
	}
	if !s.IsError {
		t.Fatal("expected is_error")
	}
}

func TestNoAssistantReturnsError(t *testing.T) {
	jsonl := `{"type":"system","subtype":"init","session_id":"x"}` + "\n"
	if _, err := Parse([]byte(jsonl)); !errors.Is(err, ErrNoAssistantMessage) {
		t.Fatalf("expected ErrNoAssistantMessage, got %v", err)
	}
}

func TestSkipsMalformedLines(t *testing.T) {
	jsonl := "not-json\n" + `{"type":"assistant","session_id":"k","message":{"content":[{"type":"text","text":"alive"}]}}` + "\n"
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "alive" {
		t.Fatalf("text: %q", s.FinalText)
	}
}

func TestReplayPreserves(t *testing.T) {
	jsonl := `{"type":"system","subtype":"init","session_id":"a"}
{"type":"assistant","session_id":"a","message":{"content":[{"type":"text","text":"hi"}]}}
`
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.JSONLReplay, `"subtype":"init"`) {
		t.Fatalf("replay missing init: %q", s.JSONLReplay)
	}
	if !strings.Contains(s.JSONLReplay, `"text":"hi"`) {
		t.Fatalf("replay missing hi")
	}
}

func TestMultiBlockTextConcat(t *testing.T) {
	jsonl := `{"type":"assistant","session_id":"c","message":{"content":[{"type":"text","text":"part1 "},{"type":"tool_use","name":"X"},{"type":"text","text":"part2"}]}}` + "\n"
	s, err := Parse([]byte(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if s.FinalText != "part1 part2" {
		t.Fatalf("text: %q", s.FinalText)
	}
}
