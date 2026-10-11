# chat-rich-text

## Purpose
Formatted messages (bold, italic, underline, strike, lists, quote, code, links, mentions) as the composer's toolbar
produces them, filtered on the server so that no client can store markup that runs or restyles anything.

## Requirements

### Requirement: Formatted content is filtered on the server
`content_format` SHALL be `plain`, `markdown` or `html` (400 otherwise). A body that contains markup is treated as
`html` whatever its label — the client renders markup wherever it finds it — while a bare `<` (as in `a < b`) is not
markup. A message or an edit with `html` content SHALL be filtered to the toolbar's allowlist before it is stored:
`p, br, strong, b, em, i, u, s, del, code, pre, blockquote, ul, ol, li`, `a` with an `http`, `https` or `mailto`
`href` (opened in a new tab with `rel` noopener noreferrer), and `span` with only a `data-mention` user id. Every
other element, attribute, style and URL scheme SHALL be dropped. A body over 20 000 characters is refused (400), and so is one with no text left once filtered. The client
filters again when it renders.

#### Scenario: Script posted straight to the API
- **WHEN** a member posts `<p>report<img onerror=…><script>…</script></p>` as html
- **THEN** the stored and returned content is `<p>report</p>`

### Requirement: Search, previews and notifications read the text, not the markup
Each message SHALL keep its readable text — markup removed, one line per paragraph or list item, entities decoded —
beside its body, and full-text search SHALL match that text: a search for a tag name does not match formatting.
Plain and markdown bodies are their own text.

### Requirement: The composer writes only what the server keeps
The composer SHALL offer bold, italic, underline, strike, bullet and numbered lists, link, code, code block and quote
(a formatting bar the reader opens with the "A" button and that remembers whether it was open), with Ctrl/Cmd+B, I, U,
Shift+X and K, and the quick syntax `*bold*`, `_italic_`, `~strike~`, `` `code` ``, `- ` and `1. `. A link SHALL be
accepted only when it starts with `https://`, `http://` or `mailto:`, and opens in a new tab with `rel` noopener
noreferrer. A person mentioned SHALL be written as the `span` carrying their user id, and their ids SHALL be sent with
the message; a typed `@word` in an HTML body SHALL NOT be treated as a mention when it is shown. Enter sends and
Shift+Enter breaks the line, except that inside a list or a code block Enter makes the next line and Ctrl/Cmd+Enter sends.

