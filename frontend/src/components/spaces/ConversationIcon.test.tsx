import { cleanup, fireEvent, render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { avatarVariant } from '../primitives/Avatar'
import { ConversationIcon } from './ConversationIcon'

const PIC = '/api/channels/avatars/0123456789abcdef0123456789abcdef'
const space = (avatarUrl?: string) => ({ kind: 'space' as const, title: 'Đối soát giao dịch', hueKey: 'c1', avatarUrl })

describe('avatarVariant for a group picture', () => {
  it('picks the 64 size up to 32 px and the 192 size above', () => {
    expect(avatarVariant(PIC, 24)).toBe(`${PIC}/64`)
    expect(avatarVariant(PIC, 36)).toBe(`${PIC}/192`)
  })
})

describe('ConversationIcon of a group', () => {
  it('draws the picture, sized for where it is drawn', () => {
    const { container } = render(<ConversationIcon conversation={space(PIC)} size={24} />)
    expect(container.querySelector('img')).toHaveAttribute('src', `${PIC}/64`)
    cleanup()
    const big = render(<ConversationIcon conversation={space(PIC)} size={44} />)
    expect(big.container.querySelector('img')).toHaveAttribute('src', `${PIC}/192`)
  })

  it('keeps the rounded square, and the picture says nothing the row does not already say', () => {
    const { container } = render(<ConversationIcon conversation={space(PIC)} size={40} />)
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true')
    expect(container.querySelector('img')).toHaveAttribute('alt', '')
    expect(container.firstElementChild!.className).toContain('rounded-surface')
  })

  it('shows the first letter when the group has no picture', () => {
    const { container } = render(<ConversationIcon conversation={space(undefined)} size={32} />)
    expect(container.querySelector('img')).toBeNull()
    expect(container).toHaveTextContent('Đ')
    cleanup()
    const empty = render(<ConversationIcon conversation={space('')} size={32} />)
    expect(empty.container.querySelector('img')).toBeNull()
  })

  it('falls back to the letter when the picture does not load, and tries a new address again', () => {
    const { container, rerender } = render(<ConversationIcon conversation={space(PIC)} size={32} />)
    fireEvent.error(container.querySelector('img')!)
    expect(container.querySelector('img')).toBeNull()
    expect(container).toHaveTextContent('Đ')
    const next = '/api/channels/avatars/fedcba9876543210fedcba9876543210'
    rerender(<ConversationIcon conversation={space(next)} size={32} />)
    expect(container.querySelector('img')).toHaveAttribute('src', `${next}/64`)
  })

  it('never gives a direct message the group picture', () => {
    const { container } = render(
      <ConversationIcon conversation={{ kind: 'dm', title: 'Phạm Hải Yến', hueKey: 'u1', avatarUrl: PIC }} size={32} />,
    )
    expect(container.querySelector('img')).toBeNull()
  })
})
