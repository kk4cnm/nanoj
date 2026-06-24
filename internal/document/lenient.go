package document

import (
	"bytes"
	"io"
)

// Lenient (JSONC) input: nanoj can optionally tolerate // and /* */ comments
// and trailing commas when reading a file. It does so by stripping those away
// and then handing clean JSON to the strict Parse, so key order, number
// precision, and escaping are all preserved by the same battle-tested path.
//
// Because the document model has no place to store comments, they are dropped —
// saving always writes strict JSON. ParseLenient reports whether any comment
// was removed so the caller can warn the user. Trailing-comma removal is not
// reported: it is not lossy (the result is the same data).

// ParseLenient reads JSONC from r: it strips comments and trailing commas, then
// parses the result strictly. hadComments is true if any comment was removed.
func ParseLenient(r io.Reader) (node *Node, hadComments bool, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	cleaned, had := StripJSONC(data)
	node, err = Parse(bytes.NewReader(cleaned))
	return node, had, err
}

// StripJSONC removes // line comments, /* */ block comments, and trailing
// commas from src, returning clean JSON plus whether any comment was present.
// It is string-aware: comment markers and commas inside string literals are
// left untouched.
func StripJSONC(src []byte) (cleaned []byte, hadComments bool) {
	noComments, had := stripComments(src)
	return stripTrailingCommas(noComments), had
}

// stripComments removes // and /* */ comments outside of string literals.
func stripComments(src []byte) (out []byte, hadComment bool) {
	out = make([]byte, 0, len(src))
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '"':
			i = copyString(src, i, &out)
		case c == '/' && i+1 < n && src[i+1] == '/':
			hadComment = true
			i += 2
			for i < n && src[i] != '\n' {
				i++
			}
			// Leave the newline to be copied normally (keeps line breaks).
		case c == '/' && i+1 < n && src[i+1] == '*':
			hadComment = true
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2 // consume the closing */
			} else {
				i = n // unterminated block comment: run to EOF
			}
		default:
			out = append(out, c)
			i++
		}
	}
	return out, hadComment
}

// stripTrailingCommas removes a comma that is followed (after optional
// whitespace) by a closing } or ], outside of string literals.
func stripTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '"':
			i = copyString(src, i, &out)
		case c == ',':
			j := i + 1
			for j < n && isJSONSpace(src[j]) {
				j++
			}
			if j < n && (src[j] == '}' || src[j] == ']') {
				i++ // drop the trailing comma; whitespace copies normally
				continue
			}
			out = append(out, c)
			i++
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}

// copyString copies a JSON string literal beginning at the opening quote
// src[start], honoring backslash escapes, and returns the index just past the
// closing quote.
func copyString(src []byte, start int, out *[]byte) int {
	n := len(src)
	*out = append(*out, src[start]) // opening quote
	i := start + 1
	for i < n {
		ch := src[i]
		*out = append(*out, ch)
		i++
		if ch == '\\' && i < n {
			*out = append(*out, src[i]) // escaped char is literal
			i++
			continue
		}
		if ch == '"' {
			break
		}
	}
	return i
}

func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
