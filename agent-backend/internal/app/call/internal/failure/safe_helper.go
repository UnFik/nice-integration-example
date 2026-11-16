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
