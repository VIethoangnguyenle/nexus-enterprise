import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SelfMentionStyle, selfMentionRule } from './SelfMentionStyle'

describe('SelfMentionStyle', () => {
  it('washes only the reader\'s own mention', () => {
    expect(selfMentionRule('3f9c-aa')).toBe(
      '.message-html [data-mention="3f9c-aa"]{background:var(--color-accent-wash);padding:0 4px;border-radius:4px}',
    )
  })

  it('writes nothing for no user, or an id that could break out of the selector', () => {
    expect(selfMentionRule(undefined)).toBeNull()
    expect(selfMentionRule('')).toBeNull()
    expect(selfMentionRule('"] body{display:none} x["')).toBeNull()
    render(<SelfMentionStyle userId={'a"b'} />)
    expect(document.head.querySelector('style[data-self-mention]')).toBeNull()
  })

  it('puts the rule in the head, keeps the page text clean, and takes it away on unmount', () => {
    const { container, unmount } = render(<SelfMentionStyle userId="u-1" />)
    expect(document.head.querySelector('style[data-self-mention]')?.textContent).toContain('[data-mention="u-1"]')
    expect(container.textContent).toBe('')
    unmount()
    expect(document.head.querySelector('style[data-self-mention]')).toBeNull()
  })
})
