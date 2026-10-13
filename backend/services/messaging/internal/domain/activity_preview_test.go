package domain

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestPreviewText(t *testing.T) {
	long := strings.Repeat("báo cáo ", 40)
	cases := []struct {
		name, format, in, want string
	}{
		{"short text is kept", "plain", "xin chào", "xin chào"},
		{"html becomes plain text", "html", "<p>Họp <strong>lúc 9h</strong></p><p>phòng A</p>", "Họp lúc 9h phòng A"},
		{"lines are joined", "markdown", "dòng một\n\ndòng hai", "dòng một dòng hai"},
		{"blank stays blank", "plain", "  \n ", ""},
		{"exactly 140 is kept whole", "plain", strings.Repeat("a", 140), strings.Repeat("a", 140)},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, previewText(c.format, c.in), c.name)
	}

	got := previewText("plain", long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), previewMaxRunes)
	assert.True(t, strings.HasSuffix(got, "…"))
	words := strings.Fields(strings.TrimSuffix(got, "…"))
	assert.Contains(t, []string{"báo", "cáo"}, words[len(words)-1], "cut between words, got %q", got)

	oneWord := previewText("plain", strings.Repeat("đ", 300))
	assert.Equal(t, previewMaxRunes, utf8.RuneCountInString(oneWord), "a single long word is cut by runes")
	assert.True(t, utf8.ValidString(oneWord))
}
