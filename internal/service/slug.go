package service

import (
	"strings"
	"unicode"
)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) && r < 128:
			b.WriteRune(r)
		case unicode.IsDigit(r) && r < 128:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}

	return b.String()
}
