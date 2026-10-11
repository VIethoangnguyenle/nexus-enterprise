// Package domain — formatted (HTML) message content.
package domain

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

// maxContentChars bounds a message body, formatting included.
const maxContentChars = 20000

// richText is the allowlist of the composer's formatting toolbar: text
// styles, lists, quote, code and links; a mention is a span carrying the
// mentioned user's id. Everything else — tags, attributes, styles, URL schemes
// other than http(s) and mailto — is dropped. The client filters again when it
// renders; this is the layer an API caller cannot skip.
var richText = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "del", "code", "pre", "blockquote", "ul", "ol", "li")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.RequireNoFollowOnLinks(false)
	p.AllowAttrs("data-mention").Matching(regexp.MustCompile(`^[\w.:-]{1,128}$`)).OnElements("span")
	return p
}()

// blockEnd marks where one line of text ends inside formatted content.
var blockEnd = regexp.MustCompile(`(?i)<br\s*/?>|</(p|li|blockquote|pre|ul|ol)>`)

// tags matches any tag, for the plain-text copy.
var tags = regexp.MustCompile(`<[^>]*>`)

// sanitizeHTML filters formatted content to the toolbar's allowlist.
func sanitizeHTML(in string) string {
	return richText.Sanitize(in)
}

// plainText is the readable text of a message, for search, notifications and
// previews: formatting removed, one line per paragraph or list item.
func plainText(format, content string) string {
	if format != "html" {
		return content
	}
	s := blockEnd.ReplaceAllString(content, "\n")
	s = html.UnescapeString(tags.ReplaceAllString(s, ""))
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// markup matches the start of a tag; a bare "<" in "a < b" does not.
var markup = regexp.MustCompile(`<[a-zA-Z!/]`)

// prepareContent returns the body to store, its plain-text copy and its
// format, refusing an unknown format and a body that is too long. A body with
// markup is filtered as HTML whatever its label: the label is the client's
// word, and the client renders markup wherever it finds it.
func prepareContent(format, content string) (stored, text, storedFormat string, err error) {
	switch format {
	case "plain", "markdown", "html":
	default:
		return "", "", "", fmt.Errorf("%w: content_format is plain, markdown or html", ErrInvalidInput)
	}
	if utf8.RuneCountInString(content) > maxContentChars {
		return "", "", "", fmt.Errorf("%w: a message is at most %d characters", ErrInvalidInput, maxContentChars)
	}
	if format != "html" && markup.MatchString(content) {
		format = "html"
	}
	if format == "html" {
		content = sanitizeHTML(content)
	}
	return content, plainText(format, content), format, nil
}
