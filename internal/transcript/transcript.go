// Package transcript parses a Claude Code session transcript (JSONL).
//
// Each line of the transcript is one JSON object describing a single event in
// the session. We extract:
//   - the final assistant text (concatenated content[].text blocks of the
//     last `assistant` event),
//   - aggregated `usage` totals across all assistant messages,
//   - the session id (from any line that includes one),
//   - flags telling us whether an error result was reported.
//
// The transcript path comes to us via the Stop hook's `transcript_path` field.
// We never write to it.
package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type Usage struct {
	InputTokens              uint64 `json:"input_tokens"`
	OutputTokens             uint64 `json:"output_tokens"`
	CacheReadInputTokens     uint64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens uint64 `json:"cache_creation_input_tokens"`
}

type Summary struct {
	FinalText     string
	SessionID     string
	IsError       bool
	NumTurns      uint32
	TotalCostUSD  float64
	DurationAPIMs uint64
	Usage         Usage
	// Pretty-printed view of the original transcript, re-serialized as
	// strict JSONL. Used by emit.StreamJSON.
	JSONLReplay string
}

// ErrNoAssistantMessage is returned when no assistant turn was seen.
var ErrNoAssistantMessage = errors.New("no assistant message in transcript")

// Parse parses transcript bytes (JSONL).
func Parse(bytesIn []byte) (Summary, error) {
	var (
		finalText     strings.Builder
		sessionID     string
		replay        bytes.Buffer
		usage         Usage
		isError       bool
		numTurns      uint32
		totalCostUSD  float64
		durationAPIMs uint64
		sawAssistant  bool
	)

	for rawLine := range bytes.SplitSeq(bytesIn, []byte{'\n'}) {
		line := bytes.TrimRight(rawLine, "\r")
		if len(line) == 0 {
			continue
		}
		replay.Write(line)
		replay.WriteByte('\n')

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(line, &obj); err != nil {
			// Skip malformed lines but don't fail the whole parse.
			continue
		}

		if sessionID == "" {
			if v, ok := obj["sessionId"]; ok {
				_ = json.Unmarshal(v, &sessionID)
			} else if v, ok := obj["session_id"]; ok {
				_ = json.Unmarshal(v, &sessionID)
			}
		}

		var ty string
		if raw, ok := obj["type"]; ok {
			_ = json.Unmarshal(raw, &ty)
		}
		switch ty {
		case "assistant":
			sawAssistant = true
			numTurns++
			finalText.Reset()
			extractAssistantBlocks(obj["message"], &finalText, &usage)
		case "result":
			if raw, ok := obj["result"]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil {
					finalText.Reset()
					finalText.WriteString(s)
					sawAssistant = true
				}
			}
			if raw, ok := obj["is_error"]; ok {
				_ = json.Unmarshal(raw, &isError)
			}
			if raw, ok := obj["num_turns"]; ok {
				_ = json.Unmarshal(raw, &numTurns)
			}
			if raw, ok := obj["total_cost_usd"]; ok {
				_ = json.Unmarshal(raw, &totalCostUSD)
			}
			if raw, ok := obj["duration_api_ms"]; ok {
				_ = json.Unmarshal(raw, &durationAPIMs)
			}
		}
	}

	if !sawAssistant {
		return Summary{}, ErrNoAssistantMessage
	}

	return Summary{
		FinalText:     finalText.String(),
		SessionID:     sessionID,
		IsError:       isError,
		NumTurns:      numTurns,
		TotalCostUSD:  totalCostUSD,
		DurationAPIMs: durationAPIMs,
		Usage:         usage,
		JSONLReplay:   replay.String(),
	}, nil
}

func extractAssistantBlocks(msgRaw json.RawMessage, finalText *strings.Builder, usage *Usage) {
	if len(msgRaw) == 0 {
		return
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return
	}
	if contentRaw, ok := msg["content"]; ok {
		var blocks []map[string]json.RawMessage
		if json.Unmarshal(contentRaw, &blocks) == nil {
			for _, block := range blocks {
				var bt string
				if raw, ok := block["type"]; ok {
					_ = json.Unmarshal(raw, &bt)
				}
				if bt != "text" {
					continue
				}
				var t string
				if raw, ok := block["text"]; ok {
					if json.Unmarshal(raw, &t) == nil {
						finalText.WriteString(t)
					}
				}
			}
		}
	}
	if uRaw, ok := msg["usage"]; ok {
		var u Usage
		if json.Unmarshal(uRaw, &u) == nil {
			usage.InputTokens += u.InputTokens
			usage.OutputTokens += u.OutputTokens
			usage.CacheReadInputTokens += u.CacheReadInputTokens
			usage.CacheCreationInputTokens += u.CacheCreationInputTokens
		}
	}
}

// ParseFile reads a transcript file and parses it.
func ParseFile(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, err
	}
	return Parse(data)
}
