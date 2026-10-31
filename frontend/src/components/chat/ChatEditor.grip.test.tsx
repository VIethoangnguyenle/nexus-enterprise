import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { CH, MEMBERS } from '../../test/chat-fixtures'
import { pointer } from '../../test/gesture-events'
import { useDraftsStore } from '../../stores/drafts.store'
import { ChatEditor } from './ChatEditor'

let phone = true
beforeEach(() => {
  phone = true
  localStorage.clear()
  useDraftsStore.getState().reset()
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true, writable: true })
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
})
afterEach(() => {
  vi.unstubAllGlobals()
  resetClient()
})

function open() {
  const onSend = vi.fn()
  renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} />)
  const root = () => document.querySelector('[data-composer-root]') as HTMLElement
  return { onSend, root, box: screen.getByRole('textbox', { name: 'Tin nhắn cho Đối soát' }) }
}

/** React reads a zero `timeStamp` as missing, so drag time starts at 1s. */
const T0 = 1000
function drag(el: Element, from: number, to: number, ms: number) {
  act(() => {
    pointer(el, 'pointerdown', { x: 200, y: from, t: T0 })
    pointer(el, 'pointermove', { x: 200, y: (from + to) / 2, t: T0 + ms / 2 })
    pointer(el, 'pointermove', { x: 200, y: to, t: T0 + ms })
    pointer(el, 'pointerup', { x: 200, y: to, t: T0 + ms + 10 })
  })
}

describe('ChatEditor on a phone: the grip', () => {
  it('opens full screen with the formatting bar from a tap, and closes the same way', async () => {
    const user = userEvent.setup()
    const { root } = open()
    expect(root()).not.toHaveAttribute('data-expanded')
    expect(screen.queryByRole('toolbar', { name: 'Định dạng' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Mở rộng ô soạn' }))
    expect(root()).toHaveAttribute('data-expanded', 'true')
    expect(root().className).toContain('fixed')
    expect(screen.getByRole('toolbar', { name: 'Định dạng' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Thu gọn ô soạn' }))
    expect(root()).not.toHaveAttribute('data-expanded')
  })

  it('opens full screen when dragged up past 25% of the screen', () => {
    const { root } = open()
    const grip = screen.getByRole('button', { name: 'Mở rộng ô soạn' })
    drag(grip, 700, 700 - 800 * 0.25 - 10, 600)
    expect(root()).toHaveAttribute('data-expanded', 'true')
  })

  it('stays as it is after a short, slow drag up, and the drag is not taken for a tap', () => {
    const { root } = open()
    const grip = screen.getByRole('button', { name: 'Mở rộng ô soạn' })
    drag(grip, 700, 600, 800)
    act(() => grip.click())
    expect(root()).not.toHaveAttribute('data-expanded')
  })

  it('opens on a fast flick up, whatever the distance', () => {
    const { root } = open()
    drag(screen.getByRole('button', { name: 'Mở rộng ô soạn' }), 700, 640, 40)
    expect(root()).toHaveAttribute('data-expanded', 'true')
  })

  it('a drag down while open brings it back', async () => {
    const user = userEvent.setup()
    const { root } = open()
    await user.click(screen.getByRole('button', { name: 'Mở rộng ô soạn' }))
    drag(screen.getByRole('button', { name: 'Thu gọn ô soạn' }), 60, 60 + 800 * 0.25 + 10, 600)
    expect(root()).not.toHaveAttribute('data-expanded')
  })

  it('a drag down while collapsed does nothing', () => {
    const { root } = open()
    drag(screen.getByRole('button', { name: 'Mở rộng ô soạn' }), 500, 780, 100)
    expect(root()).not.toHaveAttribute('data-expanded')
  })

  it('folds back once the message is sent', async () => {
    const user = userEvent.setup()
    const { root, box, onSend } = open()
    await user.click(screen.getByRole('button', { name: 'Mở rộng ô soạn' }))
    await user.click(box)
    await user.keyboard('bản dài{Enter}')
    expect(onSend).toHaveBeenCalled()
    expect(root()).not.toHaveAttribute('data-expanded')
  })

  it('has no grip on a wide screen', () => {
    phone = false
    open()
    expect(screen.queryByRole('button', { name: 'Mở rộng ô soạn' })).toBeNull()
  })
})
