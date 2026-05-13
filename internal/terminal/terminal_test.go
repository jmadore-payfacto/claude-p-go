package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func respond(in string) []byte {
	var out []byte
	RespondToDecQueries([]byte(in), &out)
	return out
}

func TestDA1(t *testing.T) {
	got := respond("\x1b[c")
	if !bytes.Equal(got, []byte("\x1b[?1;2c")) {
		t.Fatalf("got %q", got)
	}
}

func TestDA2(t *testing.T) {
	got := respond("\x1b[>c")
	if !bytes.Equal(got, []byte("\x1b[>0;0;0c")) {
		t.Fatalf("got %q", got)
	}
}

func TestDSRCursorPos(t *testing.T) {
	got := respond("\x1b[6n")
	if !bytes.Equal(got, []byte("\x1b[1;1R")) {
		t.Fatalf("got %q", got)
	}
}

func TestXTVersion(t *testing.T) {
	got := string(respond("\x1b[>q"))
	if !strings.HasPrefix(got, "\x1bP>|claude-p") || !strings.HasSuffix(got, "\x1b\\") {
		t.Fatalf("got %q", got)
	}
}

func TestIgnoresPlainText(t *testing.T) {
	got := respond("hello world without esc")
	if len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestMultipleQueriesInOneChunk(t *testing.T) {
	got := string(respond("hi\x1b[cthere\x1b[>cyo"))
	if !strings.Contains(got, "\x1b[?1;2c") || !strings.Contains(got, "\x1b[>0;0;0c") {
		t.Fatalf("got %q", got)
	}
}

func TestWindowSize(t *testing.T) {
	got := respond("\x1b[18t")
	if !bytes.Equal(got, []byte("\x1b[8;40;120t")) {
		t.Fatalf("got %q", got)
	}
}
