// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"unicode/utf8"
)

// printableWriter is how every command writes to stdout and stderr: names,
// values and messages that come from a device, a GOOSE or SV frame or a
// file reach the terminal as text, never as terminal controls. It writes a
// C0 control other than newline and tab, DEL and a C1 control as \u00XX,
// and a byte that is not part of valid UTF-8 as \xNN; everything else
// passes unchanged. JSON output stays valid and decodes to the same strings
// (encoding/json escapes C0 itself; \u007f and \u0080..\u009f are JSON
// escapes). get file writes the file's bytes to stdout unfiltered
// (rawWriter).
//
// Each Write is filtered on its own, so a rune split over two Writes is
// written escaped; fmt and text/tabwriter write whole runes.
type printableWriter struct {
	w io.Writer
}

func newPrintableWriter(w io.Writer) io.Writer {
	if w == nil {
		return nil
	}
	return &printableWriter{w: w}
}

// rawWriter returns the writer below a printableWriter, for output that is
// data rather than text.
func rawWriter(w io.Writer) io.Writer {
	if p, ok := w.(*printableWriter); ok {
		return p.w
	}
	return w
}

func (p *printableWriter) Write(b []byte) (int, error) {
	i := firstUnprintable(b)
	if i == len(b) {
		return p.w.Write(b)
	}
	out := make([]byte, 0, len(b)+16)
	out = append(out, b[:i]...)
	for i < len(b) {
		c := b[i]
		switch {
		case printableASCII(c):
			out = append(out, c)
			i++
		case c < utf8.RuneSelf:
			out = fmt.Appendf(out, `\u%04x`, c)
			i++
		default:
			r, size := utf8.DecodeRune(b[i:])
			switch {
			case r == utf8.RuneError && size == 1:
				out = fmt.Appendf(out, `\x%02x`, c)
			case isC1(r):
				out = fmt.Appendf(out, `\u%04x`, r)
			default:
				out = append(out, b[i:i+size]...)
			}
			i += size
		}
	}
	if _, err := p.w.Write(out); err != nil {
		return 0, err
	}
	return len(b), nil
}

// firstUnprintable returns the index of the first byte that Write escapes,
// or len(b).
func firstUnprintable(b []byte) int {
	for i := 0; i < len(b); {
		c := b[i]
		if c < utf8.RuneSelf {
			if !printableASCII(c) {
				return i
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if (r == utf8.RuneError && size == 1) || isC1(r) {
			return i
		}
		i += size
	}
	return len(b)
}

func printableASCII(c byte) bool {
	return c == '\n' || c == '\t' || (c >= 0x20 && c < 0x7f)
}

func isC1(r rune) bool { return r >= 0x80 && r <= 0x9f }
