import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Bold from '@tiptap/extension-bold'
import Italic from '@tiptap/extension-italic'
import Strike from '@tiptap/extension-strike'
import { markInputRule, Node, type Editor } from '@tiptap/core'

/** Links the composer may write; the server keeps the same three schemes (docs/specs/chat-rich-text). */
export const ALLOWED_LINK = /^(?:https?:\/\/|mailto:)\S+$/i

export const isAllowedLink = (url: string): boolean => ALLOWED_LINK.test(url.trim())

/** `*x*`, `_x_` and `~x~` as the quick syntax of Google Chat: one mark character on each side. */
const around = (c: string) => new RegExp(`(?:^|\\s)(${c}(?!\\s+${c})((?:[^${c}]+))${c}(?!\\s+${c}))$`)

// StarterKit's own rules take `**x**` for bold, `*x*` for italic and `~~x~~` for strike;
// the toolbar's quick syntax is the single-character one, so these three replace them.
const QuickBold = Bold.extend({
  addInputRules() { return [markInputRule({ find: around('\\*'), type: this.type })] },
  addPasteRules() { return [] },
})
const QuickItalic = Italic.extend({
  addInputRules() { return [markInputRule({ find: around('_'), type: this.type })] },
  addPasteRules() { return [] },
})
const QuickStrike = Strike.extend({
  addInputRules() { return [markInputRule({ find: around('~'), type: this.type })] },
  addPasteRules() { return [] },
  addKeyboardShortcuts() {
    return { 'Mod-Shift-x': () => this.editor.commands.toggleStrike() }
  },
})

/**
 * A person mentioned: an inline node that is written as the one `span` the
 * server keeps, carrying the user's id and, as its text, "@" and their name.
 * The id is what notifies them; the text is only what is read.
 */
export const Mention = Node.create({
  name: 'mention',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: false,
  addAttributes() {
    return { id: { default: '' }, label: { default: '' } }
  },
  parseHTML() {
    return [{
      tag: 'span[data-mention]',
      getAttrs: (el) => ({ id: el.getAttribute('data-mention') ?? '', label: (el.textContent ?? '').replace(/^@/, '') }),
    }]
  },
  renderHTML({ node }) {
    return ['span', { 'data-mention': node.attrs.id as string }, `@${node.attrs.label as string}`]
  },
  renderText({ node }) {
    return `@${node.attrs.label as string}`
  },
})

/** The people mentioned in a document, by user id: each once, in the order they appear. */
export function mentionsOf(editor: Editor): string[] {
  const ids: string[] = []
  editor.state.doc.descendants((n) => {
    const id = n.type.name === 'mention' ? (n.attrs.id as string) : ''
    if (id && !ids.includes(id)) ids.push(id)
  })
  return ids
}

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

/**
 * What the composer and the in-place editor understand: the formatting of the
 * toolbar and nothing else. StarterKit carries underline, lists, code, code
 * block, quote and link; headings and rules are off (the server drops them), no
 * empty paragraph is kept after a list, and a link opens in a new tab without
 * access to this window.
 */
export function chatExtensions(placeholder: string) {
  return [
    StarterKit.configure({
      heading: false,
      horizontalRule: false,
      trailingNode: false,
      bold: false,
      italic: false,
      strike: false,
      codeBlock: { HTMLAttributes: { class: 'chat-code-block' } },
      link: {
        openOnClick: false,
        autolink: false,
        linkOnPaste: true,
        isAllowedUri: (url) => isAllowedLink(url),
        HTMLAttributes: { target: '_blank', rel: 'noopener noreferrer' },
      },
    }),
    QuickBold,
    QuickItalic,
    QuickStrike,
    Mention,
    Placeholder.configure({ placeholder }),
  ]
}
