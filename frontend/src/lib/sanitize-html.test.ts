import { describe, expect, it } from 'vitest'
import { sanitizeHtml } from './sanitize-html'

describe('sanitizeHtml', () => {
  it('keeps what the editor writes', () => {
    const html = '<h2>Quy trình</h2><p>Một <strong>đậm</strong> và <em>nghiêng</em></p><ul><li>a</li></ul>'
    expect(sanitizeHtml(html)).toBe(html)
  })

  it('drops script, event handlers, frames, forms and styles', () => {
    const out = sanitizeHtml(
      '<p onclick="x()" style="color:red">hi</p><script>alert(1)</script><iframe src="https://e.test"></iframe><form><input></form><img src="x" onerror="y()">',
    )
    expect(out).not.toMatch(/script|onclick|onerror|iframe|<form|<input|style=/i)
    expect(out).toContain('hi')
  })

  it('refuses javascript: links and makes real links safe to follow', () => {
    expect(sanitizeHtml('<a href="javascript:alert(1)">x</a>')).not.toMatch(/javascript:/i)
    const link = sanitizeHtml('<a href="https://example.test">x</a>')
    expect(link).toContain('target="_blank"')
    expect(link).toContain('rel="noopener noreferrer"')
  })
})
