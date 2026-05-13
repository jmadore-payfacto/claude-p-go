// Package terminal is a minimal scanner for the handful of DEC/XTerm queries
// Ink (the React-for-terminals runtime Claude Code uses) emits at startup.
// Without responses the UI hangs forever waiting for a terminal it thinks is
// broken.
//
// Recognised:
//   - DA1:  ESC [ c   or  ESC [ 0 c       → "VT100 with AVO"
//   - DA2:  ESC [ > c or  ESC [ > 0 c     → "claude-p 0/0"
//   - DSR cursor position: ESC [ 6 n      → row 1 col 1
//   - XTVERSION:  ESC [ > q or ESC [ > 0 q
//   - Window-size report: ESC [ 18 t      → "8 ; rows ; cols t"
//
// Pure function: callers pass in incoming PTY bytes; we append any response
// bytes to out. Thread-safe by virtue of being state-free.
package terminal

import "bytes"

// RespondToDecQueries scans bytes for DEC/XTerm query escape sequences and
// appends response bytes that should be written back to the PTY.
func RespondToDecQueries(in []byte, out *[]byte) {
	for i := 0; i < len(in); i++ {
		if in[i] != 0x1b {
			continue
		}
		if i+1 >= len(in) {
			break
		}
		if in[i+1] != '[' {
			continue
		}

		j := i + 2
		privateGT := j < len(in) && in[j] == '>'
		if privateGT {
			j++
		}
		// Parameter bytes 0x30–0x3f.
		for j < len(in) && in[j] >= 0x30 && in[j] <= 0x3f {
			j++
		}
		// Intermediate bytes 0x20–0x2f.
		for j < len(in) && in[j] >= 0x20 && in[j] <= 0x2f {
			j++
		}
		if j >= len(in) {
			break
		}
		final := in[j]
		paramStart := i + 2
		if privateGT {
			paramStart++
		}
		params := in[paramStart:j]

		switch final {
		case 'c':
			if privateGT {
				*out = append(*out, "\x1b[>0;0;0c"...)
			} else {
				*out = append(*out, "\x1b[?1;2c"...)
			}
		case 'n':
			if bytes.Equal(params, []byte("6")) {
				*out = append(*out, "\x1b[1;1R"...)
			}
		case 'q':
			if privateGT {
				*out = append(*out, "\x1bP>|claude-p\x1b\\"...)
			}
		case 't':
			if bytes.Equal(params, []byte("18")) {
				*out = append(*out, "\x1b[8;40;120t"...)
			}
		}
		i = j
	}
}
