import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { MessageContent } from './MessageContent'

describe('MessageContent HTML', () => {
  it('strips script, event handlers and javascript: links from stored HTML', () => {
    const evil =
      '<p>Xin chào<img src="x" onerror="alert(1)"><script>alert(2)</script>' +
      '<a href="javascript:alert(3)">bấm</a><iframe src="https://evil.example"></iframe></p>'
    const { container } = render(<MessageContent content={evil} contentFormat="html" />)
    const html = container.innerHTML
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('iframe')).toBeNull()
    expect(html).not.toMatch(/onerror/i)
    expect(html).not.toMatch(/javascript:/i)
    expect(container.textContent).toContain('Xin chào')
  })

  it('keeps ordinary formatting and safe links', () => {
    const ok = '<p><strong>Đậm</strong> và <a href="https://example.com/doc">tài liệu</a></p>'
    const { container } = render(<MessageContent content={ok} contentFormat="html" />)
    expect(container.querySelector('strong')?.textContent).toBe('Đậm')
    const a = container.querySelector('a')
    expect(a?.getAttribute('href')).toBe('https://example.com/doc')
    expect(a?.getAttribute('rel')).toContain('noopener')
  })

  it('keeps the formats of the toolbar: underline, strike, lists, quote, code and a block of code', () => {
    const html = '<p><u>gạch chân</u> <s>gạch ngang</s> <code>mã</code></p><ul><li><p>một</p></li></ul><ol><li><p>hai</p></li></ol>' +
      '<blockquote><p>trích</p></blockquote><pre><code>a = 1</code></pre>'
    const { container } = render(<MessageContent content={html} contentFormat="html" />)
    for (const tag of ['u', 's', 'code', 'ul', 'ol', 'blockquote', 'pre']) expect(container.querySelector(tag)).not.toBeNull()
  })

  it('opens every link in a new tab without access to this window, and drops other schemes', () => {
    const html = '<p><a href="https://a.vn">a</a> <a href="mailto:an@nova.vn">b</a> <a href="data:text/html,x">c</a> <a href="javascript:alert(1)">d</a></p>'
    const { container } = render(<MessageContent content={html} contentFormat="html" />)
    const links = Array.from(container.querySelectorAll('a'))
    for (const a of links.filter((x) => x.getAttribute('href'))) {
      expect(a.getAttribute('target')).toBe('_blank')
      expect(a.getAttribute('rel')).toBe('noopener noreferrer')
    }
    expect(container.innerHTML).not.toMatch(/javascript:|data:text/i)
  })

  it('drops style, class and event attributes a client could have stored', () => {
    const { container } = render(
      <MessageContent content={'<p style="color:red" class="x" onclick="alert(1)">chữ</p>'} contentFormat="html" />,
    )
    const p = container.querySelector('p')!
    expect(p.getAttribute('style')).toBeNull()
    expect(p.getAttribute('onclick')).toBeNull()
  })

  it('shows a mention node as a mention, and leaves a typed @word in html alone', () => {
    const { container } = render(
      <MessageContent content={'<p><span data-mention="u1">@Nguyễn Thu Lan</span> và @duc</p>'} contentFormat="html" />,
    )
    const mentions = container.querySelectorAll('[data-mention]')
    expect(mentions).toHaveLength(1)
    expect(mentions[0]!.textContent).toBe('@Nguyễn Thu Lan')
    expect(container.querySelectorAll('span.mention-highlight')).toHaveLength(0)
    expect(container.innerHTML).not.toMatch(/bg-accent-wash/)
  })

  it('never shows the mention id as text', () => {
    const { container } = render(
      <MessageContent content={'<p><span data-mention="11111111-aaaa-4bbb-8ccc-000000000003">@Lan</span></p>'} contentFormat="html" />,
    )
    expect(container.textContent).toBe('@Lan')
  })
})
