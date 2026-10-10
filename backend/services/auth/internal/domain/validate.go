package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits on the text a person can put on their profile or a workspace. They
// count characters people see (runes), not bytes: a Vietnamese name takes up to
// three bytes a letter.
const (
	maxDisplayNameRunes   = 80
	maxWorkspaceNameRunes = 80
	maxProfileFieldRunes  = 120
)

// cleanText trims s and checks it is something a person could have typed as a
// name or title: within maxRunes, with no control characters and no invisible
// formatting characters (zero-width spaces, right-to-left overrides), which
// make one string read as another. An empty result is allowed only when
// required is false.
func cleanText(s string, maxRunes int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return "", ErrInvalidInput
		}
		return "", nil
	}
	if utf8.RuneCountInString(s) > maxRunes || !utf8.ValidString(s) {
		return "", ErrInvalidInput
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrInvalidInput
		}
	}
	return s, nil
}
