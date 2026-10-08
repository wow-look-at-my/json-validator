package cmd

import (
	"bytes"
	"encoding/json"
	"io"
)

// writeJSON writes v as indented JSON. With trailingCommas, every object and
// array that spans lines carries a comma after its last member.
func writeJSON(w io.Writer, v any, trailingCommas bool) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	out := buf.Bytes()
	if trailingCommas {
		out = addTrailingCommas(out)
	}
	_, err := w.Write(out)
	return err
}

// addTrailingCommas puts a comma after the last member of each object and
// array whose closing bracket sits on a line of its own. An empty container,
// a container on one line and a comma already present are left as they are.
// Brackets inside strings are text.
func addTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src)+len(src)/16)
	inString, escaped := false, false
	for _, b := range src {
		if inString {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			out = append(out, b)
			continue
		}
		switch b {
		case '"':
			inString = true
		case '}', ']':
			out = commaAfterLastMember(out)
		}
		out = append(out, b)
	}
	return out
}

// commaAfterLastMember inserts a comma after the last value in out when a line
// break separates that value from the closing bracket about to be written.
func commaAfterLastMember(out []byte) []byte {
	last := len(bytes.TrimRight(out, " \t\r\n")) - 1
	if last < 0 {
		return out
	}
	switch out[last] {
	case '{', '[', ',':
		return out
	}
	gap := out[last+1:]
	if !bytes.ContainsRune(gap, '\n') {
		return out
	}
	tail := append([]byte{','}, gap...)
	return append(out[:last+1], tail...)
}
