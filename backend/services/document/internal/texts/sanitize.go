package texts

import "github.com/microcosm-cc/bluemonday"

// policy is what the editor can produce and nothing else: paragraphs, headings,
// the inline marks, lists, quotes, code and rules, and links to http, https or
// mailto. No attribute survives except a link's address, so no handler, style,
// class or script can ride in on a document that other people open.
//
// This runs on every save. Readers sanitize again on render, but a stored
// payload is still a payload for anything that reads the database or the API.
var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "b", "em", "i", "u", "s", "strike", "code", "pre",
		"blockquote", "ul", "ol", "li", "hr")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(false)
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// Sanitize returns html reduced to what the editor can produce.
func Sanitize(html string) string { return policy.Sanitize(html) }
