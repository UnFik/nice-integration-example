package failure

import "strings"

const maxText = 200

// safe keeps a code or description from NICE or nadi printable on one line: no control characters, bounded length.
func safe(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxText {
		s = string(r[:maxText]) + "..."
	}
	return s
}
