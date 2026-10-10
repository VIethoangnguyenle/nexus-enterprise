import DOMPurify from 'dompurify'

// HTML that people write and others read (chat messages, documents) is stored as
// written and is therefore untrusted: anything it can run, runs in every reader's
// session. DOMPurify drops scripts, event handlers and non-http(s) URLs; links
// are forced to open in a new tab without access to this window.
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

/** The one way stored rich text reaches the DOM. */
export function sanitizeHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    FORBID_TAGS: ['style', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: ['style'],
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|\/|#)/i,
  })
}
