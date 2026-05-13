// Package emit produces stdout in the format `claude -p` does.
//
//	text         → final assistant text + "\n"
//	json         → one result object
//	stream-json  → JSONL replay of the transcript, then result
package emit

import (
	"encoding/json"
	"io"

	"github.com/jmadore-payfacto/claude-p-go/internal/args"
	"github.com/jmadore-payfacto/claude-p-go/internal/transcript"
)

// Envelope bundles a summary with the wall-clock duration.
type Envelope struct {
	Summary    *transcript.Summary
	DurationMs uint64
}

// Emit writes env in the requested format to w.
func Emit(w io.Writer, format args.OutputFormat, env Envelope) error {
	switch format {
	case args.FormatText:
		return emitText(w, env)
	case args.FormatJSON:
		return emitJSON(w, env)
	case args.FormatStreamJSON:
		return emitStreamJSON(w, env)
	}
	return nil
}

func emitText(w io.Writer, env Envelope) error {
	t := env.Summary.FinalText
	if _, err := io.WriteString(w, t); err != nil {
		return err
	}
	if len(t) == 0 || t[len(t)-1] != '\n' {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

type resultEnvelope struct {
	Type              string            `json:"type"`
	Subtype           string            `json:"subtype"`
	SessionID         string            `json:"session_id"`
	Result            string            `json:"result"`
	IsError           bool              `json:"is_error"`
	DurationMs        uint64            `json:"duration_ms"`
	DurationAPIMs     uint64            `json:"duration_api_ms"`
	NumTurns          uint32            `json:"num_turns"`
	TotalCostUSD      float64           `json:"total_cost_usd"`
	Usage             transcript.Usage  `json:"usage"`
	PermissionDenials []json.RawMessage `json:"permission_denials"`
}

func envelopeFor(env Envelope) resultEnvelope {
	s := env.Summary
	subtype := "success"
	if s.IsError {
		subtype = "error"
	}
	return resultEnvelope{
		Type:              "result",
		Subtype:           subtype,
		SessionID:         s.SessionID,
		Result:            s.FinalText,
		IsError:           s.IsError,
		DurationMs:        env.DurationMs,
		DurationAPIMs:     s.DurationAPIMs,
		NumTurns:          s.NumTurns,
		TotalCostUSD:      s.TotalCostUSD,
		Usage:             s.Usage,
		PermissionDenials: []json.RawMessage{},
	}
}

func emitJSON(w io.Writer, env Envelope) error {
	b, err := json.Marshal(envelopeFor(env))
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func emitStreamJSON(w io.Writer, env Envelope) error {
	replay := env.Summary.JSONLReplay
	if _, err := io.WriteString(w, replay); err != nil {
		return err
	}
	if len(replay) == 0 || replay[len(replay)-1] != '\n' {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return emitJSON(w, env)
}
