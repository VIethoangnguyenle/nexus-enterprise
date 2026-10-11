import { afterEach, describe, expect, it } from 'vitest'
import { Editor } from '@tiptap/core'
import { chatExtensions, isAllowedLink, mentionsOf } from './chat-extensions'

let editor: Editor | null = null
function open(content = '') {
  editor = new Editor({ extensions: chatExtensions(''), content })
  return editor
}
afterEach(() => {
  editor?.destroy()
  editor = null
})

/** Types the way a keyboard does: each character goes through the input rules before it is inserted. */
function type(ed: Editor, text: string) {
  for (const ch of text) {
    const { from, to } = ed.state.selection
    const handled = ed.view.someProp('handleTextInput', (f) => f(ed.view, from, to, ch, () => ed.state.tr.insertText(ch, from, to)))
    if (!handled) ed.view.dispatch(ed.state.tr.insertText(ch, from, to))
  }
}

describe('chatExtensions: the formatting the toolbar offers', () => {
  it('knows every format of the toolbar, and no heading', () => {
    const names = open().extensionManager.extensions.map((e) => e.name)
    for (const n of ['bold', 'italic', 'underline', 'strike', 'bulletList', 'orderedList', 'link', 'code', 'codeBlock', 'blockquote']) {
      expect(names).toContain(n)
    }
    expect(names).not.toContain('heading')
  })

  it('quick syntax: *bold*, _italic_, ~strike~ and `code`', () => {
    const ed = open()
    type(ed, 'a *đậm* b _nghiêng_ c ~gạch~ d `mã` ')
    const html = ed.getHTML()
    expect(html).toContain('<strong>đậm</strong>')
    expect(html).toContain('<em>nghiêng</em>')
    expect(html).toContain('<s>gạch</s>')
    expect(html).toContain('<code>mã</code>')
  })

  it('quick syntax: "- " and "1. " start lists, and ``` starts a code block', () => {
    let ed = open()
    type(ed, '- một')
    expect(ed.getHTML()).toBe('<ul><li><p>một</p></li></ul>')
    ed.destroy()
    ed = open()
    type(ed, '1. một')
    expect(ed.getHTML()).toBe('<ol><li><p>một</p></li></ol>')
    ed.destroy()
    ed = open()
    type(ed, '``` ')
    expect(ed.getHTML()).toContain('<pre')
  })

  it('applies underline and strike from commands the shortcuts call', () => {
    const ed = open('<p>chữ</p>')
    ed.commands.selectAll()
    ed.commands.toggleUnderline()
    ed.commands.toggleStrike()
    expect(ed.getHTML()).toMatch(/<u>.*<\/u>/)
    expect(ed.getHTML()).toContain('<s>')
  })

  it('strikes through with Ctrl+Shift+X', () => {
    const ed = open('<p>chữ</p>')
    ed.commands.selectAll()
    ed.view.dom.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', code: 'KeyX', ctrlKey: true, shiftKey: true, bubbles: true }))
    expect(ed.getHTML()).toContain('<s>')
  })
})

describe('chatExtensions: links', () => {
  it('writes http, https and mailto links that open in a new tab without access to this window', () => {
    const ed = open('<p>trang</p>')
    ed.commands.selectAll()
    expect(ed.commands.setLink({ href: 'https://nexus.zaneng.xyz' })).toBe(true)
    const html = ed.getHTML()
    expect(html).toContain('href="https://nexus.zaneng.xyz"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
  })

  it.each(['javascript:alert(1)', 'data:text/html,x', 'ftp://x.vn', 'tel:123', '//evil.vn', 'nexus.vn'])('refuses %s', (href) => {
    const ed = open('<p>trang</p>')
    ed.commands.selectAll()
    ed.commands.setLink({ href })
    expect(ed.getHTML()).not.toContain('<a')
  })

  it('accepts only http(s) and mailto by its own check', () => {
    expect(isAllowedLink('https://a.vn/x?y=1')).toBe(true)
    expect(isAllowedLink('http://a.vn')).toBe(true)
    expect(isAllowedLink('mailto:an@nova.vn')).toBe(true)
    expect(isAllowedLink('javascript:alert(1)')).toBe(false)
    expect(isAllowedLink('https://')).toBe(false)
    expect(isAllowedLink('')).toBe(false)
  })

  it('does not turn a typed address into a link by itself', () => {
    const ed = open()
    type(ed, 'https://a.vn ')
    expect(ed.getHTML()).not.toContain('<a')
  })
})

describe('chatExtensions: mentions', () => {
  const html = '<p><span data-mention="u-lan">@Nguyễn Thu Lan</span> chào <span data-mention="u-duc">@Đức</span></p>'

  it('keeps a mention as a node that writes back the span the server allows', () => {
    const ed = open(html)
    expect(ed.getHTML()).toBe(html)
    expect(ed.getText()).toBe('@Nguyễn Thu Lan chào @Đức')
  })

  it('lists the people mentioned by id, once each, in order', () => {
    const ed = open(`${html}<p><span data-mention="u-lan">@Lan</span></p>`)
    expect(mentionsOf(ed)).toEqual(['u-lan', 'u-duc'])
  })

  it('inserts a mention from a command', () => {
    const ed = open('<p>chào</p>')
    ed.commands.focus('end')
    ed.commands.insertContent([{ type: 'text', text: ' ' }, { type: 'mention', attrs: { id: 'u-lan', label: 'Lan' } }, { type: 'text', text: ' ' }])
    expect(ed.getHTML()).toBe('<p>chào <span data-mention="u-lan">@Lan</span> </p>')
  })
})
