package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHTML_KeepsTheToolbarsFormatting(t *testing.T) {
	in := `<p><strong>đậm</strong> <em>nghiêng</em> <u>gạch chân</u> <s>gạch</s> <code>x := 1</code></p>` +
		`<ul><li>một</li></ul><ol><li>hai</li></ol><blockquote>trích</blockquote><pre><code>khối</code></pre>` +
		`<p><a href="https://nexus.example/doc">link</a> <span data-mention="user-42">@Lan</span><br></p>`
	out := sanitizeHTML(in)
	for _, keep := range []string{
		"<strong>đậm</strong>", "<em>nghiêng</em>", "<u>gạch chân</u>", "<s>gạch</s>", "<code>x := 1</code>",
		"<ul><li>một</li></ul>", "<ol><li>hai</li></ol>", "<blockquote>trích</blockquote>", "<pre><code>khối</code></pre>",
		`<span data-mention="user-42">@Lan</span>`, "<br>",
	} {
		assert.Contains(t, out, keep)
	}
	assert.Contains(t, out, `href="https://nexus.example/doc"`)
	assert.Contains(t, out, "noopener")
	assert.Contains(t, out, "noreferrer")
	assert.Contains(t, out, `target="_blank"`)
}

// HTML sent straight to the API is filtered on the server, whatever the
// client does: nothing that runs script or restyles the page survives.
func TestSanitizeHTML_DropsWhatCouldRunOrRestyle(t *testing.T) {
	for name, in := range map[string]string{
		"script tag":      `<p>hi</p><script>alert(1)</script>`,
		"event handler":   `<p><img src=x onerror="alert(1)"></p>`,
		"javascript link": `<p><a href="javascript:alert(1)">x</a></p>`,
		"data link":       `<p><a href="data:text/html;base64,PHNjcmlwdD4=">x</a></p>`,
		"style attribute": `<p style="position:fixed;inset:0">cover</p>`,
		"style tag":       `<style>body{display:none}</style><p>x</p>`,
		"iframe":          `<iframe src="https://evil.example"></iframe>`,
		"mention extras":  `<span data-mention="u1" onclick="x()" class="big">@a</span>`,
		"svg":             `<svg onload="alert(1)"><circle/></svg>`,
	} {
		out := strings.ToLower(sanitizeHTML(in))
		for _, bad := range []string{"<script", "onerror", "onclick", "onload", "javascript:", "data:text", "style", "<iframe", "<img", "<svg", "class="} {
			assert.NotContains(t, out, bad, name)
		}
	}
	assert.Contains(t, sanitizeHTML(`<p><a href="mailto:a@b.vn">thư</a></p>`), `href="mailto:a@b.vn"`)
}

func TestPlainText_IsTheReadableTextWithoutTags(t *testing.T) {
	assert.Equal(t, "Chào Lan\nxem tài liệu & phản hồi",
		plainText("html", `<p>Chào <strong>Lan</strong></p><p>xem <a href="https://x">tài liệu</a> &amp; phản hồi</p>`))
	assert.Equal(t, "một\nhai", plainText("html", "<ul><li>một</li><li>hai</li></ul>"))
	assert.Equal(t, "**không phải html**", plainText("markdown", "**không phải html**"))
}

func TestPrepareContent_LimitsLength(t *testing.T) {
	_, _, _, err := prepareContent("html", "<p>"+strings.Repeat("a", maxContentChars+1)+"</p>")
	require.ErrorIs(t, err, ErrInvalidInput)
	html, text, format, err := prepareContent("html", `<p>ok<script>x</script></p>`)
	require.NoError(t, err)
	assert.Equal(t, "<p>ok</p>", html)
	assert.Equal(t, "ok", text)
	assert.Equal(t, "html", format)
}
