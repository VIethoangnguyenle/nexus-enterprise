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
})
