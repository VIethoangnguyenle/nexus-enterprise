import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { CH, MEMBERS } from '../../test/chat-fixtures'
import { useDraftsStore } from '../../stores/drafts.store'
import { ChatEditor } from './ChatEditor'

const keyboard = vi.hoisted(() => ({ open: false }))
vi.mock('../../hooks/useKeyboardOpen', () => ({ useKeyboardOpen: () => keyboard.open }))

beforeEach(() => {
  localStorage.clear()
  useDraftsStore.getState().reset()
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
})

/** The composer's outermost element: the one that carries its margins. */
function root() {
  const box = screen.getByRole('textbox', { name: 'Tin nhắn cho Đối soát' })
  return box.closest('[data-composer-root]') as HTMLElement
}

describe('ChatEditor: the on-screen keyboard', () => {
  it('keeps its margins and clears the home bar while the keyboard is down', () => {
    keyboard.open = false
    renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={vi.fn()} />)
    const cls = root().className.split(/\s+/)
    expect(root()).not.toHaveAttribute('data-docked')
    expect(cls).toEqual(expect.arrayContaining(['mx-4', 'max-md:mb-[max(0.75rem,env(safe-area-inset-bottom))]']))
  })

  it('docks flush to the keyboard: no margin, no rounding, a line on top', () => {
    keyboard.open = true
    renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={vi.fn()} />)
    const r = root()
    expect(r).toHaveAttribute('data-docked', 'true')
    expect(r.className).not.toMatch(/\bm[xb]-\d/)
    const bar = r.querySelector('[data-composer-box]') as HTMLElement
    expect(bar.className.split(/\s+/)).toEqual(expect.arrayContaining(['rounded-none', 'border-t', 'border-line', 'bg-base']))
    expect(bar.className.split(/\s+/)).not.toContain('rounded-lg')
  })
})
