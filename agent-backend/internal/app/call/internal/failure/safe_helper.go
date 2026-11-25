// Package failure turns what NICE or a service answered into a Failure the model can read.
package failure

import (
	"regexp"
	"strings"
)

// safeText keeps a code or description from NICE or a service only when it is short printable text without
// anything shaped like a token.
var safeText = regexp.MustCompile(`^[\x20-\x7E]{1,200}$`)

func safe(s string) string {
	s = strings.TrimSpace(s)
	if !safeText.MatchString(s) || strings.Contains(s, "eyJ") {
		return ""
	}
	return s
}

// Plain makes text from NICE or the model fit a message the user reads: printable ASCII only, at most 120
// characters, and nothing shaped like a token.
func Plain(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "eyJ") {
		return ""
	}
	b := []byte(s)
	for i, c := range b {
		if c < 0x20 || c > 0x7E {
			b[i] = '?'
		}
	}
	if len(b) > 120 {
		b = append(b[:117], "..."...)
	}
	return string(b)
}
