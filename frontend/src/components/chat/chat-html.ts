/**
 * Pure helpers over a message's links and saved html. They live apart from the
 * tiptap extensions so that code that only reads a message (the lifecycle hook,
 * the link dialog) does not pull the editor engine into its chunk.
 */

/** Links the composer may write; the server keeps the same three schemes (docs/specs/chat-rich-text). */
export const ALLOWED_LINK = /^(?:https?:\/\/|mailto:)\S+$/i

export const isAllowedLink = (url: string): boolean => ALLOWED_LINK.test(url.trim())

/** The people mentioned in saved html (an edit's text), by user id: each once, in the order they appear. */
export function mentionsFromHtml(html: string): string[] {
  if (!html) return []
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const ids: string[] = []
  doc.querySelectorAll('[data-mention]').forEach((el) => {
    const id = el.getAttribute('data-mention') ?? ''
    if (id && !ids.includes(id)) ids.push(id)
  })
  return ids
}
